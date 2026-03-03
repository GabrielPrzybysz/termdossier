package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

const version = "0.1.0"

// Meta is the session metadata stored in meta.json.
type Meta struct {
	SessionID string `json:"session_id"`
	StartedAt string `json:"started_at"`
	Model     string `json:"model"`
	Version   string `json:"version"`
}

func baseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".termdossier")
}

// Dir returns the storage directory for a session.
func Dir(sessionID string) string {
	return filepath.Join(baseDir(), "sessions", sessionID)
}

// Create initialises a new session and writes meta.json.
func Create(model string) (*Meta, error) {
	id := uuid.New().String()
	dir := Dir(id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}

	meta := &Meta{
		SessionID: id,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Model:     model,
		Version:   version,
	}

	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), b, 0600); err != nil {
		return nil, fmt.Errorf("write meta.json: %w", err)
	}

	return meta, setActive(id)
}

// GetActive returns the ID of the currently active session.
func GetActive() (string, error) {
	b, err := os.ReadFile(filepath.Join(baseDir(), "active_session"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no active session")
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// ClearActive removes the active session pointer.
func ClearActive() error {
	return os.Remove(filepath.Join(baseDir(), "active_session"))
}

// SetPID stores the PID of the capture process so stop can signal it.
func SetPID(sessionID string, pid int) error {
	path := filepath.Join(Dir(sessionID), "pid")
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}

// Kill sends SIGTERM to the session's capture process and clears the active pointer.
func Kill(sessionID string) error {
	pidPath := filepath.Join(Dir(sessionID), "pid")
	b, err := os.ReadFile(pidPath)
	if err != nil {
		ClearActive() //nolint
		return nil    // process already gone
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return fmt.Errorf("invalid pid file: %w", err)
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		ClearActive() //nolint
		return nil
	}
	p.Signal(syscall.SIGTERM) //nolint
	os.Remove(pidPath)        //nolint
	return ClearActive()
}

// ReadMeta loads session metadata from disk.
func ReadMeta(sessionID string) (*Meta, error) {
	b, err := os.ReadFile(filepath.Join(Dir(sessionID), "meta.json"))
	if err != nil {
		return nil, err
	}
	var m Meta
	return &m, json.Unmarshal(b, &m)
}

func setActive(id string) error {
	if err := os.MkdirAll(baseDir(), 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(baseDir(), "active_session"), []byte(id), 0600)
}
