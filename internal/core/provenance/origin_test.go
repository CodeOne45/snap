package provenance

import (
	"testing"
	"time"
)

func TestOrigin_TOML_Golden(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.2.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		Variables: map[string]any{
			"name": "payments-service",
			"port": int64(8080),
		},
	}

	got, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	// Pinned byte-for-byte: this is the one place a timestamp is allowed in
	// rendered output, so its exact shape (unquoted native TOML datetime,
	// sorted variable keys) must not silently drift with library behavior.
	want := `template = "api-starter"
version = "1.2.0"
source = "local"
generated_at = 2026-10-06T12:00:00Z

[variables]
  name = "payments-service"
  port = 8080
`
	if string(got) != want {
		t.Errorf("TOML() =\n%s\nwant:\n%s", got, want)
	}
}

func TestOrigin_TOML_ZeroGeneratedAtIsError(t *testing.T) {
	o := Origin{Template: "api-starter", Version: "1.0.0", Source: "local"}
	if _, err := o.TOML(); err == nil {
		t.Fatal("TOML with zero GeneratedAt: want error, got nil")
	}
}

func TestOrigin_TOML_Deterministic(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.0.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Variables:   map[string]any{"z": "last", "a": "first", "m": "middle"},
	}
	first, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := o.TOML()
		if err != nil {
			t.Fatalf("TOML: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("TOML() is not deterministic across calls")
		}
	}
}

func TestParseOrigin_RoundTrip(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.2.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		Variables: map[string]any{
			"name": "payments-service",
			"port": int64(8080),
		},
		Toggles: map[string]bool{"docker": true, "ci": false},
	}
	data, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	got, err := ParseOrigin(data)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if got.Template != o.Template || got.Version != o.Version || got.Source != o.Source {
		t.Errorf("ParseOrigin() = %+v, want Template/Version/Source matching %+v", got, o)
	}
	if !got.GeneratedAt.Equal(o.GeneratedAt) {
		t.Errorf("GeneratedAt = %v, want %v", got.GeneratedAt, o.GeneratedAt)
	}
	if got.Variables["name"] != "payments-service" {
		t.Errorf("Variables[name] = %v, want %q", got.Variables["name"], "payments-service")
	}
	if port, ok := got.Variables["port"].(int64); !ok || port != 8080 {
		t.Errorf("Variables[port] = %v (%T), want int64(8080)", got.Variables["port"], got.Variables["port"])
	}
	if !got.Toggles["docker"] || got.Toggles["ci"] {
		t.Errorf("Toggles = %v, want docker=true, ci=false", got.Toggles)
	}
}

func TestParseOrigin_MalformedTOML(t *testing.T) {
	if _, err := ParseOrigin([]byte("not valid toml {{{")); err == nil {
		t.Fatal("ParseOrigin(malformed toml): want error, got nil")
	}
}

func TestParseOrigin_MissingTemplate(t *testing.T) {
	data := []byte(`version = "1.0.0"
source = "local"
generated_at = 2026-01-01T00:00:00Z
`)
	if _, err := ParseOrigin(data); err == nil {
		t.Fatal("ParseOrigin with no template: want error, got nil")
	}
}

func TestParseOrigin_NoToggles(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.0.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Variables:   map[string]any{"name": "x"},
	}
	data, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	got, err := ParseOrigin(data)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if len(got.Toggles) != 0 {
		t.Errorf("Toggles = %v, want empty (none were recorded)", got.Toggles)
	}
}

func TestOrigin_TOML_TogglesGolden(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.2.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		Variables:   map[string]any{"name": "payments-service"},
		Toggles:     map[string]bool{"docker": true, "ci": false},
	}

	got, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	want := `template = "api-starter"
version = "1.2.0"
source = "local"
generated_at = 2026-10-06T12:00:00Z

[variables]
  name = "payments-service"

[toggles]
  ci = false
  docker = true
`
	if string(got) != want {
		t.Errorf("TOML() =\n%s\nwant:\n%s", got, want)
	}
}

