package executor

import (
	"fmt"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/ports"
)

// failAfterNRenames wraps a ports.FS, failing the Nth call to Rename (1
// indexed) and succeeding on every other call — used to simulate the swap
// step of ApplyUpdate failing after the old target has already been
// renamed aside, to verify the best-effort rollback.
type failAfterNRenames struct {
	ports.FS
	n     int
	calls int
}

func (f *failAfterNRenames) Rename(oldPath, newPath string) error {
	f.calls++
	if f.calls == f.n {
		return fmt.Errorf("injected rename failure")
	}
	return f.FS.Rename(oldPath, newPath)
}

func TestApplyUpdate_ReplacesExistingTreeCompletely(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("myproj/README.md", []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := fsys.WriteFile("myproj/removed.txt", []byte("gone after update"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.WriteFile("added.txt", []byte("brand new"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := e.ApplyUpdate(b.Build()); err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}

	readme, err := fsys.ReadFile("myproj/README.md")
	if err != nil || string(readme) != "new" {
		t.Errorf("README.md = %q, %v, want %q", readme, err, "new")
	}
	added, err := fsys.ReadFile("myproj/added.txt")
	if err != nil || string(added) != "brand new" {
		t.Errorf("added.txt = %q, %v, want %q", added, err, "brand new")
	}
	// Omitted from the plan -> gone, without a dedicated "remove" op.
	if _, err := fsys.ReadFile("myproj/removed.txt"); err == nil {
		t.Error("removed.txt should no longer exist: it wasn't in the update's plan")
	}
}

func TestApplyUpdate_NoLitterLeftOnSuccess(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyUpdate(b.Build()); err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}

	entries, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root): %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "myproj" {
		t.Fatalf("root entries = %+v, want only [myproj] (no temp/backup dirs left behind)", entries)
	}
}

func TestApplyUpdate_RequiresExistingTarget(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyUpdate(b.Build()); err == nil {
		t.Fatal("ApplyUpdate on a target that doesn't exist yet: want error, got nil")
	}
}

func TestApplyUpdate_TargetIsFileRejected(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.WriteFile("myproj", []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyUpdate(b.Build()); err == nil {
		t.Fatal("ApplyUpdate where the target is a file, not a directory: want error, got nil")
	}
}

func TestApplyUpdate_RollsBackOnBuildFailure(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("myproj/original.txt", []byte("untouched"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	before, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root) before: %v", err)
	}

	p := plan.Plan{
		TargetDir: "myproj",
		Ops: []plan.Op{
			plan.WriteFileOp{Path: "ok.txt", Content: []byte("x"), Mode: 0o644},
			bogusOp{},
		},
	}
	if err := e.ApplyUpdate(p); err == nil {
		t.Fatal("ApplyUpdate with an unsupported op: want error, got nil")
	}

	content, err := fsys.ReadFile("myproj/original.txt")
	if err != nil || string(content) != "untouched" {
		t.Errorf("myproj/original.txt = %q, %v, want it unchanged by the failed update", content, err)
	}
	after, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root) after: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("root entries after failed update = %+v, want unchanged from %+v (temp dir cleaned up)", after, before)
	}
}

func TestApplyUpdate_RestoresOriginalIfSwapFails(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("myproj/original.txt", []byte("untouched"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Fail the second Rename call: the first moves myproj aside to a
	// backup dir (succeeds), the second would move the new temp tree into
	// place (fails) -- ApplyUpdate must restore the backup back to myproj.
	wrapped := &failAfterNRenames{FS: fsys, n: 2}
	e := New(wrapped)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("new.txt", []byte("new content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := e.ApplyUpdate(b.Build()); err == nil {
		t.Fatal("ApplyUpdate with an injected swap failure: want error, got nil")
	}

	content, err := fsys.ReadFile("myproj/original.txt")
	if err != nil || string(content) != "untouched" {
		t.Errorf("myproj/original.txt after a failed swap = %q, %v, want the original restored", content, err)
	}
	if _, err := fsys.ReadFile("myproj/new.txt"); err == nil {
		t.Error("myproj/new.txt should not exist: the swap that would have introduced it failed")
	}
}
