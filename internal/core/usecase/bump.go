package usecase

import (
	"fmt"
	"path"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/semver"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// BumpParams are the inputs to Bump.
type BumpParams struct {
	Store    ports.Store
	FS       ports.FS
	Template string
	Level    semver.Level
}

// BumpResult is the Plan to write the new version, plus which versions
// were involved — for a confirmation message.
type BumpResult struct {
	Plan       plan.Plan
	OldVersion string
	NewVersion string
}

// Bump copies Template's latest version verbatim into a new version
// directory (no placeholder rendering — unlike New, this never touches
// "{{...}}" tokens), with its manifest's [template].version rewritten to
// the bumped value. The old version is left untouched, so an old base can
// still be regenerated from it later (M15). It performs no writes; the
// caller applies the result with executor.Apply.
func Bump(p BumpParams) (BumpResult, error) {
	versions, err := p.Store.Versions(p.Template)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}
	oldVersion := versions[len(versions)-1]

	oldSemver, err := semver.Parse(oldVersion)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}
	newSemver, err := semver.Bump(oldSemver, p.Level)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}
	newVersion := newSemver.String()

	oldRoot, err := p.Store.PathVersion(p.Template, oldVersion)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}
	newRoot := path.Join(p.Store.Root(), p.Template, newVersion)

	// walkTemplate deliberately excludes the manifest itself (it's metadata
	// about the template, never copied into a generated project) — Bump
	// needs it too, so it's read and rewritten separately, same as New and
	// Capture each handle it as their own explicit step.
	manifestData, err := p.FS.ReadFile(path.Join(oldRoot, template.ManifestFile))
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: read %s: %w", template.ManifestFile, err)
	}
	manifestData, err = rewriteManifestVersion(manifestData, newVersion)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}

	files, dirs, err := walkTemplate(p.FS, oldRoot)
	if err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}

	b := plan.NewBuilder(newRoot)
	if err := b.WriteFile(template.ManifestFile, manifestData, 0o644); err != nil {
		return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
	}
	for _, d := range dirs {
		if err := b.CreateDir(d); err != nil {
			return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
		}
	}
	for _, f := range files {
		srcPath := path.Join(oldRoot, f)
		content, err := p.FS.ReadFile(srcPath)
		if err != nil {
			return BumpResult{}, fmt.Errorf("usecase: bump: read %q: %w", f, err)
		}
		fi, err := p.FS.Stat(srcPath)
		if err != nil {
			return BumpResult{}, fmt.Errorf("usecase: bump: stat %q: %w", f, err)
		}
		if err := b.WriteFile(f, content, fi.Mode); err != nil {
			return BumpResult{}, fmt.Errorf("usecase: bump: %w", err)
		}
	}

	return BumpResult{Plan: b.Build(), OldVersion: oldVersion, NewVersion: newVersion}, nil
}

func rewriteManifestVersion(data []byte, version string) ([]byte, error) {
	m, err := template.ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", template.ManifestFile, err)
	}
	m.Version = version
	return m.TOML()
}
