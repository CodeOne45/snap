// Package gitcache resolves a remote.Ref to a local, on-disk snapshot of
// the exact commit it points to, fetching from the network only when the
// commit isn't already cached — so a pinned ref is reproducible offline
// once fetched once. This is the only part of snap that clones/fetches
// git repositories; it writes to its own cache directory under the user's
// config dir, not to a user-chosen target (that still only ever happens
// through internal/executor).
package gitcache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/CodeOne45/snap/internal/core/remote"
	"github.com/CodeOne45/snap/internal/ports"
)

// Cache manages local clones of remote template repositories under root
// (e.g. "~/.config/snap/cache"), one subdirectory per unique URL.
type Cache struct {
	root string
}

// New returns a Cache rooted at root.
func New(root string) *Cache {
	return &Cache{root: root}
}

// Resolve ensures ref's pinned commit (or the remote's current default
// branch HEAD, if unpinned) is available locally as a plain directory of
// files, fetching from the network only if that exact commit isn't
// already cached. It returns that directory and the resolved commit hash,
// for .snap/origin.toml.
func (c *Cache) Resolve(ref remote.Ref) (dir string, commit string, err error) {
	urlDir := filepath.Join(c.root, urlDigest(ref.URL))
	cloneDir := filepath.Join(urlDir, "repo")

	repo, err := c.openOrClone(cloneDir, ref.URL)
	if err != nil {
		return "", "", fmt.Errorf("gitcache: %s: %w", ref.URL, err)
	}

	hash, err := c.resolveCommit(repo, ref)
	if err != nil {
		return "", "", fmt.Errorf("gitcache: %s: %w", ref.URL, err)
	}

	snapshotDir := filepath.Join(urlDir, "commits", hash.String())
	if _, statErr := os.Stat(snapshotDir); statErr != nil {
		if err := exportCommit(repo, hash, snapshotDir); err != nil {
			return "", "", fmt.Errorf("gitcache: %s: export %s: %w", ref.URL, hash, err)
		}
	}
	return snapshotDir, hash.String(), nil
}

// openOrClone opens an existing cached clone, or creates one if this is
// the first time url has been resolved.
func (c *Cache) openOrClone(cloneDir, url string) (*git.Repository, error) {
	repo, err := git.PlainOpen(cloneDir)
	if err == nil {
		return repo, nil
	}
	if !errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("open cache: %w", err)
	}
	repo, err = git.PlainClone(cloneDir, false, &git.CloneOptions{URL: url, Tags: git.AllTags})
	if err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	return repo, nil
}

// resolveCommit resolves ref.Pin (or HEAD, if unpinned) to a commit hash,
// fetching from the network first unless the pin is already resolvable
// from what's cached locally — so a previously-resolved pinned commit
// never needs the network again.
func (c *Cache) resolveCommit(repo *git.Repository, ref remote.Ref) (plumbing.Hash, error) {
	if ref.Pin != "" {
		if hash, err := repo.ResolveRevision(plumbing.Revision(ref.Pin)); err == nil {
			return *hash, nil
		}
	}

	if err := fetch(repo); err != nil {
		// Unpinned and offline: fall back to whatever was last fetched.
		if ref.Pin == "" {
			if hash, headErr := resolveDefaultBranch(repo); headErr == nil {
				return hash, nil
			}
		}
		return plumbing.ZeroHash, fmt.Errorf("fetch: %w", err)
	}

	if ref.Pin != "" {
		hash, err := repo.ResolveRevision(plumbing.Revision(ref.Pin))
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("resolve %q: %w", ref.Pin, err)
		}
		return *hash, nil
	}
	return resolveDefaultBranch(repo)
}

// resolveDefaultBranch resolves the remote's default branch tip. Fetch
// only updates remote-tracking refs (refs/remotes/origin/*), never the
// local checked-out branch — so "unpinned" must resolve the
// remote-tracking ref for the current branch (e.g. refs/remotes/origin/
// master), not local HEAD, or a second resolve after the remote has moved
// on would silently return the stale commit from the first clone.
func resolveDefaultBranch(repo *git.Repository) (plumbing.Hash, error) {
	head, err := repo.Head()
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("resolve default branch: %w", err)
	}
	if head.Name().IsBranch() {
		remoteRef := plumbing.NewRemoteReferenceName("origin", head.Name().Short())
		if hash, err := repo.ResolveRevision(plumbing.Revision(remoteRef)); err == nil {
			return *hash, nil
		}
	}
	return head.Hash(), nil
}

func fetch(repo *git.Repository) error {
	err := repo.Fetch(&git.FetchOptions{Tags: git.AllTags})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

// exportCommit writes every file in commit's tree into dir, a plain
// directory with no git metadata of its own — exactly what
// internal/core/usecase.New/.Capture expect a template root to look like.
func exportCommit(repo *git.Repository, hash plumbing.Hash, dir string) error {
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return fmt.Errorf("load commit %s: %w", hash, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("load tree: %w", err)
	}

	tmp := dir + ".tmp"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	err = tree.Files().ForEach(func(f *object.File) error {
		content, err := f.Contents()
		if err != nil {
			return fmt.Errorf("read %s: %w", f.Name, err)
		}
		mode, err := f.Mode.ToOSFileMode()
		if err != nil {
			mode = 0o644
		}
		dest := filepath.Join(tmp, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", f.Name, err)
		}
		return os.WriteFile(dest, []byte(content), mode)
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

func urlDigest(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])[:16]
}

// SingleTemplateStore is a ports.Store that always resolves to the same
// directory regardless of the name (or version) asked for. A remote
// template reference (unlike a local template) isn't looked up by name in
// an enumerable, multi-version store — Cache.Resolve already found its one
// directory directly, with any version pinning (M11's "@tag"/"@commit")
// already resolved at the git level before this is ever constructed.
type SingleTemplateStore struct {
	Dir string
}

// List implements ports.Store. A remote reference isn't a browsable
// collection of templates, so this always returns none.
func (s SingleTemplateStore) List() ([]string, error) { return nil, nil }

// Path implements ports.Store, ignoring name.
func (s SingleTemplateStore) Path(string) (string, error) { return s.Dir, nil }

// Versions implements ports.Store. A resolved remote reference is always
// exactly one directory — versioning for remote templates happens at the
// git level (a tag or commit pin), not through this interface.
func (s SingleTemplateStore) Versions(string) ([]string, error) { return nil, nil }

// PathVersion implements ports.Store, ignoring both name and version.
func (s SingleTemplateStore) PathVersion(string, string) (string, error) { return s.Dir, nil }

// Root implements ports.Store.
func (s SingleTemplateStore) Root() string { return s.Dir }

var _ ports.Store = SingleTemplateStore{}
