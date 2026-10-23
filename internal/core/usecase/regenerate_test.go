package usecase

import (
	"time"

	"testing"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
)

const regenManifestV1 = `
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[variables]]
name = "port"
type = "int"
`

// regenManifestV2 is a later, bumped version with different content, to
// prove regeneration recovers the OLD base, not whatever the template
// looks like now.
const regenManifestV2 = `
[template]
name = "api-starter"
version = "2.0.0"

[[variables]]
name = "name"
type = "string"

[[variables]]
name = "port"
type = "int"
`

func newRegenerateFixture(t *testing.T) (*memfs.MemFS, *local.Store) {
	t.Helper()
	fsys := memfs.New()
	mustMkdirAll(t, fsys, "templates/api-starter/1.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/snap.toml", []byte(regenManifestV1))
	mustWriteFile(t, fsys, "templates/api-starter/1.0.0/README.md", []byte("# {{name}}, v1"))
	return fsys, local.New(fsys, "templates")
}

func TestRegenerateBase_MatchesOriginalByteForByte(t *testing.T) {
	fsys, store := newRegenerateFixture(t)
	generatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	original, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: "out",
		Vars:        map[string]string{"name": "acme", "port": "8080"},
		GeneratedAt: generatedAt,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	originOp := findWriteFileOp(t, original, ".snap/origin.toml")

	origin, err := provenance.ParseOrigin(originOp.Content)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}

	regen, err := RegenerateBase(RegenerateParams{
		Store: store, FS: fsys, Origin: origin, TargetDir: "out",
	})
	if err != nil {
		t.Fatalf("RegenerateBase: %v", err)
	}

	assertSamePlanOps(t, original, regen)
}

func TestRegenerateBase_SurvivesTemplateBumpedForward(t *testing.T) {
	fsys, store := newRegenerateFixture(t)
	generatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	original, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: "out",
		Vars:        map[string]string{"name": "acme", "port": "8080"},
		GeneratedAt: generatedAt,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	originOp := findWriteFileOp(t, original, ".snap/origin.toml")
	origin, err := provenance.ParseOrigin(originOp.Content)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}

	// The template moves on to a new version with different content.
	mustMkdirAll(t, fsys, "templates/api-starter/2.0.0")
	mustWriteFile(t, fsys, "templates/api-starter/2.0.0/snap.toml", []byte(regenManifestV2))
	mustWriteFile(t, fsys, "templates/api-starter/2.0.0/README.md", []byte("# {{name}}, v2 (totally different)"))

	regen, err := RegenerateBase(RegenerateParams{
		Store: store, FS: fsys, Origin: origin, TargetDir: "out",
	})
	if err != nil {
		t.Fatalf("RegenerateBase: %v", err)
	}

	readme := findWriteFileOp(t, regen, "README.md")
	if string(readme.Content) != "# acme, v1" {
		t.Errorf("regenerated README.md = %q, want the original v1 content, unaffected by the later bump", readme.Content)
	}
	assertSamePlanOps(t, original, regen)
}

func TestRegenerateBase_VariablesRoundTripThroughOrigin(t *testing.T) {
	fsys, store := newRegenerateFixture(t)
	generatedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	original, err := New(NewParams{
		Store: store, FS: fsys, Template: "api-starter", TargetDir: "out",
		Vars:        map[string]string{"name": "acme", "port": "9090"},
		GeneratedAt: generatedAt,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	origin, err := provenance.ParseOrigin(findWriteFileOp(t, original, ".snap/origin.toml").Content)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}

	regen, err := RegenerateBase(RegenerateParams{Store: store, FS: fsys, Origin: origin, TargetDir: "out"})
	if err != nil {
		t.Fatalf("RegenerateBase: %v", err)
	}
	readme := findWriteFileOp(t, regen, "README.md")
	if string(readme.Content) != "# acme, v1" {
		t.Errorf("README.md = %q, want %q (port, an int variable, round-tripped correctly as a side effect)", readme.Content, "# acme, v1")
	}
}

func TestRegenerateBase_UnknownVersionIsError(t *testing.T) {
	fsys, store := newRegenerateFixture(t)
	origin := provenance.Origin{
		Template: "api-starter", Version: "9.9.9", Source: "local",
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Variables:   map[string]any{"name": "acme", "port": int64(8080)},
	}
	if _, err := RegenerateBase(RegenerateParams{Store: store, FS: fsys, Origin: origin, TargetDir: "out"}); err == nil {
		t.Error("RegenerateBase with a version that no longer exists: want error, got nil")
	}
}

// assertSamePlanOps fails unless a and b write the exact same set of paths
// with identical content — regardless of TargetDir, which the two Plans
// are free to differ on.
func assertSamePlanOps(t *testing.T, a, b plan.Plan) {
	t.Helper()
	byPath := func(p plan.Plan) map[string]plan.WriteFileOp {
		out := map[string]plan.WriteFileOp{}
		for _, op := range p.Ops {
			if wf, ok := op.(plan.WriteFileOp); ok {
				out[wf.Path] = wf
			}
		}
		return out
	}
	aOps, bOps := byPath(a), byPath(b)
	if len(aOps) != len(bOps) {
		t.Fatalf("op count = %d, want %d (a=%v, b=%v)", len(bOps), len(aOps), keysOf(aOps), keysOf(bOps))
	}
	for path, wf := range aOps {
		other, ok := bOps[path]
		if !ok {
			t.Errorf("regenerated plan is missing %q", path)
			continue
		}
		if string(wf.Content) != string(other.Content) {
			t.Errorf("content mismatch for %q:\n--- original ---\n%s\n--- regenerated ---\n%s", path, wf.Content, other.Content)
		}
	}
}

func keysOf(m map[string]plan.WriteFileOp) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
