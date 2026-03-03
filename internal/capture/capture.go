package capture

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/perxibes/termdossier/internal/session"
)

// bashRCTemplate is sourced via bash --rcfile.
// Uses trap DEBUG + PROMPT_COMMAND since bash has no native preexec/precmd.
const bashRCTemplate = `
[ -f "$HOME/.bashrc" ] && source "$HOME/.bashrc"

export TERMDOSSIER_SESSION_ID="{{ .SessionID }}"
export TERMDOSSIER_TERMINAL_ID="{{ .TerminalID }}"
export TERMDOSSIER_BIN="{{ .BinaryPath }}"

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
    _td_cmd="$BASH_COMMAND"
    _td_start=$(date +%s%3N)
    _td_cwd="$PWD"
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
            "$TERMDOSSIER_BIN" _record \
                --session-id  "$TERMDOSSIER_SESSION_ID" \
                --terminal-id "$TERMDOSSIER_TERMINAL_ID" \
                --cwd         "$_td_cwd" \
                --cmd         "$_td_cmd" \
                --exit-code   "$_td_exit" \
                --duration-ms "$_td_dur" \
                >/dev/null 2>&1 &
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
const zshRCTemplate = `
if [ -f "$TERMDOSSIER_ORIG_ZDOTDIR/.zshrc" ]; then
    ZDOTDIR="$TERMDOSSIER_ORIG_ZDOTDIR"
    source "$TERMDOSSIER_ORIG_ZDOTDIR/.zshrc"
fi

export TERMDOSSIER_SESSION_ID="{{ .SessionID }}"
export TERMDOSSIER_TERMINAL_ID="{{ .TerminalID }}"
export TERMDOSSIER_BIN="{{ .BinaryPath }}"

_td_cmd=""
_td_start=0
_td_cwd=""
_td_count=0
_td_max="${TERMDOSSIER_MAX_EVENTS:-10000}"

_td_preexec() {
    _td_cmd="$1"
    _td_start=$(date +%s%3N)
    _td_cwd="$PWD"
}

_td_precmd() {
    local _td_exit=$?
    if [[ -n "$_td_cmd" ]]; then
        _td_count=$(( _td_count + 1 ))
        if [[ "$_td_count" -le "$_td_max" ]]; then
            local _td_end
            _td_end=$(date +%s%3N)
            local _td_dur=$(( _td_end - _td_start ))
            "$TERMDOSSIER_BIN" _record \
                --session-id  "$TERMDOSSIER_SESSION_ID" \
                --terminal-id "$TERMDOSSIER_TERMINAL_ID" \
                --cwd         "$_td_cwd" \
                --cmd         "$_td_cmd" \
                --exit-code   "$_td_exit" \
                --duration-ms "$_td_dur" \
                >/dev/null 2>&1 &
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

// Start spawns a PTY shell with recording hooks and blocks until it exits.
func Start(sessionID, sessionDir, binaryPath string, maxDuration time.Duration) error {
	terminalID := uuid.New().String()

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cfg, err := prepareShell(shell, sessionID, terminalID, binaryPath)
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
	io.Copy(os.Stdout, ptmx)   //nolint

	return nil
}

func prepareShell(shell, sessionID, terminalID, binaryPath string) (*shellConfig, error) {
	data := map[string]string{
		"SessionID":  sessionID,
		"TerminalID": terminalID,
		"BinaryPath": binaryPath,
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
