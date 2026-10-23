package render

import (
	"fmt"
	"strings"
)

// Vars maps a template variable name to its current value.
type Vars map[string]string

// Render substitutes every "{{name}}" or "{{name|filter}}" token in s.
// "{{name}}" is replaced with vars[name] verbatim; "{{name|filter}}" is
// replaced with vars[name] re-cased into filter (kebab, snake, pascal,
// camel, upper or title).
//
// A "{{...}}" whose contents don't even look like one of our tokens (e.g.
// "{{.Branch}}" — a Go text/template field reference, or "{{ foo.bar }}"
// from Jinja/Helm/Mustache) is foreign syntax that happened to survive a
// capture and is left exactly as written, not an error: a captured project
// routinely already uses "{{...}}" for something of its own. Once a token
// does look like ours — a bare identifier, optionally with "|filter" — an
// unknown variable, an unknown filter, or an unterminated "{{" is still an
// error, since that's almost certainly a real typo in an actual snap
// template.
func Render(s string, vars Vars) (string, error) {
	var out strings.Builder
	rest := s
	for {
		start := strings.Index(rest, "{{")
		if start == -1 {
			out.WriteString(rest)
			return out.String(), nil
		}

		end := strings.Index(rest[start:], "}}")
		if end == -1 {
			name, _, _ := strings.Cut(rest[start+2:], "|")
			if !isValidName(strings.TrimSpace(name)) {
				out.WriteString(rest)
				return out.String(), nil
			}
			return "", fmt.Errorf("render: unterminated placeholder in %q", rest[start:])
		}
		end += start

		out.WriteString(rest[:start])
		token := rest[start+2 : end]
		name, _, _ := strings.Cut(token, "|")
		if !isValidName(strings.TrimSpace(name)) {
			out.WriteString(rest[start : end+2])
			rest = rest[end+2:]
			continue
		}

		value, err := renderToken(token, vars)
		if err != nil {
			return "", err
		}
		out.WriteString(value)

		rest = rest[end+2:]
	}
}

func renderToken(token string, vars Vars) (string, error) {
	name, filter, hasFilter := strings.Cut(token, "|")
	name = strings.TrimSpace(name)

	value, ok := vars[name]
	if !ok {
		return "", fmt.Errorf("render: unknown variable %q", name)
	}
	if !hasFilter {
		return value, nil
	}

	filter = strings.TrimSpace(filter)
	c, ok := parseCase(filter)
	if !ok {
		return "", fmt.Errorf("render: unknown filter %q for variable %q", filter, name)
	}
	return Apply(SplitWords(value), c), nil
}

func isValidName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
