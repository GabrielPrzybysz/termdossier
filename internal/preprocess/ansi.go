package preprocess

import (
	"regexp"
	"strings"
)

// ansiRe matches all common ANSI escape sequences:
// CSI sequences (colors, cursor movement), OSC sequences (titles, hyperlinks),
// character set switches, and simple mode-setting escapes.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*(?:\x07|\x1b\\)|\x1b[()][AB012]|\x1b[=>NH]`)

// StripANSI removes all ANSI escape sequences from s and collapses
// carriage-return-based progress bar overwrites to their final visible state.
func StripANSI(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = collapseCarriageReturns(s)
	return s
}

// collapseCarriageReturns handles \r-based overwriting used by progress bars.
// First strips trailing \r from each line (CRLF normalization), then for lines
// that still contain \r mid-line (progress bar overwrites), keeps only the text
// after the last \r — this is what the user actually sees on screen.
func collapseCarriageReturns(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Strip trailing \r (CRLF → LF normalization)
		line = strings.TrimRight(line, "\r")
		// Only collapse if \r still appears mid-line (progress bar overwrite)
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}
