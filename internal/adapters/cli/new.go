package cli

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/CodeOne45/snap/internal/adapters/gitcache"
	"github.com/CodeOne45/snap/internal/adapters/hookrunner"
	"github.com/CodeOne45/snap/internal/adapters/prompt"
	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/remote"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/core/tokens"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

// newJSON is snap new --json's wire shape (M19): the Plan plus the stats
// a bare Plan doesn't carry.
type newJSON struct {
	Plan  plan.Plan  `json:"plan"`
	Stats plan.Stats `json:"stats"`
}

func newNewCmd(deps func() (Deps, error)) *cobra.Command {
	var varFlags, withFlags, withoutFlags []string
	var dryRun, jsonOut, yes, force bool

	cmd := &cobra.Command{
		Use:   "new <template> <target-dir>",
		Short: "Generate a new project from a template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateName, targetDir := args[0], args[1]

			vars, err := parseVarFlags(varFlags)
			if err != nil {
				return err
			}
			if _, ok := vars["name"]; !ok {
				vars["name"] = filepath.Base(targetDir)
			}
			toggles, err := parseToggleFlags(withFlags, withoutFlags)
			if err != nil {
				return err
			}

			d, err := deps()
			if err != nil {
				return internalErrorf("snap new: %w", err)
			}

			source, err := resolveRemoteTemplate(&d, templateName)
			if err != nil {
				return err
			}

			var prompter ports.Prompter
			if !yes {
				prompter = prompt.New(cmd.InOrStdin(), cmd.OutOrStdout())
			}

			p, err := usecase.New(usecase.NewParams{
				Store:       d.Store,
				FS:          d.FS,
				Template:    templateName,
				TargetDir:   targetDir,
				Vars:        vars,
				Toggles:     toggles,
				Prompter:    prompter,
				GeneratedAt: time.Now().UTC(),
				Source:      source,
			})
			if err != nil {
				return err
			}

			switch {
			case jsonOut:
				data, err := json.MarshalIndent(newJSON{Plan: p, Stats: plan.ComputeStats(p, tokens.Default)}, "", "  ")
				if err != nil {
					return internalErrorf("snap new: %w", err)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			case dryRun:
				return plan.DryRun(cmd.OutOrStdout(), p)
			default:
				if force {
					if err := d.FS.RemoveAll(targetDir); err != nil {
						return internalErrorf("snap new: --force: %w", err)
					}
				}
				p, err = gateHooks(cmd, d, templateName, p, yes)
				if err != nil {
					return err
				}
				ex := executor.New(d.FS)
				if len(p.Hooks()) > 0 {
					ex = executor.NewWithHooks(d.FS, hookrunner.New())
				}
				if err := ex.Apply(p); err != nil {
					return err
				}

				st := plan.ComputeStats(p, tokens.Default)
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), formatReceipt(st)); err != nil {
					return internalErrorf("snap new: %w", err)
				}
				// Usage logging is best-effort telemetry, never a reason to
				// fail a generation that already succeeded.
				_ = usagelog.Append(usagelog.Entry{
					Template: templateName, Stats: st, Time: time.Now().UTC(), Source: "cli",
				})
				return nil
			}
		},
	}

	cmd.Flags().StringArrayVar(&varFlags, "var", nil, "set a template variable: --var key=value (repeatable)")
	cmd.Flags().StringArrayVar(&withFlags, "with", nil, "enable an [[optional]] block by name (repeatable)")
	cmd.Flags().StringArrayVar(&withoutFlags, "without", nil, "disable an [[optional]] block by name (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan without writing anything")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print the plan as JSON without writing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "fail instead of prompting for missing variables (no TTY assumed); also skips hook confirmation (hooks are not run)")
	cmd.Flags().BoolVar(&force, "force", false, "remove an existing non-empty target directory first")
	return cmd
}

