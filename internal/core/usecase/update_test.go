package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/executor"
)

const updateManifestV1 = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

// newUpdateFixture captures a template at 1.0.0, generates a project from
// it at a fixed time, and returns the filesystem, store and target dir so
// the test can then mutate "ours" (the project) and/or bump the template
// before calling Update.
func newUpdateFixture(t *testing.T) (*memfs.MemFS, *local.Store, string) {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/1.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/snap.toml", []byte(updateManifestV1))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/README.md", []byte("line1\nline2\nline3\n"))
	store := local.New(fsys, "templates")

	initial, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: "proj",
		Vars:        map[string]string{"name": "acme"},
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("New (initial generation): %v", err)
	}
	if err := executor.New(fsys).Apply(initial); err != nil {
		t.Fatalf("apply initial generation: %v", err)
	}
	return fsys, store, "proj"
}

// bumpTemplate writes a new template version with the given README
// content, keeping everything else the same.
func bumpTemplate(t *testing.T, fsys *memfs.MemFS, version, readmeContent string) {
	t.Helper()
	manifest := strings.Replace(updateManifestV1, `version = "1.0.0"`, `version = "`+version+`"`, 1)
	mustMkdirAll(t, fsys, "templates/api-starter/"+version)
	mustWriteFile(t, fsys, "templates/api-starter/"+version+"/snap.toml", []byte(manifest))
	mustWriteFile(t, fsys, "templates/api-starter/"+version+"/README.md", []byte(readmeContent))
}

func TestUpdate_CleanUpdate(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	bumpTemplate(t, fsys, "1.1.0", "line1\nline2 CHANGED\nline3\n")

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none", res.Conflicts)
	}
	if res.OldVersion != "1.0.0" || res.NewVersion != "1.1.0" {
		t.Errorf("OldVersion/NewVersion = %q/%q, want %q/%q", res.OldVersion, res.NewVersion, "1.0.0", "1.1.0")
	}

	readme := findWriteFileOp(t, res.Plan, "README.md")
	if string(readme.Content) != "line1\nline2 CHANGED\nline3\n" {
		t.Errorf("README.md = %q, want the template's new content", readme.Content)
	}
	origin := findOriginOp(t, res.Plan)
	o, err := provenance.ParseOrigin(origin.Content)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if o.Version != "1.1.0" {
		t.Errorf("new origin.toml version = %q, want %q", o.Version, "1.1.0")
	}
}

func TestUpdate_ConflictingLocalEdit(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	mustWriteFile(t, fsys, target+"/README.md", []byte("line1\nUSER CHANGED\nline3\n"))
	bumpTemplate(t, fsys, "1.1.0", "line1\nTEMPLATE CHANGED\nline3\n")

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "README.md" {
		t.Fatalf("Conflicts = %v, want [README.md]", res.Conflicts)
	}

	readme := findWriteFileOp(t, res.Plan, "README.md")
	if !strings.Contains(string(readme.Content), "<<<<<<< ours") ||
		!strings.Contains(string(readme.Content), "USER CHANGED") ||
		!strings.Contains(string(readme.Content), "TEMPLATE CHANGED") {
		t.Errorf("README.md = %q, want conflict markers with both sides", readme.Content)
	}

	// origin.toml must be unchanged (byte-identical) -- updated only on success.
	originBefore, err := fsys.ReadFile(target + "/.snap/origin.toml")
	if err != nil {
		t.Fatalf("ReadFile(original origin.toml): %v", err)
	}
	originOp := findOriginOp(t, res.Plan)
	if string(originOp.Content) != string(originBefore) {
		t.Errorf("origin.toml changed despite a conflict:\ngot:  %q\nwant: %q", originOp.Content, originBefore)
	}
}

func TestUpdate_FileDeletedLocally_NoConflict(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	if err := fsys.Remove(target + "/README.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	bumpTemplate(t, fsys, "1.1.0", "line1\nline2\nline3\n") // template didn't change it

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none", res.Conflicts)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "README.md" {
		t.Errorf("Deleted = %v, want [README.md]", res.Deleted)
	}
	if hasWriteFileOp(res.Plan, "README.md") {
		t.Error("README.md should not be written: it was deleted locally and the template didn't change it")
	}
}

func TestUpdate_FileDeletedLocally_TemplateChangedIt_Conflict(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	if err := fsys.Remove(target + "/README.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	bumpTemplate(t, fsys, "1.1.0", "line1\nline2 CHANGED\nline3\n")

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0] != "README.md" {
		t.Fatalf("Conflicts = %v, want [README.md] (modify/delete conflict)", res.Conflicts)
	}
	if hasWriteFileOp(res.Plan, "README.md") {
		t.Error("README.md should stay deleted even though it's flagged as a conflict")
	}
}

