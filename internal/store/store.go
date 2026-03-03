package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func sessionDir(sessionID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".termdossier", "sessions", sessionID)
}

// AppendEvent appends an event to the session's events.jsonl with file locking.
func AppendEvent(sessionID string, event Event) error {
	path := filepath.Join(sessionDir(sessionID), "events.jsonl")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock events file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = fmt.Fprintf(f, "%s\n", b)
	return err
}

// ReadEvents reads all events from the session's events.jsonl.
func ReadEvents(sessionID string) ([]Event, error) {
	path := filepath.Join(sessionDir(sessionID), "events.jsonl")

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open events file: %w", err)
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}
