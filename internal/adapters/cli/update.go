package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/prompt"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

// updateJSON is snap update --json's wire shape: the Plan (same tagged-
// union shape as every other --json command) plus the merge outcome a
// bare Plan can't express — which paths are unresolved conflicts, which
// were deleted, and which versions were involved.
type updateJSON struct {
	Plan       plan.Plan `json:"plan"`
	Conflicts  []string  `json:"conflicts,omitempty"`
	Deleted    []string  `json:"deleted,omitempty"`
	OldVersion string    `json:"old_version"`
	NewVersion string    `json:"new_version"`
}

func newUpdateCmd(deps func() (Deps, error)) *cobra.Command {
	var dryRun, jsonOut, yes bool

	cmd := &cobra.Command{
		Use:   "update <target-dir>",
		Short: "Update a generated project to its template's latest version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetDir := args[0]

			d, err := deps()
			if err != nil {
				return internalErrorf("snap update: %w", err)
			}

			var prompter ports.Prompter
			if !yes {
				prompter = prompt.New(cmd.InOrStdin(), cmd.OutOrStdout())
			}

			res, err := usecase.Update(usecase.UpdateParams{
				Store:       d.Store,
				FS:          d.FS,
				TargetDir:   targetDir,
				Prompter:    prompter,
				GeneratedAt: time.Now().UTC(),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch {
			case jsonOut:
				data, err := json.MarshalIndent(updateJSON{
					Plan: res.Plan, Conflicts: res.Conflicts, Deleted: res.Deleted,
					OldVersion: res.OldVersion, NewVersion: res.NewVersion,
				}, "", "  ")
				if err != nil {
					return internalErrorf("snap update: %w", err)
				}
				if _, err := fmt.Fprintln(out, string(data)); err != nil {
					return internalErrorf("snap update: %w", err)
				}
			case dryRun:
				if err := plan.DryRun(out, res.Plan); err != nil {
					return internalErrorf("snap update: %w", err)
				}
				if err := printUpdateSummary(out, res); err != nil {
					return err
				}
			default:
				if err := executor.New(d.FS).ApplyUpdate(res.Plan); err != nil {
					return err
				}
				if err := printUpdateSummary(out, res); err != nil {
					return err
				}
			}

			// A conflict is a normal, expected update outcome, not a
			// malformed invocation — but scripts/CI need a non-zero exit
			// to tell "clean" from "needs manual resolution" without
			// parsing output text, the same way `git merge` exits
			// non-zero on conflict.
			if len(res.Conflicts) > 0 {
				return fmt.Errorf("snap update: %d conflict(s) need manual resolution", len(res.Conflicts))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan and merge outcome as JSON without writing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "fail instead of prompting for a variable the new template version introduces (no TTY assumed)")
	return cmd
}

func printUpdateSummary(out io.Writer, res usecase.UpdateResult) error {
	if len(res.Deleted) > 0 {
		if _, err := fmt.Fprintln(out, "Deleted:"); err != nil {
			return internalErrorf("snap update: %w", err)
		}
		for _, p := range res.Deleted {
			if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
				return internalErrorf("snap update: %w", err)
			}
		}
	}
	if len(res.Conflicts) == 0 {
		_, err := fmt.Fprintf(out, "✓ Updated %s -> %s\n", res.OldVersion, res.NewVersion)
		return err
	}
	if _, err := fmt.Fprintf(out, "⚠ Updated with %d conflict(s) — origin.toml left at %s (resolve, then re-run):\n", len(res.Conflicts), res.OldVersion); err != nil {
		return internalErrorf("snap update: %w", err)
	}
	for _, p := range res.Conflicts {
		if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
			return internalErrorf("snap update: %w", err)
		}
	}
	return nil
}
