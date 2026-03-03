package preprocess

import (
	"strings"
	"testing"
)

func TestTruncate_NoTruncation(t *testing.T) {
	input := "short text"
	got := Truncate(input, 100)
	if got != input {
		t.Errorf("Truncate short: got %q, want %q", got, input)
	}
}

func TestTruncate_ExactLimit(t *testing.T) {
	input := "12345"
	got := Truncate(input, 5)
	if got != input {
		t.Errorf("Truncate exact: got %q, want %q", got, input)
	}
}

func TestTruncate_CutsAtNewline(t *testing.T) {
	input := "line1\nline2\nline3\nline4"
	got := Truncate(input, 12)
	// 12 bytes gets "line1\nline2\n" — cut at last \n before limit
	if !strings.HasPrefix(got, "line1\nline2") {
		t.Errorf("Truncate newline: got %q, expected prefix 'line1\\nline2'", got)
	}
	if !strings.Contains(got, "[...truncated") {
		t.Error("Truncate should contain truncation notice")
	}
}

func TestTruncate_Notice(t *testing.T) {
	input := strings.Repeat("x", 100)
	got := Truncate(input, 50)
	if !strings.Contains(got, "[...truncated") {
		t.Error("Truncate should contain truncation marker")
	}
	if !strings.Contains(got, "bytes...]") {
		t.Error("Truncate notice should mention bytes")
	}
}

func TestLimitFor_Known(t *testing.T) {
	if got := LimitFor("nmap"); got != 2048 {
		t.Errorf("LimitFor(nmap) = %d, want 2048", got)
	}
}

func TestLimitFor_Unknown(t *testing.T) {
	if got := LimitFor("unknown_tool"); got != DefaultMaxBytes {
		t.Errorf("LimitFor(unknown) = %d, want %d", got, DefaultMaxBytes)
	}
}
