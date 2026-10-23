package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestShowCmd(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newShowCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{"api-starter 1.0.0", "author", "README.md", "src/{{name|pascal}}.go"} {
		if !strings.Contains(got, want) {
			t.Errorf("show output = %q, want it to contain %q", got, want)
		}
	}
}

func TestShowCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newShowCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{`"name": "api-starter"`, `"files"`, `"src/{{name|pascal}}.go"`} {
		if !strings.Contains(got, want) {
			t.Errorf("show --json output = %q, want it to contain %q", got, want)
		}
	}
}

// TestShowCmd_JSONSchemaStability pins show --json's exact byte shape for a
// canonical fixture, per M7's CLI JSON contract (docs/cli-json-contract.md).
func TestShowCmd_JSONSchemaStability(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newShowCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"api-starter", "--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := `{
  "manifest": {
    "name": "api-starter",
    "version": "1.0.0",
    "description": "test fixture",
    "variables": [
      {
        "name": "name",
        "type": "string",
        "pattern": "^[a-z][a-z0-9-]*$",
        "has_default": false
      },
      {
        "name": "author",
        "prompt": "Author name",
        "type": "string",
        "default": "Anonymous",
        "has_default": true
      }
    ]
  },
  "files": [
    "README.md",
    "src/{{name|pascal}}.go"
  ]
}
`
	if out.String() != want {
		t.Errorf("show --json shape changed — update docs/cli-json-contract.md if this is deliberate.\ngot:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestShowCmd_UnknownTemplate(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newShowCmd(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"nope"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute(nope): want error, got nil")
	}
}
