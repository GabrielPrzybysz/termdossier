package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ollamaBase = "http://localhost:11434"

// Ollama implements Provider against a local Ollama instance.
type Ollama struct {
	model  string
	client *http.Client
}

// NewOllama creates an Ollama provider and ensures the daemon and model are ready.
func NewOllama(model string) (*Ollama, error) {
	o := &Ollama{
		model: model,
		// No global timeout — streaming responses can take arbitrarily long.
		// Per-request timeouts are handled at the request level where needed.
		client: &http.Client{},
	}
	if err := o.EnsureRunning(); err != nil {
		return nil, err
	}
	if err := o.EnsureModel(model); err != nil {
		return nil, err
	}
	return o, nil
}

// EnsureRunning pings Ollama and starts it if unreachable.
func (o *Ollama) EnsureRunning() error {
	if o.ping() {
		return nil
	}

	fmt.Println("Ollama not running — starting it...")
	cmd := exec.Command("ollama", "serve")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ollama: %w", err)
	}

	// Wait up to 10 s for Ollama to become ready.
	for range 20 {
		time.Sleep(500 * time.Millisecond)
		if o.ping() {
			return nil
		}
	}
	return fmt.Errorf("ollama did not become ready after 10s")
}

// EnsureModel pulls the model if it is not already present.
func (o *Ollama) EnsureModel(model string) error {
	if o.hasModel(model) {
		return nil
	}

	fmt.Printf("Pulling model %q (this may take a while)...\n", model)
	body := map[string]string{"name": model}
	b, _ := json.Marshal(body)
	resp, err := o.client.Post(ollamaBase+"/api/pull", "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("pull model: %w", err)
	}
	defer resp.Body.Close()
	// Drain the stream so the pull completes.
	io.Copy(io.Discard, resp.Body) //nolint
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pull model: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Generate sends system and user prompts to Ollama via the chat API and returns the full response text.
func (o *Ollama) Generate(system, user string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	body := map[string]any{
		"model": o.model,
		"messages": []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		"stream":     true,
		"keep_alive": 0,
		"options": map[string]any{
			"temperature": 0.3,
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	// Start progress spinner before the request so model-loading time is visible.
	start := time.Now()
	var tokens atomic.Int64
	phase := "Waiting for LLM"
	var phaseMu sync.Mutex
	stop := make(chan struct{})
	go func() {
		spin := [4]string{"|", "/", "-", "\\"}
		i := 0
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				elapsed := time.Since(start).Truncate(time.Second)
				phaseMu.Lock()
				p := phase
				phaseMu.Unlock()
				tok := tokens.Load()
				if tok > 0 {
					fmt.Fprintf(os.Stderr, "\r  %s %s... %v elapsed, %d tokens",
						spin[i%4], p, elapsed, tok)
				} else {
					fmt.Fprintf(os.Stderr, "\r  %s %s... %v elapsed",
						spin[i%4], p, elapsed)
				}
				i++
			}
		}
	}()

	// stopSpinner is a helper to cleanly stop the spinner goroutine.
	stopSpinner := func() {
		select {
		case <-stop:
		default:
			close(stop)
		}
	}

	// Retry up to 2 times on transient errors (e.g. Ollama 500 during model reload).
	const maxRetries = 2
	var resp *http.Response
	for attempt := range maxRetries + 1 {
		resp, err = o.client.Post(ollamaBase+"/api/chat", "application/json", bytes.NewReader(b))
		if err != nil {
			stopSpinner()
			fmt.Fprintf(os.Stderr, "\n")
			return "", fmt.Errorf("ollama chat: %w", err)
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if attempt < maxRetries {
			fmt.Fprintf(os.Stderr, "\r  Ollama returned HTTP %d, retrying (%d/%d)...                    \n",
				resp.StatusCode, attempt+1, maxRetries)
			time.Sleep(3 * time.Second)
			continue
		}
		stopSpinner()
		fmt.Fprintf(os.Stderr, "\n")
		return "", fmt.Errorf("ollama chat HTTP %d: %s", resp.StatusCode, errBody)
	}
	defer resp.Body.Close()

	phaseMu.Lock()
	phase = "Generating"
	phaseMu.Unlock()

	// Read NDJSON stream, concatenating response fragments.
	// Chat API returns {"message":{"content":"..."},"done":false} per chunk.
	var full strings.Builder
	dec := json.NewDecoder(resp.Body)
	for dec.More() {
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := dec.Decode(&chunk); err != nil {
			stopSpinner()
			fmt.Fprintf(os.Stderr, "\n")
			return "", fmt.Errorf("decode stream chunk: %w", err)
		}
		full.WriteString(chunk.Message.Content)
		tokens.Add(1)
		if chunk.Done {
			break
		}
	}

	stopSpinner()
	elapsed := time.Since(start).Truncate(time.Second)
	// Clear the spinner line and print final summary.
	fmt.Fprintf(os.Stderr, "\r  Done: %d tokens in %v                              \n", tokens.Load(), elapsed)

	return full.String(), nil
}

func (o *Ollama) ping() bool {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(ollamaBase + "/api/tags")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (o *Ollama) hasModel(model string) bool {
	resp, err := o.client.Get(ollamaBase + "/api/tags")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	for _, m := range result.Models {
		if m.Name == model || m.Name == model+":latest" {
			return true
		}
	}
	return false
}
