package render

import "testing"

func TestRender(t *testing.T) {
	vars := Vars{"name": "acme-site", "port": "8080"}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no placeholders", "plain text", "plain text"},
		{"plain substitution", "{{name}}", "acme-site"},
		{"pascal filter", "{{name|pascal}}", "AcmeSite"},
		{"camel filter", "{{name|camel}}", "acmeSite"},
		{"snake filter", "{{name|snake}}", "acme_site"},
		{"upper filter", "{{name|upper}}", "ACME_SITE"},
		{"title filter", "{{name|title}}", "Acme Site"},
		{"multiple tokens", "{{name}}/{{name|pascal}}.go", "acme-site/AcmeSite.go"},
		{"filter on already-mixed-case value", "{{port}}", "8080"},
		{"whitespace inside braces", "{{ name | pascal }}", "AcmeSite"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.in, vars)
			if err != nil {
				t.Fatalf("Render(%q) error = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Render(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRender_Errors(t *testing.T) {
	vars := Vars{"name": "acme-site"}

	tests := []struct {
		name string
		in   string
	}{
		{"unknown variable", "{{missing}}"},
		{"unknown filter", "{{name|bogus}}"},
		{"unterminated valid-looking name", "{{name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Render(tt.in, vars); err == nil {
				t.Errorf("Render(%q): want error, got nil", tt.in)
			}
		})
	}
}

// TestRender_ForeignSyntaxPassesThrough covers "{{...}}" that doesn't look
// like one of our tokens at all — e.g. a Go text/template field reference,
// or Jinja/Helm/Mustache syntax — that a captured project already contains
// for its own unrelated purposes. It must survive untouched, not error.
func TestRender_ForeignSyntaxPassesThrough(t *testing.T) {
	vars := Vars{"name": "acme-site"}

	tests := []struct {
		name string
		in   string
	}{
		{"go template field access", "{{.Branch}}"},
		{"go template with spaces and dots", "{{ .Values.image.tag }}"},
		{"jinja-style expression", "{{ foo.bar }}"},
		{"mustache section", "{{#each items}}"},
		{"empty braces", "{{}}"},
		{"invalid identifier with space", "{{na me}}"},
		{"unterminated non-identifier", "{{ .Branch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.in, vars)
			if err != nil {
				t.Fatalf("Render(%q) error = %v, want it to pass through unchanged", tt.in, err)
			}
			if got != tt.in {
				t.Errorf("Render(%q) = %q, want unchanged", tt.in, got)
			}
		})
	}
}

func TestRender_ForeignSyntaxMixedWithRealTokens(t *testing.T) {
	vars := Vars{"name": "acme-site"}
	got, err := Render("# {{name|title}}\n\nBranch: {{.Branch}}\n", vars)
	if err != nil {
		t.Fatalf("Render error = %v", err)
	}
	want := "# Acme Site\n\nBranch: {{.Branch}}\n"
	if got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}
