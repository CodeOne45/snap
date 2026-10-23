package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/CodeOne45/snap/internal/adapters/osfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
)

const remoteManifest = `
[template]
name = "remote-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

// newRemoteTestRepo creates a real, local, non-bare git repository
// containing a valid snap template, and commits it. Its directory, given
// as a "file://" URL, is a git remote that go-git can clone/fetch from
// without touching the network — exercising the exact resolveRemoteTemplate
// path a real "gh:" or "https://" reference would take.
func newRemoteTestRepo(t *testing.T) (url string, firstCommit string) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	write := func(name, content string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	write("snap.toml", remoteManifest)
	write("README.md", "# {{name}}")

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hash, err := wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(0, 0)},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return "file://" + dir, hash.String()
}

// newRemoteCLITestDeps builds Deps backed by the real filesystem: gitcache
// always writes its clone/commit cache to real disk regardless of d.FS, so
// resolving a remote reference through the CLI needs a real target
// directory and a real (even if empty) local template store alongside it.
func newRemoteCLITestDeps(t *testing.T) (deps func() (Deps, error), workDir string) {
	t.Helper()
	root := t.TempDir()
	fsys := osfs.New()

	templatesDir := filepath.ToSlash(filepath.Join(root, "templates"))
	if err := fsys.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	store := local.New(fsys, templatesDir)

	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("MkdirAll(work): %v", err)
	}
	return func() (Deps, error) { return Deps{FS: fsys, Store: store}, nil }, filepath.ToSlash(work)
}

func TestNewCmd_RemoteTemplate(t *testing.T) {
	url, commit := newRemoteTestRepo(t)
	deps, workDir := newRemoteCLITestDeps(t)
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{url, targetDir, "--var", "name=acme", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(README.md): %v", err)
	}
	if string(content) != "# acme" {
		t.Errorf("README.md = %q, want %q", content, "# acme")
	}

	origin, err := os.ReadFile(filepath.Join(targetDir, ".snap", "origin.toml"))
	if err != nil {
		t.Fatalf("ReadFile(origin.toml): %v", err)
	}
	wantSource := url + "@" + commit
	if !strings.Contains(string(origin), wantSource) {
		t.Errorf("origin.toml = %q, want it to contain source %q", origin, wantSource)
	}
}

func TestNewCmd_RemoteTemplate_PinnedTag(t *testing.T) {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	write("snap.toml", remoteManifest)
	write("README.md", "# {{name}} v1")
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("Add: %v", err)
	}
	firstCommit, err := wt.Commit("v1", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(0, 0)},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := repo.CreateTag("v1.0.0", firstCommit, nil); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	write("README.md", "# {{name}} v2")
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err = wt.Commit("v2", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(0, 0)},
	}); err != nil {
		t.Fatalf("Commit (v2): %v", err)
	}

	url := "file://" + dir + "@v1.0.0"
	deps, workDir := newRemoteCLITestDeps(t)
	targetDir := workDir + "/proj"

	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{url, targetDir, "--var", "name=acme", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	content, err := os.ReadFile(filepath.Join(targetDir, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile(README.md): %v", err)
	}
	if string(content) != "# acme v1" {
		t.Errorf("README.md = %q, want %q (pinned to v1.0.0, not the later v2 commit)", content, "# acme v1")
	}
}
