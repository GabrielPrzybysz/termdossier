package preprocess

import (
	"fmt"
	"strings"
)

// DefaultMaxBytes is the default stdout limit per command after extraction.
const DefaultMaxBytes = 4096

// limits maps tool names to custom byte limits.
var limits = map[string]int{
	"nmap":     2048,
	"gobuster": 1536,
	"linpeas":  4096,
	"cat":      4096,
	"curl":     2048,
	"git":      3072,
	"find":     2048,
	"grep":     2048,
}

// LimitFor returns the byte limit for a given tool name,
// falling back to DefaultMaxBytes.
func LimitFor(tool string) int {
	if l, ok := limits[tool]; ok {
		return l
	}
	return DefaultMaxBytes
}

// Truncate trims s to at most maxBytes, cutting at the last newline
// boundary before the limit. Appends a truncation notice if trimmed.
func Truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := s[:maxBytes]
	if lastNL := strings.LastIndex(cut, "\n"); lastNL > 0 {
		cut = cut[:lastNL]
	}
	omitted := len(s) - len(cut)
	return cut + fmt.Sprintf("\n[...truncated %d bytes...]", omitted)
}
