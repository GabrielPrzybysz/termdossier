package prompt

import (
	"fmt"
	"sort"
)

// Template defines a prompt pair (system + user) for a specific report type.
type Template struct {
	Name        string
	Description string
	System      string
	User        string
}

// TemplateData is the data passed to the user template.
type TemplateData struct {
	Context      string
	SessionID    string
	StartedAt    string
	CommandCount int
	CommandList  string
}

var registry = map[string]*Template{}

// Register adds a template to the registry.
func Register(t *Template) {
	registry[t.Name] = t
}

// Get returns a template by name, or an error if not found.
func Get(name string) (*Template, error) {
	t, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown template %q", name)
	}
	return t, nil
}

// List returns all registered template names sorted alphabetically.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Default returns the default template name.
func Default() string {
	return "educational"
}
