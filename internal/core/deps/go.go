package deps

import (
	"fmt"

	"golang.org/x/mod/modfile"
)

// AddGoMod adds dep as a require in a go.mod's content, preserving the
// module/go directives and every other existing require. Go has no
// separate "dev dependency" concept, so dep.Dev is ignored. changed is
// false when dep's module path is already required at the exact version
// wanted (idempotent no-op); required at a different version is a
// *ConflictError, never silently replaced.
func AddGoMod(content []byte, dep Dependency) ([]byte, bool, error) {
	f, err := modfile.Parse("go.mod", content, nil)
	if err != nil {
		return nil, false, fmt.Errorf("deps: parse go.mod: %w", err)
	}

	for _, r := range f.Require {
		if r.Mod.Path != dep.Name {
			continue
		}
		if r.Mod.Version == dep.Version {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: dep.Name, Existing: r.Mod.Version, Wanted: dep.Version}
	}

	if err := f.AddRequire(dep.Name, dep.Version); err != nil {
		return nil, false, fmt.Errorf("deps: add require %s: %w", dep.Name, err)
	}
	f.Cleanup()

	out, err := f.Format()
	if err != nil {
		return nil, false, fmt.Errorf("deps: format go.mod: %w", err)
	}
	return out, true, nil
}
