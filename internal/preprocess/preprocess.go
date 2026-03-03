package preprocess

import "github.com/perxibes/termdossier/internal/store"

// ProcessedEvent wraps a store.Event with preprocessed stdout
// and deduplication metadata.
type ProcessedEvent struct {
	store.Event
	ProcessedStdout string `json:"processed_stdout"`
	ToolName        string `json:"tool_name"`
	RepeatCount     int    `json:"repeat_count"`
}

// Pipeline applies the full preprocessing chain to a slice of events:
// 1. Strip ANSI escape codes from stdout
// 2. Apply tool-specific extractors (if registered)
// 3. Truncate to per-tool byte limits
// 4. Deduplicate consecutive identical commands
func Pipeline(events []store.Event) []ProcessedEvent {
	processed := make([]ProcessedEvent, 0, len(events))
	for _, e := range events {
		pe := ProcessedEvent{Event: e, RepeatCount: 1}

		// Step 1: Strip ANSI
		clean := StripANSI(e.Stdout)

		// Step 2: Tool detection + extraction
		tool := ExtractBaseTool(e.Stdin)
		pe.ToolName = tool
		if ext := LookupExtractor(e.Stdin); ext != nil {
			clean = ext(clean)
		} else if PassthroughTools[tool] {
			// Passthrough tool (cat, less, etc.) — try content-based detection
			if contentExt, detectedTool := LookupByContent(clean); contentExt != nil {
				clean = contentExt(clean)
				pe.ToolName = detectedTool
				tool = detectedTool
			}
		}

		// Step 3: Truncate
		limit := LimitFor(tool)
		clean = Truncate(clean, limit)

		pe.ProcessedStdout = clean
		processed = append(processed, pe)
	}

	// Step 4: Dedup
	return Dedup(processed)
}
