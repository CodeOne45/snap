package ports

// Store is the template store port. Implementations resolve a template name
// (optionally at a specific version) to a location on an FS (local
// directory, cloned git repo, ...).
type Store interface {
	// List returns the names of all available templates.
	List() ([]string, error)
	// Path returns the root directory of the named template's latest
	// (highest semver) version on an FS.
	Path(name string) (string, error)
	// Versions returns every version of name available, as semver strings
	// sorted ascending (oldest first).
	Versions(name string) ([]string, error)
	// PathVersion returns the root directory of name at exactly version
	// (as Versions returns it).
	PathVersion(name, version string) (string, error)
	// Root returns the store's own root directory, for resolving where a
	// new template (not yet in the store) should be written — e.g. by
	// snap capture.
	Root() string
}
