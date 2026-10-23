// Package osfs implements ports.FS over the real operating system
// filesystem. This is the only adapter wired up for actual disk I/O; tests
// elsewhere use memfs instead.
package osfs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/CodeOne45/snap/internal/ports"
)

// OSFS is a ports.FS backed by the real filesystem. Paths are forward-slash
// (like all ports.FS paths) and converted to the host's native separator.
type OSFS struct{}

// New returns an OSFS.
func New() OSFS { return OSFS{} }

func toNative(p string) string { return filepath.FromSlash(p) }

func toPortable(p string) string { return filepath.ToSlash(p) }

// Stat implements ports.FS.
func (OSFS) Stat(p string) (ports.FileInfo, error) {
	fi, err := os.Stat(toNative(p))
	if err != nil {
		return ports.FileInfo{}, fmt.Errorf("osfs: stat %s: %w", p, err)
	}
	return ports.FileInfo{Path: p, Size: fi.Size(), Mode: fi.Mode(), IsDir: fi.IsDir()}, nil
}

// ReadFile implements ports.FS.
func (OSFS) ReadFile(p string) ([]byte, error) {
	data, err := os.ReadFile(toNative(p))
	if err != nil {
		return nil, fmt.Errorf("osfs: read %s: %w", p, err)
	}
	return data, nil
}

// ReadDir implements ports.FS.
func (OSFS) ReadDir(p string) ([]ports.FileInfo, error) {
	entries, err := os.ReadDir(toNative(p))
	if err != nil {
		return nil, fmt.Errorf("osfs: readdir %s: %w", p, err)
	}
	out := make([]ports.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("osfs: readdir %s: %w", p, err)
		}
		childPath := toPortable(filepath.Join(toNative(p), e.Name()))
		out = append(out, ports.FileInfo{Path: childPath, Size: info.Size(), Mode: info.Mode(), IsDir: e.IsDir()})
	}
	return out, nil
}

// WriteFile implements ports.FS.
func (OSFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	if err := os.WriteFile(toNative(p), data, perm); err != nil {
		return fmt.Errorf("osfs: write %s: %w", p, err)
	}
	return nil
}

// MkdirAll implements ports.FS.
func (OSFS) MkdirAll(p string, perm fs.FileMode) error {
	if err := os.MkdirAll(toNative(p), perm); err != nil {
		return fmt.Errorf("osfs: mkdir %s: %w", p, err)
	}
	return nil
}

// Remove implements ports.FS.
func (OSFS) Remove(p string) error {
	if err := os.Remove(toNative(p)); err != nil {
		return fmt.Errorf("osfs: remove %s: %w", p, err)
	}
	return nil
}

// RemoveAll implements ports.FS.
func (OSFS) RemoveAll(p string) error {
	if err := os.RemoveAll(toNative(p)); err != nil {
		return fmt.Errorf("osfs: removeall %s: %w", p, err)
	}
	return nil
}

// Rename implements ports.FS.
func (OSFS) Rename(oldPath, newPath string) error {
	if err := os.Rename(toNative(oldPath), toNative(newPath)); err != nil {
		return fmt.Errorf("osfs: rename %s -> %s: %w", oldPath, newPath, err)
	}
	return nil
}

var _ ports.FS = OSFS{}
