package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
)

var fixedAppliedAt = time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)

const sentryRecipeTOML = `[recipe]
name = "sentry"
version = "1.0.0"
description = "Error tracking with Sentry"

[applies_to]
files = ["package.json"]

[[variables]]
name = "service"
prompt = "Service name"
default_from = "project.name"

[[dependencies]]
ecosystem = "npm"
name = "@sentry/node"
version = "^8.0.0"
dev = false

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""

[[hooks.post]]
run = "npm install"
`

func newSentryRecipeFS(t *testing.T) *memfs.MemFS {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "recipes/sentry/files/src/lib")
	mustWriteFile(t, fsys, "recipes/sentry/recipe.toml", []byte(sentryRecipeTOML))
	mustWriteFile(t, fsys, "recipes/sentry/files/src/lib/sentry.ts", []byte("export const service = '{{service}}';\n"))
	return fsys
}

func newSentryTargetFixture(t *testing.T, fsys *memfs.MemFS) {
	t.Helper()
	mustMkdirAll(t, fsys, "proj")
	mustWriteFile(t, fsys, "proj/package.json", []byte(`{
  "name": "payments",
  "dependencies": {
    "express": "^4.18.0"
  }
}
`))
	mustWriteFile(t, fsys, "proj/.env.example", []byte("DATABASE_URL=postgres://localhost/app\n"))
	mustMkdirAll(t, fsys, "proj/.snap")
	origin := provenance.Origin{
		Template:    "node-starter",
		Version:     "1.0.0",
		Source:      "local",
		GeneratedAt: fixedTime,
		Variables:   map[string]any{"name": "payments"},
	}
	data, err := origin.TOML()
	if err != nil {
		t.Fatalf("origin.TOML: %v", err)
	}
	mustWriteFile(t, fsys, "proj/.snap/origin.toml", data)
}

