package chunk

import (
	"time"

	"github.com/perxibes/termdossier/internal/preprocess"
)

// Config controls chunking behavior.
type Config struct {
	MaxCommandsPerChunk int
	TemporalGap         time.Duration
	MinChunkSize        int
}

// DefaultConfig returns sensible defaults for chunking.
func DefaultConfig() Config {
	return Config{
		MaxCommandsPerChunk: 10,
		TemporalGap:         5 * time.Minute,
		MinChunkSize:        3,
	}
}

// Chunk represents a group of temporally related events.
type Chunk struct {
	Index     int
	Events    []preprocess.ProcessedEvent
	StartTime time.Time
	EndTime   time.Time
	Summary   string
}

// Split divides events into chunks based on temporal gaps and size limits.
func Split(events []preprocess.ProcessedEvent, cfg Config) []Chunk {
	if len(events) == 0 {
		return nil
	}

	if cfg.MaxCommandsPerChunk <= 0 {
		cfg.MaxCommandsPerChunk = 10
	}

	var chunks []Chunk
	current := Chunk{Index: 0}
	var prevTime time.Time

	for _, e := range events {
		ts := parseTimestamp(e.Timestamp)

		// Start a new chunk on temporal gap or size limit
		if len(current.Events) > 0 {
			gap := ts.Sub(prevTime)
			if gap >= cfg.TemporalGap || len(current.Events) >= cfg.MaxCommandsPerChunk {
				current.EndTime = prevTime
				chunks = append(chunks, current)
				current = Chunk{Index: len(chunks)}
			}
		}

		if len(current.Events) == 0 {
			current.StartTime = ts
		}
		current.Events = append(current.Events, e)
		prevTime = ts
	}

	// Flush last chunk
	if len(current.Events) > 0 {
		current.EndTime = prevTime
		chunks = append(chunks, current)
	}

	// Merge small chunks into adjacent ones
	return mergeSmall(chunks, cfg.MinChunkSize)
}

// mergeSmall merges any chunk with fewer than minSize events into the next
// chunk. If it is the last chunk, merge into the previous one.
func mergeSmall(chunks []Chunk, minSize int) []Chunk {
	if len(chunks) <= 1 {
		return chunks
	}

	var merged []Chunk
	for i := 0; i < len(chunks); i++ {
		c := chunks[i]
		if len(c.Events) < minSize && i+1 < len(chunks) {
			// Merge forward into next chunk
			chunks[i+1].Events = append(c.Events, chunks[i+1].Events...)
			chunks[i+1].StartTime = c.StartTime
			continue
		}
		if len(c.Events) < minSize && len(merged) > 0 {
			// Last chunk too small — merge backward
			prev := &merged[len(merged)-1]
			prev.Events = append(prev.Events, c.Events...)
			prev.EndTime = c.EndTime
			continue
		}
		c.Index = len(merged)
		merged = append(merged, c)
	}

	return merged
}

func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, ts)
	}
	return t
}
