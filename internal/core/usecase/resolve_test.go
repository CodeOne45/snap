package usecase

import (
	"testing"

	"github.com/CodeOne45/snap/internal/core/template"
)

type fakePrompter struct {
	answers        map[string]string
	confirmAnswers map[string]bool
	confirmErr     error
}

func (f fakePrompter) Confirm(message string, defaultYes bool) (bool, error) {
	if f.confirmErr != nil {
		return false, f.confirmErr
	}
	if v, ok := f.confirmAnswers[message]; ok {
		return v, nil
	}
	return defaultYes, nil
}

func (f fakePrompter) Ask(message, defaultValue string) (string, error) {
	if v, ok := f.answers[message]; ok {
		return v, nil
	}
	return defaultValue, nil
}

func TestResolveVariables_FlagWins(t *testing.T) {
	vars := []template.Variable{{Name: "name", Type: template.TypeString}}
	got, err := ResolveVariables(vars, map[string]string{"name": "northwind"}, nil)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if got["name"] != "northwind" {
		t.Errorf("name = %q, want %q", got["name"], "northwind")
	}
}

func TestResolveVariables_PromptFillsGap(t *testing.T) {
	vars := []template.Variable{{Name: "name", Prompt: "Project name", Type: template.TypeString}}
	prompter := fakePrompter{answers: map[string]string{"Project name": "acme-site"}}

	got, err := ResolveVariables(vars, nil, prompter)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if got["name"] != "acme-site" {
		t.Errorf("name = %q, want %q", got["name"], "acme-site")
	}
}

func TestResolveVariables_DefaultUsed(t *testing.T) {
	vars := []template.Variable{{Name: "port", Type: template.TypeInt, Default: "8080", HasDefault: true}}
	got, err := ResolveVariables(vars, nil, nil)
	if err != nil {
		t.Fatalf("ResolveVariables: %v", err)
	}
	if got["port"] != "8080" {
		t.Errorf("port = %q, want %q", got["port"], "8080")
	}
}

func TestResolveVariables_MissingNonInteractive(t *testing.T) {
	vars := []template.Variable{{Name: "name", Type: template.TypeString}}
	if _, err := ResolveVariables(vars, nil, nil); err == nil {
		t.Fatal("ResolveVariables with no value/default/prompter: want error, got nil")
	}
}

func TestResolveVariables_ValidationFailure(t *testing.T) {
	vars := []template.Variable{{Name: "name", Type: template.TypeString, Pattern: "^[a-z-]+$"}}
	if _, err := ResolveVariables(vars, map[string]string{"name": "Not Valid"}, nil); err == nil {
		t.Fatal("ResolveVariables with value failing pattern: want error, got nil")
	}
}

func TestResolveVariables_IntValidationFailure(t *testing.T) {
	vars := []template.Variable{{Name: "port", Type: template.TypeInt}}
	if _, err := ResolveVariables(vars, map[string]string{"port": "not-a-number"}, nil); err == nil {
		t.Fatal("ResolveVariables with non-numeric int: want error, got nil")
	}
}

func TestResolveToggles_ExplicitWins(t *testing.T) {
	optional := []template.Optional{{Name: "docker", Default: true, Paths: []string{"Dockerfile"}}}
	got, err := ResolveToggles(optional, map[string]bool{"docker": false}, nil)
	if err != nil {
		t.Fatalf("ResolveToggles: %v", err)
	}
	if got["docker"] != false {
		t.Errorf("docker = %v, want false (explicit override)", got["docker"])
	}
}

func TestResolveToggles_PromptOverridesDefault(t *testing.T) {
	optional := []template.Optional{{Name: "docker", Prompt: "Include Docker?", Default: true, Paths: []string{"Dockerfile"}}}
	prompter := fakePrompter{confirmAnswers: map[string]bool{"Include Docker?": false}}

	got, err := ResolveToggles(optional, nil, prompter)
	if err != nil {
		t.Fatalf("ResolveToggles: %v", err)
	}
	if got["docker"] != false {
		t.Errorf("docker = %v, want false (prompt answer)", got["docker"])
	}
}

func TestResolveToggles_DefaultUsedNonInteractive(t *testing.T) {
	optional := []template.Optional{{Name: "docker", Default: true, Paths: []string{"Dockerfile"}}}
	got, err := ResolveToggles(optional, nil, nil)
	if err != nil {
		t.Fatalf("ResolveToggles: %v", err)
	}
	if got["docker"] != true {
		t.Errorf("docker = %v, want true (declared default)", got["docker"])
	}
}

func TestResolveToggles_UnknownNameIsError(t *testing.T) {
	optional := []template.Optional{{Name: "docker", Paths: []string{"Dockerfile"}}}
	if _, err := ResolveToggles(optional, map[string]bool{"typo-ci": true}, nil); err == nil {
		t.Fatal("ResolveToggles with an unknown block name: want error, got nil")
	}
}

func TestFilterOptionalPaths(t *testing.T) {
	optional := []template.Optional{
		{Name: "docker", Paths: []string{"Dockerfile", "docker-compose.yml"}},
		{Name: "ci", Paths: []string{".github"}},
	}
	paths := []string{
		"README.md",
		"Dockerfile",
		"docker-compose.yml",
		".github/workflows/ci.yml",
		"main.go",
	}

	t.Run("both on: nothing filtered", func(t *testing.T) {
		toggles := map[string]bool{"docker": true, "ci": true}
		got := filterOptionalPaths(paths, optional, toggles)
		if len(got) != len(paths) {
			t.Errorf("got %v, want all paths kept", got)
		}
	})

	t.Run("both off: optional paths dropped", func(t *testing.T) {
		toggles := map[string]bool{"docker": false, "ci": false}
		got := filterOptionalPaths(paths, optional, toggles)
		want := []string{"README.md", "main.go"}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("docker off only", func(t *testing.T) {
		toggles := map[string]bool{"docker": false, "ci": true}
		got := filterOptionalPaths(paths, optional, toggles)
		for _, p := range got {
			if p == "Dockerfile" || p == "docker-compose.yml" {
				t.Errorf("got %v, docker paths should be filtered", got)
			}
		}
		found := false
		for _, p := range got {
			if p == ".github/workflows/ci.yml" {
				found = true
			}
		}
		if !found {
			t.Errorf("got %v, want .github/workflows/ci.yml kept (ci is on)", got)
		}
	})
}
