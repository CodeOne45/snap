package usecase

import (
	"reflect"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/capture"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/template"
)

func newCaptureFixture(t *testing.T) *memfs.MemFS {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "src/acme-site/cmd/acme-site")
	mustWriteFile(t, fsys, "src/acme-site/go.mod", []byte("module github.com/someuser/acme-site\n"))
	mustWriteFile(t, fsys, "src/acme-site/README.md", []byte("# AcmeSite\n\nThe acme-site project."))
	mustWriteFile(t, fsys, "src/acme-site/cmd/acme-site/main.go", []byte("package main // acme-site"))
	mustWriteFile(t, fsys, "src/acme-site/logo.png", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0})
	return fsys
}

func acceptAll(_ capture.Candidate, defaultName string) (bool, string, error) {
	return true, defaultName, nil
}

func rejectAll(_ capture.Candidate, _ string) (bool, string, error) {
	return false, "", nil
}

func opsByPath(p plan.Plan) map[string]plan.Op {
	out := map[string]plan.Op{}
	for _, op := range p.Ops {
		switch o := op.(type) {
		case plan.CreateDirOp:
			out[o.Path] = o
		case plan.WriteFileOp:
			out[o.Path] = o
		case plan.ModifyFileOp:
			out[o.Path] = o
		}
	}
	return out
}

func TestCapture_ReplacesAcceptedForms(t *testing.T) {
	fsys := newCaptureFixture(t)

	res, err := Capture(CaptureParams{
		FS:        fsys,
		SourceDir: "src/acme-site",
		TargetDir: "templates/client-site",
		AsName:    "client-site",
		Accept:    acceptAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(res.Candidates) == 0 {
		t.Fatal("accepted candidates: want at least one, got none")
	}

	ops := opsByPath(res.Plan)

	readme, ok := ops["README.md"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for README.md, got %#v", ops["README.md"])
	}
	wantReadme := "# {{name|pascal}}\n\nThe {{name}} project."
	if string(readme.Content) != wantReadme {
		t.Errorf("README.md content = %q, want %q", readme.Content, wantReadme)
	}

	main, ok := ops["cmd/{{name}}/main.go"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for cmd/{{name}}/main.go, got path not found in %v", ops)
	}
	wantMain := "package main // {{name}}"
	if string(main.Content) != wantMain {
		t.Errorf("main.go content = %q, want %q", main.Content, wantMain)
	}

	logo, ok := ops["logo.png"].(plan.WriteFileOp)
	if !ok || len(logo.Content) != 8 || logo.Content[0] != 0x89 {
		t.Errorf("logo.png = %#v, want binary content unchanged", ops["logo.png"])
	}

	manifestOp, ok := ops[template.ManifestFile].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected a WriteFileOp for %s", template.ManifestFile)
	}
	manifest, err := template.ParseManifest(manifestOp.Content)
	if err != nil {
		t.Fatalf("generated snap.toml did not parse: %v\n%s", err, manifestOp.Content)
	}
	if manifest.Name != "client-site" {
		t.Errorf("manifest.Name = %q, want %q", manifest.Name, "client-site")
	}
	foundName := false
	for _, v := range manifest.Variables {
		if v.Name == "name" {
			foundName = true
		}
	}
	if !foundName {
		t.Errorf("manifest.Variables = %+v, want a %q variable (folder-name candidate)", manifest.Variables, "name")
	}
}

func TestCapture_RejectedCandidateLeftAlone(t *testing.T) {
	fsys := newCaptureFixture(t)

	res, err := Capture(CaptureParams{
		FS:        fsys,
		SourceDir: "src/acme-site",
		TargetDir: "templates/client-site",
		AsName:    "client-site",
		Accept:    rejectAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("accepted = %+v, want none", res.Candidates)
	}

	ops := opsByPath(res.Plan)
	readme, ok := ops["README.md"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for README.md")
	}
	if string(readme.Content) != "# AcmeSite\n\nThe acme-site project." {
		t.Errorf("README.md content = %q, want original text unchanged", readme.Content)
	}
}

func TestCapture_InvalidVariableName(t *testing.T) {
	fsys := newCaptureFixture(t)
	badName := func(_ capture.Candidate, _ string) (bool, string, error) {
		return true, "not a valid name!", nil
	}

	_, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: badName,
	})
	if err == nil {
		t.Fatal("Capture with invalid variable name: want error, got nil")
	}
}

func TestCapture_DuplicateVariableName(t *testing.T) {
	fsys := newCaptureFixture(t)
	// Add a second, distinct seed (different words) so there are two
	// separate candidates for Accept to (mis)rename to the same string.
	mustWriteFile(t, fsys, "src/acme-site/package.json", []byte(`{"name": "widgets"}`))

	sameName := func(_ capture.Candidate, _ string) (bool, string, error) {
		return true, "dup", nil
	}

	_, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: sameName,
	})
	if err == nil {
		t.Fatal("Capture with two candidates renamed to the same variable: want error, got nil")
	}
}

