package report

import (
	"strings"
	"testing"

	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/store"
)

func TestBuildCommandList(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:      "nmap -sV target",
				CWD:        "/tmp",
				ExitCode:   0,
				DurationMS: 5000,
			},
			ProcessedStdout: "22/tcp open ssh\n80/tcp open http\n",
			ToolName:        "nmap",
			RepeatCount:     1,
		},
		{
			Event: store.Event{
				Stdin:      "whoami",
				CWD:        "/home/user",
				ExitCode:   0,
				DurationMS: 5,
			},
			ProcessedStdout: "root",
			ToolName:        "whoami",
			RepeatCount:     1,
		},
	}

	got := buildCommandList(events)

	// Check numbering
	if !strings.Contains(got, "#1 ") {
		t.Error("expected #1 numbering")
	}
	if !strings.Contains(got, "#2 ") {
		t.Error("expected #2 numbering")
	}

	// Check command lines
	if !strings.Contains(got, "nmap -sV target") {
		t.Error("expected nmap command in output")
	}
	if !strings.Contains(got, "whoami") {
		t.Error("expected whoami command in output")
	}

	// Check exit code and duration
	if !strings.Contains(got, "[0 | 5000ms | /tmp]") {
		t.Error("expected exit code/duration/cwd for nmap")
	}

	// Check output markers
	if !strings.Contains(got, "--- output ---") {
		t.Error("expected output start marker")
	}
	if !strings.Contains(got, "--- end output ---") {
		t.Error("expected output end marker")
	}

	// Check stdout content
	if !strings.Contains(got, "22/tcp open ssh") {
		t.Error("expected nmap output content")
	}
}

func TestBuildCommandList_WithRepeat(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:      "ping -c 1 target",
				CWD:        "/tmp",
				ExitCode:   0,
				DurationMS: 100,
			},
			ProcessedStdout: "",
			RepeatCount:     5,
		},
	}

	got := buildCommandList(events)
	if !strings.Contains(got, "(repeated 5x)") {
		t.Error("expected repeat count annotation")
	}
}

func TestBuildCommandList_NoRepeatForSingle(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:    "test",
				CWD:      "/",
				ExitCode: 0,
			},
			RepeatCount: 1,
		},
	}

	got := buildCommandList(events)
	if strings.Contains(got, "repeated") {
		t.Error("should not show repeat count for single execution")
	}
}

func TestBuildCommandList_Empty(t *testing.T) {
	got := buildCommandList(nil)
	if got != "" {
		t.Errorf("buildCommandList(nil) = %q, want empty", got)
	}
}

func TestBuildCommandList_NoStdout(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:      "mkdir /tmp/test",
				CWD:        "/tmp",
				ExitCode:   0,
				DurationMS: 2,
			},
			ProcessedStdout: "",
			RepeatCount:     1,
		},
	}

	got := buildCommandList(events)
	if strings.Contains(got, "--- output ---") {
		t.Error("should not include output markers when stdout is empty")
	}
}

func TestBuildCommandList_FailedCommand(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:      "cat /nonexistent",
				CWD:        "/tmp",
				ExitCode:   1,
				DurationMS: 3,
			},
			ProcessedStdout: "",
			RepeatCount:     1,
		},
	}

	got := buildCommandList(events)
	// Failed commands have "!" marker
	if !strings.Contains(got, "!") {
		t.Error("expected ! marker for non-zero exit code")
	}
}

func TestBuildCommandList_MultipleWithMixedStdout(t *testing.T) {
	events := []preprocess.ProcessedEvent{
		{
			Event: store.Event{
				Stdin:      "whoami",
				CWD:        "/",
				ExitCode:   0,
				DurationMS: 5,
			},
			ProcessedStdout: "root",
			RepeatCount:     1,
		},
		{
			Event: store.Event{
				Stdin:      "mkdir /tmp/test",
				CWD:        "/",
				ExitCode:   0,
				DurationMS: 2,
			},
			ProcessedStdout: "",
			RepeatCount:     1,
		},
		{
			Event: store.Event{
				Stdin:      "ls /tmp",
				CWD:        "/",
				ExitCode:   0,
				DurationMS: 3,
			},
			ProcessedStdout: "test\nfoo\nbar\n",
			RepeatCount:     1,
		},
	}

	got := buildCommandList(events)

	// Should have exactly 2 output sections (whoami and ls, not mkdir)
	count := strings.Count(got, "--- output ---")
	if count != 2 {
		t.Errorf("expected 2 output sections, got %d", count)
	}
}
