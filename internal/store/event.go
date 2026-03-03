package store

// Event represents a single recorded command.
type Event struct {
	Timestamp  string `json:"timestamp"`
	SessionID  string `json:"session_id"`
	TerminalID string `json:"terminal_id"`
	CWD        string `json:"cwd"`
	Stdin      string `json:"stdin"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
}
