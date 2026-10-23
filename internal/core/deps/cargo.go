package deps

import "fmt"

// AddCargoToml adds dep to a Cargo.toml's [dependencies] (or
// [dev-dependencies] if dep.Dev) table, preserving every other line
// untouched. The section is created, with one blank line before it, if it
// doesn't already exist. changed is false when dep is already present at
// the exact version wanted (idempotent no-op); present at a different
// version is a *ConflictError, never silently replaced. Only simple
// `name = "version"` entries are understood — a pre-existing entry
// written as an inline table (`{ version = "1", features = [...] }`) is
// still detected as "already present", conservatively treated as a
// conflict unless its raw text happens to match exactly.
func AddCargoToml(content []byte, dep Dependency) ([]byte, bool, error) {
	section := "[dependencies]"
	if dep.Dev {
		section = "[dev-dependencies]"
	}

	str := string(content)
	bodyStart, bodyEnd, found := findTOMLSection(str, section)
	if !found {
		newSection := fmt.Sprintf("\n%s\n%s = %q\n", section, dep.Name, dep.Version)
		out := make([]byte, 0, len(content)+len(newSection))
		out = append(out, content...)
		out = append(out, newSection...)
		return out, true, nil
	}

	body := str[bodyStart:bodyEnd]
	if existing, exists := findTOMLSimpleStringKey(body, dep.Name); exists {
		if existing == dep.Version {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: dep.Name, Existing: existing, Wanted: dep.Version}
	}

	insertAt := lastNonBlankLineEnd(str, bodyStart, bodyEnd)
	newLine := fmt.Sprintf("%s = %q\n", dep.Name, dep.Version)
	out := make([]byte, 0, len(content)+len(newLine))
	out = append(out, content[:insertAt]...)
	out = append(out, newLine...)
	out = append(out, content[insertAt:]...)
	return out, true, nil
}
