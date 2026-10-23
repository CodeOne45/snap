package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/hookrunner"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

// testJSON is snap test --json's wire shape: the Plan plus the check
// command a bare Plan doesn't carry.
type testJSON struct {
	Plan  plan.Plan `json:"plan"`
	Check string    `json:"check"`
}

func newTestCmd(deps func() (Deps, error)) *cobra.Command {
	var dryRun, jsonOut bool

	cmd := &cobra.Command{
		Use:   "test <template>",
		Short: "Generate a template with its declared sample variables and run its check command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName := args[0]

			d, err := deps()
			if err != nil {
				return internalErrorf("snap test: %w", err)
			}

			// scratchDir exists (os.MkdirTemp creates it); targetDir, one
			// level inside it, does not — executor.Apply's final rename
			// needs a destination that doesn't exist yet, which an
			// already-existing (if empty) directory isn't guaranteed to
			// satisfy on every filesystem.
			scratchDir, err := os.MkdirTemp("", "snap-test-*")
			if err != nil {
				return internalErrorf("snap test: create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(scratchDir) }()
			targetDir := filepath.Join(scratchDir, "project")

			res, err := usecase.Test(usecase.TestParams{
				Store:     d.Store,
				FS:        d.FS,
				Template:  templateName,
				TargetDir: filepath.ToSlash(targetDir),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch {
			case jsonOut:
				data, err := json.MarshalIndent(testJSON{Plan: res.Plan, Check: res.Check}, "", "  ")
				if err != nil {
					return internalErrorf("snap test: %w", err)
				}
				_, err = fmt.Fprintln(out, string(data))
				return err
			case dryRun:
				if err := plan.DryRun(out, res.Plan); err != nil {
					return internalErrorf("snap test: %w", err)
				}
				_, err := fmt.Fprintf(out, "Check: %s\n", res.Check)
				return err
			default:
				if err := executor.New(d.FS).Apply(res.Plan); err != nil {
					return err
				}
				result, runErr := hookrunner.New().Run(ports.Hook{Run: res.Check, Dir: targetDir})
				if runErr != nil {
					if _, err := fmt.Fprintf(out, "✗ check failed (exit %d):\n%s\n", result.ExitCode, result.Output); err != nil {
						return internalErrorf("snap test: %w", err)
					}
					return fmt.Errorf("snap test: check failed: %w", runErr)
				}
				if _, err := fmt.Fprintf(out, "✓ check passed\n%s", result.Output); err != nil {
					return internalErrorf("snap test: %w", err)
				}
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and check command without generating or running anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan and check command as JSON without generating or running anything")
	return cmd
}
