package capture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/perxibes/termdossier/internal/cache"
	"github.com/perxibes/termdossier/internal/session"
	"github.com/perxibes/termdossier/internal/store"
)

// bashRCTemplate is sourced via bash --rcfile.
// Uses trap DEBUG + PROMPT_COMMAND since bash has no native preexec/precmd.
// Emits OSC 7770 markers for PTY-level output capture by the Go process.
const bashRCTemplate = `
[ -f "$HOME/.bashrc" ] && source "$HOME/.bashrc"

export TERMDOSSIER_SESSION_ID="{{ .SessionID }}"
export TERMDOSSIER_TERMINAL_ID="{{ .TerminalID }}"

_td_cmd=""
_td_start=0
_td_cwd=""
_td_in_prompt=0
_td_count=0
_td_max="${TERMDOSSIER_MAX_EVENTS:-10000}"

_td_debug_trap() {
    [[ "$_td_in_prompt" == "1" ]] && return
    case "$BASH_COMMAND" in
        _td_*) return ;;
    esac
    if [[ -z "$_td_cmd" ]]; then
        _td_cmd="$BASH_COMMAND"
        _td_start=$(date +%s%3N)
        _td_cwd="$PWD"
        printf '\033]7770;S\007'
    fi
}

_td_precmd() {
    local _td_exit=$?
    _td_in_prompt=1
    if [[ -n "$_td_cmd" ]]; then
        _td_count=$(( _td_count + 1 ))
        if [[ "$_td_count" -le "$_td_max" ]]; then
            local _td_end
            _td_end=$(date +%s%3N)
            local _td_dur=$(( _td_end - _td_start ))
            printf '\033]7770;E;%s;%s;%s;%s\007' \
                "$_td_exit" "$_td_dur" "$_td_cwd" \
                "$(printf '%s' "$_td_cmd" | base64 -w0)"
        elif [[ "$_td_count" -eq "$(( _td_max + 1 ))" ]]; then
            printf '\n[termdossier] Max events (%s) reached. Recording paused.\n' "$_td_max" >&2
        fi
        _td_cmd=""
    fi
    _td_in_prompt=0
}

trap '_td_debug_trap' DEBUG
PROMPT_COMMAND="_td_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
`

// zshRCTemplate is placed in a temp ZDOTDIR and loaded automatically by zsh.
// Uses add-zsh-hook so existing preexec/precmd hooks (oh-my-zsh, starship, etc.) are preserved.
// TERMDOSSIER_ORIG_ZDOTDIR is set by the Go process to the user's real dotfile directory.
// Emits OSC 7770 markers for PTY-level output capture by the Go process.
const zshRCTemplate = `
if [ -f "$TERMDOSSIER_ORIG_ZDOTDIR/.zshrc" ]; then
    ZDOTDIR="$TERMDOSSIER_ORIG_ZDOTDIR"
    source "$TERMDOSSIER_ORIG_ZDOTDIR/.zshrc"
fi

export TERMDOSSIER_SESSION_ID="{{ .SessionID }}"
export TERMDOSSIER_TERMINAL_ID="{{ .TerminalID }}"

_td_cmd=""
_td_start=0
_td_cwd=""
_td_count=0
_td_max="${TERMDOSSIER_MAX_EVENTS:-10000}"

_td_preexec() {
    _td_cmd="$1"
    _td_start=$(date +%s%3N)
    _td_cwd="$PWD"
    printf '\033]7770;S\007'
}

_td_precmd() {
    local _td_exit=$?
    if [[ -n "$_td_cmd" ]]; then
        _td_count=$(( _td_count + 1 ))
        if [[ "$_td_count" -le "$_td_max" ]]; then
            local _td_end
            _td_end=$(date +%s%3N)
            local _td_dur=$(( _td_end - _td_start ))
            printf '\033]7770;E;%s;%s;%s;%s\007' \
                "$_td_exit" "$_td_dur" "$_td_cwd" \
                "$(printf '%s' "$_td_cmd" | base64 -w0)"
        elif [[ "$_td_count" -eq "$(( _td_max + 1 ))" ]]; then
            printf '\n[termdossier] Max events (%s) reached. Recording paused.\n' "$_td_max" >&2
        fi
        _td_cmd=""
    fi
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec _td_preexec
add-zsh-hook precmd  _td_precmd
`

