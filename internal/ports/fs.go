// Package ports defines the interfaces through which the core use cases
// perform all I/O. Adapters implement these; core depends only on them.
package ports

import "io/fs"

// FileInfo is a filesystem-agnostic description of a file or directory.
type FileInfo struct {
	Path  string
	Size  int64
	Mode  fs.FileMode
	IsDir bool
}

// FS is the filesystem port. Implementations back it with the real
// filesystem, an in-memory store, or anything else.
type FS interface {
	Stat(path string) (FileInfo, error)
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]FileInfo, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(path string) error
	// RemoveAll removes path and, if it is a directory, everything under
	// it. Removing a path that doesn't exist is not an error.
	RemoveAll(path string) error
	Rename(oldPath, newPath string) error
}
