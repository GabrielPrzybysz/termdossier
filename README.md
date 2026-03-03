# termdossier

Record terminal sessions and generate structured technical reports using a local LLM. Designed for penetration testers, CTF players, and anyone who wants automatic documentation of their terminal work.

## Features

- **PTY Capture** — Records commands, output, exit codes, and timing via shell hooks (bash and zsh)
- **Local LLM** — All inference runs locally via Ollama. No cloud APIs, no data leaves your machine
- **Auto-Detection** — Automatically detects session type (pentest, debug, general) and selects the appropriate report template
- **Smart Preprocessing** — Strips ANSI escapes, extracts key findings from tool output (nmap, gobuster, linpeas, etc.), redacts credentials
- **Chunked Reports** — Long sessions are automatically split into temporal chunks, summarized, then assembled into a final report
- **Caching** — Preprocessed output is cached incrementally during capture for faster report generation

## Requirements

- Linux
- Go 1.22+
- [Ollama](https://ollama.ai) (auto-started if not running)

## Installation

```bash
go install github.com/perxibes/termdossier/cmd/termdossier@latest
```

Or build from source:

```bash
git clone https://github.com/perxibes/termdossier.git
cd termdossier
go build -o termdossier ./cmd/termdossier
```

## Quick Start

```bash
# Start recording a session (spawns a new shell)
termdossier start

# Do your work...
nmap -sV target
gobuster dir -u http://target
cat /etc/passwd

# Exit the recording shell
exit

# Generate a report
termdossier report --context "HTB machine recon"
```

## CLI Reference

### `termdossier start`

Start a new recording session. Spawns an interactive shell with capture hooks.

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | `mistral` | Ollama model to use for report generation |
| `--max-duration` | `0` | Maximum capture duration (e.g. `2h`). 0 = unlimited |

### `termdossier stop`

Stop the active recording session by sending SIGTERM to the capture process.

### `termdossier report`

Generate a report from a recorded session.

| Flag | Default | Description |
|------|---------|-------------|
| `--context` | | Context for the report (e.g. "Pentest internal network") |
| `--session` | (active) | Session ID to generate report for |
| `--template` | (auto) | Report template: `pentest`, `educational`, `debug` |
| `--chunk-size` | `10` | Max commands per chunk. 0 to disable chunking |

When `--template` is not specified, termdossier auto-detects the session type based on the tools and commands used.

## Report Templates

| Template | Use Case |
|----------|----------|
| `pentest` | Penetration testing — executive summary, attack narrative, phase breakdown, credentials & loot, recommendations |
| `educational` | General sessions — learning objectives, command analysis |
| `debug` | Development/debugging — problem statement, investigation timeline, root cause analysis |

## Architecture

```
termdossier start → PTY shell + hooks → events.jsonl
termdossier report → filter → preprocess → detect → [chunk] → LLM → report.md
```

See [.github/copilot-instructions.md](.github/copilot-instructions.md) for the full architecture documentation.

## Data Storage

All data is stored locally in `~/.termdossier/sessions/{session_id}/`:

```
meta.json       # session metadata (model, timestamps)
events.jsonl    # raw captured events (append-only)
cache.jsonl     # preprocessed event cache
report.md       # generated report
```

## Security

- All LLM inference is localhost-only (Ollama on port 11434)
- Credentials and sensitive data are redacted before reaching the LLM
- No external network calls
- Session data is stored with restrictive file permissions (0600/0700)

## License

See [LICENSE](LICENSE) for details.