// zshEnvTemplate loads the user's real .zshenv (PATH, pyenv, nvm, etc.)
const zshEnvTemplate = `
if [ -f "$TERMDOSSIER_ORIG_ZDOTDIR/.zshenv" ]; then
    source "$TERMDOSSIER_ORIG_ZDOTDIR/.zshenv"
fi
`

// zshProfileTemplate loads the user's real .zprofile (login shell config)
const zshProfileTemplate = `
if [ -f "$TERMDOSSIER_ORIG_ZDOTDIR/.zprofile" ]; then
    source "$TERMDOSSIER_ORIG_ZDOTDIR/.zprofile"
fi
`

type shellConfig struct {
	cmd     *exec.Cmd
	cleanup func()
}

// maxCaptureBytes limits captured output per command to prevent unbounded memory use.
const maxCaptureBytes = 256 * 1024

// markerPrefix is the OSC 7770 escape sequence prefix used to delimit command output.
var markerPrefix = []byte("\x1b]7770;")

// ptyCapture reads PTY output, detects OSC 7770 markers emitted by shell hooks,
// captures command output between start/end markers, and creates events directly.
type ptyCapture struct {
	ptmx       *os.File
	sessionID  string
	terminalID string
	capturing  bool
	captureBuf bytes.Buffer
}

// run reads from the PTY in a loop, processes markers, and forwards output to stdout.
func (pc *ptyCapture) run() {
	buf := make([]byte, 4096)
	var pending []byte

	for {
		n, err := pc.ptmx.Read(buf)
		if n > 0 {
			var data []byte
			if len(pending) > 0 {
				data = make([]byte, len(pending)+n)
				copy(data, pending)
				copy(data[len(pending):], buf[:n])
				pending = nil
			} else {
				data = buf[:n]
			}
			pending = pc.process(data)
		}
		if err != nil {
			if len(pending) > 0 {
				os.Stdout.Write(pending) //nolint
			}
			break
		}
	}
}

// process scans data for OSC 7770 markers, strips them, captures output between
// start/end markers, and forwards non-marker bytes to stdout.
// Returns any trailing bytes that may be a partial marker (to prepend to next read).
func (pc *ptyCapture) process(data []byte) []byte {
	i := 0
	for i < len(data) {
		escIdx := bytes.IndexByte(data[i:], '\x1b')
		if escIdx == -1 {
			pc.output(data[i:])
			return nil
		}

		// Output bytes before ESC.
		if escIdx > 0 {
			pc.output(data[i : i+escIdx])
		}
		i += escIdx
		remaining := data[i:]

		// Check if remaining bytes could be the start of a marker prefix.
		if len(remaining) < len(markerPrefix) {
			if bytes.HasPrefix(markerPrefix, remaining) {
				// Partial prefix match — save for next read.
				return append([]byte(nil), remaining...)
			}
			// Not a marker, output ESC and continue.
			pc.output(data[i : i+1])
			i++
			continue
		}

		if !bytes.HasPrefix(remaining, markerPrefix) {
			// Not our marker, output ESC and continue.
			pc.output(data[i : i+1])
			i++
			continue
		}

		// Marker prefix matched — find terminating BEL (\x07).
		belIdx := bytes.IndexByte(remaining, '\x07')
		if belIdx == -1 {
			// BEL not found yet — save partial marker for next read.
			return append([]byte(nil), remaining...)
		}

		// Full marker found — extract content between prefix and BEL.
		markerContent := remaining[len(markerPrefix):belIdx]
		pc.handleMarker(markerContent)
		i += belIdx + 1
	}
	return nil
}

// output writes bytes to stdout and, if capturing, appends to the capture buffer.
func (pc *ptyCapture) output(b []byte) {
	os.Stdout.Write(b) //nolint
	if pc.capturing {
		remaining := maxCaptureBytes - pc.captureBuf.Len()
		if remaining <= 0 {
			return
		}
		if len(b) > remaining {
			pc.captureBuf.Write(b[:remaining])
		} else {
			pc.captureBuf.Write(b)
		}
	}
}

