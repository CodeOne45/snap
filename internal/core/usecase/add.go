package usecase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/CodeOne45/snap/internal/core/deps"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/core/recipe"
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/ports"
)

// AddParams are the inputs to Add.
type AddParams struct {
	Store ports.Store
	FS    ports.FS
	// Recipe is either a bare recipe name (resolved to its latest version)
	// or "name@version", mirroring New's Template field.
	Recipe string
	// TargetDir is an already-existing project; unlike New, Add never
	// creates it.
	TargetDir string
	// Vars are explicit values (e.g. from --var); they take precedence
	// over a variable's default_from and over prompting.
	Vars map[string]string
	// Prompter fills in a variable that has neither an explicit value nor
	// a resolvable default_from. nil means non-interactive: such a
	// variable is an error.
	Prompter ports.Prompter
	// AppliedAt is recorded in the target's .snap/origin.toml, in a new
	// [[recipes]] entry. Required: a zero value is an error.
	AppliedAt time.Time
	// Source is recorded in the new [[recipes]] entry. Defaults to
	// "local" when empty.
	Source string
}

// Add loads a recipe, resolves its variables against the target project,
// and builds the Plan to apply it: new files (rendered with the same
// placeholder engine as a template), dependency additions, env entries,
// and post-apply hooks. It performs no writes; callers apply the result
// with executor.ApplyRecipe, preview it with plan.DryRun, or serialize it
// with plan.ToJSON.
//
// All-or-nothing: if applies_to isn't satisfied, or any file, dependency,
// or env edit would conflict with content already at TargetDir, Add
// returns an error listing every conflict and no Plan. Re-applying a
// recipe whose files, dependencies, and env entries are already exactly
// present produces an empty Plan (zero Ops), not an error.
func Add(p AddParams) (plan.Plan, error) {
	if p.AppliedAt.IsZero() {
		return plan.Plan{}, fmt.Errorf("usecase: add: AppliedAt must be set")
	}

	root, err := resolveStoreRoot(p.Store, p.Recipe)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	data, err := p.FS.ReadFile(path.Join(root, recipe.ManifestFile))
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: read manifest: %w", err)
	}
	r, err := recipe.ParseManifest(data)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	if err := checkAppliesTo(p.FS, p.TargetDir, r.AppliesTo); err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	origin, originRaw, originExisted, err := loadTargetOrigin(p.FS, p.TargetDir)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	vars, err := resolveRecipeVariables(r.Variables, p.Vars, origin, p.Prompter)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	b := plan.NewBuilder(p.TargetDir)
	var conflicts []string

	filesChanged, err := planRecipeFiles(b, p.FS, root, p.TargetDir, vars, &conflicts)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}
	depsChanged, err := planRecipeDependencies(b, p.FS, p.TargetDir, r.Dependencies, &conflicts)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}
	envChanged, err := planRecipeEnv(b, p.FS, p.TargetDir, r.Env, &conflicts)
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}

	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return plan.Plan{}, fmt.Errorf("usecase: add: conflicts:\n  %s", strings.Join(conflicts, "\n  "))
	}

	if !filesChanged && !depsChanged && !envChanged {
		return b.Build(), nil
	}

	for _, run := range r.Hooks {
		if err := b.RunHook(run); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
		}
	}

	source := p.Source
	if source == "" {
		source = "local"
	}
	entryVars := make(map[string]any, len(vars))
	for k, v := range vars {
		entryVars[k] = v
	}
	origin.Recipes = append(origin.Recipes, provenance.RecipeEntry{
		Name: r.Name, Version: r.Version, Source: source, AppliedAt: p.AppliedAt, Variables: entryVars,
	})
	originTOML, err := origin.TOML()
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
	}
	if originExisted {
		if err := b.ModifyFile(provenance.File, sha256Hex(originRaw), originTOML, 0o644); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
		}
	} else {
		if err := b.WriteFile(provenance.File, originTOML, 0o644); err != nil {
			return plan.Plan{}, fmt.Errorf("usecase: add: %w", err)
		}
	}

	return b.Build(), nil
}

// checkAppliesTo returns an error naming every file in a.Files that's
// missing from targetDir — R9: a clear error, nothing written.
func checkAppliesTo(fsys ports.FS, targetDir string, a recipe.AppliesTo) error {
	var missing []string
	for _, f := range a.Files {
		if _, err := fsys.Stat(path.Join(targetDir, f)); err != nil {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("applies_to not satisfied: missing %s", strings.Join(missing, ", "))
	}
	return nil
}

// loadTargetOrigin reads targetDir's .snap/origin.toml. A project not
// generated by Snap has none at all — that's not an error, it's reported
// via existed=false so the caller knows to WriteFile (not ModifyFile) the
// rewritten origin.toml.
func loadTargetOrigin(fsys ports.FS, targetDir string) (origin provenance.Origin, raw []byte, existed bool, err error) {
	raw, err = fsys.ReadFile(path.Join(targetDir, provenance.File))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return provenance.Origin{}, nil, false, nil
		}
		return provenance.Origin{}, nil, false, fmt.Errorf("read %s: %w", provenance.File, err)
	}
	origin, err = provenance.ParseOrigin(raw)
	if err != nil {
		return provenance.Origin{}, nil, false, err
	}
	return origin, raw, true, nil
}