func TestParseOrigin_WithRecipe_RoundTrip(t *testing.T) {
	o := Origin{
		Template:    "api-starter",
		Version:     "1.2.0",
		Source:      "local",
		GeneratedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		Variables:   map[string]any{"name": "payments-service"},
		Recipes: []RecipeEntry{
			{
				Name:      "sentry",
				Version:   "1.0.0",
				Source:    "local",
				AppliedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
				Variables: map[string]any{"service": "payments"},
			},
		},
	}
	data, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	got, err := ParseOrigin(data)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if len(got.Recipes) != 1 {
		t.Fatalf("len(Recipes) = %d, want 1", len(got.Recipes))
	}
	r := got.Recipes[0]
	if r.Name != "sentry" || r.Version != "1.0.0" || r.Source != "local" {
		t.Errorf("Recipes[0] = %+v, want Name=sentry Version=1.0.0 Source=local", r)
	}
	if !r.AppliedAt.Equal(o.Recipes[0].AppliedAt) {
		t.Errorf("Recipes[0].AppliedAt = %v, want %v", r.AppliedAt, o.Recipes[0].AppliedAt)
	}
	if r.Variables["service"] != "payments" {
		t.Errorf("Recipes[0].Variables[service] = %v, want %q", r.Variables["service"], "payments")
	}
}

func TestParseOrigin_RecipesOnly_RoundTrip(t *testing.T) {
	// A project not created by Snap has no [template]/[version]/etc at
	// all — only recipes applied to it after the fact.
	o := Origin{
		Recipes: []RecipeEntry{
			{
				Name:      "sentry",
				Version:   "1.0.0",
				Source:    "local",
				AppliedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	data, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	got, err := ParseOrigin(data)
	if err != nil {
		t.Fatalf("ParseOrigin: %v", err)
	}
	if got.Template != "" {
		t.Errorf("Template = %q, want empty", got.Template)
	}
	if len(got.Recipes) != 1 || got.Recipes[0].Name != "sentry" {
		t.Errorf("Recipes = %+v, want one entry named sentry", got.Recipes)
	}
}

func TestParseOrigin_NoTemplateNoRecipesIsError(t *testing.T) {
	if _, err := ParseOrigin([]byte("")); err == nil {
		t.Fatal("ParseOrigin of an empty file: want error, got nil")
	}
}

func TestParseOrigin_RecipeMissingName(t *testing.T) {
	data := []byte(`[[recipes]]
version = "1.0.0"
source = "local"
applied_at = 2026-11-02T10:00:00Z
`)
	if _, err := ParseOrigin(data); err == nil {
		t.Fatal("ParseOrigin with a recipe missing a name: want error, got nil")
	}
}

func TestParseOrigin_RecipeMissingVersion(t *testing.T) {
	data := []byte(`[[recipes]]
name = "sentry"
source = "local"
applied_at = 2026-11-02T10:00:00Z
`)
	if _, err := ParseOrigin(data); err == nil {
		t.Fatal("ParseOrigin with a recipe missing a version: want error, got nil")
	}
}

func TestParseOrigin_RecipeMissingAppliedAt(t *testing.T) {
	data := []byte(`[[recipes]]
name = "sentry"
version = "1.0.0"
source = "local"
`)
	if _, err := ParseOrigin(data); err == nil {
		t.Fatal("ParseOrigin with a recipe missing applied_at: want error, got nil")
	}
}

func TestOrigin_TOML_RecipesOnly_Golden(t *testing.T) {
	o := Origin{
		Recipes: []RecipeEntry{
			{
				Name:      "sentry",
				Version:   "1.0.0",
				Source:    "local",
				AppliedAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
				Variables: map[string]any{"service": "payments"},
			},
		},
	}
	got, err := o.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	// Pinned byte-for-byte: a project not generated by Snap must get an
	// origin.toml with no [template]/version/etc noise at all — just the
	// recipes actually applied to it.
	want := `[[recipes]]
  name = "sentry"
  version = "1.0.0"
  source = "local"
  applied_at = 2026-11-02T10:00:00Z
  [recipes.variables]
    service = "payments"
`
	if string(got) != want {
		t.Errorf("TOML() =\n%s\nwant:\n%s", got, want)
	}
}

func TestOrigin_TOML_RecipeMissingAppliedAtIsError(t *testing.T) {
	o := Origin{Recipes: []RecipeEntry{{Name: "sentry", Version: "1.0.0", Source: "local"}}}
	if _, err := o.TOML(); err == nil {
		t.Fatal("TOML with a recipe missing AppliedAt: want error, got nil")
	}
}
