package usecase

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"time"

	"github.com/CodeOne45/snap/internal/core/merge"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/ports"
)

// UpdateParams are the inputs to Update.
type UpdateParams struct {
	Store ports.Store
	FS    ports.FS
	// TargetDir is the already-generated project to update. It must
	// already contain .snap/origin.toml (written by a prior New).
	TargetDir string
	// Prompter resolves a variable the latest template version
	// introduces that TargetDir's origin.toml has no recorded value for.
	// nil means non-interactive: such a variable is an error.
	Prompter ports.Prompter
	// GeneratedAt is recorded in the rewritten origin.toml, only when the
	// update completes with no conflicts.
	GeneratedAt time.Time
	// Source overrides the rewritten origin.toml's "source" field.
	// Defaults to the existing origin's Source when empty (the common
	// case: a local template bump stays "local"). A caller updating a
	// remote-sourced project to a newer commit must resolve that commit
	// itself (mirroring how New's callers resolve a remote reference) and
	// pass the resulting "<url>@<commit>" string here.
	Source string
}

// UpdateResult is the outcome of a three-way merge between a project's old
// base (M15), its template's latest version, and its current files.
type UpdateResult struct {
	// Plan represents the project's complete desired post-update state —
	// apply it with executor.ApplyUpdate, which replaces TargetDir's
	// content with exactly what Plan describes (a path not in Plan is
	// gone afterward; see Deleted).
	Plan plan.Plan
	// Conflicts lists every path with an unresolved conflict (markers
	// written inline, or a modify/delete or rename/modify situation this
	// package can't safely auto-resolve). Non-empty means origin.toml was
	// NOT rewritten to the new version — the update needs the conflicts
	// resolved and run again.
	Conflicts []string
	// Deleted lists every path present before the update that is not
	// present in Plan — report-only; there's no dedicated "remove" op.
	Deleted                []string
	OldVersion, NewVersion string
}

type fileEntry struct {
	Content []byte
	Mode    fs.FileMode
}

// Update three-way merges TargetDir's current files against its recorded
// old base and its template's latest version. It performs no writes;
// apply the result with executor.ApplyUpdate.
func Update(p UpdateParams) (UpdateResult, error) {
	originPath := path.Join(p.TargetDir, provenance.File)
	originData, err := p.FS.ReadFile(originPath)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: read %s: %w", provenance.File, err)
	}
	origin, err := provenance.ParseOrigin(originData)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
	}

	baseResult, err := RegenerateBase(RegenerateParams{
		Store: p.Store, FS: p.FS, Origin: origin, TargetDir: p.TargetDir,
	})
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: regenerate base: %w", err)
	}

	vars := make(map[string]string, len(origin.Variables))
	for k, v := range origin.Variables {
		vars[k] = fmt.Sprintf("%v", v)
	}
	theirsResult, err := New(NewParams{
		Store: p.Store, FS: p.FS, Template: origin.Template, TargetDir: p.TargetDir,
		Vars: vars, Toggles: origin.Toggles, Prompter: p.Prompter, GeneratedAt: p.GeneratedAt,
	})
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: regenerate latest: %w", err)
	}
	theirsResult = theirsResult.WithoutHooks() // update never re-runs post-generate hooks

	versions, err := p.Store.Versions(origin.Template)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
	}
	newVersion := versions[len(versions)-1]

	baseMap := opsToFileMap(baseResult)
	theirsMap := opsToFileMap(theirsResult)
	theirsOriginRaw := theirsMap[provenance.File].Content
	delete(baseMap, provenance.File)
	delete(theirsMap, provenance.File)

	oursFiles, err := walkAllFiles(p.FS, p.TargetDir)
	if err != nil {
		return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
	}
	oursMap := make(map[string]fileEntry, len(oursFiles))
	for _, f := range oursFiles {
		if f == provenance.File {
			continue
		}
		content, err := p.FS.ReadFile(path.Join(p.TargetDir, f))
		if err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: read %q: %w", f, err)
		}
		fi, err := p.FS.Stat(path.Join(p.TargetDir, f))
		if err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: stat %q: %w", f, err)
		}
		oursMap[f] = fileEntry{Content: content, Mode: fi.Mode}
	}

	renames := detectRenames(baseMap, theirsMap)
	handled := make(map[string]bool, len(renames)*2)

	b := plan.NewBuilder(p.TargetDir)
	var conflicts, deleted []string

	for oldPath, newPath := range renames {
		handled[oldPath] = true
		handled[newPath] = true
		theirsNew := theirsMap[newPath]
		oursOld, oursHad := oursMap[oldPath]
		baseOld := baseMap[oldPath]

		switch {
		case !oursHad:
			mustWrite(b, newPath, theirsNew)
		case bytes.Equal(oursOld.Content, baseOld.Content):
			deleted = append(deleted, oldPath)
			mustWrite(b, newPath, theirsNew)
		default:
			mustWrite(b, oldPath, oursOld)
			mustWrite(b, newPath, theirsNew)
			conflicts = append(conflicts, oldPath+" -> "+newPath)
		}
	}

	paths := unionPaths(baseMap, theirsMap, oursMap, handled)
	for _, pth := range paths {
		baseEntry, hasBase := baseMap[pth]
		theirsEntry, hasTheirs := theirsMap[pth]
		oursEntry, hasOurs := oursMap[pth]

		switch {
		case !hasBase && !hasTheirs: // hasOurs must be true to appear in paths at all
			mustWrite(b, pth, oursEntry)

		case !hasBase && hasTheirs && !hasOurs:
			mustWrite(b, pth, theirsEntry)

		case !hasBase && hasTheirs && hasOurs:
			if bytes.Equal(oursEntry.Content, theirsEntry.Content) {
				mustWrite(b, pth, theirsEntry)
			} else {
				content, isConflict := mergeContent(nil, oursEntry.Content, theirsEntry.Content)
				mustWrite(b, pth, fileEntry{Content: content, Mode: theirsEntry.Mode})
				if isConflict {
					conflicts = append(conflicts, pth)
				}
			}

		case hasBase && !hasTheirs && !hasOurs:
			deleted = append(deleted, pth)

		case hasBase && !hasTheirs && hasOurs:
			if bytes.Equal(oursEntry.Content, baseEntry.Content) {
				deleted = append(deleted, pth)
			} else {
				mustWrite(b, pth, oursEntry)
				conflicts = append(conflicts, pth)
			}

		case hasBase && hasTheirs && !hasOurs:
			deleted = append(deleted, pth)
			if !bytes.Equal(baseEntry.Content, theirsEntry.Content) {
				conflicts = append(conflicts, pth)
			}

		case hasBase && hasTheirs && hasOurs:
			switch {
			case bytes.Equal(oursEntry.Content, baseEntry.Content):
				mustWrite(b, pth, theirsEntry)
			case bytes.Equal(theirsEntry.Content, baseEntry.Content):
				mustWrite(b, pth, oursEntry)
			case bytes.Equal(oursEntry.Content, theirsEntry.Content):
				mustWrite(b, pth, oursEntry)
			default:
				content, isConflict := mergeContent(baseEntry.Content, oursEntry.Content, theirsEntry.Content)
				mustWrite(b, pth, fileEntry{Content: content, Mode: theirsEntry.Mode})
				if isConflict {
					conflicts = append(conflicts, pth)
				}
			}
		}
	}

	sort.Strings(conflicts)
	sort.Strings(deleted)

	if len(conflicts) == 0 {
		newOrigin, err := provenance.ParseOrigin(theirsOriginRaw)
		if err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
		}
		source := p.Source
		if source == "" {
			source = origin.Source
		}
		newOrigin.Source = source
		data, err := newOrigin.TOML()
		if err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
		}
		if err := b.WriteFile(provenance.File, data, 0o644); err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
		}
	} else {
		if err := b.WriteFile(provenance.File, originData, 0o644); err != nil {
			return UpdateResult{}, fmt.Errorf("usecase: update: %w", err)
		}
	}

	return UpdateResult{
		Plan: b.Build(), Conflicts: conflicts, Deleted: deleted,
		OldVersion: origin.Version, NewVersion: newVersion,
	}, nil
}

