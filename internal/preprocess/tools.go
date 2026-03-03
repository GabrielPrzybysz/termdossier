package preprocess

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Extractor processes raw stdout for a specific tool and returns
// a condensed version containing only the relevant findings.
type Extractor func(stdout string) string

// registry maps command base names to their extractors.
var registry = map[string]Extractor{
	"nmap":        extractNmap,
	"gobuster":    extractGobuster,
	"feroxbuster": extractGobuster,
	"linpeas":     extractLinpeas,
	"ps":          extractPs,
	"git":         extractGit,
	"ffuf":        extractFfuf,
	"nikto":       extractNikto,
}

// skipPrefixes are commands that wrap other commands.
var skipPrefixes = map[string]bool{
	"sudo": true, "time": true, "nohup": true, "strace": true,
	"nice": true, "timeout": true, "env": true,
}

// PassthroughTools are commands that display file content without transformation.
var PassthroughTools = map[string]bool{
	"cat": true, "less": true, "more": true,
	"head": true, "tail": true, "bat": true, "strings": true,
}

// contentSignatures maps regex patterns found in stdout to their extractors.
var contentSignatures = []struct {
	pattern   *regexp.Regexp
	extractor Extractor
	tool      string
}{
	{regexp.MustCompile(`(?m)(Nmap scan report for|^\d+/(tcp|udp)\s+open\s)`), extractNmap, "nmap"},
	{regexp.MustCompile(`(?i)(═{3,}|╔|╗|╚|╝).*(linpeas|Linux Privilege)`), extractLinpeas, "linpeas"},
}

// LookupExtractor returns the extractor for the base command name,
// or nil if none is registered.
func LookupExtractor(command string) Extractor {
	base := ExtractBaseTool(command)
	return registry[base]
}

// LookupByContent checks stdout content for known tool output signatures.
// Returns the matching extractor and detected tool name, or nil/"".
func LookupByContent(stdout string) (Extractor, string) {
	sample := stdout
	if len(sample) > 2048 {
		sample = sample[:2048]
	}
	for _, sig := range contentSignatures {
		if sig.pattern.MatchString(sample) {
			return sig.extractor, sig.tool
		}
	}
	return nil, ""
}

// ExtractBaseTool parses a command string to extract the base binary name,
// skipping wrapper prefixes like sudo, time, nohup and their flag arguments.
func ExtractBaseTool(command string) string {
	fields := strings.Fields(command)
	skipNext := false
	for _, f := range fields {
		if skipNext {
			skipNext = false
			continue
		}
		// Skip flags; short flags like -u may take an argument
		if strings.HasPrefix(f, "-") {
			if !strings.HasPrefix(f, "--") && !strings.Contains(f, "=") && len(f) == 2 {
				skipNext = true // next token is the flag's value
			}
			continue
		}
		name := filepath.Base(f)
		if skipPrefixes[name] {
			continue
		}
		return name
	}
	return ""
}

// --- nmap ---

var nmapOpenPort = regexp.MustCompile(`^\d+/(tcp|udp)\s+open\s`)
var nmapHeader = regexp.MustCompile(`^Nmap scan report for`)
var nmapHostUp = regexp.MustCompile(`^Host is up`)
var nmapOSLine = regexp.MustCompile(`^(OS details|Running|Aggressive OS guesses):`)

func extractNmap(stdout string) string {
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if nmapOpenPort.MatchString(trimmed) ||
			nmapHeader.MatchString(trimmed) ||
			nmapHostUp.MatchString(trimmed) ||
			nmapOSLine.MatchString(trimmed) ||
			strings.HasPrefix(trimmed, "|") ||
			strings.HasPrefix(trimmed, "Service Info:") {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return "[no open ports or services found]"
	}
	return strings.Join(out, "\n")
}

// --- gobuster / feroxbuster ---

var gobusterStatus = regexp.MustCompile(`Status:\s*(200|301|302|403)`)
var feroxStatus = regexp.MustCompile(`^\d{3}\s+`)

func extractGobuster(stdout string) string {
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if gobusterStatus.MatchString(trimmed) {
			out = append(out, line)
			continue
		}
		// feroxbuster format: "301      GET   10l   20w   300c http://..."
		if feroxStatus.MatchString(trimmed) {
			code := trimmed[:3]
			if code == "200" || code == "301" || code == "302" || code == "403" {
				out = append(out, line)
			}
		}
	}
	if len(out) == 0 {
		return "[no results with status 200/301/302/403]"
	}
	return strings.Join(out, "\n")
}

// --- linpeas ---

var linpeasSection = regexp.MustCompile(`[═╔╗╚╝]{3,}`)
var linpeasFindings = regexp.MustCompile(`(?i)(99%|95%|CVE-|SGID|SUID|writable|Vulnerable|root|password|credential|\.ssh|id_rsa|shadow|sudoers)`)

func extractLinpeas(stdout string) string {
	lines := strings.Split(stdout, "\n")
	type section struct {
		header  string
		content []string
	}

	var sections []section
	var current *section

	for _, line := range lines {
		if linpeasSection.MatchString(line) {
			s := section{header: strings.TrimSpace(line)}
			sections = append(sections, s)
			current = &sections[len(sections)-1]
			continue
		}
		if current != nil {
			current.content = append(current.content, line)
		}
	}

	var out []string
	for _, sec := range sections {
		var findings []string
		for _, line := range sec.content {
			if linpeasFindings.MatchString(line) {
				findings = append(findings, strings.TrimSpace(line))
			}
		}
		if len(findings) > 0 {
			out = append(out, sec.header)
			out = append(out, findings...)
			out = append(out, "")
		}
	}

	if len(out) == 0 {
		return "[no significant findings detected]"
	}
	return strings.Join(out, "\n")
}

// --- ps aux ---

func extractPs(stdout string) string {
	lines := strings.Split(stdout, "\n")
	if len(lines) == 0 {
		return ""
	}

	var out []string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Always keep the header line
		if i == 0 {
			out = append(out, line)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		cmd := fields[len(fields)-1]
		// Only skip kernel threads like [kworker/0:0], [kthreadd]
		if strings.HasPrefix(cmd, "[") && strings.HasSuffix(cmd, "]") {
			continue
		}
		out = append(out, line)
	}

	if len(out) <= 1 { // only header
		return "[no user-space processes]"
	}
	return strings.Join(out, "\n")
}

// --- git ---

var gitDiffFile = regexp.MustCompile(`^(diff --git|---|\+\+\+|@@)`)

func extractGit(stdout string) string {
	// For "git log" and "git status", keep as-is (already compact)
	// For "git diff", extract only file names and hunk headers
	if !strings.Contains(stdout, "diff --git") {
		return stdout
	}

	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		if gitDiffFile.MatchString(line) {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return "[no diff output]"
	}
	return strings.Join(out, "\n")
}

// --- ffuf ---

var ffufResult = regexp.MustCompile(`Status:\s*(200|301|302|403)`)

func extractFfuf(stdout string) string {
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		if ffufResult.MatchString(line) {
			out = append(out, strings.TrimSpace(line))
		}
	}
	if len(out) == 0 {
		return "[no results with status 200/301/302/403]"
	}
	return strings.Join(out, "\n")
}

// --- nikto ---

func extractNikto(stdout string) string {
	var out []string
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "+") {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return "[no findings]"
	}
	return strings.Join(out, "\n")
}
