# Project Guidelines

## Overview

**termdossier** is a Linux-only CLI tool that records terminal sessions via PTY capture and generates structured technical reports using a local LLM (Ollama). No cloud APIs. No passive monitoring. No root required.

Go module: `github.com/perxibes/termdossier` (Go 1.22+)

## Architecture

```
                          ┌────────────┐
                          │  CLI Layer │
                          │ (cobra)    │
                          └──────┬─────┘
            ┌────────────────────┼────────────────────┐
            │                    │                    │
     ┌──────▼──────┐    ┌───────▼───────┐    ┌───────▼────────┐
     │    start     │    │     stop      │    │    report       │
     │  (capture)   │    │  (kill PID)   │    │  (full pipeline)│
     └──────┬──────┘    └───────────────┘    └───────┬────────┘
            │                                        │
   ┌────────▼────────┐                    ┌──────────▼──────────┐
   │ Capture Engine  │                    │      Filter         │
   │ (PTY + hooks)   │                    │ (trivial + redact)  │
   └────────┬────────┘                    └──────────┬──────────┘
            │                                        │
   ┌────────▼────────┐                    ┌──────────▼──────────┐
   │  Event Store    │◄───────────────────│    Preprocess       │
   │  (JSONL)        │                    │ (ANSI/extract/trunc)│
   └────────┬────────┘                    └──────────┬──────────┘
            │                                        │
   ┌────────▼────────┐                    ┌──────────▼──────────┐
   │  Session Cache  │                    │   Auto-Detect       │
   │  (preprocessed) │                    │  (heuristics)       │
   └─────────────────┘                    └──────────┬──────────┘
                                                     │
                                          ┌──────────▼──────────┐
                                          │   Prompt Templates  │
                                          │ (pentest/edu/debug) │
                                          └──────────┬──────────┘
                                                     │
                                          ┌──────────▼──────────┐
                                          │  Chunk / Report     │
                                          │ (direct or chunked) │
                                          └──────────┬──────────┘
                                                     │
                                          ┌──────────▼──────────┐
                                          │   LLM Provider      │
                                          │  (Ollama streaming)  │
                                          └──────────┬──────────┘
                                                     │
                                               report.md
```

## Package Layout

### `cmd/termdossier/`
Entrypoint — calls `cli.Execute()`.

### `internal/cli/`
Cobra command definitions and flag parsing.

| Command | Flags | Description |
|---------|-------|-------------|
| `start` | `--model`, `--max-duration` | Create session, start PTY capture |
| `stop` | (none) | Send SIGTERM to active capture process |
| `report` | `--context`, `--session`, `--template`, `--chunk-size` | Generate LLM-powered report |
| `_record` | `--session-id`, `--terminal-id`, `--cwd`, `--cmd`, `--exit-code`, `--duration-ms` | Hidden — called by shell hooks |

### `internal/session/`
Session lifecycle management.

- `Meta` struct: `SessionID`, `StartedAt`, `Model`, `Version`
- `Create(model)` — UUID generation, `meta.json` write, sets active session
- `GetActive() / ClearActive()` — active session pointer (`~/.termdossier/active_session`)
- `SetPID(sid, pid) / Kill(sid)` — PID file tracking, SIGTERM signaling
- `ReadMeta(sid)` — deserialize `meta.json`
- `Dir(sid)` — returns `~/.termdossier/sessions/{sid}/`

### `internal/store/`
Append-only event persistence.

- `Event` struct: `Timestamp`, `SessionID`, `TerminalID`, `CWD`, `Stdin`, `Stdout`, `Stderr`, `ExitCode`, `DurationMS`
- `AppendEvent(sid, event)` — JSONL append with `syscall.Flock` exclusive lock
- `ReadEvents(sid)` — reads all events, skips malformed lines, 1MB scanner buffer

### `internal/capture/`
PTY capture engine. Spawns an interactive shell (bash or zsh) with injected hooks.

- OSC 7770 markers for command boundary detection
- Base64-encoded command metadata in markers
- Shell hooks: bash via custom rcfile (`PS0`/`PROMPT_COMMAND`), zsh via `precmd`/`preexec` in custom `ZDOTDIR`
- Max output: 256KB per command, 10,000 events per session
- Window resize forwarding (`SIGWINCH`)
- Concurrent event storage via goroutines

