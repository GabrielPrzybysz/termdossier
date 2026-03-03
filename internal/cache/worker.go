package cache

import (
	"os"

	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/store"
)

// ProcessAndCache takes a single raw event, runs it through the
// preprocessing pipeline (ANSI strip, tool extraction, truncation),
// and appends the result to the cache file.
// This is called from the capture goroutine.
func ProcessAndCache(sessionID string, event store.Event) error {
	clean := preprocess.StripANSI(event.Stdout)

	tool := preprocess.ExtractBaseTool(event.Stdin)
	if ext := preprocess.LookupExtractor(event.Stdin); ext != nil {
		clean = ext(clean)
	} else if preprocess.PassthroughTools[tool] {
		if contentExt, detectedTool := preprocess.LookupByContent(clean); contentExt != nil {
			clean = contentExt(clean)
			tool = detectedTool
		}
	}
	limit := preprocess.LimitFor(tool)
	clean = preprocess.Truncate(clean, limit)

	ce := CachedEvent{
		ProcessedEvent: preprocess.ProcessedEvent{
			Event:           event,
			ProcessedStdout: clean,
			ToolName:        tool,
			RepeatCount:     1,
		},
		CacheVersion: currentCacheVersion,
	}

	return AppendCached(sessionID, ce)
}

// RebuildCache reads all raw events and rebuilds the cache from scratch.
func RebuildCache(sessionID string) ([]preprocess.ProcessedEvent, error) {
	events, err := store.ReadEvents(sessionID)
	if err != nil {
		return nil, err
	}

	// Remove existing cache file to rebuild
	_ = removeCache(sessionID)

	processed := preprocess.Pipeline(events)

	// Write all to cache
	for _, pe := range processed {
		ce := CachedEvent{
			ProcessedEvent: pe,
			CacheVersion:   currentCacheVersion,
		}
		if err := AppendCached(sessionID, ce); err != nil {
			return nil, err
		}
	}

	return processed, nil
}

func removeCache(sessionID string) error {
	return os.Remove(CachePath(sessionID))
}
