package cli

import (
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
)

const testManifest = `
[template]
name = "api-starter"
version = "1.0.0"
description = "test fixture"

[[variables]]
name = "name"
type = "string"
validate = "^[a-z][a-z0-9-]*$"

[[variables]]
name = "author"
prompt = "Author name"
type = "string"
default = "Anonymous"
`

const testRecipeManifest = `
[recipe]
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

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""
`

// newTestDeps builds a hermetic Deps backed by memfs with one fixture
// template "api-starter" under "templates/" and one fixture recipe
// "sentry" under "recipes/".
func newTestDeps(t *testing.T) func() (Deps, error) {
	t.Helper()
	fsys := memfs.New()

	if err := fsys.MkdirAll("templates/api-starter/1.0.0/src", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("templates/api-starter/1.0.0/snap.toml", []byte(testManifest), 0o644); err != nil {
		t.Fatalf("WriteFile(snap.toml): %v", err)
	}
	if err := fsys.WriteFile("templates/api-starter/1.0.0/README.md", []byte("# {{name|title}} by {{author}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	if err := fsys.WriteFile("templates/api-starter/1.0.0/src/{{name|pascal}}.go", []byte("package main // {{name}}"), 0o644); err != nil {
		t.Fatalf("WriteFile(main.go): %v", err)
	}

	if err := fsys.MkdirAll("recipes/sentry/1.0.0/files/src/lib", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := fsys.WriteFile("recipes/sentry/1.0.0/recipe.toml", []byte(testRecipeManifest), 0o644); err != nil {
		t.Fatalf("WriteFile(recipe.toml): %v", err)
	}
	if err := fsys.WriteFile("recipes/sentry/1.0.0/files/src/lib/sentry.ts", []byte("export const service = '{{service}}';\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(sentry.ts): %v", err)
	}

	store := local.New(fsys, "templates")
	recipeStore := local.New(fsys, "recipes")
	return func() (Deps, error) {
		return Deps{FS: fsys, Store: store, RecipeStore: recipeStore}, nil
	}
}
