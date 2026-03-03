package chunk

import (
	"fmt"
	"os"
	"strings"

	"github.com/perxibes/termdossier/internal/llm"
	"github.com/perxibes/termdossier/internal/preprocess"
)

const chunkSummarySystemPrompt = `You are a technical analyst. Summarize the following block of terminal commands and their output into a concise paragraph (3-5 sentences). Focus on:
- What tools were used and what they targeted
- Key findings or results
- Any errors or failures
- Credentials or sensitive data found
Do NOT invent results that are not shown in the data.`

// SummarizeChunk sends a single chunk to the LLM for summarization.
func SummarizeChunk(provider llm.Provider, c *Chunk) error {
	userPrompt := buildChunkPrompt(c)
	summary, err := provider.Generate(chunkSummarySystemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("summarize chunk %d: %w", c.Index, err)
	}
	c.Summary = strings.TrimSpace(summary)
	return nil
}

// SummarizeAll summarizes all chunks sequentially, updating each chunk's Summary field.
func SummarizeAll(provider llm.Provider, chunks []Chunk) error {
	for i := range chunks {
		fmt.Fprintf(os.Stderr, "  Summarizing chunk %d/%d (%d commands)...\n",
			i+1, len(chunks), len(chunks[i].Events))
		if err := SummarizeChunk(provider, &chunks[i]); err != nil {
			return err
		}
	}
	return nil
}

// BuildFinalPrompt constructs the final report prompt from chunk summaries.
func BuildFinalPrompt(chunks []Chunk, context, sessionID, startedAt string, totalCommands int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Context: %s\n\n", context))
	sb.WriteString(fmt.Sprintf("Session: %s | Started: %s | Total commands: %d | Chunks: %d\n\n",
		sessionID, startedAt, totalCommands, len(chunks)))

	for _, c := range chunks {
		sb.WriteString(fmt.Sprintf("Phase %d (%s - %s, %d commands):\n",
			c.Index+1,
			c.StartTime.Format("15:04:05"),
			c.EndTime.Format("15:04:05"),
			len(c.Events)))
		sb.WriteString(c.Summary)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

func buildChunkPrompt(c *Chunk) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chunk %d (%d commands, %s - %s):\n\n",
		c.Index+1, len(c.Events),
		c.StartTime.Format("15:04:05"),
		c.EndTime.Format("15:04:05")))

	for i, e := range c.Events {
		marker := " "
		if e.ExitCode != 0 {
			marker = "!"
		}
		sb.WriteString(fmt.Sprintf("#%d %s[%d | %dms | %s] %s\n",
			i+1, marker, e.ExitCode, e.DurationMS, e.CWD, e.Stdin))
		if e.ProcessedStdout != "" {
			sb.WriteString("--- output ---\n")
			sb.WriteString(e.ProcessedStdout)
			if !strings.HasSuffix(e.ProcessedStdout, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("--- end output ---\n")
		}
	}

	return sb.String()
}

// BuildCommandList formats events the same way as report.buildCommandList,
// used by both direct and chunked generation paths.
func BuildCommandList(events []preprocess.ProcessedEvent) string {
	var sb strings.Builder
	for i, e := range events {
		marker := " "
		if e.ExitCode != 0 {
			marker = "!"
		}
		repeatInfo := ""
		if e.RepeatCount > 1 {
			repeatInfo = fmt.Sprintf(" (repeated %dx)", e.RepeatCount)
		}
		sb.WriteString(fmt.Sprintf("#%d %s[%d | %dms | %s] %s%s\n",
			i+1, marker, e.ExitCode, e.DurationMS, e.CWD, e.Stdin, repeatInfo))

		if e.ProcessedStdout != "" {
			sb.WriteString("--- output ---\n")
			sb.WriteString(e.ProcessedStdout)
			if !strings.HasSuffix(e.ProcessedStdout, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("--- end output ---\n")
		}
	}
	return sb.String()
}
