package usecase

import (
	"fmt"
	"path"

	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/ports"
)

// StatusParams are the inputs to Status.
type StatusParams struct {
	Store ports.Store
	FS    ports.FS
	// Dirs are the generated project directories to check, in the order
	// to report them.
	Dirs []string
}

// ProjectStatus is one directory's drift check: which template and
// version it recorded, and whether that's still the template's latest
// local version.
type ProjectStatus struct {
	Dir            string
	Template       string
	CurrentVersion string
	// LatestVersion is the template's latest local version, or "" when
	// that can't be determined: a remote-sourced project (M18 is a
	// *local* drift report — checking a remote template's latest would
	// need network access) or a template no longer in the local store.
	LatestVersion string
	// UpToDate is true only when LatestVersion is known and equals
	// CurrentVersion.
	UpToDate bool
	Source   string
	// Err is set when Dir isn't a generated project at all (no readable
	// .snap/origin.toml) — one directory's problem never stops the
	// others from being checked.
	Err error
}

// Status checks every directory in p.Dirs against its own recorded
// provenance. It never fails outright: each entry carries its own Err
// instead, so a mix of valid and invalid directories (or directories a
// caller merely guessed might be snap projects) can be checked in one
// call.
func Status(p StatusParams) []ProjectStatus {
	results := make([]ProjectStatus, len(p.Dirs))
	for i, dir := range p.Dirs {
		results[i] = statusOne(p.Store, p.FS, dir)
	}
	return results
}

func statusOne(store ports.Store, fsys ports.FS, dir string) ProjectStatus {
	st := ProjectStatus{Dir: dir}

	data, err := fsys.ReadFile(path.Join(dir, provenance.File))
	if err != nil {
		st.Err = fmt.Errorf("usecase: status: %s: read %s: %w", dir, provenance.File, err)
		return st
	}
	origin, err := provenance.ParseOrigin(data)
	if err != nil {
		st.Err = fmt.Errorf("usecase: status: %s: %w", dir, err)
		return st
	}

	st.Template = origin.Template
	st.CurrentVersion = origin.Version
	st.Source = origin.Source

	if origin.Source != "local" {
		return st
	}

	versions, err := store.Versions(origin.Template)
	if err != nil {
		return st
	}
	st.LatestVersion = versions[len(versions)-1]
	st.UpToDate = st.LatestVersion == st.CurrentVersion
	return st
}