// resolveRecipeVariables determines the final value for every declared
// recipe variable. Precedence: an explicit value in provided, then
// default_from resolved against the target's own origin (currently only
// "project.<key>", read from origin.Variables), then an interactive
// prompt (if prompter is non-nil), else an error.
func resolveRecipeVariables(vars []recipe.Variable, provided map[string]string, origin provenance.Origin, prompter ports.Prompter) (render.Vars, error) {
	out := make(render.Vars, len(vars))
	for _, v := range vars {
		value, err := resolveRecipeVariable(v, provided, origin, prompter)
		if err != nil {
			return nil, err
		}
		out[v.Name] = value
	}
	return out, nil
}

func resolveRecipeVariable(v recipe.Variable, provided map[string]string, origin provenance.Origin, prompter ports.Prompter) (string, error) {
	if value, ok := provided[v.Name]; ok {
		return value, nil
	}
	if v.DefaultFrom != "" {
		if value, ok := resolveDefaultFrom(v.DefaultFrom, origin); ok {
			return value, nil
		}
	}
	if prompter != nil {
		message := v.Prompt
		if message == "" {
			message = v.Name
		}
		return prompter.Ask(message, "")
	}
	return "", fmt.Errorf("variable %q has no value, default_from match, or prompter", v.Name)
}

// resolveDefaultFrom resolves a "project.<key>" default_from path against
// the target's own recorded origin.Variables. Any other prefix doesn't
// resolve (returns ok=false) rather than erroring, so the caller falls
// through to prompting.
func resolveDefaultFrom(defaultFrom string, origin provenance.Origin) (string, bool) {
	prefix, key, ok := strings.Cut(defaultFrom, ".")
	if !ok || prefix != "project" {
		return "", false
	}
	value, ok := origin.Variables[key]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%v", value), true
}

// planRecipeFiles renders every file under root/files and adds a
// WriteFileOp for each one missing from targetDir. A path already present
// with byte-identical rendered content is a no-op (R6); present with
// different content is a conflict (R8) rather than an overwrite.
func planRecipeFiles(b *plan.Builder, fsys ports.FS, root, targetDir string, vars render.Vars, conflicts *[]string) (bool, error) {
	filesRoot := path.Join(root, recipe.FilesDir)
	if _, err := fsys.Stat(filesRoot); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat %s: %w", recipe.FilesDir, err)
	}

	files, err := walkAllFiles(fsys, filesRoot)
	if err != nil {
		return false, err
	}

	changed := false
	for _, f := range files {
		srcPath := path.Join(filesRoot, f)
		content, err := fsys.ReadFile(srcPath)
		if err != nil {
			return false, fmt.Errorf("read %q: %w", f, err)
		}
		fi, err := fsys.Stat(srcPath)
		if err != nil {
			return false, fmt.Errorf("stat %q: %w", f, err)
		}

		renderedContent, err := render.Content(content, vars)
		if err != nil {
			return false, fmt.Errorf("render content %q: %w", f, err)
		}
		renderedPath, err := render.Path(f, vars)
		if err != nil {
			return false, fmt.Errorf("render path %q: %w", f, err)
		}

		existing, err := fsys.ReadFile(path.Join(targetDir, renderedPath))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := b.WriteFile(renderedPath, renderedContent, fi.Mode); err != nil {
				return false, fmt.Errorf("%w", err)
			}
			changed = true
		case err != nil:
			return false, fmt.Errorf("read %q: %w", renderedPath, err)
		case bytes.Equal(existing, renderedContent):
			// Already present, identical: no-op.
		default:
			*conflicts = append(*conflicts, fmt.Sprintf("%s: already exists with different content", renderedPath))
		}
	}
	return changed, nil
}

// planRecipeDependencies adds a ModifyFileOp for every dependency whose
// edit actually changes its manifest file, via the matching
// internal/core/deps editor. A missing manifest file or a version
// conflict is reported through conflicts, not a hard error.
func planRecipeDependencies(b *plan.Builder, fsys ports.FS, targetDir string, dependencies []recipe.Dependency, conflicts *[]string) (bool, error) {
	changed := false
	for _, d := range dependencies {
		dep := deps.Dependency{Name: d.Name, Version: d.Version, Dev: d.Dev}

		relPath, edit, ok := dependencyEditor(fsys, targetDir, d.Ecosystem, dep)
		if !ok {
			*conflicts = append(*conflicts, fmt.Sprintf("%s dependency %q: no manifest file found in %s", d.Ecosystem, d.Name, targetDir))
			continue
		}

		c, conflict, err := planFileEdit(b, fsys, targetDir, relPath, false, edit)
		if err != nil {
			return false, err
		}
		if conflict != "" {
			*conflicts = append(*conflicts, conflict)
			continue
		}
		changed = changed || c
	}
	return changed, nil
}