// mergeContent three-way merges a text file, or falls back to keeping
// ours unchanged (flagged as a conflict) for anything that looks binary —
// text conflict markers would corrupt binary content.
func mergeContent(base, ours, theirs []byte) (content []byte, conflict bool) {
	if render.IsBinary(ours) || render.IsBinary(theirs) || (base != nil && render.IsBinary(base)) {
		return ours, true
	}
	baseStr := ""
	if base != nil {
		baseStr = string(base)
	}
	r := merge.Merge(baseStr, string(ours), string(theirs))
	return []byte(r.Content), r.Conflict
}

func mustWrite(b *plan.Builder, p string, e fileEntry) {
	mode := e.Mode
	if mode == 0 {
		mode = 0o644
	}
	// Builder.WriteFile only fails on a path collision or an escaping
	// path, neither possible here: every path comes from a walk already
	// rooted correctly, and each is visited exactly once. A real failure
	// would be a bug in this file's own bookkeeping.
	if err := b.WriteFile(p, e.Content, mode); err != nil {
		panic(fmt.Sprintf("usecase: update: %v", err))
	}
}

// opsToFileMap extracts every WriteFileOp from p into a path -> fileEntry
// map (New/RegenerateBase never produce CreateDirOp for a non-empty
// directory, and Update doesn't need empty-directory preservation).
func opsToFileMap(p plan.Plan) map[string]fileEntry {
	out := make(map[string]fileEntry, len(p.Ops))
	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok {
			out[wf.Path] = fileEntry{Content: wf.Content, Mode: wf.Mode}
		}
	}
	return out
}

// detectRenames pairs a path that disappeared between base and theirs
// with one that appeared, when their content matches exactly — the
// signal that the template renamed a file without otherwise changing it
// in the same version bump. Each path is used in at most one pairing.
func detectRenames(baseMap, theirsMap map[string]fileEntry) map[string]string {
	var baseOnly, theirsOnly []string
	for p := range baseMap {
		if _, ok := theirsMap[p]; !ok {
			baseOnly = append(baseOnly, p)
		}
	}
	for p := range theirsMap {
		if _, ok := baseMap[p]; !ok {
			theirsOnly = append(theirsOnly, p)
		}
	}
	sort.Strings(baseOnly)
	sort.Strings(theirsOnly)

	claimed := make(map[string]bool, len(theirsOnly))
	renames := make(map[string]string)
	for _, bp := range baseOnly {
		for _, tp := range theirsOnly {
			if claimed[tp] {
				continue
			}
			if bytes.Equal(baseMap[bp].Content, theirsMap[tp].Content) {
				renames[bp] = tp
				claimed[tp] = true
				break
			}
		}
	}
	return renames
}

// unionPaths returns every path present in any of base/theirs/ours, minus
// those already handled (by rename detection), sorted.
func unionPaths(baseMap, theirsMap, oursMap map[string]fileEntry, handled map[string]bool) []string {
	seen := make(map[string]bool, len(baseMap)+len(theirsMap)+len(oursMap))
	for p := range baseMap {
		seen[p] = true
	}
	for p := range theirsMap {
		seen[p] = true
	}
	for p := range oursMap {
		seen[p] = true
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		if !handled[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}
