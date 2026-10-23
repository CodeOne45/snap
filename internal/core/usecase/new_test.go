package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
)

var fixedTime = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

const fixtureManifest = `
[template]
name = "api-starter"
version = "1.0.0"
description = "test fixture"

[[variables]]
name = "name"
type = "string"
`

func newFixtureFS(t *testing.T) *memfs.MemFS {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/src")
	mustMkdirAll(t, fsys, "templates/api-starter/assets")
	mustMkdirAll(t, fsys, "templates/api-starter/empty")
	mustWriteFile(t, fsys, "templates/api-starter/snap.toml", []byte(fixtureManifest))
	mustWriteFile(t, fsys, "templates/api-starter/README.md", []byte("# {{name|title}}"))
	mustWriteFile(t, fsys, "templates/api-starter/src/{{name|pascal}}.go", []byte("package main // {{name}}"))
	mustWriteFile(t, fsys, "templates/api-starter/assets/logo.png", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0})
	return fsys
}

func mustMkdirAll(t *testing.T, fsys *memfs.MemFS, p string) {
	t.Helper()
	if err := fsys.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", p, err)
	}
}

func mustWriteFile(t *testing.T, fsys *memfs.MemFS, p string, content []byte) {
	t.Helper()
	if err := fsys.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", p, err)
	}
}

