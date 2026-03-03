package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/perxibes/termdossier/internal/store"
)

// recordCmd is a hidden subcommand called by the shell hooks to append events.
var recordCmd = &cobra.Command{
	Use:    "_record",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionID, _ := cmd.Flags().GetString("session-id")
		terminalID, _ := cmd.Flags().GetString("terminal-id")
		cwd, _ := cmd.Flags().GetString("cwd")
		command, _ := cmd.Flags().GetString("cmd")
		exitCodeStr, _ := cmd.Flags().GetString("exit-code")
		durationStr, _ := cmd.Flags().GetString("duration-ms")

		exitCode, _ := strconv.Atoi(exitCodeStr)
		duration, _ := strconv.ParseInt(durationStr, 10, 64)

		event := store.Event{
			Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
			SessionID:  sessionID,
			TerminalID: terminalID,
			CWD:        cwd,
			Stdin:      command,
			Stdout:     "",
			Stderr:     "",
			ExitCode:   exitCode,
			DurationMS: duration,
		}

		return store.AppendEvent(sessionID, event)
	},
}

func init() {
	recordCmd.Flags().String("session-id", "", "")
	recordCmd.Flags().String("terminal-id", "", "")
	recordCmd.Flags().String("cwd", "", "")
	recordCmd.Flags().String("cmd", "", "")
	recordCmd.Flags().String("exit-code", "0", "")
	recordCmd.Flags().String("duration-ms", "0", "")
}
