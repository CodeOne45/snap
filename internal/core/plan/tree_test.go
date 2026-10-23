package plan

import (
	"strings"
	"testing"
)

func TestTree(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.CreateDir("src"); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.WriteFile("src/main.go", []byte("package main"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := b.RunHook("git init"); err != nil {
		t.Fatalf("RunHook: %v", err)
	}

	got := Tree(b.Build())
	wantLines := []string{"myproj/", "README.md", "src/", "main.go", "Hooks:", "git init"}
	for _, want := range wantLines {
		if !strings.Contains(got, want) {
			t.Errorf("Tree() = %q, want it to contain %q", got, want)
		}
	}
	// Sorted by path, not Op order: README.md (built second) still comes
	// before src/main.go (built third) because "R" < "s".
	if strings.Index(got, "README.md") > strings.Index(got, "main.go") {
		t.Errorf("Tree() = %q, want entries sorted by path", got)
	}
}

func TestTree_ImpliesParentDirsWithoutExplicitCreateDirOp(t *testing.T) {
	b := NewBuilder("myproj")
	// .snap/origin.toml is written via WriteFile alone, with no
	// corresponding CreateDir for .snap — the executor creates it as
	// needed. Tree must still show it.
	if err := b.WriteFile(".snap/origin.toml", []byte("template = \"x\""), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := Tree(b.Build())
	if !strings.Contains(got, ".snap/") {
		t.Errorf("Tree() = %q, want it to show the implied .snap/ directory", got)
	}
	if !strings.Contains(got, "origin.toml") {
		t.Errorf("Tree() = %q, want it to show origin.toml", got)
	}
}

func TestTree_NoHooks(t *testing.T) {
	b := NewBuilder("myproj")
	if err := b.WriteFile("README.md", []byte("hi"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := Tree(b.Build())
	if strings.Contains(got, "Hooks:") {
		t.Errorf("Tree() = %q, want no Hooks section when there are none", got)
	}
}
