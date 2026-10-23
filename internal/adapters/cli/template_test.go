package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestTemplateBumpCmd_Patch(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newTemplateBumpCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "patch"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "1.0.0 -> 1.0.1") {
		t.Errorf("output = %q, want it to report 1.0.0 -> 1.0.1", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("templates/api-starter/1.0.1/snap.toml"); err != nil {
		t.Errorf("templates/api-starter/1.0.1/snap.toml should exist: %v", err)
	}
	if _, err := d.FS.ReadFile("templates/api-starter/1.0.0/snap.toml"); err != nil {
		t.Errorf("templates/api-starter/1.0.0/snap.toml should still exist (old version untouched): %v", err)
	}
}

func TestTemplateBumpCmd_DryRunWritesNothing(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newTemplateBumpCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "minor", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Plan for templates/api-starter/1.1.0:") {
		t.Errorf("output = %q, want a dry-run plan header", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("templates/api-starter/1.1.0/snap.toml"); err == nil {
		t.Error("dry-run must not write anything, but the bumped version exists")
	}
}

func TestTemplateBumpCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newTemplateBumpCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "major", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), `"target_dir": "templates/api-starter/2.0.0"`) {
		t.Errorf("output = %q, want JSON naming the new version's target_dir", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("templates/api-starter/2.0.0/snap.toml"); err == nil {
		t.Error("--json must not write anything, but the bumped version exists")
	}
}

func TestTemplateBumpCmd_UnknownTemplate(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newTemplateBumpCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"nope", "patch"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with an unknown template: want error, got nil")
	}
}

func TestTemplateBumpCmd_UnknownLevel(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newTemplateBumpCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"api-starter", "sideways"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with an unknown bump level: want error, got nil")
	}
}
