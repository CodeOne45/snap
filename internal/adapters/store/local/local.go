// Package local implements ports.Store as a plain folder of template
// directories, each holding one subdirectory per version (the MVP+v1 local
// template store).
package local

import (
	"fmt"
	"path"
	"sort"

	"github.com/CodeOne45/snap/internal/core/semver"
	"github.com/CodeOne45/snap/internal/ports"
)

// Store is a ports.Store backed by a directory of template subdirectories,
// each in turn holding one subdirectory per semver version
// (root/<name>/<version>/snap.toml), read through an FS port (real or
// in-memory).
type Store struct {
	fs   ports.FS
	root string
}

// New returns a Store listing template directories under root.
func New(fs ports.FS, root string) *Store {
	return &Store{fs: fs, root: root}
}

// List implements ports.Store.
func (s *Store) List() ([]string, error) {
	entries, err := s.fs.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("store: list %s: %w", s.root, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir {
			names = append(names, path.Base(e.Path))
		}
	}
	sort.Strings(names)
	return names, nil
}

// Versions implements ports.Store: every version subdirectory of name,
// sorted ascending (oldest first). Every entry must be a valid semver
// directory name — the store only ever creates them that way, so anything
// else indicates the store was hand-edited incorrectly.
func (s *Store) Versions(name string) ([]string, error) {
	nameDir := path.Join(s.root, name)
	entries, err := s.fs.ReadDir(nameDir)
	if err != nil {
		return nil, fmt.Errorf("store: template %q not found: %w", name, err)
	}
	versions := make([]semver.Version, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir {
			continue
		}
		v, err := semver.Parse(path.Base(e.Path))
		if err != nil {
			return nil, fmt.Errorf("store: template %q: invalid version directory %q: %w", name, path.Base(e.Path), err)
		}
		versions = append(versions, v)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("store: template %q has no versions", name)
	}
	sort.Slice(versions, func(i, j int) bool { return semver.Compare(versions[i], versions[j]) < 0 })

	out := make([]string, len(versions))
	for i, v := range versions {
		out[i] = v.String()
	}
	return out, nil
}

// Path implements ports.Store: the directory of name's latest (highest
// semver) version.
func (s *Store) Path(name string) (string, error) {
	versions, err := s.Versions(name)
	if err != nil {
		return "", err
	}
	return s.PathVersion(name, versions[len(versions)-1])
}

// PathVersion implements ports.Store.
func (s *Store) PathVersion(name, version string) (string, error) {
	p := path.Join(s.root, name, version)
	fi, err := s.fs.Stat(p)
	if err != nil {
		return "", fmt.Errorf("store: template %q version %q not found: %w", name, version, err)
	}
	if !fi.IsDir {
		return "", fmt.Errorf("store: %q is not a directory", p)
	}
	return p, nil
}

// Root implements ports.Store.
func (s *Store) Root() string {
	return s.root
}

var _ ports.Store = (*Store)(nil)
