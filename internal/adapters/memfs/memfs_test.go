package memfs

import (
	"errors"
	"io/fs"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	m := New()
	if err := m.MkdirAll("src", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := m.ReadFile("src/main.go")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "package main" {
		t.Errorf("ReadFile = %q, want %q", got, "package main")
	}
}

func TestWriteFile_MissingParent(t *testing.T) {
	m := New()
	if err := m.WriteFile("src/main.go", []byte("x"), 0o644); err == nil {
		t.Fatal("WriteFile into missing parent dir: want error, got nil")
	}
}

func TestReadFile_NotExist(t *testing.T) {
	m := New()
	_, err := m.ReadFile("missing.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile error = %v, want errors.Is(fs.ErrNotExist)", err)
	}
}

func TestMkdirAll_ReadDir(t *testing.T) {
	m := New()
	if err := m.MkdirAll("a/b/c", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("a/b/file1.txt", []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.WriteFile("a/b/file2.txt", []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := m.ReadDir("a/b")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 3 { // c/, file1.txt, file2.txt
		t.Fatalf("ReadDir returned %d entries, want 3: %+v", len(entries), entries)
	}
	// Deterministic order: sorted by path.
	want := []string{"a/b/c", "a/b/file1.txt", "a/b/file2.txt"}
	for i, e := range entries {
		if e.Path != want[i] {
			t.Errorf("entries[%d].Path = %q, want %q", i, e.Path, want[i])
		}
	}
	if !entries[0].IsDir {
		t.Errorf("entries[0] (%q) should be a directory", entries[0].Path)
	}
}

func TestReadDir_Root(t *testing.T) {
	m := New()
	if err := m.WriteFile("top.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err := m.ReadDir("")
	if err != nil {
		t.Fatalf("ReadDir(root): %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "top.txt" {
		t.Fatalf("ReadDir(root) = %+v, want [top.txt]", entries)
	}
}

func TestStat(t *testing.T) {
	m := New()
	if err := m.MkdirAll("dir", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("dir/file.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	fi, err := m.Stat("dir/file.txt")
	if err != nil {
		t.Fatalf("Stat(file): %v", err)
	}
	if fi.IsDir || fi.Size != 5 {
		t.Errorf("Stat(file) = %+v, want IsDir=false Size=5", fi)
	}

	fi, err = m.Stat("dir")
	if err != nil {
		t.Fatalf("Stat(dir): %v", err)
	}
	if !fi.IsDir {
		t.Errorf("Stat(dir).IsDir = false, want true")
	}

	if _, err := m.Stat("nope"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(missing) error = %v, want errors.Is(fs.ErrNotExist)", err)
	}
}

func TestRemove(t *testing.T) {
	m := New()
	if err := m.MkdirAll("dir", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("dir/file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.Remove("dir"); err == nil {
		t.Fatal("Remove(non-empty dir): want error, got nil")
	}
	if err := m.Remove("dir/file.txt"); err != nil {
		t.Fatalf("Remove(file): %v", err)
	}
	if err := m.Remove("dir"); err != nil {
		t.Fatalf("Remove(empty dir): %v", err)
	}
	if _, err := m.Stat("dir"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat after Remove: want fs.ErrNotExist, got %v", err)
	}
}

func TestRemoveAll(t *testing.T) {
	m := New()
	if err := m.MkdirAll("dir/sub", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("dir/file.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.WriteFile("dir/sub/nested.txt", []byte("y"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.RemoveAll("dir"); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	for _, p := range []string{"dir", "dir/file.txt", "dir/sub", "dir/sub/nested.txt"} {
		if _, err := m.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Stat(%q) after RemoveAll = %v, want fs.ErrNotExist", p, err)
		}
	}

	// Removing a path that doesn't exist is not an error.
	if err := m.RemoveAll("nope"); err != nil {
		t.Errorf("RemoveAll(missing) = %v, want nil", err)
	}
}

func TestRename(t *testing.T) {
	m := New()
	if err := m.MkdirAll("src", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := m.WriteFile("src/a.txt", []byte("1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := m.WriteFile("src/b.txt", []byte("2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := m.Rename("src", "dst"); err != nil {
		t.Fatalf("Rename(dir): %v", err)
	}

	if _, err := m.Stat("src"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat(old dir) after rename: want fs.ErrNotExist, got %v", err)
	}
	got, err := m.ReadFile("dst/a.txt")
	if err != nil {
		t.Fatalf("ReadFile(dst/a.txt): %v", err)
	}
	if string(got) != "1" {
		t.Errorf("ReadFile(dst/a.txt) = %q, want %q", got, "1")
	}
	if _, err := m.ReadFile("dst/b.txt"); err != nil {
		t.Fatalf("ReadFile(dst/b.txt): %v", err)
	}
}
