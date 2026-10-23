package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/usagelog"
)

func TestNewCmd_DryRun(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--dry-run", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "Plan for my-site:") {
		t.Errorf("dry-run output = %q, want it to mention the target dir", got)
	}
	if !strings.Contains(got, "README.md") {
		t.Errorf("dry-run output = %q, want it to list README.md", got)
	}
}

func TestNewCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--json", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, `"target_dir": "my-site"`) {
		t.Errorf("json output = %q, want it to contain target_dir", got)
	}
	if !strings.Contains(got, `"stats"`) || !strings.Contains(got, `"files"`) {
		t.Errorf("json output = %q, want a stats object (M19)", got)
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.Stat("my-site"); err == nil {
		t.Error("--json must not write anything, but my-site exists")
	}
}

func TestNewCmd_PrintsReceiptOnSuccess(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{"✓", "files", "tokens", "written by Snap instead of the agent"} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want it to contain %q", got, want)
		}
	}
}

func TestNewCmd_LogsUsage(t *testing.T) {
	t.Setenv(usagelog.DisableEnvVar, "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	entries, err := usagelog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Template != "api-starter" || entries[0].Source != "cli" {
		t.Errorf("entries[0] = %+v, want Template=api-starter Source=cli", entries[0])
	}
	if entries[0].Stats.Files == 0 {
		t.Error("entries[0].Stats.Files = 0, want it populated")
	}
}

func TestNewCmd_DryRunAndJSONDoNotLogUsage(t *testing.T) {
	t.Setenv(usagelog.DisableEnvVar, "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps := newTestDeps(t)
	for _, args := range [][]string{
		{"api-starter", "my-site", "--yes", "--dry-run"},
		{"api-starter", "my-site2", "--yes", "--json"},
	} {
		cmd := newNewCmd(deps)
		out := &bytes.Buffer{}
		cmd.SetOut(out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v): %v, output: %s", args, err, out.String())
		}
	}

	entries, err := usagelog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %+v, want none (only a real apply logs usage)", entries)
	}
}

func TestNewCmd_Apply(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("my-site/README.md")
	if err != nil {
		t.Fatalf("ReadFile(my-site/README.md): %v", err)
	}
	if string(content) != "# My Site by Anonymous" {
		t.Errorf("README.md content = %q, want %q", content, "# My Site by Anonymous")
	}
}

func TestNewCmd_YesModeMissingVar(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes", "--var", "name=Not Valid"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with invalid var value: want error, got nil")
	}
}

func TestNewCmd_PromptsWhenInteractive(t *testing.T) {
	// "name" is auto-seeded from the target dir, so it's never prompted;
	// "author" has a default but still gets prompted interactively, and a
	// non-empty answer overrides that default.
	deps := newTestDeps(t)
	cmd := newNewCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("Jane Doe\n"))
	cmd.SetArgs([]string{"api-starter", "my-site"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("my-site/README.md")
	if err != nil {
		t.Fatalf("ReadFile(my-site/README.md): %v", err)
	}
	if string(content) != "# My Site by Jane Doe" {
		t.Errorf("README.md content = %q, want %q", content, "# My Site by Jane Doe")
	}
}

func TestNewCmd_RefusesNonEmptyTarget(t *testing.T) {
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("my-site", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile("my-site/existing.txt", []byte("keep me"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := newNewCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute onto a non-empty target: want error, got nil")
	}
	content, err := d.FS.ReadFile("my-site/existing.txt")
	if err != nil || string(content) != "keep me" {
		t.Errorf("existing.txt disturbed by rejected apply: content=%q err=%v", content, err)
	}
}

func TestNewCmd_Force(t *testing.T) {
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("my-site", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile("my-site/stale.txt", []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := newNewCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"api-starter", "my-site", "--yes", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute --force: %v", err)
	}
	if _, err := d.FS.ReadFile("my-site/stale.txt"); err == nil {
		t.Error("stale.txt should have been removed by --force")
	}
	if _, err := d.FS.ReadFile("my-site/README.md"); err != nil {
		t.Errorf("README.md should exist after --force apply: %v", err)
	}
}

func TestParseVarFlags(t *testing.T) {
	got, err := parseVarFlags([]string{"name=acme", "port=8080"})
	if err != nil {
		t.Fatalf("parseVarFlags: %v", err)
	}
	if got["name"] != "acme" || got["port"] != "8080" {
		t.Errorf("parseVarFlags = %v, want name=acme port=8080", got)
	}

	if _, err := parseVarFlags([]string{"no-equals-sign"}); err == nil {
		t.Error("parseVarFlags(malformed): want error, got nil")
	}
}
