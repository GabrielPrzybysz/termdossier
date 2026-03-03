package capture

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"text/template"
	"strings"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"golang.org/x/term"

	"github.com/perxibes/termdossier/internal/session"
)

// rcfileTemplate is sourced by the recording shell.
// It injects hooks that call "termdossier _record" after every interactive command.
const rcfileTemplate = `
# Load the user's real bashrc first so the shell feels normal.
[ -f "$HOME/.bashrc" ] && source "$HOME/.bashrc"

export TERMDOSSIER_SESSION_ID="{{ .SessionID }}"
export TERMDOSSIER_TERMINAL_ID="{{ .TerminalID }}"
export TERMDOSSIER_BIN="{{ .BinaryPath }}"

_td_cmd=""
_td_start=0
_td_cwd=""
_td_in_prompt=0

_td_debug_trap() {
    # Skip if we are executing PROMPT_COMMAND or our own helpers.
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
        _td_cmd=""
    fi
    _td_in_prompt=0
}

trap '_td_debug_trap' DEBUG
PROMPT_COMMAND="_td_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
`

// Start spawns a PTY shell with recording hooks and blocks until it exits.
func Start(sessionID, sessionDir, binaryPath string) error {
	terminalID := uuid.New().String()

	// Write the temp rcfile.
	rcFile, err := writeRCFile(sessionID, terminalID, binaryPath)
	if err != nil {
		return err
	}
	defer os.Remove(rcFile)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell, "--rcfile", rcFile)
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer ptmx.Close()

	// Persist PID so "termdossier stop" can signal the process.
	session.SetPID(sessionID, cmd.Process.Pid) //nolint

	// Forward SIGWINCH → PTY resize.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	go func() {
		for range sigCh {
			pty.InheritSize(os.Stdin, ptmx) //nolint
		}
	}()
	sigCh <- syscall.SIGWINCH // Set initial size.
	defer signal.Stop(sigCh)

	// Switch stdin to raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint

	// Bidirectional copy: user ↔ PTY.
	go io.Copy(ptmx, os.Stdin) //nolint
	io.Copy(os.Stdout, ptmx)   //nolint

	return nil
}

func writeRCFile(sessionID, terminalID, binaryPath string) (string, error) {
	tmpl, err := template.New("rc").Parse(rcfileTemplate)
	if err != nil {
		return "", err
	}

	f, err := os.CreateTemp("", "termdossier-rc-*.bash")
	if err != nil {
		return "", fmt.Errorf("create rcfile: %w", err)
	}
	defer f.Close()

	var buf strings.Builder
	err = tmpl.Execute(&buf, map[string]string{
		"SessionID":  sessionID,
		"TerminalID": terminalID,
		"BinaryPath": binaryPath,
	})
	if err != nil {
		return "", err
	}

	if _, err := f.WriteString(buf.String()); err != nil {
		return "", err
	}

	return f.Name(), nil
}
