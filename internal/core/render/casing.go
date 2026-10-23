// Package render implements the placeholder templating engine: splitting
// identifiers into words, re-casing them, substituting "{{name|filter}}"
// tokens, and deciding whether content is binary. It is pure logic with no
// I/O, importable from anywhere in core.
package render

import (
	"strings"
	"unicode"
)

// Case is one of the supported re-casing filters.
type Case string

// Supported case forms.
const (
	CaseKebab  Case = "kebab"
	CaseSnake  Case = "snake"
	CasePascal Case = "pascal"
	CaseCamel  Case = "camel"
	CaseUpper  Case = "upper"
	CaseTitle  Case = "title"
)

// parseCase maps a filter name from a template token to a Case, reporting
// whether it was recognized.
func parseCase(filter string) (Case, bool) {
	switch Case(filter) {
	case CaseKebab, CaseSnake, CasePascal, CaseCamel, CaseUpper, CaseTitle:
		return Case(filter), true
	default:
		return "", false
	}
}

// SplitWords breaks an identifier into lowercase words, regardless of its
// original casing: "acme-site", "acme_site", "AcmeSite" and "Acme Site" all
// yield ["acme", "site"]. A run of uppercase letters is kept together as one
// word unless followed by a lowercase letter, so "HTTPServer" yields
// ["http", "server"] and "ACME" yields ["acme"].
func SplitWords(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(s)

	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = nil
		}
	}

	for i, r := range runes {
		switch {
		case r == '-' || r == '_' || r == ' ' || r == '.':
			flush()
		case unicode.IsUpper(r) && len(cur) > 0:
			prev := cur[len(cur)-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
				flush()
			}
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

// Apply joins words into the given case form.
func Apply(words []string, c Case) string {
	switch c {
	case CaseKebab:
		return strings.Join(words, "-")
	case CaseSnake:
		return strings.Join(words, "_")
	case CaseUpper:
		return strings.ToUpper(strings.Join(words, "_"))
	case CasePascal:
		return joinCapitalized(words, 0)
	case CaseCamel:
		return joinCapitalized(words, 1)
	case CaseTitle:
		return strings.Join(capitalizeEach(words), " ")
	default:
		return strings.Join(words, "")
	}
}

func joinCapitalized(words []string, lowerUntil int) string {
	var b strings.Builder
	for i, w := range words {
		if i < lowerUntil {
			b.WriteString(w)
			continue
		}
		b.WriteString(capitalize(w))
	}
	return b.String()
}

func capitalizeEach(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = capitalize(w)
	}
	return out
}

func capitalize(w string) string {
	if w == "" {
		return w
	}
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