// resolveRemoteTemplate checks whether templateName is a remote reference
// (gh:user/repo, a git URL, optionally @pinned). If so, it ensures the
// pinned (or latest) commit is cached locally — fetching from the network
// only when necessary — points d.Store at that one directory, and returns
// the "<url>@<resolved commit>" string for .snap/origin.toml's source
// field. Returns "" unchanged if templateName is a plain local template
// name.
func resolveRemoteTemplate(d *Deps, templateName string) (string, error) {
	ref, ok := remote.ParseRef(templateName)
	if !ok {
		return "", nil
	}
	cacheRoot := path.Join(path.Dir(d.Store.Root()), "cache")
	dir, commit, err := gitcache.New(cacheRoot).Resolve(ref)
	if err != nil {
		return "", fmt.Errorf("snap new: resolve %s: %w", templateName, err)
	}
	d.Store = gitcache.SingleTemplateStore{Dir: dir}
	return ref.URL + "@" + commit, nil
}

// parseToggleFlags builds a toggle map from --with/--without, rejecting a
// name given to both.
func parseToggleFlags(with, without []string) (map[string]bool, error) {
	out := make(map[string]bool, len(with)+len(without))
	for _, name := range with {
		out[name] = true
	}
	for _, name := range without {
		if v, ok := out[name]; ok && v {
			return nil, fmt.Errorf("snap new: %q given to both --with and --without", name)
		}
		out[name] = false
	}
	return out, nil
}

// gateHooks decides whether p's hooks (if any) actually run: already
// approved for this exact template@version -> keep them; otherwise confirm
// (interactive) or skip (--yes, since there's no one to ask). A declined or
// skipped plan has its hooks stripped via plan.Plan.WithoutHooks — the rest
// of the plan still applies.
func gateHooks(cmd *cobra.Command, d Deps, templateName string, p plan.Plan, yes bool) (plan.Plan, error) {
	hooks := p.Hooks()
	if len(hooks) == 0 {
		return p, nil
	}

	version, err := templateVersion(d, templateName)
	if err != nil {
		return plan.Plan{}, internalErrorf("snap new: %w", err)
	}
	approvals := newHookApprovals(d.FS, path.Dir(d.Store.Root()))
	key := hookApprovalKey(templateName, version)

	out := cmd.OutOrStdout()
	if approvals.IsApproved(key) {
		return p, nil
	}

	if yes {
		if _, err := fmt.Fprintln(out, "Hooks skipped (--yes; rerun without it to approve them once)"); err != nil {
			return plan.Plan{}, internalErrorf("snap new: %w", err)
		}
		return p.WithoutHooks(), nil
	}

	msg := fmt.Sprintf("This template has %d post-generate hook(s) that will run:\n", len(hooks))
	for _, h := range hooks {
		msg += "  " + h.Run + "\n"
	}
	msg += "Run them?"

	prompter := prompt.New(cmd.InOrStdin(), out)
	ok, err := prompter.Confirm(msg, false)
	if err != nil {
		return plan.Plan{}, internalErrorf("snap new: %w", err)
	}
	if !ok {
		if _, err := fmt.Fprintln(out, "Hooks declined"); err != nil {
			return plan.Plan{}, internalErrorf("snap new: %w", err)
		}
		return p.WithoutHooks(), nil
	}
	if err := approvals.Approve(key); err != nil {
		return plan.Plan{}, internalErrorf("snap new: %w", err)
	}
	return p, nil
}

func templateVersion(d Deps, templateName string) (string, error) {
	root, err := d.Store.Path(templateName)
	if err != nil {
		return "", err
	}
	data, err := d.FS.ReadFile(path.Join(root, template.ManifestFile))
	if err != nil {
		return "", err
	}
	m, err := template.ParseManifest(data)
	if err != nil {
		return "", err
	}
	return m.Version, nil
}

func parseVarFlags(vals []string) (map[string]string, error) {
	out := make(map[string]string, len(vals))
	for _, v := range vals {
		key, value, ok := strings.Cut(v, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("snap new: invalid --var %q, want key=value", v)
		}
		out[key] = value
	}
	return out, nil
}
