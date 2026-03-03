package preprocess

// Dedup collapses consecutive events that have identical Stdin AND
// identical ProcessedStdout into a single event with a RepeatCount.
func Dedup(events []ProcessedEvent) []ProcessedEvent {
	if len(events) == 0 {
		return events
	}

	out := make([]ProcessedEvent, 0, len(events))
	out = append(out, events[0])

	for i := 1; i < len(events); i++ {
		prev := &out[len(out)-1]
		cur := events[i]

		if cur.Stdin == prev.Stdin && cur.ProcessedStdout == prev.ProcessedStdout {
			prev.RepeatCount++
			continue
		}
		out = append(out, cur)
	}

	return out
}
