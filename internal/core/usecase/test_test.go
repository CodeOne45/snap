package usecase

import (
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
)

const testFixtureManifestWithTest = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[test]
check = "go build ./..."

[test.vars]
name = "sample-project"
`

const testFixtureManifestWithoutTest = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

func TestTest_BuildsPlanWithSampleVars(t *testing.T) {
	fsys := newFixtureFSWith(t, testFixtureManifestWithTest)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	res, err := Test(TestParams{Store: store, FS: fsys, Template: "api-starter", TargetDir: "/tmp/snap-test-xyz"})
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if res.Check != "go build ./..." {
		t.Errorf("Check = %q, want %q", res.Check, "go build ./...")
	}
	readme := findWriteFileOp(t, res.Plan, "README.md")
	if string(readme.Content) != "# sample-project" {
		t.Errorf("README.md = %q, want %q", readme.Content, "# sample-project")
	}
	if res.Plan.TargetDir != "/tmp/snap-test-xyz" {
		t.Errorf("Plan.TargetDir = %q, want %q", res.Plan.TargetDir, "/tmp/snap-test-xyz")
	}
}

func TestTest_NoTestBlockIsError(t *testing.T) {
	fsys := newFixtureFSWith(t, testFixtureManifestWithoutTest)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	if _, err := Test(TestParams{Store: store, FS: fsys, Template: "api-starter", TargetDir: "/tmp/x"}); err == nil {
		t.Fatal("Test on a template with no [test] block: want error, got nil")
	}
}

func TestTest_UnknownTemplate(t *testing.T) {
	fsys := newFixtureFSWith(t, testFixtureManifestWithTest)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	if _, err := Test(TestParams{Store: store, FS: fsys, Template: "nope", TargetDir: "/tmp/x"}); err == nil {
		t.Fatal("Test on an unknown template: want error, got nil")
	}
}

func TestTest_MissingRequiredVarIsError(t *testing.T) {
	fsys := newFixtureFSWith(t, `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[variables]]
name = "port"
type = "int"

[test]
check = "go build ./..."

[test.vars]
name = "sample-project"
`)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	if _, err := Test(TestParams{Store: store, FS: fsys, Template: "api-starter", TargetDir: "/tmp/x"}); err == nil {
		t.Fatal("Test with test.vars missing a required variable (port): want error, got nil")
	}
}

// newFixtureFSWith builds a hermetic memfs template fixture at
// templates/api-starter, with the given manifest and a single
// README.md that renders "{{name}}".
func newFixtureFSWith(t *testing.T, manifest string) *memfs.MemFS {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter")
	mustWriteFile(t, fsys, "templates/api-starter/snap.toml", []byte(manifest))
	mustWriteFile(t, fsys, "templates/api-starter/README.md", []byte("# {{name}}"))
	return fsys
}
