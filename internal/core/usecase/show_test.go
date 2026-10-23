package usecase

import (
	"reflect"
	"testing"
)

func TestShow(t *testing.T) {
	fsys := newFixtureFS(t)
	store := newMapStore(map[string]string{"api-starter": "templates/api-starter"})

	got, err := Show(store, fsys, "api-starter")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got.Manifest.Name != "api-starter" {
		t.Errorf("Manifest.Name = %q, want %q", got.Manifest.Name, "api-starter")
	}

	wantFiles := []string{"README.md", "assets/logo.png", "src/{{name|pascal}}.go"}
	if !reflect.DeepEqual(got.Files, wantFiles) {
		t.Errorf("Files = %v, want %v", got.Files, wantFiles)
	}
	wantDirs := []string{"empty"}
	if !reflect.DeepEqual(got.Dirs, wantDirs) {
		t.Errorf("Dirs = %v, want %v", got.Dirs, wantDirs)
	}
}
