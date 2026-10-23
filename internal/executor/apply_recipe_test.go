package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/plan"
)

func sha256HexOf(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestApplyRecipe_WritesNewFileAndModifiesExisting(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := []byte(`{"dependencies": {}}`)
	if err := fsys.WriteFile("myproj/package.json", original, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("src/lib/sentry.ts", []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	modified := []byte(`{"dependencies": {"@sentry/node": "^8.0.0"}}`)
	if err := b.ModifyFile("package.json", sha256HexOf(original), modified, 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}

	if err := e.ApplyRecipe(b.Build()); err != nil {
		t.Fatalf("ApplyRecipe: %v", err)
	}

	newFile, err := fsys.ReadFile("myproj/src/lib/sentry.ts")
	if err != nil || string(newFile) != "export const x = 1;\n" {
		t.Errorf("src/lib/sentry.ts = %q, %v, want the new content", newFile, err)
	}
	pkg, err := fsys.ReadFile("myproj/package.json")
	if err != nil || string(pkg) != string(modified) {
		t.Errorf("package.json = %q, %v, want %q", pkg, err, modified)
	}
}

func TestApplyRecipe_CreateDirOp(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.CreateDir("empty"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := e.ApplyRecipe(b.Build()); err != nil {
		t.Fatalf("ApplyRecipe: %v", err)
	}
	if fi, err := fsys.Stat("myproj/empty"); err != nil || !fi.IsDir {
		t.Errorf("myproj/empty: Stat = %+v, %v, want an existing directory", fi, err)
	}
}

func TestApplyRecipe_RefusesHashMismatch(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	original := []byte(`{"dependencies": {}}`)
	if err := fsys.WriteFile("myproj/package.json", original, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	// Hash computed from a snapshot that's no longer current.
	staleHash := sha256HexOf([]byte("something else entirely"))
	if err := b.ModifyFile("package.json", staleHash, []byte("new content"), 0o644); err != nil {
		t.Fatalf("ModifyFile: %v", err)
	}
	if err := b.WriteFile("new.txt", []byte("brand new"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := e.ApplyRecipe(b.Build())
	if err == nil {
		t.Fatal("ApplyRecipe with a stale ModifyFileOp hash: want error, got nil")
	}
	if !strings.Contains(err.Error(), "package.json") {
		t.Errorf("error = %v, want it to mention package.json", err)
	}

	// Nothing written: neither the conflicting file nor any other op in
	// the same plan.
	pkg, _ := fsys.ReadFile("myproj/package.json")
	if string(pkg) != string(original) {
		t.Errorf("package.json = %q, want it unchanged (%q)", pkg, original)
	}
	if _, err := fsys.ReadFile("myproj/new.txt"); err == nil {
		t.Error("new.txt should not exist: the whole plan was refused")
	}
}

func TestApplyRecipe_RefusesWriteFileConflict(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj/src/lib", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("myproj/src/lib/sentry.ts", []byte("already here, unrelated"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("src/lib/sentry.ts", []byte("export const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := e.ApplyRecipe(b.Build())
	if err == nil {
		t.Fatal("ApplyRecipe with a WriteFileOp whose target already has different content: want error, got nil")
	}
	if !strings.Contains(err.Error(), "src/lib/sentry.ts") {
		t.Errorf("error = %v, want it to mention the conflicting path", err)
	}

	content, _ := fsys.ReadFile("myproj/src/lib/sentry.ts")
	if string(content) != "already here, unrelated" {
		t.Errorf("src/lib/sentry.ts = %q, want it unchanged", content)
	}
}

func TestApplyRecipe_WriteFileSameContentIsNotAConflict(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj/src/lib", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := []byte("export const x = 1;\n")
	if err := fsys.WriteFile("myproj/src/lib/sentry.ts", content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("src/lib/sentry.ts", content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyRecipe(b.Build()); err != nil {
		t.Fatalf("ApplyRecipe with a WriteFileOp whose target already matches exactly: %v", err)
	}
}

func TestApplyRecipe_RequiresExistingTarget(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyRecipe(b.Build()); err == nil {
		t.Fatal("ApplyRecipe on a target that doesn't exist yet: want error, got nil")
	}
}

func TestApplyRecipe_TargetIsFileRejected(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.WriteFile("myproj", []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.ApplyRecipe(b.Build()); err == nil {
		t.Fatal("ApplyRecipe where the target is a file, not a directory: want error, got nil")
	}
}

func TestApplyRecipe_RunsHooksAfterWrites(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	hooks := &fakeHookRunner{}
	e := NewWithHooks(fsys, hooks)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("npm install"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if err := e.ApplyRecipe(b.Build()); err != nil {
		t.Fatalf("ApplyRecipe: %v", err)
	}
	if len(hooks.calls) != 1 || hooks.calls[0].Run != "npm install" {
		t.Errorf("hook calls = %+v, want [npm install]", hooks.calls)
	}
	if hooks.calls[0].Dir != "myproj" {
		t.Errorf("hook ran with Dir = %q, want %q", hooks.calls[0].Dir, "myproj")
	}
}

func TestApplyRecipe_EmptyPlanIsNoOp(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	e := New(fsys)
	if err := e.ApplyRecipe(plan.Plan{TargetDir: "myproj"}); err != nil {
		t.Fatalf("ApplyRecipe with an empty plan: %v", err)
	}
}

func TestApplyRecipe_UnsupportedOpType(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	e := New(fsys)

	p := plan.Plan{TargetDir: "myproj", Ops: []plan.Op{bogusOp{}}}
	if err := e.ApplyRecipe(p); err == nil {
		t.Fatal("ApplyRecipe with an unsupported op: want error, got nil")
	}
}
