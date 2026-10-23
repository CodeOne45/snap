// Package cli wires Cobra commands to the core use cases. It contains no
// business logic itself.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/hookrunner"
	"github.com/CodeOne45/snap/internal/adapters/tui"
)

// version is overridden at build time via -ldflags.
var version = "dev"

// NewRootCmd builds the snap root command. Run with no subcommand, it
// launches the interactive TUI wizard (M12) instead of printing help.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "snap",
		Short:   "Capture existing projects as templates and generate new ones from them.",
		Version: version,
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			d, err := defaultDeps()
			if err != nil {
				return internalErrorf("snap: %w", err)
			}
			if err := tui.Run(d.FS, d.Store, hookrunner.New()); err != nil {
				return internalErrorf("snap: %w", err)
			}
			return nil
		},
	}
	cmd.AddCommand(newNewCmd(defaultDeps), newListCmd(defaultDeps), newShowCmd(defaultDeps), newCaptureCmd(defaultDeps), newMCPCmd(defaultDeps), newTemplateCmd(defaultDeps), newUpdateCmd(defaultDeps), newTestCmd(defaultDeps), newStatusCmd(defaultDeps), newStatsCmd(), newAddCmd(defaultDeps), newRecipesCmd(defaultDeps))
	return cmd
}
