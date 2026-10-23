package usecase

import (
	"fmt"
	"path"

	"github.com/CodeOne45/snap/internal/core/recipe"
	"github.com/CodeOne45/snap/internal/ports"
)

// ListRecipes returns the manifest of every recipe in store, in the order
// store.List() returns names (expected to be sorted).
func ListRecipes(store ports.Store, fsys ports.FS) ([]recipe.Recipe, error) {
	names, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("usecase: list recipes: %w", err)
	}

	recipes := make([]recipe.Recipe, 0, len(names))
	for _, name := range names {
		root, err := store.Path(name)
		if err != nil {
			return nil, fmt.Errorf("usecase: %w", err)
		}
		data, err := fsys.ReadFile(path.Join(root, recipe.ManifestFile))
		if err != nil {
			return nil, fmt.Errorf("usecase: read manifest for %q: %w", name, err)
		}
		r, err := recipe.ParseManifest(data)
		if err != nil {
			return nil, fmt.Errorf("usecase: %w", err)
		}
		recipes = append(recipes, r)
	}
	return recipes, nil
}
