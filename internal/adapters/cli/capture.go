package cli

import (
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/prompt"
	"github.com/CodeOne45/snap/internal/core/capture"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
)

func newCaptureCmd(deps func() (Deps, error)) *cobra.Command {
	var asName string
	var yes, dryRun, jsonOut, force bool
	var excludes []string

	cmd := &cobra.Command{
		Use:   "capture <source-dir>",
		Short: "Capture a project as a reusable template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceDir := args[0]
			if asName == "" {
				return fmt.Errorf("snap capture: --as is required")
			}

			d, err := deps()
			if err != nil {
				return internalErrorf("snap capture: %w", err)
			}
			targetDir := path.Join(d.Store.Root(), asName, usecase.InitialVersion)

			out := cmd.OutOrStdout()
			accept := newAcceptFunc(cmd, yes)

			res, err := usecase.Capture(usecase.CaptureParams{
				FS:        d.FS,
				SourceDir: sourceDir,
				TargetDir: targetDir,
				AsName:    asName,
				Options:   capture.Options{ExtraExcludes: excludes},
				Accept:    accept,
			})
			if err != nil {
				return err
			}

			if len(res.Scan.Ignored) > 0 {
				if _, err := fmt.Fprintf(out, "Ignored: %s\n", strings.Join(res.Scan.Ignored, ", ")); err != nil {
					return internalErrorf("snap capture: %w", err)
				}
			}

			switch {
			case jsonOut:
				data, err := plan.ToJSON(res.Plan)
				if err != nil {
					return internalErrorf("snap capture: %w", err)
				}
				if _, err := fmt.Fprintln(out, string(data)); err != nil {
					return internalErrorf("snap capture: %w", err)
				}
				return nil
			case dryRun:
				return plan.DryRun(out, res.Plan)
			default:
				if force {
					if err := d.FS.RemoveAll(targetDir); err != nil {
						return internalErrorf("snap capture: --force: %w", err)
					}
				}
				if err := executor.New(d.FS).Apply(res.Plan); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(out, "✓ Template %q saved (%d variables)\n", asName, len(res.Candidates)); err != nil {
					return internalErrorf("snap capture: %w", err)
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&asName, "as", "", "name to save the template as (required)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "accept all detected variables with default names, non-interactively")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan as JSON without writing anything")
	cmd.Flags().BoolVar(&force, "force", false, "remove an existing non-empty template of the same name first")
	cmd.Flags().StringArrayVar(&excludes, "exclude", nil, "additional gitignore-style exclude pattern (repeatable)")
	return cmd
}

// newAcceptFunc builds a usecase.Accept: --yes accepts every candidate with
// its default name; otherwise it confirms and lets the user rename
// interactively via the real terminal.
func newAcceptFunc(cmd *cobra.Command, yes bool) usecase.Accept {
	if yes {
		return func(_ capture.Candidate, defaultName string) (bool, string, error) {
			return true, defaultName, nil
		}
	}

	prompter := prompt.New(cmd.InOrStdin(), cmd.OutOrStdout())
	return func(c capture.Candidate, defaultName string) (bool, string, error) {
		msg := fmt.Sprintf("Found %s (%d occurrences across %d files) → variable?",
			strings.Join(c.RawForms, ", "), c.PathHits+c.ContentHits, c.FileCount)
		ok, err := prompter.Confirm(msg, true)
		if err != nil || !ok {
			return false, "", err
		}
		name, err := prompter.Ask("Variable name", defaultName)
		return true, name, err
	}
}
