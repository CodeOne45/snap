package cli

import (
	"encoding/json"
	"fmt"
	"path"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/hookrunner"
	"github.com/CodeOne45/snap/internal/adapters/prompt"
	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/recipe"
	"github.com/CodeOne45/snap/internal/core/tokens"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

// addJSON is snap add --json's wire shape, mirroring snap new --json: the
// Plan plus the stats a bare Plan doesn't carry.
type addJSON struct {
	Plan  plan.Plan  `json:"plan"`
	Stats plan.Stats `json:"stats"`
}

func newAddCmd(deps func() (Deps, error)) *cobra.Command {
	var varFlags []string
	var dryRun, jsonOut, yes bool

	cmd := &cobra.Command{
		Use:   "add <recipe> [target-dir]",
		Short: "Apply a recipe to an existing project",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipeName := args[0]
			targetDir := "."
			if len(args) == 2 {
				targetDir = args[1]
			}

			vars, err := parseVarFlags(varFlags)
			if err != nil {
				return err
			}

			d, err := deps()
			if err != nil {
				return internalErrorf("snap add: %w", err)
			}

			var prompter ports.Prompter
			if !yes {
				prompter = prompt.New(cmd.InOrStdin(), cmd.OutOrStdout())
			}

			p, err := usecase.Add(usecase.AddParams{
				Store:     d.RecipeStore,
				FS:        d.FS,
				Recipe:    recipeName,
				TargetDir: targetDir,
				Vars:      vars,
				Prompter:  prompter,
				AppliedAt: time.Now().UTC(),
			})
			if err != nil {
				return err
			}

			switch {
			case jsonOut:
				data, err := json.MarshalIndent(addJSON{Plan: p, Stats: plan.ComputeStats(p, tokens.Default)}, "", "  ")
				if err != nil {
					return internalErrorf("snap add: %w", err)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			case dryRun:
				return plan.DryRun(cmd.OutOrStdout(), p)
			default:
				if len(p.Ops) == 0 {
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s is already applied to %s; nothing to do.\n", recipeName, targetDir)
					if err != nil {
						return internalErrorf("snap add: %w", err)
					}
					return nil
				}

				p, err = gateRecipeHooks(cmd, d, recipeName, p, yes)
				if err != nil {
					return err
				}
				ex := executor.New(d.FS)
				if len(p.Hooks()) > 0 {
					ex = executor.NewWithHooks(d.FS, hookrunner.New())
				}
				if err := ex.ApplyRecipe(p); err != nil {
					return err
				}

				st := plan.ComputeStats(p, tokens.Default)
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatReceipt(st)); err != nil {
					return internalErrorf("snap add: %w", err)
				}
				// Usage logging is best-effort telemetry, never a reason to
				// fail an apply that already succeeded.
				_ = usagelog.Append(usagelog.Entry{
					Recipe: recipeName, Stats: st, Time: time.Now().UTC(), Source: "cli",
				})
				return nil
			}
		},
	}

	cmd.Flags().StringArrayVar(&varFlags, "var", nil, "set a recipe variable: --var key=value (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan as JSON without writing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "fail instead of prompting for missing variables (no TTY assumed); also skips hook confirmation (hooks are not run)")
	return cmd
}

// gateRecipeHooks mirrors gateHooks for a recipe's post-apply hooks,
// tracking approval under the same per-user hook-approvals store keyed by
// "name@version".
func gateRecipeHooks(cmd *cobra.Command, d Deps, recipeName string, p plan.Plan, yes bool) (plan.Plan, error) {
	hooks := p.Hooks()
	if len(hooks) == 0 {
		return p, nil
	}

	version, err := recipeVersion(d, recipeName)
	if err != nil {
		return plan.Plan{}, internalErrorf("snap add: %w", err)
	}
	approvals := newHookApprovals(d.FS, path.Dir(d.RecipeStore.Root()))
	key := hookApprovalKey(recipeName, version)

	out := cmd.OutOrStdout()
	if approvals.IsApproved(key) {
		return p, nil
	}

	if yes {
		if _, err := fmt.Fprintln(out, "Hooks skipped (--yes; rerun without it to approve them once)"); err != nil {
			return plan.Plan{}, internalErrorf("snap add: %w", err)
		}
		return p.WithoutHooks(), nil
	}

	msg := fmt.Sprintf("This recipe has %d post-apply hook(s) that will run:\n", len(hooks))
	for _, h := range hooks {
		msg += "  " + h.Run + "\n"
	}
	msg += "Run them?"

	prompter := prompt.New(cmd.InOrStdin(), out)
	ok, err := prompter.Confirm(msg, false)
	if err != nil {
		return plan.Plan{}, internalErrorf("snap add: %w", err)
	}
	if !ok {
		if _, err := fmt.Fprintln(out, "Hooks declined"); err != nil {
			return plan.Plan{}, internalErrorf("snap add: %w", err)
		}
		return p.WithoutHooks(), nil
	}
	if err := approvals.Approve(key); err != nil {
		return plan.Plan{}, internalErrorf("snap add: %w", err)
	}
	return p, nil
}

func recipeVersion(d Deps, recipeName string) (string, error) {
	root, err := d.RecipeStore.Path(recipeName)
	if err != nil {
		return "", err
	}
	data, err := d.FS.ReadFile(path.Join(root, recipe.ManifestFile))
	if err != nil {
		return "", err
	}
	r, err := recipe.ParseManifest(data)
	if err != nil {
		return "", err
	}
	return r.Version, nil
}
