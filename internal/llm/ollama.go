package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
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
		model:  model,
		client: &http.Client{Timeout: 5 * time.Minute},
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

// Generate sends prompt to Ollama and returns the response text.
func (o *Ollama) Generate(prompt string) (string, error) {
	body := map[string]any{
		"model":  o.model,
		"prompt": prompt,
		"stream": false,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	resp, err := o.client.Post(ollamaBase+"/api/generate", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama generate HTTP %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Response, nil
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
