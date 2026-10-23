package deps

import "strings"

// findTOMLSection locates a top-level "[header]" line (e.g. "[dependencies]")
// in content and returns the byte range of its body: everything between
// that line and the next line starting with "[" (or the end of the file).
// Nested dotted headers ([a.b]) and array-of-tables ([[a]]) both still
// start with "[", so they correctly end a preceding section's body too.
func findTOMLSection(content, header string) (bodyStart, bodyEnd int, found bool) {
	lines := splitLinesKeepEnds(content)

	offset := 0
	headerIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == header {
			headerIdx = i
			bodyStart = offset + len(line)
			found = true
			break
		}
		offset += len(line)
	}
	if !found {
		return 0, 0, false
	}

	bodyEnd = len(content)
	running := bodyStart
	for i := headerIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			bodyEnd = running
			break
		}
		running += len(lines[i])
	}
	return bodyStart, bodyEnd, true
}

// lastNonBlankLineEnd returns the byte offset, relative to content, right
// after the last non-blank line in content[bodyStart:bodyEnd] — so
// appending a new entry there lands directly after the existing entries,
// never after a section's own trailing blank line(s).
func lastNonBlankLineEnd(content string, bodyStart, bodyEnd int) int {
	cursor := bodyStart
	end := bodyStart
	for _, line := range splitLinesKeepEnds(content[bodyStart:bodyEnd]) {
		cursor += len(line)
		if strings.TrimSpace(line) != "" {
			end = cursor
		}
	}
	if end == bodyStart {
		return bodyEnd
	}
	return end
}

// findTOMLSimpleStringKey finds "key = value" on its own line within body
// and returns value, unquoted if it's a simple TOML basic string. A value
// that isn't a simple quoted string (an inline table, an array, ...) is
// still returned as found=true with its raw text, so the caller can at
// least detect "something is already here" even without understanding it.
func findTOMLSimpleStringKey(body, key string) (value string, found bool) {
	for _, line := range splitLinesKeepEnds(body) {
		trimmed := strings.TrimSpace(line)
		k, v, ok := splitTOMLKeyValue(trimmed)
		if !ok || k != key {
			continue
		}
		if unquoted, isString := unquoteTOMLString(v); isString {
			return unquoted, true
		}
		return v, true
	}
	return "", false
}

func splitTOMLKeyValue(line string) (key, value string, ok bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	i := strings.Index(line, "=")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

func unquoteTOMLString(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1], true
	}
	return s, false
}

// splitLinesKeepEnds splits s into lines, each retaining its own trailing
// "\n" (the last line may lack one) — so reconstructing text is just
// concatenation, never a join separator.
func splitLinesKeepEnds(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
