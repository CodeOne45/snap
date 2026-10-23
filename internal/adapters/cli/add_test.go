package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/usagelog"
)

func newAddTestProject(t *testing.T, deps func() (Deps, error)) {
	t.Helper()
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.MkdirAll("proj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile("proj/package.json", []byte(`{"dependencies": {}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestAddCmd_DryRun(t *testing.T) {
	deps := newTestDeps(t)
	newAddTestProject(t, deps)
	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"sentry", "proj", "--dry-run", "--yes", "--var", "service=payments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "package.json") {
		t.Errorf("dry-run output = %q, want it to mention package.json", got)
	}
	if !strings.Contains(got, "sentry.ts") {
		t.Errorf("dry-run output = %q, want it to mention sentry.ts", got)
	}
}

func TestAddCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	newAddTestProject(t, deps)
	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetArgs([]string{"sentry", "proj", "--json", "--yes", "--var", "service=payments"})
	cmd.SetOut(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, `"stats"`) {
		t.Errorf("json output = %q, want a stats object", got)
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.Stat("proj/src/lib/sentry.ts"); err == nil {
		t.Error("--json must not write anything, but sentry.ts exists")
	}
}

func TestAddCmd_AppliesForReal(t *testing.T) {
	deps := newTestDeps(t)
	newAddTestProject(t, deps)
	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"sentry", "proj", "--yes", "--var", "service=payments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("proj/src/lib/sentry.ts")
	if err != nil || string(content) != "export const service = 'payments';\n" {
		t.Errorf("proj/src/lib/sentry.ts = %q, %v, want the rendered recipe file", content, err)
	}
	pkg, err := d.FS.ReadFile("proj/package.json")
	if err != nil || !strings.Contains(string(pkg), "@sentry/node") {
		t.Errorf("proj/package.json = %q, %v, want the dependency added", pkg, err)
	}
	if !strings.Contains(out.String(), "✓") {
		t.Errorf("output = %q, want a success receipt", out.String())
	}
}

func TestAddCmd_IdempotentReapplyReportsNoOp(t *testing.T) {
	deps := newTestDeps(t)
	newAddTestProject(t, deps)

	first := newAddCmd(deps)
	first.SetOut(&bytes.Buffer{})
	first.SetArgs([]string{"sentry", "proj", "--yes", "--var", "service=payments"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	second := newAddCmd(deps)
	out := &bytes.Buffer{}
	second.SetOut(out)
	second.SetArgs([]string{"sentry", "proj", "--yes", "--var", "service=payments"})
	if err := second.Execute(); err != nil {
		t.Fatalf("second Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "nothing to do") {
		t.Errorf("second apply output = %q, want it to report a no-op", out.String())
	}
}

func TestAddCmd_DefaultTargetDirIsCurrentDir(t *testing.T) {
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if err := d.FS.WriteFile("package.json", []byte(`{"dependencies": {}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"sentry", "--dry-run", "--yes", "--var", "service=payments"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "package.json") {
		t.Errorf("output = %q, want it to find package.json in the current dir", out.String())
	}
}

func TestAddCmd_LogsUsage(t *testing.T) {
	t.Setenv(usagelog.DisableEnvVar, "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	deps := newTestDeps(t)
	newAddTestProject(t, deps)
	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"sentry", "proj", "--yes", "--var", "service=payments"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	entries, err := usagelog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 || entries[0].Recipe != "sentry" {
		t.Errorf("entries = %+v, want one entry for recipe sentry", entries)
	}
}

func TestAddCmd_MissingVariableNonInteractive(t *testing.T) {
	deps := newTestDeps(t)
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	// No .snap/origin.toml, so default_from can't resolve either.
	if err := d.FS.MkdirAll("proj", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := d.FS.WriteFile("proj/package.json", []byte(`{"dependencies": {}}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := newAddCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"sentry", "proj", "--yes"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute with a missing variable and --yes: want error, got nil")
	}
}
