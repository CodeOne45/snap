package cli

import (
	"bytes"
	"strings"
	"testing"
)

const optionalManifest = `
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
`

func newOptionalTestDeps(t *testing.T) func() (Deps, error) {
	t.Helper()
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("templates/withdocker/1.0.0", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile("templates/withdocker/1.0.0/snap.toml", []byte(optionalManifest), 0o644); err != nil {
		t.Fatalf("WriteFile(snap.toml): %v", err)
	}
	if err := d.FS.WriteFile("templates/withdocker/1.0.0/README.md", []byte("# {{name}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	if err := d.FS.WriteFile("templates/withdocker/1.0.0/Dockerfile", []byte("FROM scratch"), 0o644); err != nil {
		t.Fatalf("WriteFile(Dockerfile): %v", err)
	}
	return deps
}

func TestNewCmd_WithoutFlagExcludesOptionalBlock(t *testing.T) {
	deps := newOptionalTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"withdocker", "my-site", "--yes", "--without", "docker"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("my-site/Dockerfile"); err == nil {
		t.Error("Dockerfile should not exist: docker was toggled off with --without")
	}
	origin, err := d.FS.ReadFile("my-site/.snap/origin.toml")
	if err != nil {
		t.Fatalf("ReadFile(origin.toml): %v", err)
	}
	if !strings.Contains(string(origin), "docker = false") {
		t.Errorf("origin.toml = %s, want docker = false recorded", origin)
	}
}

func TestNewCmd_WithFlagIncludesOptionalBlock(t *testing.T) {
	deps := newOptionalTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"withdocker", "my-site", "--yes", "--with", "docker"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("my-site/Dockerfile")
	if err != nil || string(content) != "FROM scratch" {
		t.Errorf("Dockerfile = %q, %v, want it present (docker toggled on)", content, err)
	}
}

func TestNewCmd_DefaultToggleUsedWithYes(t *testing.T) {
	// optionalManifest declares docker's default as true, and no --with/
	// --without is given: --yes (non-interactive) must fall back to the
	// declared default, not error or silently disable it.
	deps := newOptionalTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"withdocker", "my-site", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("my-site/Dockerfile"); err != nil {
		t.Errorf("Dockerfile should exist (default is true): %v", err)
	}
}

func TestNewCmd_ConflictingWithAndWithoutIsError(t *testing.T) {
	deps := newOptionalTestDeps(t)
	cmd := newNewCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"withdocker", "my-site", "--yes", "--with", "docker", "--without", "docker"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with docker in both --with and --without: want error, got nil")
	}
}

func TestShowCmd_DisplaysOptionalBlocks(t *testing.T) {
	deps := newOptionalTestDeps(t)
	cmd := newShowCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"withdocker"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{"Optional:", "docker", "Dockerfile"} {
		if !strings.Contains(got, want) {
			t.Errorf("show output = %q, want it to contain %q", got, want)
		}
	}
}
