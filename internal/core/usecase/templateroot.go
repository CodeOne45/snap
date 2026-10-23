package usecase

import (
	"strings"

	"github.com/CodeOne45/snap/internal/ports"
)

// resolveStoreRoot resolves nameOrPin to a root directory in store — a
// template store or (M20) a recipe store, both the same ports.Store shape.
// nameOrPin is either a bare name (resolved to its latest version) or
// "name@version" — M14's local pinning syntax, mirroring M11's remote
// "@tag"/"@commit" pins — resolved to exactly that version. The plain name
// is tried first, so a name that happens to contain an "@" still resolves
// correctly if one exists under that literal name.
func resolveStoreRoot(store ports.Store, nameOrPin string) (string, error) {
	if root, err := store.Path(nameOrPin); err == nil {
		return root, nil
	}
	if name, version, ok := splitVersionPin(nameOrPin); ok {
		return store.PathVersion(name, version)
	}
	return store.Path(nameOrPin)
}

// splitVersionPin splits a trailing "@version" off s. Both the name and
// version must be non-empty, so "@1.0.0" and "name@" don't match.
func splitVersionPin(s string) (name, version string, ok bool) {
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return "", "", false
	}
	return s[:at], s[at+1:], true
}