### `internal/filter/`
Pre-LLM data cleanup.

- `Apply(events)` — removes trivial commands (`cd`, `ls`, `pwd`, `clear`, `history`, `exit`, `logout`, `cls`, `ll`)
- Redaction patterns (run before LLM sees data):
  - `password|passwd|secret|token|key = value`
  - AWS access keys (`AKIA[0-9A-Z]{16}`)
  - Bearer tokens

### `internal/preprocess/`
Stdout preprocessing pipeline: `StripANSI → Tool Extraction → Truncation → Dedup`

**`ansi.go`** — `StripANSI(s)`: removes ANSI escapes, collapses `\r`-based progress bars, normalizes CRLF

**`tools.go`** — Tool-specific extractors:
- `ExtractBaseTool(cmd)` — strips sudo/time/nohup prefixes, returns base binary
- `LookupExtractor(cmd)` — command-based extractor lookup
- `LookupByContent(stdout)` — content signature detection for passthrough tools (cat, less, etc.)
- Registered extractors: `nmap` (open ports), `gobuster`/`feroxbuster`/`ffuf` (HTTP results), `linpeas` (CVE/SUID findings), `ps` (user-space processes), `nikto`, `git`
- `PassthroughTools` map: `cat`, `less`, `more`, `head`, `tail`, `bat`, `strings`

**`truncate.go`** — Per-tool byte limits:
- Default: 4096B. Overrides: nmap=2048, gobuster/feroxbuster/ffuf=1536, linpeas=4096, cat=4096, curl=2048, git=3072, find/grep=2048
- `Truncate(s, maxBytes)` — cuts at line boundary, appends `[...truncated N bytes...]`
- `LimitFor(tool)` — returns byte limit for tool

**`dedup.go`** — `Dedup(events)`: collapses consecutive identical `Stdin + ProcessedStdout` into one with `RepeatCount`

**`preprocess.go`** — `ProcessedEvent` struct (embeds `store.Event` + `ProcessedStdout`, `ToolName`, `RepeatCount`), `Pipeline(events)` orchestrator

### `internal/detect/`
Session type auto-detection via weighted heuristic scoring.

- `SessionType`: `"pentest"`, `"debug"`, `"educational"`
- `Detect(events) Result` — returns type, confidence (0-1), reasons
- Pentest signals: tool weights (nmap=0.3, sqlmap=0.4, msfconsole=0.5, ...), HTB IP patterns (`10.10.x.x`, `10.129.x.x`), reverse shell patterns
- Dev signals: tool weights (go=0.2, cargo=0.3, pytest=0.3, ...), command patterns (go build/test, npm test, git commit, docker build)
- Confidence formula: `score / (score + 1.0)`, minimum 0.3 to override default
- Tool deduplication: each tool only counted once per session

### `internal/prompt/`
Template registry for report generation prompts.

- `Template` struct: `Name`, `Description`, `System`, `User` (Go text/template syntax)
- `TemplateData`: `Context`, `SessionID`, `StartedAt`, `CommandCount`, `CommandList`
- `Register(t)`, `Get(name)`, `List()`, `Default()` ("educational")
- Built-in templates:
  - **pentest** — Executive Summary, Attack Narrative, Phase Breakdown, Credentials & Loot, Recommendations
  - **educational** — General session analysis with learning objectives
  - **debug** — Problem Statement, Investigation Timeline, Root Cause Analysis, Changes Made

### `internal/chunk/`
Intelligent session chunking for long sessions.

- `Config`: `MaxCommandsPerChunk` (10), `TemporalGap` (5min), `MinChunkSize` (3)
- `Split(events, cfg)` — temporal gap + max size splitting, small chunk merging
- `SummarizeChunk(provider, chunk)` / `SummarizeAll(provider, chunks)` — LLM summarization per chunk
- `BuildFinalPrompt(chunks, ...)` — assembles chunk summaries into final report prompt
- `BuildCommandList(events)` — formats events with markers

