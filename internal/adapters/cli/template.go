package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/semver"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
)

// newTemplateCmd groups template-management subcommands (currently just
// bump) under "snap template ...".
func newTemplateCmd(deps func() (Deps, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage template versions",
	}
	cmd.AddCommand(newTemplateBumpCmd(deps))
	return cmd
}

func newTemplateBumpCmd(deps func() (Deps, error)) *cobra.Command {
	var dryRun, jsonOut bool

	cmd := &cobra.Command{
		Use:   "bump <name> <major|minor|patch>",
		Short: "Create a new version of a template by bumping its latest version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, level := args[0], args[1]

			d, err := deps()
			if err != nil {
				return internalErrorf("snap template bump: %w", err)
			}

			res, err := usecase.Bump(usecase.BumpParams{
				Store:    d.Store,
				FS:       d.FS,
				Template: name,
				Level:    semver.Level(level),
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch {
			case jsonOut:
				data, err := plan.ToJSON(res.Plan)
				if err != nil {
					return internalErrorf("snap template bump: %w", err)
				}
				_, err = fmt.Fprintln(out, string(data))
				return err
			case dryRun:
				return plan.DryRun(out, res.Plan)
			default:
				if err := executor.New(d.FS).Apply(res.Plan); err != nil {
					return err
				}
				_, err := fmt.Fprintf(out, "✓ %s bumped: %s -> %s\n", name, res.OldVersion, res.NewVersion)
				return err
			}
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan as JSON without writing anything")
	return cmd
}
