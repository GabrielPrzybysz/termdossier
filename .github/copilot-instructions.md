# Project Guidelines

## Overview

**termdossier** is a Linux-only CLI tool that records terminal sessions (PTY capture) and generates structured technical reports using a local LLM (Ollama/llama.cpp). No cloud APIs. No passive monitoring. No root required.

See `terminal-command-recorder-spec.md` for the full design. MVP implementation is in progress — Go, module `github.com/perxibes/termdossier`.

## Architecture

```
CLI Layer → Session Manager → PTY Capture Engine → Event Store (JSONL)
  → Preprocessing/Filtering → LLM Provider Interface → Report Generator
```

Core components:
- **CLI Layer** — command parsing (`start`, `stop`, `report`)
- **Session Manager** — UUID-based sessions, multi-terminal state, `~/.termdossier/sessions/{id}/`
- **Capture Engine** — PTY shell wrapper intercepting stdin/stdout/stderr
- **Event Store** — append-only JSONL with file locking for concurrent writes
- **Preprocessing** — configurable command filtering + regex redaction (passwords, secrets)
- **LLM Provider** — adapter interface over Ollama (primary) and llama.cpp (secondary); localhost-only, auto-bootstrap
- **Report Generator** — builder pattern producing Markdown (HTML/PDF via pandoc optional)

**Session layout:**
```
~/.termdossier/sessions/{session_id}/
├── meta.json      # model name, version, session metadata
├── events.jsonl   # append-only event log
└── report.md      # generated report
```

**Event schema:**
```json
{ "timestamp": "ISO8601", "session_id": "uuid", "terminal_id": "uuid",
  "cwd": "/path", "stdin": "cmd", "stdout": "...", "stderr": "...",
  "exit_code": 0, "duration_ms": 1523 }
```

## Code Style

**Go** — module `github.com/perxibes/termdossier`. Follow standard Go conventions (`gofmt`, idiomatic error wrapping with `fmt.Errorf("context: %w", err)`).

Package layout:
- `cmd/termdossier/` — entrypoint
- `internal/cli/` — cobra commands (`start`, `stop`, `report`, hidden `_record`)
- `internal/session/` — UUID creation, meta.json, active-session pointer, PID tracking
- `internal/store/` — append-only JSONL (`Event` struct, `AppendEvent`, `ReadEvents`)
- `internal/capture/` — PTY engine (`creack/pty`), bash rcfile injection, `golang.org/x/term` raw mode
- `internal/filter/` — trivial-command removal, regex redaction (runs before LLM)
- `internal/llm/` — `Provider` interface + `Ollama` adapter (HTTP to `localhost:11434`)
- `internal/report/` — markdown builder via text/template prompt → LLM → `report.md`

Key external deps: `github.com/spf13/cobra`, `github.com/creack/pty`, `github.com/google/uuid`, `golang.org/x/term`.

## Build and Test

```bash
# Download dependencies
go mod tidy

# Build
go build -o termdossier ./cmd/termdossier

# Run
./termdossier start
./termdossier report --context "Pentest internal network"

# Tests (unit — no PTY or Ollama required)
go test ./internal/filter/... ./internal/store/...
```

## Project Conventions

- Linux-only; no cross-platform abstractions needed
- All LLM inference is localhost-only (Ollama default port 11434); auto-start and auto-pull models if missing
- Storage is append-only; never mutate existing JSONL events
- Redaction runs before any LLM processing — regex patterns for passwords (`(?i)password\s*=\s*\S+`), AWS keys (`AKIA[0-9A-Z]{16}`), and user-defined patterns
- Fail gracefully if LLM is unavailable; do not crash the recorder
- File locking required for concurrent multi-terminal writes

## Security

- Redaction engine must run before data reaches the LLM
- No external network calls; bind only to 127.0.0.1
- Sensitive command filtering is user-configurable (YAML)
- Encrypted session storage is a planned Phase 3 feature — do not design Phase 1 storage in a way that blocks adding encryption later
