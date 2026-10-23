package usecase

import "testing"

func TestListRecipes(t *testing.T) {
	fsys := newSentryRecipeFS(t)
	mustMkdirAll(t, fsys, "recipes/docker")
	mustWriteFile(t, fsys, "recipes/docker/recipe.toml", []byte(`
[recipe]
name = "docker"
version = "0.1.0"
description = "Dockerfile and compose"
`))
	store := newMapStore(map[string]string{
		"sentry": "recipes/sentry",
		"docker": "recipes/docker",
	})

	got, err := ListRecipes(store, fsys)
	if err != nil {
		t.Fatalf("ListRecipes: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(ListRecipes) = %d, want 2", len(got))
	}
	if got[0].Name != "docker" || got[1].Name != "sentry" {
		t.Errorf("ListRecipes order = [%s, %s], want [docker, sentry]", got[0].Name, got[1].Name)
	}
	if got[0].Description != "Dockerfile and compose" {
		t.Errorf("docker.Description = %q, want %q", got[0].Description, "Dockerfile and compose")
	}
}
