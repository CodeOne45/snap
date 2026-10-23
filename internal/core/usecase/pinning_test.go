package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/core/plan"
)

const pinningManifestV1 = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

const pinningManifestV2 = `
[template]
name = "api-starter"
version = "2.0.0"

[[variables]]
name = "name"
type = "string"
`

// newPinningFixture builds a real, multi-version local store: v1.0.0 says
// "hello v1", v2.0.0 says "hello v2" — so a test can tell which version
// actually got resolved from the generated/shown content.
func newPinningFixture(t *testing.T) (*memfs.MemFS, *local.Store) {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/1.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/snap.toml", []byte(pinningManifestV1))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/README.md", []byte("hello v1"))
	mustMkdirAll(t, fsys, "templates/api-starter/2.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/2.0.0/snap.toml", []byte(pinningManifestV2))
	mustWriteFile(t, fsys, "templates/api-starter/2.0.0/README.md", []byte("hello v2"))
	return fsys, local.New(fsys, "templates")
}

func findWriteFileOp(t *testing.T, p plan.Plan, wantPath string) plan.WriteFileOp {
	t.Helper()
	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == wantPath {
			return wf
		}
	}
	t.Fatalf("no WriteFileOp at %q", wantPath)
	return plan.WriteFileOp{}
}

func TestShow_UnpinnedUsesLatestVersion(t *testing.T) {
	fsys, store := newPinningFixture(t)
	got, err := Show(store, fsys, "api-starter")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.Manifest.Version != "2.0.0" {
		t.Errorf("Manifest.Version = %q, want %q (the latest)", got.Manifest.Version, "2.0.0")
	}
}

func TestShow_Pinned(t *testing.T) {
	fsys, store := newPinningFixture(t)
	got, err := Show(store, fsys, "api-starter@1.0.0")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.Manifest.Version != "1.0.0" {
		t.Errorf("Manifest.Version = %q, want %q (pinned)", got.Manifest.Version, "1.0.0")
	}
}

func TestShow_PinnedUnknownVersion(t *testing.T) {
	fsys, store := newPinningFixture(t)
	if _, err := Show(store, fsys, "api-starter@9.9.9"); err == nil {
		t.Error("Show(a version that doesn't exist): want error, got nil")
	}
}

func TestNew_UnpinnedUsesLatestVersion(t *testing.T) {
	fsys, store := newPinningFixture(t)
	p, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: "out",
		Vars: map[string]string{"name": "acme"}, GeneratedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	readme := findWriteFileOp(t, p, "README.md")
	if string(readme.Content) != "hello v2" {
		t.Errorf("README.md = %q, want %q (the latest)", readme.Content, "hello v2")
	}
}

func TestNew_Pinned(t *testing.T) {
	fsys, store := newPinningFixture(t)
	p, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter@1.0.0", TargetDir: "out",
		Vars: map[string]string{"name": "acme"}, GeneratedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	readme := findWriteFileOp(t, p, "README.md")
	if string(readme.Content) != "hello v1" {
		t.Errorf("README.md = %q, want %q (pinned)", readme.Content, "hello v1")
	}

	origin := findOriginOp(t, p)
	if !strings.Contains(string(origin.Content), `version = "1.0.0"`) {
		t.Errorf("origin.toml = %q, want it to record the pinned version 1.0.0", origin.Content)
	}
}

func TestNew_PinnedUnknownVersion(t *testing.T) {
	fsys, store := newPinningFixture(t)
	_, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter@9.9.9", TargetDir: "out",
		Vars: map[string]string{"name": "acme"}, GeneratedAt: time.Now().UTC(),
	})
	if err == nil {
		t.Error("New(a version that doesn't exist): want error, got nil")
	}
}
