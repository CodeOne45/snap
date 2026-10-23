package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestListCmd(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newListCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "api-starter") || !strings.Contains(got, "test fixture") {
		t.Errorf("list output = %q, want it to mention api-starter and its description", got)
	}
}

func TestListCmd_JSON(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newListCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	for _, want := range []string{`"name": "api-starter"`, `"description": "test fixture"`} {
		if !strings.Contains(got, want) {
			t.Errorf("list --json output = %q, want it to contain %q", got, want)
		}
	}
}

// TestListCmd_JSONSchemaStability pins list --json's exact byte shape for a
// canonical fixture, per M7's CLI JSON contract (docs/cli-json-contract.md).
func TestListCmd_JSONSchemaStability(t *testing.T) {
	deps := newTestDeps(t)
	cmd := newListCmd(deps)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	want := `[
  {
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
  }
]
`
	if out.String() != want {
		t.Errorf("list --json shape changed — update docs/cli-json-contract.md if this is deliberate.\ngot:\n%s\nwant:\n%s", out.String(), want)
	}
}
