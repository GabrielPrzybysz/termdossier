package filter

import (
	"regexp"
	"strings"

	"github.com/perxibes/termdossier/internal/store"
)

// trivialCommands are skipped — they produce no useful signal for a report.
var trivialCommands = map[string]bool{
	"cd": true, "clear": true, "cls": true, "history": true,
	"ls": true, "ll": true, "pwd": true, "exit": true, "logout": true,
}

// defaultRedactPatterns scrub sensitive values before they reach the LLM.
var defaultRedactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|secret|token|key)\s*=\s*\S+`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),
}

// Apply removes trivial commands and redacts sensitive data.
func Apply(events []store.Event) []store.Event {
	out := make([]store.Event, 0, len(events))
	for _, e := range events {
		cmd := strings.TrimSpace(e.Stdin)
		if cmd == "" {
			continue
		}
		// Check first word of command
		first := strings.Fields(cmd)[0]
		if trivialCommands[first] {
			continue
		}
		e.Stdin = redact(e.Stdin)
		e.Stdout = redact(e.Stdout)
		e.Stderr = redact(e.Stderr)
		out = append(out, e)
	}
	return out
}

func redact(s string) string {
	for _, re := range defaultRedactPatterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
