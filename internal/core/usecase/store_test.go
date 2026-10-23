package usecase

import (
	"fmt"
	"sort"
)

// mapStore is a minimal ports.Store for tests: template name -> root path.
type mapStore struct {
	roots map[string]string
}

func newMapStore(roots map[string]string) mapStore {
	return mapStore{roots: roots}
}

func (s mapStore) List() ([]string, error) {
	names := make([]string, 0, len(s.roots))
	for name := range s.roots {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func (s mapStore) Path(name string) (string, error) {
	root, ok := s.roots[name]
	if !ok {
		return "", fmt.Errorf("mapStore: template %q not found", name)
	}
	return root, nil
}

// Versions and PathVersion don't model real versioning — mapStore's roots
// are each template's one and only version, for tests that don't exercise
// M14 directly (those use local.Store instead).
func (s mapStore) Versions(name string) ([]string, error) {
	if _, ok := s.roots[name]; !ok {
		return nil, fmt.Errorf("mapStore: template %q not found", name)
	}
	return []string{"0.1.0"}, nil
}

func (s mapStore) PathVersion(name, _ string) (string, error) {
	return s.Path(name)
}

func (s mapStore) Root() string {
	return "templates"
}
