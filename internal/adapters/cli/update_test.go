package cli

import (
	"bytes"
	"strings"
	"testing"
)

// generateFixtureProject runs the real "new" command to produce a project
// at targetDir from the api-starter@1.0.0 fixture, so update tests start
// from a real, generated .snap/origin.toml like any real project would.
func generateFixtureProject(t *testing.T, deps func() (Deps, error), targetDir string) {
	t.Helper()
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", targetDir, "--var", "name=acme", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("generate fixture project: %v, output: %s", err, out.String())
	}
}

func bumpFixtureTemplate(t *testing.T, deps func() (Deps, error), version, readmeContent string) {
	t.Helper()
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	manifest := strings.Replace(testManifest, `version = "1.0.0"`, `version = "`+version+`"`, 1)
	dir := "templates/api-starter/" + version
	if err := d.FS.MkdirAll(dir+"/src", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile(dir+"/snap.toml", []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(snap.toml): %v", err)
	}
	if err := d.FS.WriteFile(dir+"/README.md", []byte(readmeContent), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	if err := d.FS.WriteFile(dir+"/src/{{name|pascal}}.go", []byte("package main // {{name}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go): %v", err)
	}
}

func TestUpdateCmd_CleanUpdate(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")
	bumpFixtureTemplate(t, deps, "1.1.0", "# {{name|title}} by {{author}}, v2")

	cmd := newUpdateCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Updated 1.0.0 -> 1.1.0") {
		t.Errorf("output = %q, want an update confirmation", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("proj/README.md")
	if err != nil || string(content) != "# Acme by Anonymous, v2" {
		t.Errorf("README.md = %q, %v, want the updated content", content, err)
	}
}

func TestUpdateCmd_DryRunWritesNothing(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")
	bumpFixtureTemplate(t, deps, "1.1.0", "# {{name|title}} by {{author}}, v2")

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	before, err := d.FS.ReadFile("proj/README.md")
	if err != nil {
		t.Fatalf("ReadFile before: %v", err)
	}

	cmd := newUpdateCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "--yes", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Plan for proj:") {
		t.Errorf("output = %q, want a dry-run plan header", out.String())
	}

	after, err := d.FS.ReadFile("proj/README.md")
	if err != nil || string(after) != string(before) {
		t.Errorf("README.md changed during --dry-run: before=%q after=%q", before, after)
	}
}

func TestUpdateCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")
	bumpFixtureTemplate(t, deps, "1.1.0", "# {{name|title}} by {{author}}, v2")

	cmd := newUpdateCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "--yes", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	for _, want := range []string{`"old_version": "1.0.0"`, `"new_version": "1.1.0"`, `"target_dir": "proj"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %s, want it to contain %q", out.String(), want)
		}
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if content, err := d.FS.ReadFile("proj/README.md"); err != nil || strings.Contains(string(content), "v2") {
		t.Error("--json must not write anything, but README.md already reflects the update")
	}
}

func TestUpdateCmd_ConflictReportedAndOriginUnchanged(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.WriteFile("proj/README.md", []byte("user's own edit"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	bumpFixtureTemplate(t, deps, "1.1.0", "# {{name|title}} by {{author}}, template's own edit")

	cmd := newUpdateCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "--yes"})
	// A conflict is reported via a non-zero exit (like git merge), even
	// though the merge did run and wrote the conflict-marker file.
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with an unresolved conflict: want a non-nil error (non-zero exit), got nil")
	}
	if !strings.Contains(out.String(), "conflict") || !strings.Contains(out.String(), "README.md") {
		t.Errorf("output = %q, want it to report a conflict on README.md", out.String())
	}

	origin, err := d.FS.ReadFile("proj/.snap/origin.toml")
	if err != nil || !strings.Contains(string(origin), `version = "1.0.0"`) {
		t.Errorf("origin.toml = %q, %v, want it to still say version 1.0.0 (unresolved conflict)", origin, err)
	}
}

func TestUpdateCmd_NotAGeneratedProject(t *testing.T) {
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("not-a-project", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cmd := newUpdateCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"not-a-project"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute on a directory with no .snap/origin.toml: want error, got nil")
	}
}
