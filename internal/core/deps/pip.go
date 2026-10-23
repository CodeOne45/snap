package deps

import "strings"

// AddRequirementsTxt adds dep to a requirements.txt's content, appending a
// new line at the end and leaving every existing line (including comments
// and blank lines) untouched. changed is false when dep is already present
// at the exact version wanted (idempotent no-op); an existing entry for
// the same package at a different version is a *ConflictError, never
// silently replaced.
//
// Version-range matching (">=2.0,<3.0" and the like) isn't attempted: an
// existing entry for the package is treated as either an exact match for
// what would be written, or a conflict — nothing in between. This is a
// deliberate simplification of PEP 440's full version-spec grammar.
func AddRequirementsTxt(content []byte, dep Dependency) ([]byte, bool, error) {
	newLine := dep.Name
	if dep.Version != "" {
		newLine += "==" + dep.Version
	}

	for _, line := range strings.Split(string(content), "\n") {
		name, spec, ok := parseRequirementLine(line)
		if !ok || !strings.EqualFold(normalizePackageName(name), normalizePackageName(dep.Name)) {
			continue
		}
		if spec == dep.Version {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: dep.Name, Existing: spec, Wanted: dep.Version}
	}

	out := content
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	out = append(append([]byte{}, out...), []byte(newLine+"\n")...)
	return out, true, nil
}

// parseRequirementLine extracts a requirement line's package name and
// version specifier (everything after the name), ignoring comments and
// blank lines.
func parseRequirementLine(line string) (name, versionSpec string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	i := strings.IndexAny(trimmed, "=<>!~[; ")
	if i < 0 {
		return trimmed, "", true
	}
	name = trimmed[:i]
	spec := strings.TrimSpace(trimmed[i:])
	spec = strings.TrimPrefix(spec, "==")
	return name, spec, true
}

// normalizePackageName applies PEP 503's name normalization (case and
// -/_/. insensitivity) so "Flask" and "flask" are recognized as the same
// package.
func normalizePackageName(name string) string {
	name = strings.ToLower(name)
	name = strings.NewReplacer("_", "-", ".", "-").Replace(name)
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	return name
}
