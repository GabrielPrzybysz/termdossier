package detect

import "github.com/perxibes/termdossier/internal/store"

// SessionType represents the detected session type, matching template names.
type SessionType string

const (
	TypePentest     SessionType = "pentest"
	TypeDebug       SessionType = "debug"
	TypeEducational SessionType = "educational"
)

// Result holds the detection outcome with confidence scoring.
type Result struct {
	Type       SessionType
	Confidence float64  // 0.0-1.0
	Reasons    []string // Human-readable explanations
}

// Detect analyzes events and returns the most likely session type.
func Detect(events []store.Event) Result {
	signals := collectSignals(events)
	return scoreSignals(signals)
}
