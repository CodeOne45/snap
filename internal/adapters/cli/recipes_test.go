package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRecipesCmd(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newRecipesCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "sentry") || !strings.Contains(got, "Error tracking with Sentry") {
		t.Errorf("recipes output = %q, want it to mention sentry and its description", got)
	}
}

func TestRecipesCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newRecipesCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{`"name": "sentry"`, `"version": "1.0.0"`} {
		if !strings.Contains(got, want) {
			t.Errorf("recipes --json output = %q, want it to contain %q", got, want)
		}
	}
}

func TestListCmd_RecipesFlag(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newListCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--recipes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "sentry") {
		t.Errorf("list --recipes output = %q, want it to mention sentry", got)
	}
	if strings.Contains(got, "api-starter") {
		t.Errorf("list --recipes output = %q, want it to list only recipes, not templates", got)
	}
}