func TestNew_RendersTemplate(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if p.TargetDir != "out" {
		t.Fatalf("TargetDir = %q, want %q", p.TargetDir, "out")
	}

	byPath := map[string]plan.Op{}
	for _, op := range p.Ops {
		switch o := op.(type) {
		case plan.CreateDirOp:
			byPath[o.Path] = o
		case plan.WriteFileOp:
			byPath[o.Path] = o
		}
	}

	if len(p.Ops) != 5 {
		t.Fatalf("len(Ops) = %d, want 5 (3 files + 1 dir + origin.toml): %+v", len(p.Ops), p.Ops)
	}
	if _, ok := byPath["empty"].(plan.CreateDirOp); !ok {
		t.Errorf("expected CreateDirOp for %q, got %#v", "empty", byPath["empty"])
	}
	if _, ok := byPath["snap.toml"]; ok {
		t.Error("snap.toml must not appear in the rendered plan")
	}

	readme, ok := byPath["README.md"].(plan.WriteFileOp)
	if !ok || string(readme.Content) != "# Acme Site" {
		t.Errorf("README.md = %#v, want content %q", byPath["README.md"], "# Acme Site")
	}

	src, ok := byPath["src/AcmeSite.go"].(plan.WriteFileOp)
	if !ok || string(src.Content) != "package main // acme-site" {
		t.Errorf("src/AcmeSite.go = %#v, want rendered path+content", byPath["src/AcmeSite.go"])
	}

	logo, ok := byPath["assets/logo.png"].(plan.WriteFileOp)
	if !ok || len(logo.Content) != 8 || logo.Content[0] != 0x89 {
		t.Errorf("assets/logo.png = %#v, want binary content passed through unchanged", byPath["assets/logo.png"])
	}

	origin, ok := byPath[provenance.File].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected a WriteFileOp at %q, got %#v", provenance.File, byPath[provenance.File])
	}
	wantLines := []string{
		`template = "api-starter"`,
		`version = "1.0.0"`,
		`source = "local"`,
		`generated_at = 2026-10-06T12:00:00Z`,
		`name = "acme-site"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(string(origin.Content), want) {
			t.Errorf("origin.toml = %s, want it to contain %q", origin.Content, want)
		}
	}
}

func TestNew_MissingVariableNonInteractive(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	_, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		GeneratedAt: fixedTime,
	})
	if err == nil {
		t.Fatal("New with missing required variable and no prompter: want error, got nil")
	}
}

func TestNew_UnknownTemplate(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(nil)

	_, err := New(NewParams{Store: store, FS: fsys, Template: "nope", TargetDir: "out", GeneratedAt: fixedTime})
	if err == nil {
		t.Fatal("New with unknown template: want error, got nil")
	}
}

func TestNew_TemplateContainingOriginFileCollides(t *testing.T) {
	// If a template (e.g. one captured from a project snap itself
	// generated, before M4's .snap/ exclusion existed, or hand-edited)
	// already ships .snap/origin.toml, New's own provenance write collides
	// with it. The Plan builder's duplicate-path guard must fail loudly
	// rather than let one silently clobber the other.
	fsys := newFixtureFS(t)
	mustMkdirAll(t, fsys, "templates/api-starter/.snap")
	mustWriteFile(t, fsys, "templates/api-starter/.snap/origin.toml", []byte("stale = true"))
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	_, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
	})
	if err == nil {
		t.Fatal("New with a template already containing .snap/origin.toml: want error, got nil")
	}
}

// TestNew_DeterministicAcrossRuns covers Gate 1's C1: the same template,
// same variables, and the same fixed GeneratedAt must produce byte-for-byte
// identical output on every run.
func TestNew_DeterministicAcrossRuns(t *testing.T) {
	params := func(fsys *memfs.MemFS) NewParams {
		return NewParams{
			Store:       newMapStore(map[string]string{"api-starter": "templates/api-starter"}),
			FS:          fsys,
			Template:    "api-starter",
			TargetDir:   "out",
			Vars:        map[string]string{"name": "acme-site"},
			GeneratedAt: fixedTime,
		}
	}

	p1, err := New(params(newFixtureFS(t)))
	if err != nil {
		t.Fatalf("New (run 1): %v", err)
	}
	p2, err := New(params(newFixtureFS(t)))
	if err != nil {
		t.Fatalf("New (run 2): %v", err)
	}

	j1, err := plan.ToJSON(p1)
	if err != nil {
		t.Fatalf("ToJSON(p1): %v", err)
	}
	j2, err := plan.ToJSON(p2)
	if err != nil {
		t.Fatalf("ToJSON(p2): %v", err)
	}
	if string(j1) != string(j2) {
		t.Errorf("New() is not deterministic:\nrun 1:\n%s\nrun 2:\n%s", j1, j2)
	}
}

// TestNew_VariableValuesNeverRendered covers Gate 1's C3: a variable value
// that looks like placeholder syntax is stored in origin.toml as the
// literal string the user passed, never run through the render pipeline.
func TestNew_VariableValuesNeverRendered(t *testing.T) {
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/with-desc")
	mustWriteFile(t, fsys, "templates/with-desc/snap.toml", []byte(`
[template]
name = "with-desc"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[variables]]
name = "description"
type = "string"
`))
	mustWriteFile(t, fsys, "templates/with-desc/README.md", []byte("# {{name}}"))
	store := newMapStore(map[string]string{"with-desc": "templates/with-desc"})

	p, err := New(NewParams{
		Store:     store,
		FS:        fsys,
		Template:  "with-desc",
		TargetDir: "out",
		Vars: map[string]string{
			"name":        "acme-site",
			"description": "uses {{handlebars}}",
		},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == provenance.File {
			if !strings.Contains(string(wf.Content), `description = "uses {{handlebars}}"`) {
				t.Errorf("origin.toml = %s, want the literal description value unrendered", wf.Content)
			}
			return
		}
	}
	t.Fatal("no origin.toml WriteFileOp found")
}

// TestNew_PreservesFileMode covers Gate 1's D2: an executable file's mode
// survives into the generated project.
func TestNew_PreservesFileMode(t *testing.T) {
	fsys := newFixtureFS(t)
	if err := fsys.WriteFile("templates/api-starter/run.sh", []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == "run.sh" {
			if wf.Mode&0o111 == 0 {
				t.Errorf("run.sh mode = %o, want an executable bit set", wf.Mode)
			}
			return
		}
	}
	t.Fatal("no run.sh WriteFileOp found")
}

func TestNew_ZeroGeneratedAtIsError(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	_, err := New(NewParams{
		Store:     store,
		FS:        fsys,
		Template:  "api-starter",
		TargetDir: "out",
		Vars:      map[string]string{"name": "acme-site"},
	})
	if err == nil {
		t.Fatal("New with zero GeneratedAt: want error, got nil")
	}
}

func TestNew_HooksTranslatedToRunHookOps(t *testing.T) {
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/hooked")
	mustWriteFile(t, fsys, "templates/hooked/snap.toml", []byte(`
