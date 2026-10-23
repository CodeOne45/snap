package deps

import (
	"errors"
	"strings"
	"testing"
)

const goModFixture = `module example.com/myapp

go 1.23

require (
	github.com/foo/bar v1.2.3
)
`

func TestAddGoMod_AddsNewRequire(t *testing.T) {
	got, changed, err := AddGoMod([]byte(goModFixture), Dependency{Name: "github.com/sentry/sentry-go", Version: "v0.27.0"})
	if err != nil {
		t.Fatalf("AddGoMod: %v", err)
	}
	if !changed {
		t.Error("changed = false, want true")
	}
	if !strings.Contains(string(got), "github.com/sentry/sentry-go v0.27.0") {
		t.Errorf("content = %s, want it to contain the new require", got)
	}
	if !strings.Contains(string(got), "github.com/foo/bar v1.2.3") {
		t.Errorf("content = %s, want the existing require preserved", got)
	}
	if !strings.Contains(string(got), "module example.com/myapp") {
		t.Errorf("content = %s, want the module directive preserved", got)
	}
}

func TestAddGoMod_Idempotent(t *testing.T) {
	_, changed, err := AddGoMod([]byte(goModFixture), Dependency{Name: "github.com/foo/bar", Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("AddGoMod: %v", err)
	}
	if changed {
		t.Error("changed = true, want false (already present at the same version)")
	}
}

func TestAddGoMod_ConflictDifferentVersion(t *testing.T) {
	_, _, err := AddGoMod([]byte(goModFixture), Dependency{Name: "github.com/foo/bar", Version: "v2.0.0"})
	if err == nil {
		t.Fatal("AddGoMod with a conflicting version: want error, got nil")
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want *ConflictError", err)
	}
	if conflict.Name != "github.com/foo/bar" || conflict.Existing != "v1.2.3" || conflict.Wanted != "v2.0.0" {
		t.Errorf("conflict = %+v, want Existing=v1.2.3 Wanted=v2.0.0", conflict)
	}
}

func TestAddGoMod_InvalidGoMod(t *testing.T) {
	if _, _, err := AddGoMod([]byte("not a go.mod file {{{"), Dependency{Name: "x", Version: "v1.0.0"}); err == nil {
		t.Fatal("AddGoMod with malformed content: want error, got nil")
	}
}
