package preprocess

import "testing"

func TestDedup_Empty(t *testing.T) {
	got := Dedup(nil)
	if got != nil {
		t.Errorf("Dedup(nil) = %v, want nil", got)
	}
}

func TestDedup_NoDuplicates(t *testing.T) {
	events := []ProcessedEvent{
		{ProcessedStdout: "a", RepeatCount: 1},
		{ProcessedStdout: "b", RepeatCount: 1},
	}
	events[0].Stdin = "cmd1"
	events[1].Stdin = "cmd2"

	got := Dedup(events)
	if len(got) != 2 {
		t.Errorf("Dedup no-dups: got %d events, want 2", len(got))
	}
}

func TestDedup_ConsecutiveDuplicates(t *testing.T) {
	events := []ProcessedEvent{
		{ProcessedStdout: "same output", RepeatCount: 1},
		{ProcessedStdout: "same output", RepeatCount: 1},
		{ProcessedStdout: "same output", RepeatCount: 1},
	}
	events[0].Stdin = "ping 10.10.10.1"
	events[1].Stdin = "ping 10.10.10.1"
	events[2].Stdin = "ping 10.10.10.1"

	got := Dedup(events)
	if len(got) != 1 {
		t.Errorf("Dedup consecutive: got %d events, want 1", len(got))
	}
	if got[0].RepeatCount != 3 {
		t.Errorf("Dedup repeat count: got %d, want 3", got[0].RepeatCount)
	}
}

func TestDedup_NonConsecutiveSame(t *testing.T) {
	events := []ProcessedEvent{
		{ProcessedStdout: "a", RepeatCount: 1},
		{ProcessedStdout: "b", RepeatCount: 1},
		{ProcessedStdout: "a", RepeatCount: 1},
	}
	events[0].Stdin = "cmd"
	events[1].Stdin = "other"
	events[2].Stdin = "cmd"

	got := Dedup(events)
	if len(got) != 3 {
		t.Errorf("Dedup non-consecutive: got %d events, want 3 (only consecutive dedup)", len(got))
	}
}

func TestDedup_SameCommandDifferentOutput(t *testing.T) {
	events := []ProcessedEvent{
		{ProcessedStdout: "output1", RepeatCount: 1},
		{ProcessedStdout: "output2", RepeatCount: 1},
	}
	events[0].Stdin = "ps aux"
	events[1].Stdin = "ps aux"

	got := Dedup(events)
	if len(got) != 2 {
		t.Errorf("Dedup diff output: got %d events, want 2", len(got))
	}
}
