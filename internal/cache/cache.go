package cache

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/session"
)

const currentCacheVersion = 2

// CachedEvent is a preprocessed event stored in the cache.
type CachedEvent struct {
	preprocess.ProcessedEvent
	CacheVersion int `json:"cache_version"`
}

// CachePath returns the path to the cache file for a session.
func CachePath(sessionID string) string {
	return filepath.Join(session.Dir(sessionID), "cache.jsonl")
}

// AppendCached appends a preprocessed event to the session cache.
func AppendCached(sessionID string, ce CachedEvent) error {
	path := CachePath(sessionID)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open cache file: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock cache file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint

	b, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("marshal cached event: %w", err)
	}

	_, err = fmt.Fprintf(f, "%s\n", b)
	return err
}

// ReadCached reads all cached events for a session.
func ReadCached(sessionID string) ([]CachedEvent, error) {
	path := CachePath(sessionID)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open cache file: %w", err)
	}
	defer f.Close()

	var events []CachedEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ce CachedEvent
		if err := json.Unmarshal(line, &ce); err != nil {
			continue
		}
		events = append(events, ce)
	}
	return events, scanner.Err()
}

// CountCached returns the number of cached events without loading them all.
func CountCached(sessionID string) int {
	path := CachePath(sessionID)

	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			count++
		}
	}
	return count
}

// IsStale checks if the cache is outdated (fewer cached events than
// raw events, or cache version mismatch).
func IsStale(sessionID string, rawEventCount int) bool {
	cached, err := ReadCached(sessionID)
	if err != nil || cached == nil {
		return true
	}
	if len(cached) != rawEventCount {
		return true
	}
	for _, ce := range cached {
		if ce.CacheVersion != currentCacheVersion {
			return true
		}
	}
	return false
}

// ExtractProcessed converts cached events to processed events.
func ExtractProcessed(cached []CachedEvent) []preprocess.ProcessedEvent {
	out := make([]preprocess.ProcessedEvent, len(cached))
	for i, ce := range cached {
		out[i] = ce.ProcessedEvent
	}
	return out
}
