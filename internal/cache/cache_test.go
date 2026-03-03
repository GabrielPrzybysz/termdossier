package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/store"
)

func setupTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func createSessionDir(t *testing.T, home, sid string) {
	t.Helper()
	dir := filepath.Join(home, ".termdossier", "sessions", sid)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
}

func makeCachedEvent(stdin, stdout, tool string, version int) CachedEvent {
	return CachedEvent{
		ProcessedEvent: preprocess.ProcessedEvent{
			Event: store.Event{
				Stdin:  stdin,
				Stdout: stdout,
			},
			ProcessedStdout: stdout,
			ToolName:        tool,
			RepeatCount:     1,
		},
		CacheVersion: version,
	}
}

func TestAppendAndRead_RoundTrip(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-cache-roundtrip"
	createSessionDir(t, home, sid)

	events := []CachedEvent{
		makeCachedEvent("nmap -sV target", "22/tcp open ssh", "nmap", currentCacheVersion),
		makeCachedEvent("whoami", "root", "whoami", currentCacheVersion),
		makeCachedEvent("cat /etc/passwd", "root:x:0:0:root", "cat", currentCacheVersion),
	}

	for _, ce := range events {
		if err := AppendCached(sid, ce); err != nil {
			t.Fatalf("AppendCached: %v", err)
		}
	}

	got, err := ReadCached(sid)
	if err != nil {
		t.Fatalf("ReadCached: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadCached = %d events, want 3", len(got))
	}

	if got[0].Stdin != "nmap -sV target" {
		t.Errorf("event[0].Stdin = %q, want %q", got[0].Stdin, "nmap -sV target")
	}
	if got[0].ToolName != "nmap" {
		t.Errorf("event[0].ToolName = %q, want %q", got[0].ToolName, "nmap")
	}
	if got[1].ProcessedStdout != "root" {
		t.Errorf("event[1].ProcessedStdout = %q, want %q", got[1].ProcessedStdout, "root")
	}
}

func TestReadCached_NoFile(t *testing.T) {
	setupTestHome(t)

	got, err := ReadCached("nonexistent-session")
	if err != nil {
		t.Fatalf("ReadCached(nonexistent) error: %v", err)
	}
	if got != nil {
		t.Errorf("ReadCached(nonexistent) = %v, want nil", got)
	}
}

func TestIsStale_VersionMismatch(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-cache-stale-version"
	createSessionDir(t, home, sid)

	// Write with old version
	ce := makeCachedEvent("cmd", "out", "cmd", currentCacheVersion-1)
	if err := AppendCached(sid, ce); err != nil {
		t.Fatal(err)
	}

	if !IsStale(sid, 1) {
		t.Error("IsStale should return true for version mismatch")
	}
}

func TestIsStale_CountMismatch(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-cache-stale-count"
	createSessionDir(t, home, sid)

	ce := makeCachedEvent("cmd", "out", "cmd", currentCacheVersion)
	if err := AppendCached(sid, ce); err != nil {
		t.Fatal(err)
	}

	// Cache has 1 event, but raw has 5
	if !IsStale(sid, 5) {
		t.Error("IsStale should return true when cached count != raw count")
	}
}

func TestIsStale_Fresh(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-cache-fresh"
	createSessionDir(t, home, sid)

	for i := range 3 {
		ce := makeCachedEvent("cmd-"+string(rune('a'+i)), "out", "cmd", currentCacheVersion)
		if err := AppendCached(sid, ce); err != nil {
			t.Fatal(err)
		}
	}

	if IsStale(sid, 3) {
		t.Error("IsStale should return false when cache is fresh")
	}
}

func TestIsStale_NoCache(t *testing.T) {
	setupTestHome(t)

	if !IsStale("nonexistent-session", 1) {
		t.Error("IsStale should return true for nonexistent cache")
	}
}

func TestCountCached(t *testing.T) {
	home := setupTestHome(t)
	sid := "test-cache-count"
	createSessionDir(t, home, sid)

	for i := range 7 {
		ce := makeCachedEvent("cmd-"+string(rune('a'+i)), "out", "cmd", currentCacheVersion)
		if err := AppendCached(sid, ce); err != nil {
			t.Fatal(err)
		}
	}

	count := CountCached(sid)
	if count != 7 {
		t.Errorf("CountCached = %d, want 7", count)
	}
}

func TestCountCached_NoFile(t *testing.T) {
	setupTestHome(t)

	count := CountCached("nonexistent-session")
	if count != 0 {
		t.Errorf("CountCached(nonexistent) = %d, want 0", count)
	}
}

func TestExtractProcessed(t *testing.T) {
	cached := []CachedEvent{
		makeCachedEvent("nmap target", "22/tcp open", "nmap", currentCacheVersion),
		makeCachedEvent("whoami", "root", "whoami", currentCacheVersion),
	}

	processed := ExtractProcessed(cached)
	if len(processed) != 2 {
		t.Fatalf("ExtractProcessed = %d events, want 2", len(processed))
	}
	if processed[0].Stdin != "nmap target" {
		t.Errorf("processed[0].Stdin = %q, want %q", processed[0].Stdin, "nmap target")
	}
	if processed[1].ToolName != "whoami" {
		t.Errorf("processed[1].ToolName = %q, want %q", processed[1].ToolName, "whoami")
	}
}

func TestExtractProcessed_Empty(t *testing.T) {
	processed := ExtractProcessed(nil)
	if len(processed) != 0 {
		t.Errorf("ExtractProcessed(nil) = %d events, want 0", len(processed))
	}
}

func TestCachePath(t *testing.T) {
	setupTestHome(t)

	path := CachePath("sid-123")
	if filepath.Base(path) != "cache.jsonl" {
		t.Errorf("CachePath base = %q, want cache.jsonl", filepath.Base(path))
	}
	if !filepath.IsAbs(path) {
		t.Errorf("CachePath should be absolute, got %q", path)
	}
}
