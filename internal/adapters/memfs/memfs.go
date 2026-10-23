// Package memfs implements ports.FS entirely in memory, for use in core
// unit tests and golden tests where no real filesystem access is wanted.
package memfs

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/CodeOne45/snap/internal/ports"
)

type entry struct {
	data  []byte
	mode  fs.FileMode
	isDir bool
}

// MemFS is an in-memory implementation of ports.FS.
type MemFS struct {
	mu      sync.RWMutex
	entries map[string]*entry // cleaned path ("" is root) -> entry
}

// New returns an empty MemFS.
func New() *MemFS {
	return &MemFS{entries: make(map[string]*entry)}
}

// clean normalizes a path to forward-slash, no leading/trailing slash form.
// The root directory is represented by "".
func clean(p string) string {
	p = path.Clean(filepath.ToSlash(p))
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func notExist(op, p string) error {
	return fmt.Errorf("memfs: %s %s: %w", op, p, fs.ErrNotExist)
}

// Stat implements ports.FS.
func (m *MemFS) Stat(p string) (ports.FileInfo, error) {
	p = clean(p)
	m.mu.RLock()
	defer m.mu.RUnlock()

	if p == "" {
		return ports.FileInfo{Path: "", Mode: fs.ModeDir, IsDir: true}, nil
	}
	e, ok := m.entries[p]
	if !ok {
		return ports.FileInfo{}, notExist("stat", p)
	}
	return ports.FileInfo{Path: p, Size: int64(len(e.data)), Mode: e.mode, IsDir: e.isDir}, nil
}

// ReadFile implements ports.FS.
func (m *MemFS) ReadFile(p string) ([]byte, error) {
	p = clean(p)
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, ok := m.entries[p]
	if !ok {
		return nil, notExist("read", p)
	}
	if e.isDir {
		return nil, fmt.Errorf("memfs: read %s: is a directory", p)
	}
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, nil
}

// ReadDir implements ports.FS.
func (m *MemFS) ReadDir(p string) ([]ports.FileInfo, error) {
	p = clean(p)
	m.mu.RLock()
	defer m.mu.RUnlock()

	if p != "" {
		e, ok := m.entries[p]
		if !ok {
			return nil, notExist("readdir", p)
		}
		if !e.isDir {
			return nil, fmt.Errorf("memfs: readdir %s: not a directory", p)
		}
	}

	var out []ports.FileInfo
	for childPath, e := range m.entries {
		if path.Dir(childPath) == dirKey(p) {
			out = append(out, ports.FileInfo{
				Path:  childPath,
				Size:  int64(len(e.data)),
				Mode:  e.mode,
				IsDir: e.isDir,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// dirKey mirrors what path.Dir returns for a root-relative child: path.Dir
// of a top-level entry "foo" is ".", which is how we address the root here.
func dirKey(p string) string {
	if p == "" {
		return "."
	}
	return p
}

// WriteFile implements ports.FS.
func (m *MemFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	p = clean(p)
	if p == "" {
		return fmt.Errorf("memfs: write %s: is a directory", p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	parent := clean(path.Dir(p))
	if parent != "" {
		pe, ok := m.entries[parent]
		if !ok || !pe.isDir {
			return fmt.Errorf("memfs: write %s: parent directory does not exist", p)
		}
	}
	if existing, ok := m.entries[p]; ok && existing.isDir {
		return fmt.Errorf("memfs: write %s: is a directory", p)
	}

	buf := make([]byte, len(data))
	copy(buf, data)
	m.entries[p] = &entry{data: buf, mode: perm}
	return nil
}

// MkdirAll implements ports.FS.
func (m *MemFS) MkdirAll(p string, perm fs.FileMode) error {
	p = clean(p)
	if p == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	parts := strings.Split(p, "/")
	cur := ""
	for _, part := range parts {
		if cur == "" {
			cur = part
		} else {
			cur = cur + "/" + part
		}
		if e, ok := m.entries[cur]; ok {
			if !e.isDir {
				return fmt.Errorf("memfs: mkdir %s: not a directory", cur)
			}
			continue
		}
		m.entries[cur] = &entry{isDir: true, mode: perm | fs.ModeDir}
	}
	return nil
}

// Remove implements ports.FS.
func (m *MemFS) Remove(p string) error {
	p = clean(p)
	if p == "" {
		return fmt.Errorf("memfs: remove %s: refusing to remove root", p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[p]
	if !ok {
		return notExist("remove", p)
	}
	if e.isDir {
		prefix := p + "/"
		for childPath := range m.entries {
			if strings.HasPrefix(childPath, prefix) {
				return fmt.Errorf("memfs: remove %s: directory not empty", p)
			}
		}
	}
	delete(m.entries, p)
	return nil
}

// RemoveAll implements ports.FS.
func (m *MemFS) RemoveAll(p string) error {
	p = clean(p)
	if p == "" {
		return fmt.Errorf("memfs: removeall %s: refusing to remove root", p)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.entries[p]; !ok {
		return nil
	}
	delete(m.entries, p)
	prefix := p + "/"
	for childPath := range m.entries {
		if strings.HasPrefix(childPath, prefix) {
			delete(m.entries, childPath)
		}
	}
	return nil
}

// Rename implements ports.FS.
func (m *MemFS) Rename(oldPath, newPath string) error {
	oldPath = clean(oldPath)
	newPath = clean(newPath)
	if oldPath == "" || newPath == "" {
		return fmt.Errorf("memfs: rename %s -> %s: cannot rename root", oldPath, newPath)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	e, ok := m.entries[oldPath]
	if !ok {
		return notExist("rename", oldPath)
	}
	newParent := clean(path.Dir(newPath))
	if newParent != "" {
		pe, ok := m.entries[newParent]
		if !ok || !pe.isDir {
			return fmt.Errorf("memfs: rename %s -> %s: parent directory does not exist", oldPath, newPath)
		}
	}

	if !e.isDir {
		m.entries[newPath] = e
		delete(m.entries, oldPath)
		return nil
	}

	prefix := oldPath + "/"
	moved := map[string]*entry{newPath: e}
	for childPath, ce := range m.entries {
		if strings.HasPrefix(childPath, prefix) {
			moved[newPath+"/"+strings.TrimPrefix(childPath, prefix)] = ce
		}
	}
	for childPath := range m.entries {
		if childPath == oldPath || strings.HasPrefix(childPath, prefix) {
			delete(m.entries, childPath)
		}
	}
	for newChildPath, ce := range moved {
		m.entries[newChildPath] = ce
	}
	return nil
}

var _ ports.FS = (*MemFS)(nil)