// TestAdd_AppliesCleanFixture covers R5: applying sentry to a clean
// fixture adds the new file, the dependency, and the env key.
func TestAdd_AppliesCleanFixture(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	newSentryTargetFixture(t, fsys)
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	p, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	byPath := opsByPath(p)

	newFile, ok := byPath["src/lib/sentry.ts"].(plan.WriteFileOp)
	if !ok || string(newFile.Content) != "export const service = 'payments';\n" {
		t.Errorf("src/lib/sentry.ts = %#v, want a WriteFileOp with the rendered content", byPath["src/lib/sentry.ts"])
	}

	pkg, ok := byPath["package.json"].(plan.ModifyFileOp)
	if !ok {
		t.Fatalf("package.json = %#v, want a ModifyFileOp", byPath["package.json"])
	}
	if !strings.Contains(string(pkg.Content), `"@sentry/node": "^8.0.0"`) {
		t.Errorf("package.json content = %s, want it to contain the new dependency", pkg.Content)
	}
	if !strings.Contains(string(pkg.Content), `"express": "^4.18.0"`) {
		t.Errorf("package.json content = %s, want the existing dependency preserved", pkg.Content)
	}
	if pkg.ExpectedSHA256 == "" {
		t.Error("package.json ModifyFileOp has no ExpectedSHA256")
	}

	env, ok := byPath[".env.example"].(plan.ModifyFileOp)
	if !ok || !strings.Contains(string(env.Content), "SENTRY_DSN=") {
		t.Errorf(".env.example = %#v, want a ModifyFileOp containing SENTRY_DSN=", byPath[".env.example"])
	}
	if !strings.Contains(string(env.Content), "DATABASE_URL=") {
		t.Errorf(".env.example content = %s, want the existing key preserved", env.Content)
	}

	origin, ok := byPath[provenance.File].(plan.ModifyFileOp)
	if !ok {
		t.Fatalf("%s = %#v, want a ModifyFileOp", provenance.File, byPath[provenance.File])
	}
	wantLines := []string{
		`name = "sentry"`,
		`version = "1.0.0"`,
		`applied_at = 2026-11-02T10:00:00Z`,
		`service = "payments"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(string(origin.Content), want) {
			t.Errorf("origin.toml = %s, want it to contain %q", origin.Content, want)
		}
	}
	// The project's own template provenance must survive the rewrite.
	if !strings.Contains(string(origin.Content), `template = "node-starter"`) {
		t.Errorf("origin.toml = %s, want the existing template provenance preserved", origin.Content)
	}

	hooks := p.Hooks()
	if len(hooks) != 1 || hooks[0].Run != "npm install" {
		t.Errorf("Hooks() = %+v, want [npm install]", hooks)
	}
}

// TestAdd_Idempotent covers R6: re-applying a recipe whose files,
// dependencies, and env entries are already present produces an empty
// plan.
func TestAdd_Idempotent(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "proj/src/lib")
	mustWriteFile(t, fsys, "proj/src/lib/sentry.ts", []byte("export const service = 'payments';\n"))
	mustWriteFile(t, fsys, "proj/package.json", []byte(`{
  "name": "payments",
  "dependencies": {
    "express": "^4.18.0",
    "@sentry/node": "^8.0.0"
  }
}
`))
	mustWriteFile(t, fsys, "proj/.env.example", []byte("DATABASE_URL=postgres://localhost/app\nSENTRY_DSN=\n"))
	mustMkdirAll(t, fsys, "proj/.snap")
	origin := provenance.Origin{
		Template:    "node-starter",
		Version:     "1.0.0",
		Source:      "local",
		GeneratedAt: fixedTime,
		Variables:   map[string]any{"name": "payments"},
	}
	data, err := origin.TOML()
	if err != nil {
		t.Fatalf("origin.TOML: %v", err)
	}
	mustWriteFile(t, fsys, "proj/.snap/origin.toml", data)

	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})
	p, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(p.Ops) != 0 {
		t.Errorf("len(Ops) = %d, want 0 (fully idempotent re-apply): %+v", len(p.Ops), p.Ops)
	}
}

// TestAdd_ConflictingFileContent covers R8: target already has the
// recipe's file with different content.
func TestAdd_ConflictingFileContent(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	newSentryTargetFixture(t, fsys)
	mustMkdirAll(t, fsys, "proj/src/lib")
	mustWriteFile(t, fsys, "proj/src/lib/sentry.ts", []byte("something completely different"))
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	_, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
	})
	if err == nil {
		t.Fatal("Add with a conflicting existing file: want error, got nil")
	}
	if !strings.Contains(err.Error(), "src/lib/sentry.ts") {
		t.Errorf("error = %v, want it to mention the conflicting path", err)
	}
}

// TestAdd_AppliesToNotSatisfied covers R9: applies_to not met -> clear
// error, nothing written.
func TestAdd_AppliesToNotSatisfied(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "proj")
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	_, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
	})
	if err == nil {
		t.Fatal("Add with applies_to unsatisfied (no package.json): want error, got nil")
	}
	if !strings.Contains(err.Error(), "package.json") {
		t.Errorf("error = %v, want it to mention package.json", err)
	}
}

func TestAdd_DependencyVersionConflict(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "proj")
	mustWriteFile(t, fsys, "proj/package.json", []byte(`{
  "dependencies": {
    "@sentry/node": "^7.0.0"
  }
}
`))
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	_, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
		Vars:      map[string]string{"service": "payments"},
	})
	if err == nil {
		t.Fatal("Add with a conflicting dependency version: want error, got nil")
	}
	if !strings.Contains(err.Error(), "@sentry/node") {
		t.Errorf("error = %v, want it to mention the conflicting dependency", err)
	}
}

func TestAdd_ZeroAppliedAtIsError(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	newSentryTargetFixture(t, fsys)
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	_, err := Add(AddParams{Store: store, FS: fsys, Recipe: "sentry", TargetDir: "proj"})
	if err == nil {
		t.Fatal("Add with zero AppliedAt: want error, got nil")
	}
}

func TestAdd_UnknownRecipe(t *testing.T) {
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "proj")
	store := newMapStore(nil)

	_, err := Add(AddParams{Store: store, FS: fsys, Recipe: "nope", TargetDir: "proj", AppliedAt: fixedAppliedAt})
	if err == nil {
		t.Fatal("Add with unknown recipe: want error, got nil")
	}
}

// TestAdd_VariableExplicitOverridesDefaultFrom confirms an explicit --var
// wins over default_from.
func TestAdd_VariableExplicitOverridesDefaultFrom(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	newSentryTargetFixture(t, fsys)
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	p, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
		Vars:      map[string]string{"service": "checkout"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	newFile := opsByPath(p)["src/lib/sentry.ts"].(plan.WriteFileOp)
	if string(newFile.Content) != "export const service = 'checkout';\n" {
		t.Errorf("content = %q, want the explicit var value, not default_from", newFile.Content)
	}
}

// TestAdd_VariableNoDefaultFromNoPrompterIsError confirms a recipe
// variable with no resolvable default_from and no explicit value errors
// out when there's no prompter.
func TestAdd_VariableNoDefaultFromNoPrompterIsError(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "proj")
	mustWriteFile(t, fsys, "proj/package.json", []byte(`{"dependencies": {}}`))
	// No .snap/origin.toml at all, so default_from = "project.name" can't
	// resolve.
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	_, err := Add(AddParams{Store: store, FS: fsys, Recipe: "sentry", TargetDir: "proj", AppliedAt: fixedAppliedAt})
	if err == nil {
		t.Fatal("Add with no default_from match and no prompter: want error, got nil")
	}
}

// TestAdd_ProjectNotCreatedBySnap confirms a target with no .snap/origin.toml
// at all gets one created with only a [[recipes]] entry (no [template]
// noise), per PLAN-v3.md's "projects not created by Snap" case.
func TestAdd_ProjectNotCreatedBySnap(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "proj")
	mustWriteFile(t, fsys, "proj/package.json", []byte(`{"dependencies": {}}`))
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	p, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
		Vars:      map[string]string{"service": "payments"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	origin, ok := opsByPath(p)[provenance.File].(plan.WriteFileOp)
	if !ok {
		t.Fatalf("%s = %#v, want a WriteFileOp (brand new origin.toml)", provenance.File, opsByPath(p)[provenance.File])
	}
	if strings.Contains(string(origin.Content), "template") {
		t.Errorf("origin.toml = %s, want no template provenance for a non-Snap project", origin.Content)
	}
	if !strings.Contains(string(origin.Content), `name = "sentry"`) {
		t.Errorf("origin.toml = %s, want the recipe entry", origin.Content)
	}
}

func TestAdd_SourceDefaultsToLocal(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	newSentryTargetFixture(t, fsys)
	store := newMapStore(map[string]string{"sentry": "recipes/sentry"})

	p, err := Add(AddParams{
		Store:     store,
		FS:        fsys,
		Recipe:    "sentry",
		TargetDir: "proj",
		AppliedAt: fixedAppliedAt,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	origin := opsByPath(p)[provenance.File].(plan.ModifyFileOp)
	if !strings.Contains(string(origin.Content), `source = "local"`) {
		t.Errorf("origin.toml = %s, want the recipe entry's source = \"local\"", origin.Content)
	}
}
