package preprocess

import (
	"testing"

	"github.com/perxibes/termdossier/internal/store"
)

func TestPipeline_Basic(t *testing.T) {
	events := []store.Event{
		{
			Stdin:  "echo hello",
			Stdout: "\x1b[32mhello\x1b[0m\n",
		},
	}

	got := Pipeline(events)
	if len(got) != 1 {
		t.Fatalf("Pipeline: got %d events, want 1", len(got))
	}
	if got[0].ProcessedStdout != "hello\n" {
		t.Errorf("Pipeline ANSI strip: got %q, want %q", got[0].ProcessedStdout, "hello\n")
	}
	if got[0].RepeatCount != 1 {
		t.Errorf("Pipeline repeat count: got %d, want 1", got[0].RepeatCount)
	}
}

func TestPipeline_NmapExtraction(t *testing.T) {
	events := []store.Event{
		{
			Stdin: "nmap -sV 10.10.10.1",
			Stdout: `Starting Nmap 7.94
Nmap scan report for 10.10.10.1
Host is up (0.03s latency).
Not shown: 997 closed ports
22/tcp open ssh OpenSSH 8.9p1
Nmap done: 1 IP address scanned in 15.32 seconds`,
		},
	}

	got := Pipeline(events)
	if len(got) != 1 {
		t.Fatalf("Pipeline nmap: got %d events, want 1", len(got))
	}
	if got[0].ToolName != "nmap" {
		t.Errorf("Pipeline tool name: got %q, want %q", got[0].ToolName, "nmap")
	}
	// Should have extracted only relevant lines
	if containsString(got[0].ProcessedStdout, "Not shown") {
		t.Error("Pipeline nmap: should not contain 'Not shown' line")
	}
	if !containsString(got[0].ProcessedStdout, "22/tcp") {
		t.Error("Pipeline nmap: should contain open port")
	}
}

func TestPipeline_Dedup(t *testing.T) {
	events := []store.Event{
		{Stdin: "whoami", Stdout: "user\n"},
		{Stdin: "whoami", Stdout: "user\n"},
		{Stdin: "whoami", Stdout: "user\n"},
		{Stdin: "id", Stdout: "uid=1000\n"},
	}

	got := Pipeline(events)
	if len(got) != 2 {
		t.Fatalf("Pipeline dedup: got %d events, want 2", len(got))
	}
	if got[0].RepeatCount != 3 {
		t.Errorf("Pipeline dedup repeat: got %d, want 3", got[0].RepeatCount)
	}
	if got[1].RepeatCount != 1 {
		t.Errorf("Pipeline second event repeat: got %d, want 1", got[1].RepeatCount)
	}
}

func TestPipeline_EmptyStdout(t *testing.T) {
	events := []store.Event{
		{Stdin: "mkdir /tmp/test", Stdout: ""},
	}

	got := Pipeline(events)
	if len(got) != 1 {
		t.Fatalf("Pipeline empty: got %d events, want 1", len(got))
	}
	if got[0].ProcessedStdout != "" {
		t.Errorf("Pipeline empty stdout: got %q, want empty", got[0].ProcessedStdout)
	}
}
