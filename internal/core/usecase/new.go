package usecase

import (
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// NewParams are the inputs to New.
type NewParams struct {
	Store     ports.Store
	FS        ports.FS
	Template  string
	TargetDir string
	// Vars are explicit values (e.g. from --var); they take precedence
	// over prompting and over defaults.
	Vars map[string]string
	// Prompter fills in variables missing from Vars that have no default,
	// and also resolves [[optional]] blocks missing from Toggles. nil means
	// non-interactive: a missing-default variable is an error; a
	// missing-from-Toggles optional block uses its declared Default.
	Prompter ports.Prompter
	// Toggles are explicit --with/--without (or MCP) overrides for
	// [[optional]] blocks; a block not listed here is resolved via
	// Prompter or its declared Default. Naming a block the template
	// doesn't declare is an error.
	Toggles map[string]bool
	// GeneratedAt is recorded in .snap/origin.toml. Required: a zero value
	// is an error rather than silently writing 0001-01-01T00:00:00Z. Callers
	// pass time.Now().UTC(); tests pass a fixed time for determinism.
	GeneratedAt time.Time
	// Source is recorded in .snap/origin.toml's "source" field. Defaults
	// to "local" when empty; a caller resolving a remote template (M11)
	// passes "<url>@<resolved commit>".
	Source string
}

// New loads a template, resolves its variables, renders every file and
// path, and builds the Plan to generate TargetDir. It performs no writes;
// callers apply the result with executor.Apply, preview it with
// plan.DryRun, or serialize it with plan.ToJSON.
func New(p NewParams) (plan.Plan, error) {
	if p.GeneratedAt.IsZero() {
		return plan.Plan{}, fmt.Errorf("usecase: new: GeneratedAt must be set")
	}

	root, err := resolveStoreRoot(p.Store, p.Template)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}

	data, err := p.FS.ReadFile(path.Join(root, template.ManifestFile))
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: read manifest: %w", err)
	}
	manifest, err := template.ParseManifest(data)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}

	vars, err := ResolveVariables(manifest.Variables, p.Vars, p.Prompter)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}

	toggles, err := ResolveToggles(manifest.Optional, p.Toggles, p.Prompter)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}

	files, dirs, err := walkTemplate(p.FS, root)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}
	files = filterOptionalPaths(files, manifest.Optional, toggles)
	dirs = filterOptionalPaths(dirs, manifest.Optional, toggles)

	b := plan.NewBuilder(p.TargetDir)
	for _, d := range dirs {
		rendered, err := render.Path(d, vars)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: render dir %q: %w", d, err)
		}
		if err := b.CreateDir(rendered); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
		}
	}
	for _, f := range files {
		srcPath := path.Join(root, f)
		content, err := p.FS.ReadFile(srcPath)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: read %q: %w", f, err)
		}
		fi, err := p.FS.Stat(srcPath)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: stat %q: %w", f, err)
		}

		renderedContent, err := render.Content(content, vars)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: render content %q: %w", f, err)
		}
		renderedPath, err := render.Path(f, vars)
		if err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: render path %q: %w", f, err)
		}
		if err := b.WriteFile(renderedPath, renderedContent, fi.Mode); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
		}
	}

	source := p.Source
	if source == "" {
		source = "local"
	}
	origin := provenance.Origin{
		Template:    manifest.Name,
		Version:     manifest.Version,
		Source:      source,
		GeneratedAt: p.GeneratedAt,
		Variables:   typedVariables(manifest.Variables, vars),
		Toggles:     toggles,
	}
	originTOML, err := origin.TOML()
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}
	// Written directly, not through render.Content: variable values come
	// from the user (e.g. --var description="uses {{handlebars}}") and must
	// never be re-interpreted as template syntax.
	if err := b.WriteFile(provenance.File, originTOML, 0o644); err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
	}

	// Hook commands are not rendered through the placeholder engine (they're
	// plain shell commands like "git init" — PLAN.md shows no templating
	// here); they only ever run once the calling adapter has confirmed
	// (CLI) or allowed (MCP's allow_hooks) it, and the executor runs them
	// last, after the target directory is committed.
	for _, run := range manifest.Hooks {
		if err := b.RunHook(run); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: new: %w", err)
		}
	}

	return b.Build(), nil
}

// typedVariables converts resolved string values back to the type each
// variable declares (e.g. int64 for a "int" variable), so origin.toml
// records port = 8080 rather than port = "8080".
func typedVariables(declared []template.Variable, resolved render.Vars) map[string]any {
	out := make(map[string]any, len(declared))
	for _, v := range declared {
		val := resolved[v.Name]
		if v.Type == template.TypeInt {
			if n, err := strconv.Atoi(val); err == nil {
				out[v.Name] = int64(n)
				continue
			}
		}
		out[v.Name] = val
	}
	return out
}
