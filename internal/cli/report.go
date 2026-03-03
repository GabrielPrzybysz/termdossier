package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/perxibes/termdossier/internal/cache"
	"github.com/perxibes/termdossier/internal/detect"
	"github.com/perxibes/termdossier/internal/filter"
	"github.com/perxibes/termdossier/internal/llm"
	"github.com/perxibes/termdossier/internal/preprocess"
	"github.com/perxibes/termdossier/internal/prompt"
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
		templateName, _ := cmd.Flags().GetString("template")
		chunkSize, _ := cmd.Flags().GetInt("chunk-size")

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

		// Auto-detect template when not explicitly set
		if !cmd.Flags().Changed("template") {
			result := detect.Detect(events)
			if result.Confidence >= 0.3 {
				templateName = string(result.Type)
				fmt.Fprintf(os.Stderr, "Auto-detected session type: %s (confidence: %.0f%%)\n",
					templateName, result.Confidence*100)
				for _, r := range result.Reasons {
					fmt.Fprintf(os.Stderr, "  - %s\n", r)
				}
			}
		}

		tmpl, err := prompt.Get(templateName)
		if err != nil {
			return fmt.Errorf("unknown template %q (available: %s)", templateName, strings.Join(prompt.List(), ", "))
		}

		// Try using cached preprocessing results
		var processed []preprocess.ProcessedEvent
		if !cache.IsStale(sessionID, len(events)) {
			cached, cerr := cache.ReadCached(sessionID)
			if cerr == nil && cached != nil {
				processed = preprocess.Dedup(cache.ExtractProcessed(cached))
				fmt.Fprintf(os.Stderr, "Using cached preprocessing (%d events)\n", len(processed))
			}
		}
		if processed == nil {
			processed = preprocess.Pipeline(events)
		}

		fmt.Printf("Generating report for session %s (%d commands, template: %s)...\n",
			sessionID, len(processed), tmpl.Name)

		provider, err := llm.NewOllama(meta.Model)
		if err != nil {
			return fmt.Errorf("init LLM: %w", err)
		}
		defer provider.Shutdown()

		output, err := report.Generate(provider, meta, processed, context, tmpl, chunkSize)
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
	reportCmd.Flags().String("template", prompt.Default(), "Report template ("+strings.Join(prompt.List(), ", ")+")")
	reportCmd.Flags().Int("chunk-size", 10, "Max commands per chunk (0 to disable chunking)")
}
