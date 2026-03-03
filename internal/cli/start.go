package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/perxibes/termdossier/internal/capture"
	"github.com/perxibes/termdossier/internal/session"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new recording session",
	RunE: func(cmd *cobra.Command, args []string) error {
		model, _ := cmd.Flags().GetString("model")

		meta, err := session.Create(model)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Recording session %s\nType 'exit' to stop.\n\n", meta.SessionID)

		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve binary path: %w", err)
		}

		if err := capture.Start(meta.SessionID, session.Dir(meta.SessionID), binaryPath); err != nil {
			return fmt.Errorf("capture: %w", err)
		}

		fmt.Fprintf(os.Stderr, "\nSession ended. Run 'termdossier report' to generate a report.\n")
		return nil
	},
}

func init() {
	startCmd.Flags().String("model", "llama3", "Ollama model to use for report generation")
}