func TestCapture_ReportsIgnoredTopLevelEntries(t *testing.T) {
	fsys := newCaptureFixture(t)
	mustMkdirAll(t, fsys, "src/acme-site/node_modules/pkg")
	mustWriteFile(t, fsys, "src/acme-site/node_modules/pkg/index.js", []byte("module.exports = {}"))
	mustWriteFile(t, fsys, "src/acme-site/.env", []byte("SECRET=1"))

	res, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: acceptAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	want := []string{".env", "node_modules"}
	if !reflect.DeepEqual(res.Scan.Ignored, want) {
		t.Errorf("Scan.Ignored = %v, want %v", res.Scan.Ignored, want)
	}
}

func TestCapture_ManifestExcludedFromOwnOutput(t *testing.T) {
	fsys := newCaptureFixture(t)
	res, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: acceptAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	count := 0
	for _, op := range res.Plan.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == template.ManifestFile {
			count++
		}
	}
	if count != 1 {
		t.Errorf("snap.toml WriteFileOp count = %d, want exactly 1", count)
	}
}

// TestCapture_IncludesClaudeCodeAndMCPFiles covers M13's requirement that
// CLAUDE.md, .claude/, and .mcp.json are captured and placeholder-renamed
// like any other file — none of them are in capture's default-exclude
// list (unlike .env*, *.pem, *.key, id_rsa*, which are secrets), so an
// agent-assisted project's own tooling config travels with the template.
func TestCapture_IncludesClaudeCodeAndMCPFiles(t *testing.T) {
	fsys := newCaptureFixture(t)
	mustMkdirAll(t, fsys, "src/acme-site/.claude/commands")
	mustWriteFile(t, fsys, "src/acme-site/CLAUDE.md", []byte("# acme-site\n\nNotes about acme-site."))
	mustWriteFile(t, fsys, "src/acme-site/.claude/commands/deploy.md", []byte("Deploy acme-site."))
	mustWriteFile(t, fsys, "src/acme-site/.mcp.json", []byte(`{"mcpServers": {"acme-site": {}}}`))

	res, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: acceptAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	ops := opsByPath(res.Plan)

	claudeMD, ok := ops["CLAUDE.md"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for CLAUDE.md, got %#v (ops: %v)", ops["CLAUDE.md"], ops)
	}
	if want := "# {{name}}\n\nNotes about {{name}}."; string(claudeMD.Content) != want {
		t.Errorf("CLAUDE.md content = %q, want %q (renamed like any other file)", claudeMD.Content, want)
	}

	deployCmd, ok := ops[".claude/commands/deploy.md"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for .claude/commands/deploy.md, got %#v (ops: %v)", ops[".claude/commands/deploy.md"], ops)
	}
	if want := "Deploy {{name}}."; string(deployCmd.Content) != want {
		t.Errorf(".claude/commands/deploy.md content = %q, want %q", deployCmd.Content, want)
	}

	mcpJSON, ok := ops[".mcp.json"].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("expected WriteFileOp for .mcp.json, got %#v (ops: %v)", ops[".mcp.json"], ops)
	}
	if want := `{"mcpServers": {"{{name}}": {}}}`; string(mcpJSON.Content) != want {
		t.Errorf(".mcp.json content = %q, want %q", mcpJSON.Content, want)
	}
}

// TestCapture_RecapturingGeneratedProjectExcludesSnapDir covers Gate 1's B3:
// a project that was itself produced by `snap new` (so it has its own
// .snap/origin.toml) must not carry that stale provenance file into a
// template captured from it.
func TestCapture_RecapturingGeneratedProjectExcludesSnapDir(t *testing.T) {
	fsys := newCaptureFixture(t)
	mustMkdirAll(t, fsys, "src/acme-site/.snap")
	mustWriteFile(t, fsys, "src/acme-site/.snap/origin.toml", []byte(`template = "other-template"
version = "0.1.0"
source = "local"
generated_at = 2026-01-01T00:00:00Z
`))

	res, err := Capture(CaptureParams{
		FS: fsys, SourceDir: "src/acme-site", TargetDir: "templates/out", AsName: "out", Accept: acceptAll,
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	for _, op := range res.Plan.Ops {
		var p string
		switch o := op.(type) {
		case plan.CreateDirOp:
			p = o.Path
		case plan.WriteFileOp:
			p = o.Path
		}
		if p == ".snap" || p == ".snap/origin.toml" || len(p) >= 6 && p[:6] == ".snap/" {
			t.Errorf("Plan contains %q: stale .snap/ provenance must never be captured", p)
		}
	}
}
