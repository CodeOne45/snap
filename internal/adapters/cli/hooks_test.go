package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/osfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
)

const hookedManifest = `
[template]
name = "hooked"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[hooks.post]]
run = "%s"
`

// newHookTestDeps builds Deps backed by the REAL filesystem (a t.TempDir):
// hooks genuinely spawn a subprocess via os/exec with Dir set to the
// target directory, so — unlike the rest of this package's memfs-based
// tests — exercising a hook that actually runs needs a real directory to
// chdir into.
func newHookTestDeps(t *testing.T, hookCommand string) (deps func() (Deps, error), workDir string) {
	t.Helper()
	root := t.TempDir()
	fsys := osfs.New()

	templatesDir := filepath.ToSlash(filepath.Join(root, "templates"))
	if err := fsys.MkdirAll(templatesDir+"/hooked/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifest := strings.ReplaceAll(hookedManifest, "%s", hookCommand)
	if err := fsys.WriteFile(templatesDir+"/hooked/1.0.0/snap.toml", []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(snap.toml): %v", err)
	}
	if err := fsys.WriteFile(templatesDir+"/hooked/1.0.0/README.md", []byte("# {{name}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}

	store := local.New(fsys, templatesDir)
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("MkdirAll(work): %v", err)
	}
	return func() (Deps, error) { return Deps{FS: fsys, Store: store}, nil }, filepath.ToSlash(work)
}

func TestNewCmd_HookApprovedRuns(t *testing.T) {
	deps, workDir := newHookTestDeps(t, "touch hook-ran.txt")
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"hooked", targetDir, "--var", "name=acme"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "hook-ran.txt")); err != nil {
		t.Errorf("hook-ran.txt should exist (the hook ran): %v", err)
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	approvals := newHookApprovals(d.FS, filepath.ToSlash(filepath.Dir(d.Store.Root())))
	if !approvals.IsApproved(hookApprovalKey("hooked", "1.0.0")) {
		t.Error("hooked@1.0.0 should be recorded as approved after confirming")
	}
}

func TestNewCmd_HookDeclined(t *testing.T) {
	deps, workDir := newHookTestDeps(t, "touch hook-ran.txt")
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("n\n"))
	cmd.SetArgs([]string{"hooked", targetDir, "--var", "name=acme"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Hooks declined") {
		t.Errorf("output = %q, want a decline message", out.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "hook-ran.txt")); err == nil {
		t.Error("hook-ran.txt should not exist: the hook was declined")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); err != nil {
		t.Errorf("README.md should still have been generated: %v", err)
	}
}

func TestNewCmd_HookFailure_FilesRemain(t *testing.T) {
	deps, workDir := newHookTestDeps(t, "exit 3")
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("y\n"))
	cmd.SetArgs([]string{"hooked", targetDir, "--var", "name=acme"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with a failing hook: want error, got nil")
	}
	if _, err := os.Stat(filepath.Join(targetDir, "README.md")); err != nil {
		t.Errorf("README.md should survive a failing hook: %v", err)
	}
}

func TestNewCmd_HookSkippedWithYesFlag(t *testing.T) {
	deps, workDir := newHookTestDeps(t, "touch hook-ran.txt")
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"hooked", targetDir, "--var", "name=acme", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Hooks skipped") {
		t.Errorf("output = %q, want a skip message", out.String())
	}
	if _, err := os.Stat(filepath.Join(targetDir, "hook-ran.txt")); err == nil {
		t.Error("hook-ran.txt should not exist: --yes must not run unapproved hooks")
	}
}

func TestNewCmd_HookApprovedOnceNotReprompted(t *testing.T) {
	deps, workDir := newHookTestDeps(t, "touch hook-ran.txt")

	cmd1 := newNewCmd(deps)
	out1 := &bytes.Buffer{}
	cmd1.SetOut(out1)
	cmd1.SetIn(strings.NewReader("y\n"))
	cmd1.SetArgs([]string{"hooked", workDir + "/proj1", "--var", "name=acme"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("Execute (first): %v, output: %s", err, out1.String())
	}

	// Second generation of the same template@version: no stdin available to
	// answer a prompt, so if it tried to prompt again this would fail to
	// read a confirmation and error out. It must not: the approval from the
	// first run is remembered.
	cmd2 := newNewCmd(deps)
	out2 := &bytes.Buffer{}
	cmd2.SetOut(out2)
	cmd2.SetIn(strings.NewReader(""))
	cmd2.SetArgs([]string{"hooked", workDir + "/proj2", "--var", "name=acme"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("Execute (second): %v, output: %s", err, out2.String())
	}
	if _, err := os.Stat(filepath.Join(workDir, "proj2", "hook-ran.txt")); err != nil {
		t.Errorf("hook-ran.txt should exist in the second project too (approval remembered): %v", err)
	}
}
