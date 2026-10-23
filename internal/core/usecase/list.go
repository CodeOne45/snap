package usecase

import (
	"fmt"
	"path"

	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// List returns the manifest of every template in store, in the order
// store.List() returns names (expected to be sorted).
func List(store ports.Store, fsys ports.FS) ([]template.Manifest, error) {
	names, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("usecase: list: %w", err)
	}

	manifests := make([]template.Manifest, 0, len(names))
	for _, name := range names {
		m, err := loadManifest(store, fsys, name)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

func loadManifest(store ports.Store, fsys ports.FS, name string) (template.Manifest, error) {
	root, err := store.Path(name)
	if err != nil {
		return template.Manifest{}, fmt.Errorf("usecase: %w", err)
	}
	data, err := fsys.ReadFile(path.Join(root, template.ManifestFile))
	if err != nil {
		return template.Manifest{}, fmt.Errorf("usecase: read manifest for %q: %w", name, err)
	}
	m, err := template.ParseManifest(data)
	if err != nil {
		return template.Manifest{}, fmt.Errorf("usecase: %w", err)
	}
	return m, nil
}
