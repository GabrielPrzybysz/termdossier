package detect

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/perxibes/termdossier/internal/store"
)

// signal represents a weighted detection signal.
type signal struct {
	Type   SessionType
	Weight float64
	Reason string
}

var pentestTools = map[string]float64{
	"nmap": 0.3, "gobuster": 0.3, "feroxbuster": 0.3,
	"sqlmap": 0.4, "hydra": 0.3, "john": 0.3, "hashcat": 0.3,
	"msfconsole": 0.5, "msfvenom": 0.4,
	"nikto": 0.3, "dirb": 0.3, "wfuzz": 0.3,
	"ffuf": 0.3, "linpeas": 0.4, "winpeas": 0.4, "pspy": 0.3,
	"chisel": 0.3, "ligolo": 0.3, "crackmapexec": 0.4,
	"enum4linux": 0.3, "smbclient": 0.2, "evil-winrm": 0.4,
	"bloodhound": 0.4, "responder": 0.3,
	"nc": 0.1, "netcat": 0.1, "socat": 0.1,
	"searchsploit": 0.3,
	"burpsuite":    0.3, "zap": 0.3,
}

var devTools = map[string]float64{
	"go": 0.2, "python": 0.1, "python3": 0.1, "node": 0.1, "npm": 0.2,
	"cargo": 0.3, "rustc": 0.3, "gcc": 0.2, "make": 0.15,
	"docker": 0.1, "kubectl": 0.2, "docker-compose": 0.15,
	"pytest": 0.3, "jest": 0.3, "mocha": 0.3,
	"pip": 0.15, "pip3": 0.15, "yarn": 0.2, "mvn": 0.2, "gradle": 0.2,
	"terraform": 0.2, "ansible": 0.2,
	"code": 0.1, "mvim": 0.05,
}

var htbIPPattern = regexp.MustCompile(`10\.(10|129)\.\d{1,3}\.\d{1,3}`)
var reverseShellPattern = regexp.MustCompile(`(?i)(bash\s+-i\s+>&\s*/dev/tcp|nc\s+.*-e\s|python[3]?\s+-c\s+.*socket|socat\s+.*exec)`)

var devCommandPatterns = []struct {
	pattern *regexp.Regexp
	weight  float64
	reason  string
}{
	{regexp.MustCompile(`go\s+(build|test|run|vet|mod)`), 0.25, "Go build/test commands"},
	{regexp.MustCompile(`npm\s+(test|run|start|install)`), 0.2, "npm commands"},
	{regexp.MustCompile(`git\s+(commit|push|pull|merge|rebase)`), 0.15, "Git workflow commands"},
	{regexp.MustCompile(`pytest|go\s+test|npm\s+test|jest`), 0.25, "Test runner commands"},
	{regexp.MustCompile(`docker\s+(build|run|compose)`), 0.15, "Docker commands"},
}

// extractBaseTool extracts the base binary from a command, skipping sudo/time/nohup.
func extractBaseTool(command string) string {
	skip := map[string]bool{
		"sudo": true, "time": true, "nohup": true, "strace": true,
		"nice": true, "timeout": true, "env": true,
	}
	fields := strings.Fields(command)
	skipNext := false
	for _, f := range fields {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(f, "-") {
			if !strings.HasPrefix(f, "--") && !strings.Contains(f, "=") && len(f) == 2 {
				skipNext = true
			}
			continue
		}
		name := filepath.Base(f)
		if skip[name] {
			continue
		}
		return name
	}
	return ""
}

func collectSignals(events []store.Event) []signal {
	var signals []signal
	seenTools := map[string]bool{}

	for _, e := range events {
		cmd := strings.TrimSpace(e.Stdin)
		if cmd == "" {
			continue
		}

		// Tool-based detection (deduplicate per tool)
		tool := extractBaseTool(cmd)
		if tool != "" && !seenTools[tool] {
			if w, ok := pentestTools[tool]; ok {
				seenTools[tool] = true
				signals = append(signals, signal{
					Type:   TypePentest,
					Weight: w,
					Reason: "pentest tool: " + tool,
				})
			}
			if w, ok := devTools[tool]; ok {
				seenTools[tool] = true
				signals = append(signals, signal{
					Type:   TypeDebug,
					Weight: w,
					Reason: "dev tool: " + tool,
				})
			}
		}

		// HTB/CTF IP detection (only signal once)
		if htbIPPattern.MatchString(cmd) {
			if !seenTools["__htb_ip"] {
				seenTools["__htb_ip"] = true
				signals = append(signals, signal{
					Type:   TypePentest,
					Weight: 0.4,
					Reason: "HTB/CTF IP range detected (10.10.x.x / 10.129.x.x)",
				})
			}
		}

		// Reverse shell pattern
		if reverseShellPattern.MatchString(cmd) {
			if !seenTools["__revshell"] {
				seenTools["__revshell"] = true
				signals = append(signals, signal{
					Type:   TypePentest,
					Weight: 0.5,
					Reason: "reverse shell pattern detected",
				})
			}
		}

		// Dev command patterns
		for i, p := range devCommandPatterns {
			key := "__devpat_" + string(rune(i))
			if p.pattern.MatchString(cmd) && !seenTools[key] {
				seenTools[key] = true
				signals = append(signals, signal{
					Type:   TypeDebug,
					Weight: p.weight,
					Reason: p.reason,
				})
			}
		}
	}

	return signals
}

func scoreSignals(signals []signal) Result {
	scores := map[SessionType]float64{}
	reasons := map[SessionType][]string{}

	for _, s := range signals {
		scores[s.Type] += s.Weight
		reasons[s.Type] = append(reasons[s.Type], s.Reason)
	}

	best := TypeEducational
	bestScore := 0.0
	for t, s := range scores {
		if s > bestScore {
			best = t
			bestScore = s
		}
	}

	confidence := bestScore / (bestScore + 1.0)

	if confidence < 0.3 {
		return Result{
			Type:       TypeEducational,
			Confidence: confidence,
			Reasons:    []string{"no strong signals detected"},
		}
	}

	return Result{
		Type:       best,
		Confidence: confidence,
		Reasons:    reasons[best],
	}
}
