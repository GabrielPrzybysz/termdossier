# Terminal Command Recorder & AI Report Generator

## Technical Architecture & Implementation Specification

------------------------------------------------------------------------

## 1. Project Overview

This project is a Linux-only utility that:

-   Records terminal sessions when explicitly activated
-   Captures `stdin`, `stdout`, `stderr`, exit codes, metadata, and
    timing
-   Supports multiple concurrent terminals within the same session
-   Stores events in structured JSONL format (append-only)
-   Uses a locally running LLM to generate structured technical reports
-   Allows contextual input (e.g., "Pentest report", "Debugging session
    analysis")
-   Runs fully offline and free (local LLM only)

The system must be production-grade, modular, secure by default, and
extensible.

------------------------------------------------------------------------

## 2. Core Design Principles

1.  Explicit activation --- no passive monitoring.
2.  Event-sourced architecture --- append-only log.
3.  Multi-terminal aware.
4.  Local-first --- no cloud API calls.
5.  Pluggable LLM backend.
6.  Secure by default (automatic redaction support).
7.  Clean separation of concerns.

------------------------------------------------------------------------

## 3. High-Level Architecture

CLI Layer\
↓\
Session Manager\
↓\
PTY Capture Engine\
↓\
Event Store (JSONL)\
↓\
Preprocessing + Filtering\
↓\
LLM Provider Interface\
↓\
Report Generator

------------------------------------------------------------------------

## 4. Functional Requirements

### 4.1 Recording

Start recording:

    record-command start

Stop recording:

    record-command stop

Generate report:

    record-command report --context "Pentest internal network"

------------------------------------------------------------------------

### 4.2 Multi-Terminal Support

-   Each session has a UUID.
-   Each terminal instance has its own `terminal_id`.
-   All terminals write to the same session directory.
-   Writes must be concurrency-safe (file locking).

------------------------------------------------------------------------

### 4.3 Stored Data Format

Events must be stored in JSONL format:

``` json
{
  "timestamp": "ISO8601",
  "session_id": "uuid",
  "terminal_id": "uuid",
  "cwd": "/current/path",
  "stdin": "command executed",
  "stdout": "output text",
  "stderr": "error output",
  "exit_code": 0,
  "duration_ms": 1523
}
```

Session directory layout:

    ~/.record-command/sessions/{session_id}/
        meta.json
        events.jsonl
        report.md

------------------------------------------------------------------------

## 5. Technical Components

### 5.1 CLI Layer

Responsibilities:

-   Command parsing
-   Session orchestration
-   LLM invocation
-   Report generation trigger

Pattern: Command Pattern

------------------------------------------------------------------------

### 5.2 Session Manager

Responsibilities:

-   Generate session UUID
-   Maintain active session state
-   Coordinate multi-terminal writes
-   Manage session metadata

------------------------------------------------------------------------

### 5.3 Capture Engine

Must use a PTY-based wrapper.

The utility must:

-   Spawn a pseudo-terminal
-   Launch user shell inside PTY
-   Intercept stdin, stdout, stderr
-   Capture exit code and duration

Do not rely on shell history.

Preferred implementation language:

-   Go (recommended)
-   Rust (alternative)

------------------------------------------------------------------------

### 5.4 Event Store

-   Append-only JSONL file
-   File locking required
-   No overwrites
-   Crash-safe writes

Future upgrade path: SQLite (optional)

------------------------------------------------------------------------

### 5.5 Preprocessing & Filtering

Before sending to LLM:

-   Remove trivial commands (configurable)
-   Ignore common typos
-   Remove commands like cd, clear, history

Example config:

``` yaml
ignore_commands:
  - cd
  - clear
  - history

redact_patterns:
  - '(?i)password\s*=\s*\S+'
  - 'AKIA[0-9A-Z]{16}'
```

Pattern: Strategy Pattern

------------------------------------------------------------------------

## 6. LLM Integration

Must use a local LLM runtime.

Primary backend: - Ollama

Alternative: - llama.cpp

------------------------------------------------------------------------

### 6.1 LLM Provider Interface

Example abstraction:

    interface LLMProvider {
        EnsureRunning() error
        EnsureModel(modelName string) error
        Generate(prompt string) (string, error)
    }

Patterns: - Adapter - Dependency Inversion

------------------------------------------------------------------------

### 6.2 Bootstrapping Behavior

1.  Check if runtime reachable.
2.  If not, start automatically.
3.  Verify model availability.
4.  Pull model if missing.
5.  Run inference locally (localhost only).

------------------------------------------------------------------------

### 6.3 Prompt Construction

Must include:

-   User context
-   Structured command list
-   Execution metadata
-   Instructions to:
    -   Ignore trivial errors
    -   Explain flags
    -   Identify security implications
    -   Produce structured report

------------------------------------------------------------------------

## 7. Report Structure

Default output: Markdown

    # Technical Report

    ## Context
    ## Executive Summary
    ## Timeline of Actions
    ## Detailed Command Analysis
    ## Observations
    ## Security Considerations
    ## Conclusion

Optional export: - HTML - PDF (via pandoc)

Pattern: Builder Pattern

------------------------------------------------------------------------

## 8. Security Requirements

Mandatory safeguards:

1.  Regex-based redaction engine.
2.  No external API calls.
3.  Localhost-only LLM binding.
4.  Optional encrypted session storage (future).
5.  Model + version logged in metadata.

Example:

``` json
{
  "llm_model": "mistral:7b-q4",
  "llm_version": "hash"
}
```

------------------------------------------------------------------------

## 9. Non-Functional Requirements

-   Handle long sessions.
-   Non-blocking terminal I/O.
-   Memory efficient.
-   Linux compatible.
-   Fully offline.
-   Deterministic storage format.
-   Configurable and extensible.

------------------------------------------------------------------------

## 10. Development Phases

### Phase 1 -- MVP

-   PTY capture
-   JSONL storage
-   Ollama integration
-   Basic markdown report

### Phase 2

-   Multi-terminal support
-   Configurable filters
-   Redaction engine

### Phase 3

-   Encrypted sessions
-   TUI interface
-   Replay mode

### Phase 4

-   Incremental summarization
-   Timeline visualization
-   Session diffing

------------------------------------------------------------------------

## 11. Constraints

-   Linux only
-   No cloud APIs
-   No background monitoring
-   No root requirement
-   Must fail safely if LLM unavailable

------------------------------------------------------------------------

## 12. Expected Outcome

A modular, extensible, local-first terminal session recorder that
transforms raw CLI activity into structured, professional-grade
technical reports using a locally managed LLM runtime.
