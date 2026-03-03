package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestCreate(t *testing.T) {
	home := setupTestHome(t)

	meta, err := Create("mistral")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if meta.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	if meta.Model != "mistral" {
		t.Errorf("Model = %q, want %q", meta.Model, "mistral")
	}
	if meta.StartedAt == "" {
		t.Error("StartedAt should not be empty")
	}
	if meta.Version == "" {
		t.Error("Version should not be empty")
	}

	// Verify directory exists
	dir := filepath.Join(home, ".termdossier", "sessions", meta.SessionID)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("session dir not created: %v", err)
	}

	// Verify meta.json exists and is valid
	metaPath := filepath.Join(dir, "meta.json")
	b, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var readMeta Meta
	if err := json.Unmarshal(b, &readMeta); err != nil {
		t.Fatalf("unmarshal meta.json: %v", err)
	}
	if readMeta.SessionID != meta.SessionID {
		t.Errorf("meta.json SessionID = %q, want %q", readMeta.SessionID, meta.SessionID)
	}
}

func TestGetActive(t *testing.T) {
	setupTestHome(t)

	meta, err := Create("mistral")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := GetActive()
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if got != meta.SessionID {
		t.Errorf("GetActive = %q, want %q", got, meta.SessionID)
	}
}

func TestGetActive_NoSession(t *testing.T) {
	setupTestHome(t)

	_, err := GetActive()
	if err == nil {
		t.Error("GetActive with no session should return error")
	}
}

func TestClearActive(t *testing.T) {
	setupTestHome(t)

	_, err := Create("mistral")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := ClearActive(); err != nil {
		t.Fatalf("ClearActive: %v", err)
	}

	_, err = GetActive()
	if err == nil {
		t.Error("GetActive after ClearActive should return error")
	}
}

func TestReadMeta(t *testing.T) {
	setupTestHome(t)

	created, err := Create("llama3")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := ReadMeta(created.SessionID)
	if err != nil {
		t.Fatalf("ReadMeta: %v", err)
	}
	if got.SessionID != created.SessionID {
		t.Errorf("ReadMeta.SessionID = %q, want %q", got.SessionID, created.SessionID)
	}
	if got.Model != "llama3" {
		t.Errorf("ReadMeta.Model = %q, want %q", got.Model, "llama3")
	}
	if got.StartedAt != created.StartedAt {
		t.Errorf("ReadMeta.StartedAt = %q, want %q", got.StartedAt, created.StartedAt)
	}
}

func TestReadMeta_Nonexistent(t *testing.T) {
	setupTestHome(t)

	_, err := ReadMeta("nonexistent-session")
	if err == nil {
		t.Error("ReadMeta(nonexistent) should return error")
	}
}

func TestDir(t *testing.T) {
	setupTestHome(t)

	dir := Dir("abc-123")
	if !filepath.IsAbs(dir) {
		t.Errorf("Dir should return absolute path, got %q", dir)
	}
	if filepath.Base(dir) != "abc-123" {
		t.Errorf("Dir base = %q, want %q", filepath.Base(dir), "abc-123")
	}
}

func TestSetPID(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-pid-session"

	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}

	if err := SetPID(sid, 12345); err != nil {
		t.Fatalf("SetPID: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	if string(b) != "12345" {
		t.Errorf("pid file = %q, want %q", string(b), "12345")
	}
}

func TestMultipleSessions(t *testing.T) {
	setupTestHome(t)

	meta1, err := Create("model1")
	if err != nil {
		t.Fatalf("Create 1: %v", err)
	}

	meta2, err := Create("model2")
	if err != nil {
		t.Fatalf("Create 2: %v", err)
	}

	// Active should be the last one
	active, err := GetActive()
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if active != meta2.SessionID {
		t.Errorf("active = %q, want last created %q", active, meta2.SessionID)
	}

	// Both sessions should have valid metadata
	got1, err := ReadMeta(meta1.SessionID)
	if err != nil {
		t.Fatalf("ReadMeta(1): %v", err)
	}
	if got1.Model != "model1" {
		t.Errorf("session 1 model = %q, want %q", got1.Model, "model1")
	}

	got2, err := ReadMeta(meta2.SessionID)
	if err != nil {
		t.Fatalf("ReadMeta(2): %v", err)
	}
	if got2.Model != "model2" {
		t.Errorf("session 2 model = %q, want %q", got2.Model, "model2")
	}
}
