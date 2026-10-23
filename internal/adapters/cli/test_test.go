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

const testCmdManifest = `
[template]
name = "checked"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[test]
check = "%s"

[test.vars]
name = "sample-project"
`

// newTestCmdDeps builds Deps backed by the real filesystem: snap test
// spawns a real subprocess (the check command) in a real temp directory,
// so this needs real files to check against, like newHookTestDeps.
func newTestCmdDeps(t *testing.T, checkCommand string) func() (Deps, error) {
	t.Helper()
	root := t.TempDir()
	fsys := osfs.New()

	templatesDir := filepath.ToSlash(filepath.Join(root, "templates"))
	if err := fsys.MkdirAll(templatesDir+"/checked/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifest := strings.Replace(testCmdManifest, "%s", checkCommand, 1)
	if err := fsys.WriteFile(templatesDir+"/checked/1.0.0/snap.toml", []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(snap.toml): %v", err)
	}
	if err := fsys.WriteFile(templatesDir+"/checked/1.0.0/README.md", []byte("# {{name}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}

	store := local.New(fsys, templatesDir)
	return func() (Deps, error) { return Deps{FS: fsys, Store: store}, nil }
}

func TestTestCmd_CheckPasses(t *testing.T) {
	deps := newTestCmdDeps(t, "test -f README.md")
	cmd := newTestCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"checked"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "check passed") {
		t.Errorf("output = %q, want a pass confirmation", out.String())
	}
}

func TestTestCmd_CheckFails(t *testing.T) {
	deps := newTestCmdDeps(t, "exit 7")
	cmd := newTestCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"checked"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with a failing check: want error, got nil")
	}
	if !strings.Contains(out.String(), "check failed") {
		t.Errorf("output = %q, want a failure message", out.String())
	}
}

// snapTestScratchDirs counts os.TempDir() entries matching snap test's own
// "snap-test-*" prefix — not the directory's total entry count, which
// `go test ./...` makes flaky: other packages' tests run concurrently as
// separate processes sharing the same real os.TempDir(), each creating and
// removing their own unrelated temp entries throughout.
func snapTestScratchDirs(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		t.Fatalf("ReadDir(os.TempDir()): %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "snap-test-") {
			n++
		}
	}
	return n
}

func TestTestCmd_CleansUpTempDir(t *testing.T) {
	deps := newTestCmdDeps(t, "test -f README.md")
	before := snapTestScratchDirs(t)

	cmd := newTestCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"checked"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	after := snapTestScratchDirs(t)
	if after != before {
		t.Errorf("snap-test-* dirs in os.TempDir() after = %d, want %d (none left behind)", after, before)
	}
}

func TestTestCmd_DryRunWritesNothingAndRunsNothing(t *testing.T) {
	// A check command that would fail loudly if it ever ran — proving
	// --dry-run never executes it.
	deps := newTestCmdDeps(t, "exit 1")
	cmd := newTestCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"checked", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Check: exit 1") {
		t.Errorf("output = %q, want it to show the check command", out.String())
	}
}

func TestTestCmd_NoTestBlockIsError(t *testing.T) {
	root := t.TempDir()
	fsys := osfs.New()
	templatesDir := filepath.ToSlash(filepath.Join(root, "templates"))
	if err := fsys.MkdirAll(templatesDir+"/untested/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile(templatesDir+"/untested/1.0.0/snap.toml", []byte(`
[template]
name = "untested"
version = "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store := local.New(fsys, templatesDir)
	deps := func() (Deps, error) { return Deps{FS: fsys, Store: store}, nil }

	cmd := newTestCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"untested"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute on a template with no [test] block: want error, got nil")
	}
}
