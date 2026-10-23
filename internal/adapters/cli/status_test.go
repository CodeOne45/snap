package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestStatusCmd_UpToDate(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")

	cmd := newStatusCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output = %q, want it to report up to date", out.String())
	}
}

func TestStatusCmd_Behind(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")
	bumpFixtureTemplate(t, deps, "1.1.0", "# {{name|title}} by {{author}}, v2")

	cmd := newStatusCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with a project behind: want a non-nil error (non-zero exit), got nil")
	}
	if !strings.Contains(out.String(), "behind") || !strings.Contains(out.String(), "1.0.0") || !strings.Contains(out.String(), "1.1.0") {
		t.Errorf("output = %q, want it to report behind with both versions", out.String())
	}
}

func TestStatusCmd_MultipleDirsPartialFailure(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("not-a-project", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cmd := newStatusCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "not-a-project"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with one unreadable dir: want a non-nil error (non-zero exit), got nil")
	}
	if !strings.Contains(out.String(), "proj") || !strings.Contains(out.String(), "not-a-project") {
		t.Errorf("output = %q, want both dirs reported", out.String())
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("output = %q, want proj still reported as up to date despite the other dir's problem", out.String())
	}
}

func TestStatusCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	generateFixtureProject(t, deps, "proj")

	cmd := newStatusCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"proj", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	for _, want := range []string{`"dir": "proj"`, `"template": "api-starter"`, `"up_to_date": true`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %s, want it to contain %q", out.String(), want)
		}
	}
}

func TestStatusCmd_RequiresAtLeastOneDir(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newStatusCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with no directories: want error, got nil")
	}
}
