package template

import (
	"reflect"
	"strings"
	"testing"
)

const validManifest = `
[template]
name = "api-starter"
version = "1.2.0"
description = "Go REST API"

[[variables]]
name = "name"
prompt = "Project name"
type = "string"
validate = "^[a-z][a-z0-9-]+$"

[[variables]]
name = "port"
type = "int"
default = 8080
`

func TestParseManifest_Valid(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Name != "api-starter" || m.Version != "1.2.0" {
		t.Errorf("Manifest = %+v, want name=api-starter version=1.2.0", m)
	}
	if len(m.Variables) != 2 {
		t.Fatalf("len(Variables) = %d, want 2", len(m.Variables))
	}

	name := m.Variables[0]
	if name.Name != "name" || name.Type != TypeString || name.Pattern == "" || name.HasDefault {
		t.Errorf("Variables[0] = %+v, want name/string/pattern set/no default", name)
	}

	port := m.Variables[1]
	if port.Name != "port" || port.Type != TypeInt || !port.HasDefault || port.Default != "8080" {
		t.Errorf("Variables[1] = %+v, want port/int/default 8080", port)
	}
}

func TestManifest_TOML_RoundTrip(t *testing.T) {
	want, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	data, err := want.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(round-tripped): %v\n%s", err, data)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip mismatch:\ngot:  %+v\nwant: %+v\nTOML:\n%s", got, want, data)
	}
}

func TestManifest_TOML_TypedDefault(t *testing.T) {
	m := Manifest{
		Name: "x",
		Variables: []Variable{
			{Name: "port", Type: TypeInt, Default: "8080", HasDefault: true},
		},
	}
	data, err := m.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	if !strings.Contains(string(data), "default = 8080") {
		t.Errorf("TOML() = %s, want unquoted integer default", data)
	}
}

func TestParseManifest_Test(t *testing.T) {
	m, err := ParseManifest([]byte(`
[template]
name = "api-starter"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[test]
check = "go build ./..."

[test.vars]
name = "sample-project"
`))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Test == nil {
		t.Fatal("Test = nil, want a TestConfig")
	}
	if m.Test.Check != "go build ./..." {
		t.Errorf("Test.Check = %q, want %q", m.Test.Check, "go build ./...")
	}
	if m.Test.Vars["name"] != "sample-project" {
		t.Errorf("Test.Vars[name] = %q, want %q", m.Test.Vars["name"], "sample-project")
	}
}

func TestParseManifest_NoTestBlock(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Test != nil {
		t.Errorf("Test = %+v, want nil (no [test] block declared)", m.Test)
	}
}

func TestParseManifest_EmptyTestCheckIsError(t *testing.T) {
	_, err := ParseManifest([]byte(`
[template]
name = "api-starter"
version = "1.0.0"

[test]
check = ""
`))
	if err == nil {
		t.Fatal("ParseManifest with an empty [test].check: want error, got nil")
	}
}

func TestManifest_TOML_TestRoundTrip(t *testing.T) {
	m := Manifest{
		Name:    "api-starter",
		Version: "1.0.0",
		Test: &TestConfig{
			Check: "go build ./...",
			Vars:  map[string]string{"name": "sample-project", "port": "8080"},
		},
	}
	data, err := m.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(round trip): %v\n%s", err, data)
	}
	if got.Test == nil {
		t.Fatal("round-tripped Test = nil")
	}
	if got.Test.Check != m.Test.Check {
		t.Errorf("Test.Check = %q, want %q", got.Test.Check, m.Test.Check)
	}
	if got.Test.Vars["name"] != "sample-project" || got.Test.Vars["port"] != "8080" {
		t.Errorf("Test.Vars = %v, want %v", got.Test.Vars, m.Test.Vars)
	}
}