[template]
name = "hooked"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[hooks.post]]
run = "git init"

[[hooks.post]]
run = "go mod tidy"
`))
	mustWriteFile(t, fsys, "templates/hooked/README.md", []byte("# {{name}}"))
	store := newMapStore(map[string]string{"hooked": "templates/hooked"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "hooked",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	hooks := p.Hooks()
	if len(hooks) != 2 || hooks[0].Run != "git init" || hooks[1].Run != "go mod tidy" {
		t.Errorf("Hooks() = %+v, want [git init, go mod tidy] in order", hooks)
	}
}

// TestNew_OptionalBlockToggledOff covers Gate 1's "each toggle on/off
// produces exactly the expected file set" (M10): with docker off, neither
// Dockerfile nor its content ends up in the plan, and the choice is
// recorded in origin.toml.
func TestNew_OptionalBlockToggledOff(t *testing.T) {
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/withdocker")
	mustWriteFile(t, fsys, "templates/withdocker/snap.toml", []byte(`
[template]
name = "withdocker"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[optional]]
name = "docker"
prompt = "Include Docker?"
default = true
paths = ["Dockerfile"]
`))
	mustWriteFile(t, fsys, "templates/withdocker/README.md", []byte("# {{name}}"))
	mustWriteFile(t, fsys, "templates/withdocker/Dockerfile", []byte("FROM scratch"))
	store := newMapStore(map[string]string{"withdocker": "templates/withdocker"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "withdocker",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		Toggles:     map[string]bool{"docker": false},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == "Dockerfile" {
			t.Fatalf("Dockerfile should not be in the plan when docker is toggled off: %+v", wf)
		}
	}

	origin := findOriginOp(t, p)
	if !strings.Contains(string(origin.Content), "[toggles]") || !strings.Contains(string(origin.Content), "docker = false") {
		t.Errorf("origin.toml = %s, want toggles.docker = false recorded", origin.Content)
	}
}

func TestNew_OptionalBlockToggledOn(t *testing.T) {
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/withdocker")
	mustWriteFile(t, fsys, "templates/withdocker/snap.toml", []byte(`
[template]
name = "withdocker"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[optional]]
name = "docker"
default = false
paths = ["Dockerfile"]
`))
	mustWriteFile(t, fsys, "templates/withdocker/README.md", []byte("# {{name}}"))
	mustWriteFile(t, fsys, "templates/withdocker/Dockerfile", []byte("FROM scratch"))
	store := newMapStore(map[string]string{"withdocker": "templates/withdocker"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "withdocker",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		Toggles:     map[string]bool{"docker": true},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	found := false
	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == "Dockerfile" {
			found = true
			if string(wf.Content) != "FROM scratch" {
				t.Errorf("Dockerfile content = %q, want %q", wf.Content, "FROM scratch")
			}
		}
	}
	if !found {
		t.Fatal("Dockerfile should be in the plan when docker is toggled on")
	}
}

func TestNew_UnknownToggleIsError(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	_, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		Toggles:     map[string]bool{"nonexistent": true},
		GeneratedAt: fixedTime,
	})
	if err == nil {
		t.Fatal("New with an unknown toggle name: want error, got nil")
	}
}

func findOriginOp(t *testing.T, p plan.Plan) plan.WriteFileOp {
	t.Helper()
	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == provenance.File {
			return wf
		}
	}
	t.Fatal("no origin.toml WriteFileOp found")
	return plan.WriteFileOp{}
}

func TestNew_SourceDefaultsToLocal(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	origin := findOriginOp(t, p)
	if !strings.Contains(string(origin.Content), `source = "local"`) {
		t.Errorf("origin.toml = %s, want source = \"local\"", origin.Content)
	}
}

func TestNew_SourceOverride(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	p, err := New(NewParams{
		Store:       store,
		FS:          fsys,
		Template:    "api-starter",
		TargetDir:   "out",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: fixedTime,
		Source:      "https://github.com/user/repo.git@abc1234",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	origin := findOriginOp(t, p)
	if !strings.Contains(string(origin.Content), `source = "https://github.com/user/repo.git@abc1234"`) {
		t.Errorf("origin.toml = %s, want the overridden source", origin.Content)
	}
}
