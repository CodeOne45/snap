package usecase

import (
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/executor"
)

const statusManifestV1 = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

func newStatusFixture(t *testing.T) (*memfs.MemFS, *local.Store) {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/1.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/snap.toml", []byte(statusManifestV1))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/README.md", []byte("# {{name}}"))
	return fsys, local.New(fsys, "templates")
}

func generateStatusFixtureProject(t *testing.T, fsys *memfs.MemFS, store *local.Store, dir string) {
	t.Helper()
	p, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: dir,
		Vars: map[string]string{"name": "acme"}, GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := executor.New(fsys).Apply(p); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestStatus_UpToDate(t *testing.T) {
	fsys, store := newStatusFixture(t)
	generateStatusFixtureProject(t, fsys, store, "proj")

	results := Status(StatusParams{Store: store, FS: fsys, Dirs: []string{"proj"}})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	r := results[0]
	if r.Err != nil {
		t.Fatalf("Err = %v, want nil", r.Err)
	}
	if r.Template != "api-starter" || r.CurrentVersion != "1.0.0" || r.LatestVersion != "1.0.0" {
		t.Errorf("r = %+v, want api-starter 1.0.0/1.0.0", r)
	}
	if !r.UpToDate {
		t.Error("UpToDate = false, want true")
	}
}

func TestStatus_Behind(t *testing.T) {
	fsys, store := newStatusFixture(t)
	generateStatusFixtureProject(t, fsys, store, "proj")

	mustMkdirAll(t, fsys, "templates/api-starter/1.1.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/snap.toml", []byte(`
[template]
name = "api-starter"
version = "1.1.0"

[[variables]]
name = "name"
type = "string"
`))
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/README.md", []byte("# {{name}} v2"))

	results := Status(StatusParams{Store: store, FS: fsys, Dirs: []string{"proj"}})
	r := results[0]
	if r.Err != nil {
		t.Fatalf("Err = %v, want nil", r.Err)
	}
	if r.CurrentVersion != "1.0.0" || r.LatestVersion != "1.1.0" {
		t.Errorf("CurrentVersion/LatestVersion = %q/%q, want %q/%q", r.CurrentVersion, r.LatestVersion, "1.0.0", "1.1.0")
	}
	if r.UpToDate {
		t.Error("UpToDate = true, want false")
	}
}

func TestStatus_NotASnapProject(t *testing.T) {
	fsys, store := newStatusFixture(t)
	mustMkdirAll(t, fsys, "not-a-project")

	results := Status(StatusParams{Store: store, FS: fsys, Dirs: []string{"not-a-project"}})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Err == nil {
		t.Error("Err = nil, want an error (no .snap/origin.toml)")
	}
}

func TestStatus_RemoteSourceSkipsDriftCheck(t *testing.T) {
	fsys, store := newStatusFixture(t)
	generateStatusFixtureProject(t, fsys, store, "proj")
	// Simulate a remote-sourced project by rewriting its origin.toml.
	mustWriteFile(t, fsys, "proj/.snap/origin.toml", []byte(`template = "api-starter"
version = "1.0.0"
source = "https://example.com/templates/api-starter.git@abc1234"
generated_at = 2026-01-01T00:00:00Z

[variables]
  name = "acme"
`))

	results := Status(StatusParams{Store: store, FS: fsys, Dirs: []string{"proj"}})
	r := results[0]
	if r.Err != nil {
		t.Fatalf("Err = %v, want nil", r.Err)
	}
	if r.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty (remote source, no local drift check)", r.LatestVersion)
	}
	if r.UpToDate {
		t.Error("UpToDate = true, want false (unknown, not claimed up to date)")
	}
	if r.Source != "https://example.com/templates/api-starter.git@abc1234" {
		t.Errorf("Source = %q, want the remote source string preserved", r.Source)
	}
}

func TestStatus_MultipleDirs(t *testing.T) {
	fsys, store := newStatusFixture(t)
	generateStatusFixtureProject(t, fsys, store, "proj1")
	generateStatusFixtureProject(t, fsys, store, "proj2")
	mustMkdirAll(t, fsys, "not-a-project")

	results := Status(StatusParams{Store: store, FS: fsys, Dirs: []string{"proj1", "not-a-project", "proj2"}})
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Dir != "proj1" || results[1].Dir != "not-a-project" || results[2].Dir != "proj2" {
		t.Errorf("results out of order: %+v", results)
	}
	if results[0].Err != nil || results[2].Err != nil {
		t.Errorf("proj1/proj2 should have no error: %+v / %+v", results[0].Err, results[2].Err)
	}
	if results[1].Err == nil {
		t.Error("not-a-project should have an error")
	}
}
