package chunk

import (
	"testing"
	"time"

	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/store"
)

func makeEvent(t time.Time, cmd string) preprocess.ProcessedEvent {
	return preprocess.ProcessedEvent{
		Event: store.Event{
			Timestamp: t.Format(time.RFC3339Nano),
			Stdin:     cmd,
		},
		RepeatCount: 1,
	}
}

func TestSplit_Empty(t *testing.T) {
	got := Split(nil, DefaultConfig())
	if got != nil {
		t.Errorf("Split(nil) = %v, want nil", got)
	}
}

func TestSplit_SingleChunk(t *testing.T) {
	base := time.Now()
	events := []preprocess.ProcessedEvent{
		makeEvent(base, "cmd1"),
		makeEvent(base.Add(10*time.Second), "cmd2"),
		makeEvent(base.Add(20*time.Second), "cmd3"),
	}

	chunks := Split(events, DefaultConfig())
	if len(chunks) != 1 {
		t.Fatalf("Split single: got %d chunks, want 1", len(chunks))
	}
	if len(chunks[0].Events) != 3 {
		t.Errorf("Split single events: got %d, want 3", len(chunks[0].Events))
	}
}

func TestSplit_TemporalGap(t *testing.T) {
	base := time.Now()
	events := []preprocess.ProcessedEvent{
		makeEvent(base, "cmd1"),
		makeEvent(base.Add(1*time.Minute), "cmd2"),
		makeEvent(base.Add(2*time.Minute), "cmd3"),
		// 10 minute gap
		makeEvent(base.Add(12*time.Minute), "cmd4"),
		makeEvent(base.Add(13*time.Minute), "cmd5"),
		makeEvent(base.Add(14*time.Minute), "cmd6"),
	}

	cfg := DefaultConfig()
	cfg.TemporalGap = 5 * time.Minute

	chunks := Split(events, cfg)
	if len(chunks) != 2 {
		t.Fatalf("Split temporal: got %d chunks, want 2", len(chunks))
	}
	if len(chunks[0].Events) != 3 {
		t.Errorf("Chunk 0: got %d events, want 3", len(chunks[0].Events))
	}
	if len(chunks[1].Events) != 3 {
		t.Errorf("Chunk 1: got %d events, want 3", len(chunks[1].Events))
	}
}

func TestSplit_MaxSize(t *testing.T) {
	base := time.Now()
	var events []preprocess.ProcessedEvent
	for i := range 12 {
		events = append(events, makeEvent(base.Add(time.Duration(i)*time.Second), "cmd"))
	}

	cfg := DefaultConfig()
	cfg.MaxCommandsPerChunk = 5
	cfg.MinChunkSize = 1

	chunks := Split(events, cfg)
	if len(chunks) != 3 {
		t.Fatalf("Split max size: got %d chunks, want 3", len(chunks))
	}
}

func TestSplit_MergeSmall(t *testing.T) {
	base := time.Now()
	events := []preprocess.ProcessedEvent{
		makeEvent(base, "cmd1"),
		makeEvent(base.Add(1*time.Minute), "cmd2"),
		makeEvent(base.Add(2*time.Minute), "cmd3"),
		// gap
		makeEvent(base.Add(10*time.Minute), "cmd4"),
		// gap (only 1 command, below MinChunkSize of 3)
		makeEvent(base.Add(20*time.Minute), "cmd5"),
		makeEvent(base.Add(21*time.Minute), "cmd6"),
		makeEvent(base.Add(22*time.Minute), "cmd7"),
	}

	cfg := DefaultConfig()
	cfg.TemporalGap = 5 * time.Minute
	cfg.MinChunkSize = 3

	chunks := Split(events, cfg)
	// cmd4 alone should be merged into an adjacent chunk
	for _, c := range chunks {
		if len(c.Events) < cfg.MinChunkSize {
			t.Errorf("Chunk %d has only %d events, below min %d", c.Index, len(c.Events), cfg.MinChunkSize)
		}
	}
}

func TestBuildFinalPrompt(t *testing.T) {
	base := time.Now()
	chunks := []Chunk{
		{
			Index:     0,
			StartTime: base,
			EndTime:   base.Add(5 * time.Minute),
			Events:    make([]preprocess.ProcessedEvent, 3),
			Summary:   "Performed reconnaissance scan.",
		},
		{
			Index:     1,
			StartTime: base.Add(10 * time.Minute),
			EndTime:   base.Add(15 * time.Minute),
			Events:    make([]preprocess.ProcessedEvent, 5),
			Summary:   "Exploited web vulnerability.",
		},
	}

	result := BuildFinalPrompt(chunks, "HTB box", "abc-123", "2024-01-01T12:00:00Z", 8)

	if !containsStr(result, "HTB box") {
		t.Error("expected context in final prompt")
	}
	if !containsStr(result, "Performed reconnaissance") {
		t.Error("expected chunk 1 summary")
	}
	if !containsStr(result, "Exploited web") {
		t.Error("expected chunk 2 summary")
	}
	if !containsStr(result, "Phase 1") {
		t.Error("expected Phase 1 label")
	}
	if !containsStr(result, "Phase 2") {
		t.Error("expected Phase 2 label")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