func TestUpdate_FileRenamedInTemplate_Clean(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	// README.md is renamed to GUIDE.md with the exact same content.
	manifest := strings.Replace(updateManifestV1, `version = "1.0.0"`, `version = "1.1.0"`, 1)
	mustMkdirAll(t, fsys, "templates/api-starter/1.1.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/snap.toml", []byte(manifest))
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/GUIDE.md", []byte("line1\nline2\nline3\n"))

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none (clean rename)", res.Conflicts)
	}
	if hasWriteFileOp(res.Plan, "README.md") {
		t.Error("README.md should be gone (renamed)")
	}
	if !hasWriteFileOp(res.Plan, "GUIDE.md") {
		t.Error("GUIDE.md should exist (the renamed file)")
	}
	for _, d := range res.Deleted {
		if d == "README.md" {
			return
		}
	}
	t.Errorf("Deleted = %v, want it to include README.md", res.Deleted)
}

func TestUpdate_FileRenamedInTemplate_UserEditedOldPath_Conflict(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	mustWriteFile(t, fsys, target+"/README.md", []byte("line1\nUSER EDIT\nline3\n"))
	manifest := strings.Replace(updateManifestV1, `version = "1.0.0"`, `version = "1.1.0"`, 1)
	mustMkdirAll(t, fsys, "templates/api-starter/1.1.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/snap.toml", []byte(manifest))
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/GUIDE.md", []byte("line1\nline2\nline3\n"))

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) == 0 {
		t.Fatal("Conflicts = none, want a rename/modify conflict reported")
	}
	readme := findWriteFileOp(t, res.Plan, "README.md")
	if string(readme.Content) != "line1\nUSER EDIT\nline3\n" {
		t.Errorf("README.md = %q, want the user's edit preserved", readme.Content)
	}
	if !hasWriteFileOp(res.Plan, "GUIDE.md") {
		t.Error("GUIDE.md should still exist with the template's content")
	}
}

func TestUpdate_FileAddedInTemplate(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	manifest := strings.Replace(updateManifestV1, `version = "1.0.0"`, `version = "1.1.0"`, 1)
	mustMkdirAll(t, fsys, "templates/api-starter/1.1.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/snap.toml", []byte(manifest))
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/README.md", []byte("line1\nline2\nline3\n"))
	mustWriteFile(t, fsys, "templates/api-starter/1.1.0/NEWFILE.md", []byte("brand new"))

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none", res.Conflicts)
	}
	newfile := findWriteFileOp(t, res.Plan, "NEWFILE.md")
	if string(newfile.Content) != "brand new" {
		t.Errorf("NEWFILE.md = %q, want %q", newfile.Content, "brand new")
	}
}

func TestUpdate_UserAddedFileUnrelatedToTemplate(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	mustWriteFile(t, fsys, target+"/NOTES.md", []byte("my own notes"))
	bumpTemplate(t, fsys, "1.1.0", "line1\nline2 CHANGED\nline3\n")

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	notes := findWriteFileOp(t, res.Plan, "NOTES.md")
	if string(notes.Content) != "my own notes" {
		t.Errorf("NOTES.md = %q, want it preserved unchanged", notes.Content)
	}
}

func hasWriteFileOp(p plan.Plan, path string) bool {
	for _, op := range p.Ops {
		if wf, ok := op.(plan.WriteFileOp); ok && wf.Path == path {
			return true
		}
	}
	return false
}

func TestUpdate_EndToEnd_ApplyAndReread(t *testing.T) {
	fsys, store, target := newUpdateFixture(t)
	bumpTemplate(t, fsys, "1.1.0", "line1\nline2 CHANGED\nline3\n")

	res, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: target,
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none", res.Conflicts)
	}

	if err := executor.New(fsys).ApplyUpdate(res.Plan); err != nil {
		t.Fatalf("ApplyUpdate: %v", err)
	}

	readme, err := fsys.ReadFile(target + "/README.md")
	if err != nil || string(readme) != "line1\nline2 CHANGED\nline3\n" {
		t.Errorf("README.md = %q, %v, want the updated content", readme, err)
	}
	originData, err := fsys.ReadFile(target + "/.snap/origin.toml")
	if err != nil {
		t.Fatalf("ReadFile(origin.toml): %v", err)
	}
	o, err := provenance.ParseOrigin(originData)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if o.Version != "1.1.0" {
		t.Errorf("origin.toml version = %q, want %q", o.Version, "1.1.0")
	}
	if o.Variables["name"] != "acme" {
		t.Errorf("origin.toml variables[name] = %v, want %q", o.Variables["name"], "acme")
	}
}

func TestUpdate_UnknownTargetOrOrigin(t *testing.T) {
	fsys, store, _ := newUpdateFixture(t)
	_, err := Update(UpdateParams{
		Store: store, FS: fsys, TargetDir: "does-not-exist",
		GeneratedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Error("Update on a dir with no .snap/origin.toml: want error, got nil")
	}
}
