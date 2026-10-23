package gitcache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/CodeOne45/snap/internal/core/remote"
)

// testRemote creates a real, local, non-bare git repository to act as a
// "remote" for Cache to clone/fetch from — fully offline (clone sources
// are plain local paths, which git/go-git support natively), so these
// tests never touch the network.
type testRemote struct {
	dir  string
	repo *git.Repository
}

func newTestRemote(t *testing.T) *testRemote {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return &testRemote{dir: dir, repo: repo}
}

func (r *testRemote) commit(t *testing.T, files map[string]string, msg string) string {
	t.Helper()
	wt, err := r.repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(r.dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("Add(%q): %v", name, err)
		}
	}
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(0, 0)},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return hash.String()
}

func (r *testRemote) tag(t *testing.T, name, commitHash string) {
	t.Helper()
	if _, err := r.repo.CreateTag(name, plumbing.NewHash(commitHash), nil); err != nil {
		t.Fatalf("CreateTag(%q): %v", name, err)
	}
}

func TestCache_ResolveUnpinnedUsesHEAD(t *testing.T) {
	remoteRepo := newTestRemote(t)
	remoteRepo.commit(t, map[string]string{"snap.toml": "v1", "README.md": "first"}, "first commit")

	c := New(t.TempDir())
	dir, commit, err := c.Resolve(remote.Ref{URL: remoteRepo.dir})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if commit == "" {
		t.Error("commit hash is empty")
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(content) != "first" {
		t.Errorf("README.md = %q, %v, want %q", content, err, "first")
	}
}

func TestCache_ResolvePinnedTag(t *testing.T) {
	remoteRepo := newTestRemote(t)
	firstCommit := remoteRepo.commit(t, map[string]string{"README.md": "v1 content"}, "v1")
	remoteRepo.tag(t, "v1.0.0", firstCommit)
	remoteRepo.commit(t, map[string]string{"README.md": "v2 content"}, "v2")

	c := New(t.TempDir())
	dir, commit, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if commit != firstCommit {
		t.Errorf("commit = %q, want the tagged commit %q", commit, firstCommit)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(content) != "v1 content" {
		t.Errorf("README.md = %q, %v, want %q (pinned to v1.0.0, not HEAD)", content, err, "v1 content")
	}
}

func TestCache_PinnedReproducibleAfterRemoteChanges(t *testing.T) {
	remoteRepo := newTestRemote(t)
	firstCommit := remoteRepo.commit(t, map[string]string{"README.md": "v1 content"}, "v1")
	remoteRepo.tag(t, "v1.0.0", firstCommit)

	c := New(t.TempDir())
	_, commit1, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve (first): %v", err)
	}

	// The remote moves on after we've pinned to v1.0.0.
	remoteRepo.commit(t, map[string]string{"README.md": "v2 content"}, "v2")

	dir2, commit2, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve (second): %v", err)
	}
	if commit2 != commit1 {
		t.Errorf("commit changed across resolves of the same pin: %q -> %q", commit1, commit2)
	}
	content, err := os.ReadFile(filepath.Join(dir2, "README.md"))
	if err != nil || string(content) != "v1 content" {
		t.Errorf("README.md = %q, %v, want the pinned v1 content, unaffected by the later commit", content, err)
	}
}

func TestCache_OfflineReproducibleFromCache(t *testing.T) {
	remoteRepo := newTestRemote(t)
	firstCommit := remoteRepo.commit(t, map[string]string{"README.md": "v1 content"}, "v1")
	remoteRepo.tag(t, "v1.0.0", firstCommit)

	c := New(t.TempDir())
	if _, _, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: "v1.0.0"}); err != nil {
		t.Fatalf("Resolve (first, online): %v", err)
	}

	// Simulate the remote becoming unreachable entirely.
	if err := os.RemoveAll(remoteRepo.dir); err != nil {
		t.Fatalf("RemoveAll(remote): %v", err)
	}

	dir, commit, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: "v1.0.0"})
	if err != nil {
		t.Fatalf("Resolve (second, offline): want success from cache, got error: %v", err)
	}
	if commit != firstCommit {
		t.Errorf("commit = %q, want %q", commit, firstCommit)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(content) != "v1 content" {
		t.Errorf("README.md = %q, %v, want the cached v1 content", content, err)
	}
}

func TestCache_ResolvePinnedCommitHash(t *testing.T) {
	remoteRepo := newTestRemote(t)
	firstCommit := remoteRepo.commit(t, map[string]string{"README.md": "v1 content"}, "v1")
	remoteRepo.commit(t, map[string]string{"README.md": "v2 content"}, "v2")

	c := New(t.TempDir())
	dir, commit, err := c.Resolve(remote.Ref{URL: remoteRepo.dir, Pin: firstCommit})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if commit != firstCommit {
		t.Errorf("commit = %q, want %q", commit, firstCommit)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(content) != "v1 content" {
		t.Errorf("README.md = %q, %v, want %q", content, err, "v1 content")
	}
}

func TestCache_UnpinnedPicksUpRemoteChanges(t *testing.T) {
	remoteRepo := newTestRemote(t)
	remoteRepo.commit(t, map[string]string{"README.md": "v1 content"}, "v1")

	c := New(t.TempDir())
	if _, _, err := c.Resolve(remote.Ref{URL: remoteRepo.dir}); err != nil {
		t.Fatalf("Resolve (first): %v", err)
	}

	secondCommit := remoteRepo.commit(t, map[string]string{"README.md": "v2 content"}, "v2")

	dir, commit, err := c.Resolve(remote.Ref{URL: remoteRepo.dir})
	if err != nil {
		t.Fatalf("Resolve (second): %v", err)
	}
	if commit != secondCommit {
		t.Errorf("commit = %q, want the new HEAD %q (unpinned should track the remote)", commit, secondCommit)
	}
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil || string(content) != "v2 content" {
		t.Errorf("README.md = %q, %v, want %q", content, err, "v2 content")
	}
}
