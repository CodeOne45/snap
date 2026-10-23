package deps

import (
	"fmt"
	"strings"
)

// AddPyprojectToml adds dep to a pyproject.toml's [project] dependencies
// array (PEP 621 — a plain array of requirement-spec strings, not a
// sub-table), preserving every other line untouched. The array is created
// if [project] exists but has no dependencies array yet; a missing
// [project] section entirely is an error (nothing to anchor the insertion
// to). changed is false when dep is already present at the exact version
// wanted (idempotent no-op); present at a different version is a
// *ConflictError, never silently replaced. Indentation for a newly
// created array always matches PLAN-v3.md's own convention (4 spaces) —
// existing arrays keep whatever's already there since only a new line is
// appended among them.
func AddPyprojectToml(content []byte, dep Dependency) ([]byte, bool, error) {
	str := string(content)
	bodyStart, bodyEnd, found := findTOMLSection(str, "[project]")
	if !found {
		return nil, false, fmt.Errorf("deps: pyproject.toml has no [project] section")
	}

	newSpec := dep.Name
	if dep.Version != "" {
		newSpec += "==" + dep.Version
	}

	openIdx, closeIdx, arrayFound := findTOMLArray(str, bodyStart, bodyEnd, "dependencies")
	if !arrayFound {
		insertAt := lastNonBlankLineEnd(str, bodyStart, bodyEnd)
		newArray := fmt.Sprintf("dependencies = [\n    %q,\n]\n", newSpec)
		out := make([]byte, 0, len(content)+len(newArray))
		out = append(out, content[:insertAt]...)
		out = append(out, newArray...)
		out = append(out, content[insertAt:]...)
		return out, true, nil
	}

	for _, entry := range parseTOMLStringArrayEntries(str[openIdx:closeIdx]) {
		name, spec, ok := parseRequirementLine(entry)
		if !ok || !strings.EqualFold(normalizePackageName(name), normalizePackageName(dep.Name)) {
			continue
		}
		if spec == dep.Version {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: dep.Name, Existing: spec, Wanted: dep.Version}
	}

	insertAt := lastNonBlankLineEnd(str, openIdx, closeIdx)
	newLine := fmt.Sprintf("    %q,\n", newSpec)
	out := make([]byte, 0, len(content)+len(newLine))
	out = append(out, content[:insertAt]...)
	out = append(out, newLine...)
	out = append(out, content[insertAt:]...)
	return out, true, nil
}

// findTOMLArray locates "key = [" within content[searchStart:searchEnd]
// and returns the absolute byte offsets of the array's inner content
// (just after "[", up to but excluding its matching "]"). The matching
// close bracket is found with a string-aware depth count, so a requirement
// spec containing "[" or "]" (pip extras, e.g. "flask[async]==2.0") never
// confuses it.
func findTOMLArray(content string, searchStart, searchEnd int, key string) (openIdx, closeIdx int, found bool) {
	offset := searchStart
	openIdx = -1
	for _, line := range splitLinesKeepEnds(content[searchStart:searchEnd]) {
		trimmed := strings.TrimSpace(line)
		if k, v, ok := splitTOMLKeyValue(trimmed); ok && k == key && strings.HasPrefix(v, "[") {
			bracketPos := strings.IndexByte(line, '[')
			openIdx = offset + bracketPos + 1
			break
		}
		offset += len(line)
	}
	if openIdx < 0 {
		return 0, 0, false
	}

	depth := 1
	inString := false
	for i := openIdx; i < len(content); i++ {
		c := content[i]
		switch {
		case c == '"' && (i == 0 || content[i-1] != '\\'):
			inString = !inString
		case !inString && c == '[':
			depth++
		case !inString && c == ']':
			depth--
			if depth == 0 {
				return openIdx, i, true
			}
		}
	}
	return 0, 0, false
}

// parseTOMLStringArrayEntries extracts each quoted string's unescaped
// content from a TOML array's inner text (between "[" and "]").
func parseTOMLStringArrayEntries(arrayBody string) []string {
	var entries []string
	var cur strings.Builder
	inString := false
	for i := 0; i < len(arrayBody); i++ {
		c := arrayBody[i]
		if c == '"' && (i == 0 || arrayBody[i-1] != '\\') {
			if inString {
				entries = append(entries, cur.String())
				cur.Reset()
			}
			inString = !inString
			continue
		}
		if inString {
			cur.WriteByte(c)
		}
	}
	return entries
}
