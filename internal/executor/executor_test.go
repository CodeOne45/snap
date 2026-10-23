package executor

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/ports"
)

// bogusOp is a test-only Op implementation to exercise the executor's
// unsupported-op-type path, simulating a future op kind arriving before the
// executor knows how to apply it.
type bogusOp struct{}

func (bogusOp) Kind() plan.OpKind { return "bogus" }

func TestApply_WritesTree(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("README.md", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := e.Apply(b.Build()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := fsys.ReadFile("myproj/README.md")
	if err != nil {
		t.Fatalf("ReadFile(README.md): %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("README.md content = %q, want %q", got, "hello")
	}

	got, err = fsys.ReadFile("myproj/src/main.go")
	if err != nil {
		t.Fatalf("ReadFile(src/main.go): %v", err)
	}
	if string(got) != "package main" {
		t.Errorf("src/main.go content = %q, want %q", got, "package main")
	}

	fi, err := fsys.Stat("myproj/src")
	if err != nil || !fi.IsDir {
		t.Errorf("Stat(myproj/src) = %+v, %v, want a directory", fi, err)
	}

	// No temp directory litter left behind after a successful apply.
	entries, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root): %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "myproj" {
		t.Fatalf("root entries = %+v, want only [myproj]", entries)
	}
}

func TestApply_NonEmptyTargetRejected(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("myproj/existing.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}

	if err := e.Apply(b.Build()); err == nil {
		t.Fatal("Apply onto a non-empty existing target: want error, got nil")
	}
	if _, err := fsys.ReadFile("myproj/existing.txt"); err != nil {
		t.Errorf("existing.txt was disturbed by the rejected apply: %v", err)
	}
}

func TestApply_TargetIsFileRejected(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.WriteFile("myproj", []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.Apply(b.Build()); err == nil {
		t.Fatal("Apply onto a target that is an existing file: want error, got nil")
	}
}

func TestApply_EmptyExistingTargetAllowed(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("myproj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	e := New(fsys)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := e.Apply(b.Build()); err != nil {
		t.Fatalf("Apply onto an empty existing target: %v", err)
	}
	got, err := fsys.ReadFile("myproj/README.md")
	if err != nil || string(got) != "hello" {
		t.Errorf("ReadFile(myproj/README.md) = %q, %v, want %q, nil", got, err, "hello")
	}
}

func TestApply_RollsBackOnFailure(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys)

	before, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root) before: %v", err)
	}

	p := plan.Plan{
		TargetDir: "myproj",
		Ops: []plan.Op{
			plan.CreateDirOp{Path: "src"},
			bogusOp{},
		},
	}

	if err := e.Apply(p); err == nil {
		t.Fatal("Apply with unsupported op: want error, got nil")
	}

	if _, err := fsys.Stat("myproj"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(myproj) after failed apply = %v, want fs.ErrNotExist", err)
	}

	after, err := fsys.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root) after: %v", err)
	}
	if len(after) != len(before) {
		t.Errorf("root entries after failed apply = %+v, want unchanged from %+v (temp dir not cleaned up)", after, before)
	}
}

// fakeHookRunner records the hooks it was asked to run and returns canned
// results per command, so Executor's own orchestration (when hooks run,
// what Dir they get, how failures surface) can be tested without a real
// subprocess or real disk.
type fakeHookRunner struct {
	calls   []ports.Hook
	results map[string]hookOutcome
}

type hookOutcome struct {
	result ports.HookResult
	err    error
}

func (f *fakeHookRunner) Run(hook ports.Hook) (ports.HookResult, error) {
	f.calls = append(f.calls, hook)
	if o, ok := f.results[hook.Run]; ok {
		return o.result, o.err
	}
	return ports.HookResult{}, nil
}

func TestApply_RunsHooksAfterRename(t *testing.T) {
	fsys := memfs.New()
	hooks := &fakeHookRunner{}
	e := NewWithHooks(fsys, hooks)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}
	if err := b.RunHook("go mod tidy"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	if err := e.Apply(b.Build()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(hooks.calls) != 2 {
		t.Fatalf("hook calls = %+v, want 2", hooks.calls)
	}
	if hooks.calls[0].Run != "git init" || hooks.calls[1].Run != "go mod tidy" {
		t.Errorf("hooks ran out of order: %+v", hooks.calls)
	}
	for _, c := range hooks.calls {
		if c.Dir != "myproj" {
			t.Errorf("hook %q ran with Dir = %q, want %q (the final target, not the temp dir)", c.Run, c.Dir, "myproj")
		}
	}
}

func TestApply_HookFailure_FilesStayAndErrorIncludesOutput(t *testing.T) {
	fsys := memfs.New()
	hooks := &fakeHookRunner{results: map[string]hookOutcome{
		"go mod tidy": {
			result: ports.HookResult{Output: "network unreachable", ExitCode: 1},
			err:    errors.New("exit 1"),
		},
	}}
	e := NewWithHooks(fsys, hooks)

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("go mod tidy"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	err := e.Apply(b.Build())
	if err == nil {
		t.Fatal("Apply with a failing hook: want error, got nil")
	}
	if !strings.Contains(err.Error(), "network unreachable") {
		t.Errorf("error = %v, want it to include the hook's output", err)
	}

	// Files were already committed (hooks run after rename) and must stay.
	content, readErr := fsys.ReadFile("myproj/README.md")
	if readErr != nil || string(content) != "hi" {
		t.Errorf("README.md = %q, %v, want it to survive a failing hook", content, readErr)
	}
}

func TestApply_HooksWithoutRunnerIsError(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys) // no hook runner configured

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	if err := e.Apply(b.Build()); err == nil {
		t.Fatal("Apply with a hook but no HookRunner: want error, got nil")
	}
	// Files were already committed; only the hook step failed.
	if _, err := fsys.ReadFile("myproj/README.md"); err != nil {
		t.Errorf("README.md should still exist: %v", err)
	}
}

func TestApply_HooksStrippedBeforeApply_NeverRun(t *testing.T) {
	fsys := memfs.New()
	e := New(fsys) // no hook runner — proves WithoutHooks is what makes this safe

	b := plan.NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("rm -rf /"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	p := b.Build().WithoutHooks()
	if err := e.Apply(p); err != nil {
		t.Fatalf("Apply(declined hooks): %v", err)
	}
	if _, err := fsys.ReadFile("myproj/README.md"); err != nil {
		t.Errorf("README.md should exist: %v", err)
	}
}
