package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

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
// containing a valid snap template and commits it, returning it as a
// "file://" URL — a git remote go-git can clone/fetch from without
// touching the network.
func newRemoteTestRepo(t *testing.T) string {
	t.Helper()
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
	write("README.md", "# {{name}}")

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if _, err := wt.Add("."); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(0, 0)},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return "file://" + dir
}

// newRemoteTestServer wires an MCP server backed by the real filesystem:
// gitcache always writes its clone/commit cache to real disk regardless of
// h.fs, so resolving a remote reference needs a real target directory and
// a real (even if empty) local template store alongside it — unlike
// newTestServer's memfs harness used for every other test in this package.
func newRemoteTestServer(t *testing.T) (*testServer, string) {
	t.Helper()
	root := t.TempDir()
	fsys := osfs.New()
	templatesDir := filepath.ToSlash(filepath.Join(root, "templates"))
	if err := fsys.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	store := local.New(fsys, templatesDir)
	hooks := &fakeHookRunner{}
	server := NewServer(fsys, store, hooks, "test")

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test-client"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	work := filepath.ToSlash(filepath.Join(root, "work"))
	if err := fsys.MkdirAll(work, 0o755); err != nil {
		t.Fatalf("MkdirAll(work): %v", err)
	}
	return &testServer{fs: fsys, store: store, hooks: hooks, cs: cs}, work
}

func TestCreateProject_RemoteTemplate(t *testing.T) {
	url := newRemoteTestRepo(t)
	s, work := newRemoteTestServer(t)
	targetDir := work + "/proj"

	res := s.callTool(t, "create_project", planProjectIn{
		Template:  url,
		Variables: map[string]string{"name": "acme"},
		TargetDir: targetDir,
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
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
	if !strings.Contains(string(origin), url+"@") {
		t.Errorf("origin.toml = %q, want it to record source %q + a resolved commit", origin, url)
	}
}

func TestPlanProject_RemoteTemplate(t *testing.T) {
	url := newRemoteTestRepo(t)
	s, work := newRemoteTestServer(t)

	res := s.callTool(t, "plan_project", planProjectIn{
		Template:  url,
		Variables: map[string]string{"name": "acme"},
		TargetDir: work + "/proj",
	})
	if res.IsError {
		t.Fatalf("plan_project returned an error: %s", s.text(t, res))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(s.text(t, res)), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// plan_project must not write anything.
	if _, err := os.Stat(work + "/proj"); err == nil {
		t.Error("plan_project must not write anything, but the target dir exists")
	}
}
