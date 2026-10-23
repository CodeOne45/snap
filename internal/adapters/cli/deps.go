package cli

import (
	"os"
	"path"
	"path/filepath"

	"github.com/CodeOne45/snap/internal/adapters/osfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/ports"
)

// Deps are the dependencies a command needs. Production code gets them from
// defaultDeps; tests construct a Deps with memfs + an in-memory store to
// stay hermetic.
type Deps struct {
	FS ports.FS
	// Store resolves templates (snap new, snap list, ...).
	Store ports.Store
	// RecipeStore resolves recipes (snap add, snap recipes), a sibling
	// store of the same ports.Store shape (M20).
	RecipeStore ports.Store
}

// defaultDeps wires the real, disk-backed dependencies: templates and
// recipes both live under the user's config directory, per PLAN.md's and
// PLAN-v3.md's local stores.
func defaultDeps() (Deps, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return Deps{}, err
	}
	fsys := osfs.New()
	snapDir := filepath.ToSlash(filepath.Join(cfgDir, "snap"))
	return Deps{
		FS:          fsys,
		Store:       local.New(fsys, path.Join(snapDir, "templates")),
		RecipeStore: local.New(fsys, path.Join(snapDir, "recipes")),
	}, nil
}
