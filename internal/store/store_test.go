package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestAppendAndRead_RoundTrip(t *testing.T) {
	setupTestHome(t)
	sid := "test-session-roundtrip"

	// Create session directory
	dir := filepath.Join(os.Getenv("HOME"), ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	events := []Event{
		{
			Timestamp:  "2024-01-01T00:00:00Z",
			SessionID:  sid,
			TerminalID: "t1",
			CWD:        "/home/user",
			Stdin:      "whoami",
			Stdout:     "root",
			ExitCode:   0,
			DurationMS: 12,
		},
		{
			Timestamp:  "2024-01-01T00:01:00Z",
			SessionID:  sid,
			TerminalID: "t1",
			CWD:        "/tmp",
			Stdin:      "nmap -sV 10.10.10.1",
			Stdout:     "22/tcp open ssh",
			ExitCode:   0,
			DurationMS: 5000,
		},
		{
			Timestamp:  "2024-01-01T00:02:00Z",
			SessionID:  sid,
			TerminalID: "t1",
			CWD:        "/var/log",
			Stdin:      "cat auth.log",
			Stdout:     "",
			Stderr:     "Permission denied",
			ExitCode:   1,
			DurationMS: 5,
		},
	}

	for _, e := range events {
		if err := AppendEvent(sid, e); err != nil {
			t.Fatalf("AppendEvent: %v", err)
		}
	}

	got, err := ReadEvents(sid)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadEvents = %d events, want 3", len(got))
	}

	if got[0].Stdin != "whoami" {
		t.Errorf("event[0].Stdin = %q, want %q", got[0].Stdin, "whoami")
	}
	if got[0].Stdout != "root" {
		t.Errorf("event[0].Stdout = %q, want %q", got[0].Stdout, "root")
	}
	if got[1].DurationMS != 5000 {
		t.Errorf("event[1].DurationMS = %d, want 5000", got[1].DurationMS)
	}
	if got[2].ExitCode != 1 {
		t.Errorf("event[2].ExitCode = %d, want 1", got[2].ExitCode)
	}
	if got[2].Stderr != "Permission denied" {
		t.Errorf("event[2].Stderr = %q, want %q", got[2].Stderr, "Permission denied")
	}
}

func TestReadEvents_NoFile(t *testing.T) {
	setupTestHome(t)

	got, err := ReadEvents("nonexistent-session")
	if err != nil {
		t.Fatalf("ReadEvents(nonexistent) error: %v", err)
	}
	if got != nil {
		t.Errorf("ReadEvents(nonexistent) = %v, want nil", got)
	}
}

func TestAppendEvent_CreatesDir(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-session-autodir"

	// Create only session dir (the parent)
	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	err := AppendEvent(sid, Event{Stdin: "test"})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "events.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("events.jsonl not created: %v", err)
	}
}

func TestReadEvents_MalformedLine(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-session-malformed"

	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	// Write a mix of valid and invalid lines
	valid := Event{Stdin: "valid-command", ExitCode: 0}
	b, _ := json.Marshal(valid)

	content := string(b) + "\n{invalid json}\n" + string(b) + "\n"
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadEvents(sid)
	if err != nil {
		t.Fatalf("ReadEvents(malformed) error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ReadEvents(malformed) = %d events, want 2 (skipping bad line)", len(got))
	}
}

func TestMultipleAppends(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-session-multi"

	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	for i := range 5 {
		e := Event{Stdin: "cmd-" + string(rune('A'+i))}
		if err := AppendEvent(sid, e); err != nil {
			t.Fatalf("AppendEvent %d: %v", i, err)
		}
	}

	got, err := ReadEvents(sid)
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("ReadEvents = %d events, want 5", len(got))
	}
}

func TestReadEvents_EmptyLines(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-session-emptylines"

	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	valid := Event{Stdin: "test"}
	b, _ := json.Marshal(valid)
	content := "\n\n" + string(b) + "\n\n" + string(b) + "\n\n"
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadEvents(sid)
	if err != nil {
		t.Fatalf("ReadEvents(emptylines): %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ReadEvents(emptylines) = %d events, want 2", len(got))
	}
}