// handleMarker processes a parsed OSC 7770 marker.
// "S" = start capture, "E;exit;dur;cwd;cmd_b64" = end capture + store event.
func (pc *ptyCapture) handleMarker(content []byte) {
	if len(content) == 1 && content[0] == 'S' {
		pc.capturing = true
		pc.captureBuf.Reset()
		return
	}

	if len(content) > 2 && content[0] == 'E' && content[1] == ';' {
		pc.capturing = false

		parts := strings.SplitN(string(content[2:]), ";", 4)
		if len(parts) < 4 {
			pc.captureBuf.Reset()
			return
		}

		exitCode, _ := strconv.Atoi(parts[0])
		duration, _ := strconv.ParseInt(parts[1], 10, 64)
		cwd := parts[2]

		cmdBytes, err := base64.StdEncoding.DecodeString(parts[3])
		if err != nil {
			pc.captureBuf.Reset()
			return
		}

		event := store.Event{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			SessionID:  pc.sessionID,
			TerminalID: pc.terminalID,
			CWD:        cwd,
			Stdin:      string(cmdBytes),
			Stdout:     pc.captureBuf.String(),
			Stderr:     "",
			ExitCode:   exitCode,
			DurationMS: duration,
		}

		go store.AppendEvent(pc.sessionID, event)     //nolint
		go cache.ProcessAndCache(pc.sessionID, event) //nolint

		pc.captureBuf.Reset()
	}
}

// Start spawns a PTY shell with recording hooks and blocks until it exits.
func Start(sessionID, sessionDir string, maxDuration time.Duration) error {
	terminalID := uuid.New().String()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cfg, err := prepareShell(shell, sessionID, terminalID)
	if err != nil {
		return err
	}
	defer cfg.cleanup()

	ptmx, err := pty.Start(cfg.cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer ptmx.Close()

	session.SetPID(sessionID, cfg.cmd.Process.Pid) //nolint

	// Auto-close session after maxDuration to prevent unbounded recording.
	if maxDuration > 0 {
		timer := time.AfterFunc(maxDuration, func() {
			fmt.Fprintf(os.Stderr, "\n[termdossier] Max session duration (%s) reached. Closing session.\n", maxDuration)
			cfg.cmd.Process.Signal(syscall.SIGHUP) //nolint
		})
		defer timer.Stop()
	}

	// Forward SIGWINCH → PTY resize.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			pty.InheritSize(os.Stdin, ptmx) //nolint
		}
	}()
	sigCh <- syscall.SIGWINCH
	defer signal.Stop(sigCh)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint

	go io.Copy(ptmx, os.Stdin) //nolint

	// Capture PTY output, detect markers, and create events.
	pc := &ptyCapture{
		ptmx:       ptmx,
		sessionID:  sessionID,
		terminalID: terminalID,
	}
	pc.run()

	return nil
}

func prepareShell(shell, sessionID, terminalID string) (*shellConfig, error) {
	data := map[string]string{
		"SessionID":  sessionID,
		"TerminalID": terminalID,
	}

	base := filepath.Base(shell)
	switch {
	case strings.HasPrefix(base, "zsh"):
		return prepareZsh(shell, data)
	default:
		return prepareBash(shell, data)
	}
}

func prepareBash(shell string, data map[string]string) (*shellConfig, error) {
	rc, err := renderToFile("termdossier-rc-*.bash", bashRCTemplate, data)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(shell, "--rcfile", rc)
	cmd.Env = os.Environ()
	return &shellConfig{
		cmd:     cmd,
		cleanup: func() { os.Remove(rc) },
	}, nil
}

func prepareZsh(shell string, data map[string]string) (*shellConfig, error) {
	dir, err := os.MkdirTemp("", "termdossier-zdotdir-*")
	if err != nil {
		return nil, fmt.Errorf("create zdotdir: %w", err)
	}

	rc := filepath.Join(dir, ".zshrc")
	if err := renderToPath(rc, zshRCTemplate, data); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	if err := renderToPath(filepath.Join(dir, ".zshenv"), zshEnvTemplate, data); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	if err := renderToPath(filepath.Join(dir, ".zprofile"), zshProfileTemplate, data); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	origZdotdir := os.Getenv("ZDOTDIR")
	if origZdotdir == "" {
		origZdotdir, _ = os.UserHomeDir()
	}

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(),
		"ZDOTDIR="+dir,
		"TERMDOSSIER_ORIG_ZDOTDIR="+origZdotdir,
	)
	return &shellConfig{
		cmd:     cmd,
		cleanup: func() { os.RemoveAll(dir) },
	}, nil
}

func renderToFile(pattern, tmplStr string, data map[string]string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()
	if err := render(f, tmplStr, data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func renderToPath(path, tmplStr string, data map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	defer f.Close()
	return render(f, tmplStr, data)
}

func render(w io.Writer, tmplStr string, data map[string]string) error {
	tmpl, err := template.New("rc").Parse(tmplStr)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}
