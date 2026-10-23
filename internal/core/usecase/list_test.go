package usecase

import "testing"

func TestList(t *testing.T) {
	fsys := newFixtureFS(t)
	mustMkdirAll(t, fsys, "templates/web-app")
	mustWriteFile(t, fsys, "templates/web-app/snap.toml", []byte(`
[template]
name = "web-app"
version = "0.1.0"
description = "a web app"
`))
	store := newMapStore(map[string]string{
		"api-starter": "templates/api-starter",
		"web-app":     "templates/web-app",
	})

	got, err := List(store, fsys)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(got))
	}
	if got[0].Name != "api-starter" || got[1].Name != "web-app" {
		t.Errorf("List order = [%s, %s], want [api-starter, web-app]", got[0].Name, got[1].Name)
	}
	if got[1].Description != "a web app" {
		t.Errorf("web-app.Description = %q, want %q", got[1].Description, "a web app")
	}
}
