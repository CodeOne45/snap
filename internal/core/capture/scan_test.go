package capture

import (
	"reflect"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func write(t *testing.T, fsys *memfs.MemFS, p, content string) {
	t.Helper()
	dir := p[:lastSlash(p)]
	if dir != "" {
		if err := fsys.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}
	if err := fsys.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", p, err)
	}
}

func lastSlash(p string) int {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return i
		}
	}
	return 0
}

func filePaths(r Result) []string {
	out := make([]string, len(r.Files))
	for i, f := range r.Files {
		out[i] = f.Path
	}
	return out
}

func TestScan_RootGitignore(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.gitignore", "*.log\n")
	write(t, fsys, "proj/main.go", "package main")
	write(t, fsys, "proj/debug.log", "noisy")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{".gitignore", "main.go"}
	if got := filePaths(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}

func TestScan_NestedGitignoreScopedToSubtree(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/pkg/a/.gitignore", "*.tmp\n")
	write(t, fsys, "proj/pkg/a/keep.go", "a")
	write(t, fsys, "proj/pkg/a/scratch.tmp", "a-tmp")
	write(t, fsys, "proj/pkg/b/scratch.tmp", "b-tmp") // unaffected: different subtree

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := filePaths(result)
	want := []string{"pkg/a/.gitignore", "pkg/a/keep.go", "pkg/b/scratch.tmp"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}

func TestScan_NestedNegationOverridesParent(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.gitignore", "*.md\n")
	write(t, fsys, "proj/docs/.gitignore", "!important.md\n")
	write(t, fsys, "proj/README.md", "root")
	write(t, fsys, "proj/docs/important.md", "kept")
	write(t, fsys, "proj/docs/other.md", "still ignored")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := filePaths(result)
	want := []string{".gitignore", "docs/.gitignore", "docs/important.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}

func TestScan_BuiltinJunkPrunedWithoutRecursing(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/main.go", "package main")
	write(t, fsys, "proj/node_modules/pkg/index.js", "module.exports = {}")
	write(t, fsys, "proj/.git/HEAD", "ref: refs/heads/main")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got := filePaths(result); !reflect.DeepEqual(got, []string{"main.go"}) {
		t.Errorf("Files = %v, want [main.go]", got)
	}
	wantIgnored := []string{".git", "node_modules"}
	if !reflect.DeepEqual(result.Ignored, wantIgnored) {
		t.Errorf("Ignored = %v, want %v", result.Ignored, wantIgnored)
	}
}

func TestScan_IgnoredIsTopLevelOnly(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.gitignore", "*.log\n")
	write(t, fsys, "proj/debug.log", "x")             // top-level: should appear in Ignored
	write(t, fsys, "proj/sub/nested.log", "x")        // nested: should NOT appear in Ignored
	write(t, fsys, "proj/sub/keep.go", "package sub") // keeps "sub" itself out of Ignored

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{"debug.log"}
	if !reflect.DeepEqual(result.Ignored, want) {
		t.Errorf("Ignored = %v, want %v", result.Ignored, want)
	}
}

func TestScan_Snapignore(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.snapignore", "fixtures/\n")
	write(t, fsys, "proj/main.go", "package main")
	write(t, fsys, "proj/fixtures/data.json", "{}")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{".snapignore", "main.go"}
	if got := filePaths(result); !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}

func TestScan_ExtraExcludes(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/main.go", "package main")
	write(t, fsys, "proj/scratch/notes.txt", "notes")

	result, err := Scan(fsys, "proj", Options{ExtraExcludes: []string{"scratch/"}})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got := filePaths(result); !reflect.DeepEqual(got, []string{"main.go"}) {
		t.Errorf("Files = %v, want [main.go]", got)
	}
}

func TestScan_Secrets(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/main.go", "package main")
	write(t, fsys, "proj/.env", "SECRET=1")
	write(t, fsys, "proj/.env.example", "SECRET=")
	write(t, fsys, "proj/server.pem", "cert")
	write(t, fsys, "proj/id_rsa", "key")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := filePaths(result)
	want := []string{".env.example", "main.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v", got, want)
	}
}

func TestScan_SecretsAreNonOverridable(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/.gitignore", "!.env\n")
	write(t, fsys, "proj/.env", "SECRET=1")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, f := range result.Files {
		if f.Path == ".env" {
			t.Fatalf("Files = %v: a .gitignore negation must not un-ignore a secret", result.Files)
		}
	}
}

func TestScan_BinaryClassification(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/main.go", "package main")
	if err := fsys.WriteFile("proj/logo.png", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	byPath := map[string]File{}
	for _, f := range result.Files {
		byPath[f.Path] = f
	}
	if byPath["main.go"].IsBinary {
		t.Error("main.go: IsBinary = true, want false")
	}
	if !byPath["logo.png"].IsBinary {
		t.Error("logo.png: IsBinary = false, want true")
	}
}

func TestScan_EmptyDirectories(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("proj/empty", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// All-ignored directory: should also surface as "empty" after filtering.
	write(t, fsys, "proj/only-junk/debug.log", "x")
	write(t, fsys, "proj/.gitignore", "*.log\n")
	// A directory matching builtin junk should NOT appear in Dirs at all.
	if err := fsys.MkdirAll("proj/node_modules", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{"empty", "only-junk"}
	if !reflect.DeepEqual(result.Dirs, want) {
		t.Errorf("Dirs = %v, want %v", result.Dirs, want)
	}
}

func TestScan_DeterministicOrder(t *testing.T) {
	fsys := memfs.New()
	write(t, fsys, "proj/zebra.go", "z")
	write(t, fsys, "proj/alpha.go", "a")
	write(t, fsys, "proj/mid/beta.go", "b")

	result, err := Scan(fsys, "proj", Options{})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := filePaths(result)
	want := []string{"alpha.go", "mid/beta.go", "zebra.go"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %v, want %v (sorted)", got, want)
	}
}
