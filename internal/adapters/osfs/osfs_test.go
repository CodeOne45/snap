package osfs

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f := New()

	p := filepath.ToSlash(filepath.Join(dir, "sub", "file.txt"))
	if err := f.MkdirAll(filepath.ToSlash(filepath.Join(dir, "sub")), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := f.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := f.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("ReadFile = %q, want %q", got, "hello")
	}
}

func TestStatAndReadDir(t *testing.T) {
	dir := t.TempDir()
	f := New()
	root := filepath.ToSlash(dir)

	if err := f.WriteFile(root+"/a.txt", []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := f.MkdirAll(root+"/sub", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	fi, err := f.Stat(root + "/a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.IsDir || fi.Size != 1 {
		t.Errorf("Stat(a.txt) = %+v, want file of size 1", fi)
	}

	entries, err := f.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadDir returned %d entries, want 2: %+v", len(entries), entries)
	}
}

func TestRemoveAllAndNotExist(t *testing.T) {
	dir := t.TempDir()
	f := New()
	root := filepath.ToSlash(dir)

	if err := f.MkdirAll(root+"/a/b", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := f.WriteFile(root+"/a/b/f.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := f.RemoveAll(root + "/a"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := f.Stat(root + "/a"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat after RemoveAll = %v, want fs.ErrNotExist", err)
	}
}

func TestRename(t *testing.T) {
	dir := t.TempDir()
	f := New()
	root := filepath.ToSlash(dir)

	if err := f.WriteFile(root+"/old.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := f.Rename(root+"/old.txt", root+"/new.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := f.ReadFile(root + "/new.txt"); err != nil {
		t.Fatalf("ReadFile(new.txt): %v", err)
	}
}