// dependencyEditor returns the manifest path and edit function for d's
// ecosystem. ok is false only for "pip" when neither pyproject.toml nor
// requirements.txt exists in targetDir yet.
func dependencyEditor(fsys ports.FS, targetDir, ecosystem string, dep deps.Dependency) (relPath string, edit func([]byte) ([]byte, bool, error), ok bool) {
	switch ecosystem {
	case "npm":
		return "package.json", func(c []byte) ([]byte, bool, error) { return deps.AddPackageJSON(c, dep) }, true
	case "go":
		return "go.mod", func(c []byte) ([]byte, bool, error) { return deps.AddGoMod(c, dep) }, true
	case "cargo":
		return "Cargo.toml", func(c []byte) ([]byte, bool, error) { return deps.AddCargoToml(c, dep) }, true
	case "pip":
		manifest, found := resolvePipManifest(fsys, targetDir)
		if !found {
			return "", nil, false
		}
		if manifest == "pyproject.toml" {
			return manifest, func(c []byte) ([]byte, bool, error) { return deps.AddPyprojectToml(c, dep) }, true
		}
		return manifest, func(c []byte) ([]byte, bool, error) { return deps.AddRequirementsTxt(c, dep) }, true
	default:
		// recipe.ParseManifest already rejects any other ecosystem, so
		// this is unreachable in practice.
		return "", nil, false
	}
}

// resolvePipManifest prefers pyproject.toml (PEP 621) over requirements.txt
// when a target happens to have both.
func resolvePipManifest(fsys ports.FS, targetDir string) (string, bool) {
	if _, err := fsys.Stat(path.Join(targetDir, "pyproject.toml")); err == nil {
		return "pyproject.toml", true
	}
	if _, err := fsys.Stat(path.Join(targetDir, "requirements.txt")); err == nil {
		return "requirements.txt", true
	}
	return "", false
}

// planRecipeEnv adds an op for every env entry whose edit actually
// changes its file. Unlike a dependency's manifest, a missing env file is
// created fresh rather than treated as a conflict.
func planRecipeEnv(b *plan.Builder, fsys ports.FS, targetDir string, entries []recipe.EnvEntry, conflicts *[]string) (bool, error) {
	changed := false
	for _, e := range entries {
		key, value := e.Key, e.Value
		edit := func(content []byte) ([]byte, bool, error) { return deps.AddEnvEntry(content, key, value) }

		c, conflict, err := planFileEdit(b, fsys, targetDir, e.File, true, edit)
		if err != nil {
			return false, err
		}
		if conflict != "" {
			*conflicts = append(*conflicts, conflict)
			continue
		}
		changed = changed || c
	}
	return changed, nil
}

// planFileEdit applies edit to relPath's current content (empty if it
// doesn't exist and allowCreate is true) and adds the resulting op: a
// WriteFileOp for a brand-new file, a ModifyFileOp for an existing one
// whose content changed, or nothing when edit reports no change.
//
// A *deps.ConflictError from edit, or a missing file when !allowCreate,
// is returned as conflict text rather than err — the caller collects
// every conflict before failing the whole Plan. A non-conflict error from
// edit (e.g. malformed JSON) is returned as err and aborts immediately.
func planFileEdit(b *plan.Builder, fsys ports.FS, targetDir, relPath string, allowCreate bool, edit func([]byte) ([]byte, bool, error)) (changed bool, conflict string, err error) {
	targetPath := path.Join(targetDir, relPath)
	content, readErr := fsys.ReadFile(targetPath)
	existed := readErr == nil
	mode := fs.FileMode(0o644)

	switch {
	case existed:
		if fi, statErr := fsys.Stat(targetPath); statErr == nil {
			mode = fi.Mode
		}
	case errors.Is(readErr, fs.ErrNotExist):
		if !allowCreate {
			return false, fmt.Sprintf("%s: not found", relPath), nil
		}
		content = nil
	default:
		return false, "", fmt.Errorf("read %q: %w", relPath, readErr)
	}

	newContent, isChanged, editErr := edit(content)
	if editErr != nil {
		var ce *deps.ConflictError
		if errors.As(editErr, &ce) {
			return false, fmt.Sprintf("%s: %v", relPath, ce), nil
		}
		return false, "", fmt.Errorf("%s: %w", relPath, editErr)
	}
	if !isChanged {
		return false, "", nil
	}

	if existed {
		if err := b.ModifyFile(relPath, sha256Hex(content), newContent, mode); err != nil {
			return false, "", err
		}
	} else if err := b.WriteFile(relPath, newContent, mode); err != nil {
		return false, "", err
	}
	return true, "", nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
