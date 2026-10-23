package render

import (
	"reflect"
	"testing"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"kebab", "acme-site", []string{"acme", "site"}},
		{"snake", "acme_site", []string{"acme", "site"}},
		{"pascal", "AcmeSite", []string{"acme", "site"}},
		{"camel", "acmeSite", []string{"acme", "site"}},
		{"all caps", "ACME", []string{"acme"}},
		{"title with space", "Acme Site", []string{"acme", "site"}},
		{"acronym boundary", "HTTPServer", []string{"http", "server"}},
		{"single lowercase word", "already", []string{"already"}},
		{"empty", "", nil},
		{"dotted", "acme.site", []string{"acme", "site"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitWords(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitWords(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestApply(t *testing.T) {
	words := []string{"acme", "site"}
	tests := []struct {
		c    Case
		want string
	}{
		{CaseKebab, "acme-site"},
		{CaseSnake, "acme_site"},
		{CasePascal, "AcmeSite"},
		{CaseCamel, "acmeSite"},
		{CaseUpper, "ACME_SITE"},
		{CaseTitle, "Acme Site"},
	}
	for _, tt := range tests {
		t.Run(string(tt.c), func(t *testing.T) {
			if got := Apply(words, tt.c); got != tt.want {
				t.Errorf("Apply(%v, %q) = %q, want %q", words, tt.c, got, tt.want)
			}
		})
	}
}

func TestApply_SingleWord(t *testing.T) {
	words := []string{"acme"}
	if got := Apply(words, CaseCamel); got != "acme" {
		t.Errorf("Apply(%v, camel) = %q, want %q", words, got, "acme")
	}
	if got := Apply(words, CasePascal); got != "Acme" {
		t.Errorf("Apply(%v, pascal) = %q, want %q", words, got, "Acme")
	}
}