func TestParseManifest_Errors(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{"missing template name", `[template]
version = "1.0.0"`},
		{"missing variable name", `[template]
name = "x"
[[variables]]
type = "string"`},
		{"duplicate variable", `[template]
name = "x"
[[variables]]
name = "port"
type = "int"
[[variables]]
name = "port"
type = "string"`},
		{"unknown type", `[template]
name = "x"
[[variables]]
name = "port"
type = "float"`},
		{"invalid regex", `[template]
name = "x"
[[variables]]
name = "slug"
type = "string"
validate = "("`},
		{"invalid default for type", `[template]
name = "x"
[[variables]]
name = "port"
type = "int"
default = "not-a-number"`},
		{"malformed toml", `not valid toml {{{`},
		{"missing version", `[template]
name = "x"`},
		{"non-semver version", `[template]
name = "x"
version = "1.0"`},
		{"version with v prefix", `[template]
name = "x"
version = "v1.0.0"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(tt.toml)); err == nil {
				t.Errorf("ParseManifest(%q): want error, got nil", tt.name)
			}
		})
	}
}

func TestVariable_CheckValue(t *testing.T) {
	strVar := Variable{Name: "name", Type: TypeString, Pattern: "^[a-z-]+$"}
	if err := strVar.CheckValue("acme-site"); err != nil {
		t.Errorf("CheckValue(acme-site) = %v, want nil", err)
	}
	if err := strVar.CheckValue("Acme Site"); err == nil {
		t.Error("CheckValue(Acme Site): want error, got nil")
	}

	intVar := Variable{Name: "port", Type: TypeInt}
	if err := intVar.CheckValue("8080"); err != nil {
		t.Errorf("CheckValue(8080) = %v, want nil", err)
	}
	if err := intVar.CheckValue("-1"); err != nil {
		t.Errorf("CheckValue(-1) = %v, want nil", err)
	}
	if err := intVar.CheckValue("abc"); err == nil {
		t.Error("CheckValue(abc): want error, got nil")
	}
}

func TestParseManifest_Hooks(t *testing.T) {
	data := []byte(`
[template]
name = "api-starter"
version = "1.0.0"

[[hooks.post]]
run = "git init"

[[hooks.post]]
run = "go mod tidy"
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	want := []string{"git init", "go mod tidy"}
	if !reflect.DeepEqual(m.Hooks, want) {
		t.Errorf("Hooks = %v, want %v", m.Hooks, want)
	}
}

func TestParseManifest_EmptyHookCommandIsError(t *testing.T) {
	data := []byte(`
[template]
name = "x"

[[hooks.post]]
run = ""
`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatal("ParseManifest with an empty hook command: want error, got nil")
	}
}

func TestManifest_TOML_HooksRoundTrip(t *testing.T) {
	want := Manifest{
		Name:    "api-starter",
		Version: "1.0.0",
		Hooks:   []string{"git init", "go mod tidy"},
	}
	data, err := want.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(round-tripped): %v\n%s", err, data)
	}
	if !reflect.DeepEqual(got.Hooks, want.Hooks) {
		t.Errorf("Hooks = %v, want %v\nTOML:\n%s", got.Hooks, want.Hooks, data)
	}
}

func TestParseManifest_Optional(t *testing.T) {
	data := []byte(`
[template]
name = "api-starter"
version = "1.0.0"

[[optional]]
name = "docker"
prompt = "Include Docker?"
default = true
paths = ["Dockerfile", "docker-compose.yml"]
`)
	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(m.Optional) != 1 {
		t.Fatalf("len(Optional) = %d, want 1", len(m.Optional))
	}
	o := m.Optional[0]
	if o.Name != "docker" || o.Prompt != "Include Docker?" || !o.Default {
		t.Errorf("Optional[0] = %+v, want name=docker prompt set default=true", o)
	}
	wantPaths := []string{"Dockerfile", "docker-compose.yml"}
	if !reflect.DeepEqual(o.Paths, wantPaths) {
		t.Errorf("Paths = %v, want %v", o.Paths, wantPaths)
	}
}

func TestParseManifest_OptionalErrors(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{"missing name", `[template]
name = "x"
[[optional]]
paths = ["a"]`},
		{"missing paths", `[template]
name = "x"
[[optional]]
name = "docker"`},
		{"duplicate name", `[template]
name = "x"
[[optional]]
name = "docker"
paths = ["a"]
[[optional]]
name = "docker"
paths = ["b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(tt.toml)); err == nil {
				t.Errorf("ParseManifest(%q): want error, got nil", tt.name)
			}
		})
	}
}

func TestManifest_TOML_OptionalRoundTrip(t *testing.T) {
	want := Manifest{
		Name:    "api-starter",
		Version: "1.0.0",
		Optional: []Optional{
			{Name: "docker", Prompt: "Include Docker?", Default: true, Paths: []string{"Dockerfile", "docker-compose.yml"}},
			{Name: "ci", Default: false, Paths: []string{".github/"}},
		},
	}
	data, err := want.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(round-tripped): %v\n%s", err, data)
	}
	if !reflect.DeepEqual(got.Optional, want.Optional) {
		t.Errorf("Optional = %+v, want %+v\nTOML:\n%s", got.Optional, want.Optional, data)
	}
}
