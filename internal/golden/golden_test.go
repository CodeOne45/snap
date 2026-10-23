// Package golden holds the M6 gate: round-tripping real-shaped fixture
// projects through capture -> new and diffing the result against committed
// expected output, byte for byte.
package golden

import (
	"flag"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/core/capture"
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
)

// update regenerates testdata/golden/<fixture>/expected/ from the generated
// output instead of comparing against it. Run: go test ./... -run Golden -update
var update = flag.Bool("update", false, "update golden files")

// fixedGeneratedAt makes .snap/origin.toml's timestamp reproducible across
// runs, so the full generated tree is byte-for-byte comparable.
var fixedGeneratedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// fixture describes one testdata/golden/<Name> round trip: capture the
// project at ProjectDir (loaded in-memory from <Name>/source/) as
// TemplateName, then generate with Vars, and diff against <Name>/expected/.
type fixture struct {
	Name         string
	ProjectDir   string
	TemplateName string
	Vars         map[string]string
	SourceWords  []string // original identifier, asserted absent from output
	// ExtraFiles are written into the in-memory project after loading
	// source/ from disk, for content that can never be a real committed
	// file — notably a fake .git/config: git silently refuses to track
	// anything under a directory literally named ".git", so it can't live
	// in testdata at all.
	ExtraFiles map[string]string
}

var fixtures = []fixture{
	{
		Name:         "go-api",
		ProjectDir:   "orders-api",
		TemplateName: "go-api-template",
		Vars:         map[string]string{"name": "payments-api"},
		SourceWords:  []string{"orders", "api"},
		ExtraFiles: map[string]string{
			".git/config": "[core]\n\trepositoryformatversion = 0\n\tbare = false\n" +
				"[remote \"origin\"]\n\turl = git@github.com:exampleuser/orders-api.git\n" +
				"\tfetch = +refs/heads/*:refs/remotes/origin/*\n",
		},
	},
	{
		Name:         "nextjs-app",
		ProjectDir:   "storefront-app",
		TemplateName: "nextjs-template",
		Vars:         map[string]string{"name": "shoe-store"},
		SourceWords:  []string{"storefront", "app"},
	},
	{
		Name:         "python-package",
		ProjectDir:   "data-utils",
		TemplateName: "python-template",
		Vars:         map[string]string{"name": "csv-tools"},
		SourceWords:  []string{"data", "utils"},
	},
	{
		Name:         "rust-crate",
		ProjectDir:   "fast-parser",
		TemplateName: "rust-template",
		Vars:         map[string]string{"name": "json-parser"},
		SourceWords:  []string{"fast", "parser"},
	},
	{
		Name:         "static-site",
		ProjectDir:   "portfolio-site",
		TemplateName: "static-template",
		Vars:         map[string]string{"name": "agency-site"},
		SourceWords:  []string{"portfolio", "site"},
	},
}

func TestGolden(t *testing.T) {
	for _, fx := range fixtures {
		t.Run(fx.Name, func(t *testing.T) { runFixture(t, fx) })
	}
}

