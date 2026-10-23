package usecase

import (
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/core/semver"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/executor"
)

const bumpFixtureManifest = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

func newBumpFixture(t *testing.T) (*memfs.MemFS, *local.Store) {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/1.0.0/src")
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/snap.toml", []byte(bumpFixtureManifest))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/README.md", []byte("# {{name}}"))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/src/main.go", []byte("package main"))
	return fsys, local.New(fsys, "templates")
}

func TestBump_Patch(t *testing.T) {
	fsys, store := newBumpFixture(t)

	res, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: semver.Patch})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if res.OldVersion != "1.0.0" || res.NewVersion != "1.0.1" {
		t.Errorf("OldVersion/NewVersion = %q/%q, want %q/%q", res.OldVersion, res.NewVersion, "1.0.0", "1.0.1")
	}
	if res.Plan.TargetDir != "templates/api-starter/1.0.1" {
		t.Errorf("Plan.TargetDir = %q, want %q", res.Plan.TargetDir, "templates/api-starter/1.0.1")
	}

	if err := executor.New(fsys).Apply(res.Plan); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The old version must be untouched.
	oldManifest, err := fsys.ReadFile("templates/api-starter/1.0.0/snap.toml")
	if err != nil || !strings.Contains(string(oldManifest), `version = "1.0.0"`) {
		t.Errorf("old version's snap.toml = %q, %v, want it unchanged", oldManifest, err)
	}

	// The new version must have every file, verbatim, except the manifest's
	// version field.
	newManifestData, err := fsys.ReadFile("templates/api-starter/1.0.1/snap.toml")
	if err != nil {
		t.Fatalf("ReadFile(new snap.toml): %v", err)
	}
	m, err := template.ParseManifest(newManifestData)
	if err != nil {
		t.Fatalf("ParseManifest(new snap.toml): %v", err)
	}
	if m.Version != "1.0.1" {
		t.Errorf("new manifest.Version = %q, want %q", m.Version, "1.0.1")
	}
	if m.Name != "api-starter" || len(m.Variables) != 1 || m.Variables[0].Name != "name" {
		t.Errorf("new manifest = %+v, want name/variables unchanged", m)
	}

	readme, err := fsys.ReadFile("templates/api-starter/1.0.1/README.md")
	if err != nil || string(readme) != "# {{name}}" {
		t.Errorf("new README.md = %q, %v, want it copied verbatim (no rendering)", readme, err)
	}
	main, err := fsys.ReadFile("templates/api-starter/1.0.1/src/main.go")
	if err != nil || string(main) != "package main" {
		t.Errorf("new src/main.go = %q, %v, want it copied verbatim", main, err)
	}
}

func TestBump_MinorResetsPatch(t *testing.T) {
	fsys, store := newBumpFixture(t)
	res, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: semver.Minor})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if res.NewVersion != "1.1.0" {
		t.Errorf("NewVersion = %q, want %q", res.NewVersion, "1.1.0")
	}
}

func TestBump_MajorResetsMinorAndPatch(t *testing.T) {
	fsys, store := newBumpFixture(t)
	res, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: semver.Major})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if res.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", res.NewVersion, "2.0.0")
	}
}

func TestBump_UsesLatestVersion(t *testing.T) {
	fsys, store := newBumpFixture(t)
	// A second, newer version already exists — bump must start from it, not
	// from 1.0.0.
	mustMkdirAll(t, fsys, "templates/api-starter/1.5.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.5.0/snap.toml", []byte(strings.Replace(bumpFixtureManifest, "1.0.0", "1.5.0", 1)))
	mustWriteFile(t, fsys, "templates/api-starter/1.5.0/README.md", []byte("# {{name}} v2"))

	res, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: semver.Patch})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if res.OldVersion != "1.5.0" || res.NewVersion != "1.5.1" {
		t.Errorf("OldVersion/NewVersion = %q/%q, want %q/%q", res.OldVersion, res.NewVersion, "1.5.0", "1.5.1")
	}
}

func TestBump_UnknownTemplate(t *testing.T) {
	fsys, store := newBumpFixture(t)
	if _, err := Bump(BumpParams{Store: store, FS: fsys, Template: "nope", Level: semver.Patch}); err == nil {
		t.Error("Bump(unknown template): want error, got nil")
	}
}

func TestBump_UnknownLevel(t *testing.T) {
	fsys, store := newBumpFixture(t)
	if _, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: "sideways"}); err == nil {
		t.Error("Bump(unknown level): want error, got nil")
	}
}

func TestBump_AlreadyExistingVersionRefused(t *testing.T) {
	fsys, store := newBumpFixture(t)
	res, err := Bump(BumpParams{Store: store, FS: fsys, Template: "api-starter", Level: semver.Patch})
	if err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if err := executor.New(fsys).Apply(res.Plan); err != nil {
		t.Fatalf("Apply (first): %v", err)
	}
	// Applying the exact same plan again targets the version that now
	// already exists — refused, exactly like snap new's own safety rule.
	if err := executor.New(fsys).Apply(res.Plan); err == nil {
		t.Error("re-applying a Bump plan onto an already-bumped version: want error, got nil")
	}
}
