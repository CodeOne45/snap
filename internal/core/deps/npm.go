package deps

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AddPackageJSON adds dep to a package.json's "dependencies" (or
// "devDependencies" if dep.Dev) object, preserving every other line,
// including indentation and key order. The object is created, as a new
// top-level key, if it doesn't already exist. changed is false when dep
// is already present at the exact version wanted (idempotent no-op);
// present at a different version is a *ConflictError, never silently
// replaced. New entries use 2-space indentation, matching the default
// `npm install` convention, regardless of the file's own indentation
// style.
func AddPackageJSON(content []byte, dep Dependency) ([]byte, bool, error) {
	if !json.Valid(content) {
		return nil, false, fmt.Errorf("deps: package.json is not valid JSON")
	}

	key := "dependencies"
	if dep.Dev {
		key = "devDependencies"
	}

	str := string(content)
	openIdx, closeIdx, found := findJSONObject(str, key)
	if !found {
		return addNewPackageJSONKey(content, key, dep)
	}

	body := str[openIdx:closeIdx]
	if existing, exists := findJSONStringKey(body, dep.Name); exists {
		if existing == dep.Version {
			return content, false, nil
		}
		return nil, false, &ConflictError{Name: dep.Name, Existing: existing, Wanted: dep.Version}
	}

	entry := fmt.Sprintf("    %q: %q\n", dep.Name, dep.Version)
	return insertJSONEntry(content, openIdx, closeIdx, entry, "  "), true, nil
}

// addNewPackageJSONKey inserts a brand-new "key": { "name": "version" }
// entry into the root object, when key (e.g. "dependencies") doesn't
// exist in the file at all yet.
func addNewPackageJSONKey(content []byte, key string, dep Dependency) ([]byte, bool, error) {
	rootOpen, rootClose, ok := findJSONRootObject(string(content))
	if !ok {
		return nil, false, fmt.Errorf("deps: could not locate package.json's root object")
	}
	section := fmt.Sprintf("  %q: {\n    %q: %q\n  }\n", key, dep.Name, dep.Version)
	return insertJSONEntry(content, rootOpen, rootClose, section, ""), true, nil
}

// insertJSONEntry inserts entry as a new last member of the object whose
// body spans content[bodyStart:bodyEnd] (exclusive of its braces). An
// empty object ("{}", nothing between the braces at all) gets a newline
// on each side of entry plus closingIndent before the now-reachable
// closing brace, so "{}" becomes a properly indented multi-line object
// instead of gluing entry directly onto the opening brace's line. A
// non-empty object has a comma added after its current last line first
// (JSON allows no trailing comma on the last member, so whichever entry
// ends up last must be the one without one).
func insertJSONEntry(content []byte, bodyStart, bodyEnd int, entry, closingIndent string) []byte {
	str := string(content)
	insertAt := lastNonBlankLineEnd(str, bodyStart, bodyEnd)

	if insertAt == bodyStart {
		return spliceAt(content, bodyEnd, "\n"+entry+closingIndent)
	}

	commaPos := insertAt
	if commaPos > 0 && content[commaPos-1] == '\n' {
		commaPos--
	}
	out := make([]byte, 0, len(content)+1+len(entry))
	out = append(out, content[:commaPos]...)
	out = append(out, ',')
	out = append(out, content[commaPos:insertAt]...)
	out = append(out, entry...)
	out = append(out, content[insertAt:]...)
	return out
}

func spliceAt(content []byte, at int, insert string) []byte {
	out := make([]byte, 0, len(content)+len(insert))
	out = append(out, content[:at]...)
	out = append(out, insert...)
	out = append(out, content[at:]...)
	return out
}

// findJSONObject locates a `"key": { ... }` member anywhere in content
// and returns the byte offsets of its value's inner content (just after
// the object's own "{", up to but excluding its matching "}"). The
// matching close brace is found with a string-aware depth count, so a
// version string or key name containing "{"/"}" never confuses it, and a
// false match (the key text appearing as part of a longer key, or as a
// string value) is skipped in favor of a real match further on.
func findJSONObject(content, key string) (openIdx, closeIdx int, found bool) {
	quoted := `"` + key + `"`
	pos := 0
	for {
		i := strings.Index(content[pos:], quoted)
		if i < 0 {
			return 0, 0, false
		}
		j := pos + i + len(quoted)
		for j < len(content) && isJSONSpace(content[j]) {
			j++
		}
		if j >= len(content) || content[j] != ':' {
			pos += i + len(quoted)
			continue
		}
		j++
		for j < len(content) && isJSONSpace(content[j]) {
			j++
		}
		if j >= len(content) || content[j] != '{' {
			pos += i + len(quoted)
			continue
		}
		open := j + 1
		if closeIdx, ok := matchJSONBrace(content, open); ok {
			return open, closeIdx, true
		}
		pos += i + len(quoted)
	}
}

// findJSONRootObject returns the byte offsets of the file's own outermost
// object's body (content is expected to be a JSON object at its root, as
// package.json always is).
func findJSONRootObject(content string) (openIdx, closeIdx int, found bool) {
	start := strings.IndexByte(content, '{')
	if start < 0 {
		return 0, 0, false
	}
	open := start + 1
	closeIdx, ok := matchJSONBrace(content, open)
	return open, closeIdx, ok
}

// matchJSONBrace returns the index of the "}" matching the "{" that ends
// just before open (i.e. open is the position right after that "{"),
// scanning forward with a string-aware depth count.
func matchJSONBrace(content string, open int) (int, bool) {
	depth := 1
	inString := false
	for i := open; i < len(content); i++ {
		c := content[i]
		switch {
		case c == '"' && content[i-1] != '\\':
			inString = !inString
		case !inString && c == '{':
			depth++
		case !inString && c == '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// findJSONStringKey finds `"key": value` within body (a flat object's
// inner content) and returns value. A simple string value is unquoted; a
// non-string value (a nested object/array, a number, true/false/null) is
// still reported as found=true, with its raw, untrimmed-of-quotes text —
// enough to detect "something is already here" and report a conflict even
// without fully understanding it.
func findJSONStringKey(body, key string) (value string, found bool) {
	quoted := `"` + key + `"`
	pos := 0
	for {
		i := strings.Index(body[pos:], quoted)
		if i < 0 {
			return "", false
		}
		j := pos + i + len(quoted)
		for j < len(body) && isJSONSpace(body[j]) {
			j++
		}
		if j >= len(body) || body[j] != ':' {
			pos += i + len(quoted)
			continue
		}
		j++
		for j < len(body) && isJSONSpace(body[j]) {
			j++
		}
		if j >= len(body) || body[j] != '"' {
			end := strings.IndexAny(body[j:], ",\n}")
			if end < 0 {
				end = len(body) - j
			}
			return strings.TrimSpace(body[j : j+end]), true
		}
		start := j + 1
		k := start
		for k < len(body) && (body[k] != '"' || body[k-1] == '\\') {
			k++
		}
		return body[start:k], true
	}
}

func isJSONSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