func runFixture(t *testing.T, fx fixture) {
	t.Helper()
	fixtureDir := filepath.Join("..", "..", "testdata", "golden", fx.Name)
	sourceDir := filepath.Join(fixtureDir, "source")
	expectedDir := filepath.Join(fixtureDir, "expected")

	fsys := memfs.New()
	if err := loadIntoMemFS(fsys, sourceDir, fx.ProjectDir); err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	for rel, content := range fx.ExtraFiles {
		p := path.Join(fx.ProjectDir, rel)
		if err := fsys.MkdirAll(path.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir for extra file %q: %v", rel, err)
		}
		if err := fsys.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write extra file %q: %v", rel, err)
		}
	}

	templateDir := path.Join("templates", fx.TemplateName, usecase.InitialVersion)
	captureRes, err := usecase.Capture(usecase.CaptureParams{
		FS:        fsys,
		SourceDir: fx.ProjectDir,
		TargetDir: templateDir,
		AsName:    fx.TemplateName,
		Accept:    acceptDefaultName,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if err := executor.New(fsys).Apply(captureRes.Plan); err != nil {
		t.Fatalf("apply capture plan: %v", err)
	}

	genDir := path.Join("generated", fx.Vars["name"])
	newPlan, err := usecase.New(usecase.NewParams{
		Store:       local.New(fsys, "templates"),
		FS:          fsys,
		Template:    fx.TemplateName,
		TargetDir:   genDir,
		Vars:        fx.Vars,
		GeneratedAt: fixedGeneratedAt,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := executor.New(fsys).Apply(newPlan); err != nil {
		t.Fatalf("apply new plan: %v", err)
	}

	assertNoOriginalIdentifiers(t, fsys, genDir, fx.SourceWords)

	if *update {
		if err := dumpToDisk(fsys, genDir, expectedDir); err != nil {
			t.Fatalf("write expected/: %v", err)
		}
		return
	}
	compareTree(t, fsys, genDir, expectedDir)
}

// acceptDefaultName accepts every detected candidate under its default
// name, equivalent to the CLI's --yes flag.
func acceptDefaultName(_ capture.Candidate, defaultName string) (bool, string, error) {
	return true, defaultName, nil
}

// assertNoOriginalIdentifiers fails if any generated path or non-binary
// file content still contains a case-form of the project's original
// identifying words — PLAN.md's §2.1 acceptance criterion, checked
// independently of the expected/ diff below.
func assertNoOriginalIdentifiers(t *testing.T, fsys *memfs.MemFS, root string, words []string) {
	t.Helper()
	forms := capture.CandidateForms(words)

	files, err := walkMemFS(fsys, root)
	if err != nil {
		t.Fatalf("walk generated tree: %v", err)
	}
	for _, rel := range files {
		for _, f := range forms {
			if strings.Contains(rel, f.Value) {
				t.Errorf("generated path %q still contains original identifier %q", rel, f.Value)
			}
		}
		data, err := fsys.ReadFile(path.Join(root, rel))
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", rel, err)
		}
		if render.IsBinary(data) {
			continue
		}
		text := string(data)
		for _, f := range forms {
			if strings.Contains(text, f.Value) {
				t.Errorf("generated file %q still contains original identifier %q", rel, f.Value)
			}
		}
	}
}

// loadIntoMemFS copies every file under realDir on disk into fsys at
// memRoot, preserving the directory structure and converting to
// forward-slash paths.
func loadIntoMemFS(fsys *memfs.MemFS, realDir, memRoot string) error {
	return filepath.WalkDir(realDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(realDir, p)
		if err != nil {
			return err
		}
		memPath := memRoot
		if rel != "." {
			memPath = path.Join(memRoot, filepath.ToSlash(rel))
		}
		if d.IsDir() {
			return fsys.MkdirAll(memPath, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return fsys.WriteFile(memPath, data, 0o644)
	})
}

// dumpToDisk replaces realDir with the contents of fsys at memRoot.
func dumpToDisk(fsys *memfs.MemFS, memRoot, realDir string) error {
	if err := os.RemoveAll(realDir); err != nil {
		return err
	}
	files, err := walkMemFS(fsys, memRoot)
	if err != nil {
		return err
	}
	for _, rel := range files {
		data, err := fsys.ReadFile(path.Join(memRoot, rel))
		if err != nil {
			return err
		}
		dest := filepath.Join(realDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// compareTree diffs fsys at memRoot against expectedDir on disk: every path
// must exist in both with identical content.
func compareTree(t *testing.T, fsys *memfs.MemFS, memRoot, expectedDir string) {
	t.Helper()
	got, err := walkMemFS(fsys, memRoot)
	if err != nil {
		t.Fatalf("walk generated tree: %v", err)
	}
	want, err := walkDisk(expectedDir)
	if err != nil {
		t.Fatalf("walk expected tree: %v", err)
	}

	gotSet, wantSet := toSet(got), toSet(want)
	for rel := range gotSet {
		if !wantSet[rel] {
			t.Errorf("unexpected generated file %q (not in expected/ — run with -update if intended)", rel)
		}
	}
	for rel := range wantSet {
		if !gotSet[rel] {
			t.Errorf("missing generated file %q (present in expected/)", rel)
		}
	}

	for rel := range gotSet {
		if !wantSet[rel] {
			continue
		}
		gotContent, err := fsys.ReadFile(path.Join(memRoot, rel))
		if err != nil {
			t.Errorf("ReadFile(%q): %v", rel, err)
			continue
		}
		wantContent, err := os.ReadFile(filepath.Join(expectedDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("ReadFile(expected %q): %v", rel, err)
			continue
		}
		if string(gotContent) != string(wantContent) {
			t.Errorf("content mismatch for %q:\n--- got ---\n%s\n--- want ---\n%s", rel, gotContent, wantContent)
		}
	}
}

// walkMemFS recursively lists every file (relative to root, sorted) in fsys.
func walkMemFS(fsys *memfs.MemFS, root string) ([]string, error) {
	var files []string
	var walk func(rel string) error
	walk = func(rel string) error {
		entries, err := fsys.ReadDir(path.Join(root, rel))
		if err != nil {
			return err
		}
		for _, e := range entries {
			childRel := path.Join(rel, path.Base(e.Path))
			if e.IsDir {
				if err := walk(childRel); err != nil {
					return err
				}
				continue
			}
			files = append(files, childRel)
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// walkDisk recursively lists every file (relative to dir, forward-slash,
// sorted) under dir on the real filesystem. A missing dir yields no files,
// not an error — the natural state before the first -update run.
func walkDisk(dir string) ([]string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, i := range items {
		set[i] = true
	}
	return set
}