### `internal/cache/`
Session preprocessing cache (incremental, versioned).

- `CachedEvent`: embeds `ProcessedEvent` + `CacheVersion`
- `AppendCached(sid, ce)` — JSONL append with flock
- `ReadCached(sid)` / `CountCached(sid)` — read/count cached events
- `IsStale(sid, rawCount)` — checks version match and event count
- `ExtractProcessed(cached)` — strips cache metadata
- `ProcessAndCache(sid, event)` — capture-time single-event preprocessing
- `RebuildCache(sid)` — full rebuild from `events.jsonl`
- Cache file: `~/.termdossier/sessions/{id}/cache.jsonl`
- Version: `currentCacheVersion = 2` (bumped when preprocessing logic changes)

### `internal/report/`
Report generation orchestrator.

- `ChunkThreshold = 15` — event count threshold for chunked generation
- `Generate(provider, meta, events, context, tmpl, chunkSize)` — routes to direct or chunked
- `generateDirect(...)` — renders template, single LLM call
- `generateChunked(...)` — splits → summarizes chunks → final report from summaries
- `buildCommandList(events)` — formats events with exit codes, output markers, repeat counts
- Output: `~/.termdossier/sessions/{id}/report.md` with metadata header

### `internal/llm/`
Ollama LLM provider.

- `Provider` interface: `EnsureRunning()`, `EnsureModel(model)`, `Generate(system, user)`, `Shutdown()`
- `Ollama` struct: manages daemon lifecycle, model pulling, streaming chat API
- Auto-starts Ollama daemon if not running, auto-pulls model if missing
- Streaming NDJSON chat API (`localhost:11434/api/chat`)
- Retry logic (up to 2 retries on failure)
- `Shutdown()` — unloads model, stops daemon if we started it

## Data Flow

### Capture Flow
```
Shell hook → OSC 7770 marker → capture.handleMarker() →
  store.AppendEvent() (JSONL)
  cache.ProcessAndCache() (preprocessed JSONL, async)
```

### Report Flow
```
store.ReadEvents() → filter.Apply() → detect.Detect() →
  cache check (IsStale?) →
    YES: preprocess.Pipeline()
    NO:  cache.ReadCached() → preprocess.Dedup()
  → prompt.Get(template) → report.Generate() →
    <15 events: generateDirect() → LLM → report.md
    ≥15 events: chunk.Split() → chunk.SummarizeAll() → LLM → report.md
```

## File Formats

**`events.jsonl`** — One JSON object per line:
```json
{"timestamp":"2024-01-01T00:00:00Z","session_id":"uuid","terminal_id":"uuid","cwd":"/path","stdin":"cmd","stdout":"...","stderr":"...","exit_code":0,"duration_ms":123}
```

**`meta.json`** — Session metadata:
```json
{"session_id":"uuid","started_at":"2024-01-01T00:00:00Z","model":"mistral","version":"0.1.0"}
```

**`cache.jsonl`** — Preprocessed events with version:
```json
{"timestamp":"...","stdin":"...","processed_stdout":"...","tool_name":"nmap","repeat_count":1,"cache_version":2}
```

## Session Directory Structure
```
~/.termdossier/
├── active_session          # current session ID (plain text)
└── sessions/
    └── {session_id}/
        ├── meta.json       # session metadata
        ├── events.jsonl    # raw captured events
        ├── cache.jsonl     # preprocessed event cache
        ├── pid             # capture process PID
        └── report.md       # generated report
```

## Build and Test

```bash
go mod tidy
go build -o termdossier ./cmd/termdossier
go test ./internal/...
go vet ./...
```

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/creack/pty` | PTY spawning |
| `github.com/google/uuid` | Session/terminal IDs |
| `golang.org/x/term` | Raw terminal mode |

## Conventions

- Linux-only; no cross-platform abstractions
- All LLM inference is localhost-only (Ollama port 11434)
- Storage is append-only; never mutate existing JSONL events
- Redaction runs before any LLM processing
- File locking (`syscall.Flock`) for concurrent writes
- Cache is a derived artifact — always rebuildable from `events.jsonl`
- Template names match `detect.SessionType` values for auto-detection
