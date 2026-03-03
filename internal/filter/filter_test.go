package filter

import (
	"strings"
	"testing"

	"github.com/perxibes/termdossier/internal/store"
)

func TestApply_TrivialCommands(t *testing.T) {
	events := []store.Event{
		{Stdin: "cd /tmp"},
		{Stdin: "ls"},
		{Stdin: "ll"},
		{Stdin: "pwd"},
		{Stdin: "clear"},
		{Stdin: "cls"},
		{Stdin: "history"},
		{Stdin: "exit"},
		{Stdin: "logout"},
	}

	got := Apply(events)
	if len(got) != 0 {
		t.Errorf("Apply(trivial) = %d events, want 0", len(got))
	}
}

func TestApply_PreserveNonTrivial(t *testing.T) {
	events := []store.Event{
		{Stdin: "nmap -sV 10.10.10.1"},
		{Stdin: "gobuster dir -u http://target"},
		{Stdin: "python3 exploit.py"},
		{Stdin: "cat /etc/passwd"},
		{Stdin: "whoami"},
	}

	got := Apply(events)
	if len(got) != len(events) {
		t.Errorf("Apply(non-trivial) = %d events, want %d", len(got), len(events))
	}
}

func TestApply_EmptyStdin(t *testing.T) {
	events := []store.Event{
		{Stdin: ""},
		{Stdin: "   "},
	}

	got := Apply(events)
	if len(got) != 0 {
		t.Errorf("Apply(empty stdin) = %d events, want 0", len(got))
	}
}

func TestApply_RedactPasswords(t *testing.T) {
	events := []store.Event{
		{Stdin: "mysql -u root password=s3cret123"},
		{Stdin: "echo hello", Stdout: "token=abc123def"},
	}

	got := Apply(events)
	if len(got) != 2 {
		t.Fatalf("Apply(redact) = %d events, want 2", len(got))
	}
	if strings.Contains(got[0].Stdin, "s3cret123") {
		t.Error("expected password value to be redacted from stdin")
	}
	if !strings.Contains(got[0].Stdin, "[REDACTED]") {
		t.Error("expected [REDACTED] marker in stdin")
	}
	if strings.Contains(got[1].Stdout, "abc123def") {
		t.Error("expected token value to be redacted from stdout")
	}
}

func TestApply_RedactAWSKeys(t *testing.T) {
	events := []store.Event{
		{Stdin: "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"},
	}

	got := Apply(events)
	if len(got) != 1 {
		t.Fatalf("Apply(AWS) = %d events, want 1", len(got))
	}
	if strings.Contains(got[0].Stdin, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("expected AWS key to be redacted")
	}
	if !strings.Contains(got[0].Stdin, "[REDACTED]") {
		t.Error("expected [REDACTED] marker for AWS key")
	}
}

func TestApply_RedactBearerTokens(t *testing.T) {
	events := []store.Event{
		{Stdin: `curl -H "Authorization: Bearer eyJhbGciOi.xyz" http://api`},
	}

	got := Apply(events)
	if len(got) != 1 {
		t.Fatalf("Apply(bearer) = %d events, want 1", len(got))
	}
	if strings.Contains(got[0].Stdin, "eyJhbGciOi") {
		t.Error("expected bearer token to be redacted")
	}
}

func TestApply_RedactStderr(t *testing.T) {
	events := []store.Event{
		{Stdin: "some-cmd", Stderr: "Error: key=supersecret"},
	}

	got := Apply(events)
	if len(got) != 1 {
		t.Fatalf("Apply(stderr) = %d events, want 1", len(got))
	}
	if strings.Contains(got[0].Stderr, "supersecret") {
		t.Error("expected sensitive data in stderr to be redacted")
	}
}

func TestApply_MixedTrivialAndReal(t *testing.T) {
	events := []store.Event{
		{Stdin: "cd /opt"},
		{Stdin: "nmap -sV target"},
		{Stdin: "ls -la"},
		{Stdin: "gobuster dir -u http://target"},
		{Stdin: "clear"},
		{Stdin: "cat /etc/shadow"},
		{Stdin: "pwd"},
	}

	got := Apply(events)
	if len(got) != 3 {
		t.Errorf("Apply(mixed) = %d events, want 3", len(got))
	}
	if got[0].Stdin != "nmap -sV target" {
		t.Errorf("event[0].Stdin = %q, want nmap command", got[0].Stdin)
	}
	if got[1].Stdin != "gobuster dir -u http://target" {
		t.Errorf("event[1].Stdin = %q, want gobuster command", got[1].Stdin)
	}
}

func TestApply_PreservesEventFields(t *testing.T) {
	events := []store.Event{
		{
			Timestamp:  "2024-01-01T00:00:00Z",
			SessionID:  "test-session",
			TerminalID: "term-1",
			CWD:        "/home/user",
			Stdin:      "whoami",
			Stdout:     "user",
			ExitCode:   0,
			DurationMS: 50,
		},
	}

	got := Apply(events)
	if len(got) != 1 {
		t.Fatalf("Apply(fields) = %d events, want 1", len(got))
	}
	e := got[0]
	if e.Timestamp != "2024-01-01T00:00:00Z" {
		t.Error("timestamp not preserved")
	}
	if e.SessionID != "test-session" {
		t.Error("session_id not preserved")
	}
	if e.CWD != "/home/user" {
		t.Error("cwd not preserved")
	}
	if e.ExitCode != 0 {
		t.Error("exit_code not preserved")
	}
}

func TestApply_TrivialWithArgs(t *testing.T) {
	events := []store.Event{
		{Stdin: "cd /var/log"},
		{Stdin: "ls -la /tmp"},
		{Stdin: "history 50"},
	}

	got := Apply(events)
	if len(got) != 0 {
		t.Errorf("Apply(trivial with args) = %d events, want 0", len(got))
	}
}
