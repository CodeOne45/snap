package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

func addCaptureFixture(t *testing.T, deps func() (Deps, error)) {
	t.Helper()
	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	fsys := d.FS.(*memfs.MemFS)
	if err := fsys.MkdirAll("src/acme-site", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("src/acme-site/go.mod", []byte("module github.com/someuser/acme-site\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod): %v", err)
	}
	if err := fsys.WriteFile("src/acme-site/README.md", []byte("# AcmeSite\n\nThe acme-site project."), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
}

func TestCaptureCmd_Yes(t *testing.T) {
	deps := newTestDeps(t)
	addCaptureFixture(t, deps)

	cmd := newCaptureCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"src/acme-site", "--as", "client-site", "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Template \"client-site\" saved") {
		t.Errorf("output = %q, want a save confirmation", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("templates/client-site/0.1.0/README.md")
	if err != nil {
		t.Fatalf("ReadFile(templates/client-site/0.1.0/README.md): %v", err)
	}
	if !strings.Contains(string(content), "{{name") {
		t.Errorf("captured README.md = %q, want a {{name...}} placeholder", content)
	}
	if _, err := d.FS.ReadFile("templates/client-site/0.1.0/snap.toml"); err != nil {
		t.Errorf("expected templates/client-site/0.1.0/snap.toml to exist: %v", err)
	}
}

func TestCaptureCmd_DryRun(t *testing.T) {
	deps := newTestDeps(t)
	addCaptureFixture(t, deps)

	cmd := newCaptureCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"src/acme-site", "--as", "client-site", "--yes", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Plan for templates/client-site/0.1.0:") {
		t.Errorf("output = %q, want a dry-run plan header", out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	if _, err := d.FS.ReadFile("templates/client-site/0.1.0/snap.toml"); err == nil {
		t.Error("dry-run must not write anything, but templates/client-site/0.1.0/snap.toml exists")
	}
}

func TestCaptureCmd_MissingAsFlag(t *testing.T) {
	deps := newTestDeps(t)
	addCaptureFixture(t, deps)

	cmd := newCaptureCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"src/acme-site"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute without --as: want error, got nil")
	}
}

func TestCaptureCmd_InteractiveRejectAll(t *testing.T) {
	deps := newTestDeps(t)
	addCaptureFixture(t, deps)

	cmd := newCaptureCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("n\nn\nn\nn\n")) // reject every candidate confirm prompt
	cmd.SetArgs([]string{"src/acme-site", "--as", "client-site"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v, output: %s", err, out.String())
	}

	d, err := deps()
	if err != nil {
		t.Fatalf("deps: %v", err)
	}
	content, err := d.FS.ReadFile("templates/client-site/0.1.0/README.md")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(content), "{{") {
		t.Errorf("README.md = %q, want no placeholders when every candidate is rejected", content)
	}
}
