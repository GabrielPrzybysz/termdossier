package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/perxibes/termdossier/internal/filter"
	"github.com/perxibes/termdossier/internal/llm"
	"github.com/perxibes/termdossier/internal/report"
	"github.com/perxibes/termdossier/internal/session"
	"github.com/perxibes/termdossier/internal/store"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a report for the last session",
	RunE: func(cmd *cobra.Command, args []string) error {
		context, _ := cmd.Flags().GetString("context")
		sessionID, _ := cmd.Flags().GetString("session")

		if sessionID == "" {
			var err error
			sessionID, err = session.GetActive()
			if err != nil {
				return fmt.Errorf("no active session; use --session to specify one")
			}
		}

		meta, err := session.ReadMeta(sessionID)
		if err != nil {
			return fmt.Errorf("read session metadata: %w", err)
		}

		events, err := store.ReadEvents(sessionID)
		if err != nil {
			return fmt.Errorf("read events: %w", err)
		}

		events = filter.Apply(events)
		if len(events) == 0 {
			return fmt.Errorf("no commands recorded in this session")
		}

		fmt.Printf("Generating report for session %s (%d commands)...\n", sessionID, len(events))

		provider, err := llm.NewOllama(meta.Model)
		if err != nil {
			return fmt.Errorf("init LLM: %w", err)
		}
		defer provider.Shutdown()

		output, err := report.Generate(provider, meta, events, context)
		if err != nil {
			return fmt.Errorf("generate report: %w", err)
		}

		fmt.Printf("Report saved to: %s\n", output)
		return nil
	},
}

func init() {
	reportCmd.Flags().String("context", "", "Context for the report (e.g. 'Pentest internal network')")
	reportCmd.Flags().String("session", "", "Session ID (defaults to last active session)")
}
