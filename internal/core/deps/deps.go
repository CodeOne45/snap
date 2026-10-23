// Package deps implements format-preserving dependency editors: given a
// project manifest file's current content and a dependency to add, each
// editor returns the new content with everything else about the file
// (indentation, key order, comments) left untouched. Pure — no I/O; the
// caller (usecase.Add) reads the current content and writes the result.
package deps

import "fmt"

// Dependency is one package to add to a project's dependency manifest.
type Dependency struct {
	Name    string
	Version string
	Dev     bool
}

// ConflictError is returned when Name is already present in the manifest
// at a different version than Wanted — an editor never silently
// overwrites an existing dependency.
type ConflictError struct {
	Name     string
	Existing string
	Wanted   string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("deps: %q is already present at version %q, wanted %q", e.Name, e.Existing, e.Wanted)
}
