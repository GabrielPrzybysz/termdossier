package llm

// Provider is the interface all LLM backends must implement.
type Provider interface {
	// EnsureRunning verifies the backend is reachable, starting it if needed.
	EnsureRunning() error
	// EnsureModel makes sure the model is available locally, pulling if needed.
	EnsureModel(model string) error
	// Generate sends a prompt and returns the full response.
	Generate(prompt string) (string, error)
}
