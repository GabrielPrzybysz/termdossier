package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/perxibes/termdossier/internal/session"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the active recording session",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := session.GetActive()
		if err != nil {
			return fmt.Errorf("no active session: %w", err)
		}

		if err := session.Kill(id); err != nil {
			return fmt.Errorf("stop session: %w", err)
		}

		fmt.Printf("Session %s stopped.\n", id)
		return nil
	},
}
