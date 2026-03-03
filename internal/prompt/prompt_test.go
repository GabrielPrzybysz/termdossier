package prompt

import (
	"sort"
	"testing"
)

func TestGet_Pentest(t *testing.T) {
	tmpl, err := Get("pentest")
	if err != nil {
		t.Fatalf("Get(pentest) error: %v", err)
	}
	if tmpl.Name != "pentest" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "pentest")
	}
	if tmpl.System == "" {
		t.Error("System prompt should not be empty")
	}
	if tmpl.User == "" {
		t.Error("User prompt template should not be empty")
	}
}

func TestGet_Educational(t *testing.T) {
	tmpl, err := Get("educational")
	if err != nil {
		t.Fatalf("Get(educational) error: %v", err)
	}
	if tmpl.Name != "educational" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "educational")
	}
}

func TestGet_Debug(t *testing.T) {
	tmpl, err := Get("debug")
	if err != nil {
		t.Fatalf("Get(debug) error: %v", err)
	}
	if tmpl.Name != "debug" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "debug")
	}
}

func TestGet_Unknown(t *testing.T) {
	_, err := Get("nonexistent-template")
	if err == nil {
		t.Error("Get(unknown) should return error")
	}
}

func TestList(t *testing.T) {
	names := List()
	if len(names) < 3 {
		t.Errorf("List() = %d templates, want at least 3", len(names))
	}

	expected := []string{"debug", "educational", "pentest"}
	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("List() missing template %q", name)
		}
	}

	// Verify sorted
	if !sort.StringsAreSorted(names) {
		t.Errorf("List() not sorted: %v", names)
	}
}

func TestDefault(t *testing.T) {
	d := Default()
	if d != "educational" {
		t.Errorf("Default() = %q, want %q", d, "educational")
	}

	// Default must be a valid template
	_, err := Get(d)
	if err != nil {
		t.Errorf("Get(Default()) error: %v", err)
	}
}

func TestRegister_Custom(t *testing.T) {
	custom := &Template{
		Name:        "test-custom",
		Description: "A test template",
		System:      "You are a test assistant.",
		User:        "Analyze: {{.CommandList}}",
	}

	Register(custom)

	got, err := Get("test-custom")
	if err != nil {
		t.Fatalf("Get(test-custom) error: %v", err)
	}
	if got.Name != "test-custom" {
		t.Errorf("Name = %q, want %q", got.Name, "test-custom")
	}
	if got.System != "You are a test assistant." {
		t.Error("System prompt mismatch")
	}

	// Verify it appears in List()
	names := List()
	found := false
	for _, n := range names {
		if n == "test-custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom template not in List()")
	}

	// Cleanup: remove from registry to not affect other tests
	delete(registry, "test-custom")
}

func TestTemplateFields(t *testing.T) {
	for _, name := range List() {
		tmpl, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q) error: %v", name, err)
			continue
		}
		if tmpl.Name == "" {
			t.Errorf("template %q has empty Name", name)
		}
		if tmpl.System == "" {
			t.Errorf("template %q has empty System prompt", name)
		}
		if tmpl.User == "" {
			t.Errorf("template %q has empty User prompt", name)
		}
		if tmpl.Description == "" {
			t.Errorf("template %q has empty Description", name)
		}
	}
}

func TestRegister_Overwrite(t *testing.T) {
	original := &Template{
		Name:        "test-overwrite",
		Description: "Original",
		System:      "System A",
		User:        "User A",
	}
	Register(original)

	replacement := &Template{
		Name:        "test-overwrite",
		Description: "Replacement",
		System:      "System B",
		User:        "User B",
	}
	Register(replacement)

	got, err := Get("test-overwrite")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Replacement" {
		t.Errorf("expected overwritten template, got Description=%q", got.Description)
	}

	delete(registry, "test-overwrite")
}
