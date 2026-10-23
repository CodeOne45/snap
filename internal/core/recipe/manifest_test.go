package recipe

import "testing"

const validRecipeTOML = `[recipe]
name = "sentry"
version = "1.0.0"
description = "Error tracking with Sentry"

[applies_to]
files = ["package.json"]

[[variables]]
name = "service"
prompt = "Service name"
default_from = "project.name"

[[dependencies]]
ecosystem = "npm"
name = "@sentry/node"
version = "^8.0.0"
dev = false

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""

[[hooks.post]]
run = "npm install"
`

func TestParseManifest_Valid(t *testing.T) {
	r, err := ParseManifest([]byte(validRecipeTOML))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	if r.Name != "sentry" {
		t.Errorf("Name = %q, want %q", r.Name, "sentry")
	}
	if r.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", r.Version, "1.0.0")
	}
	if r.Description != "Error tracking with Sentry" {
		t.Errorf("Description = %q, want %q", r.Description, "Error tracking with Sentry")
	}

	if want := []string{"package.json"}; len(r.AppliesTo.Files) != 1 || r.AppliesTo.Files[0] != want[0] {
		t.Errorf("AppliesTo.Files = %v, want %v", r.AppliesTo.Files, want)
	}

	if len(r.Variables) != 1 {
		t.Fatalf("len(Variables) = %d, want 1", len(r.Variables))
	}
	v := r.Variables[0]
	if v.Name != "service" || v.Prompt != "Service name" || v.DefaultFrom != "project.name" {
		t.Errorf("Variables[0] = %+v, want {service, Service name, project.name}", v)
	}

	if len(r.Dependencies) != 1 {
		t.Fatalf("len(Dependencies) = %d, want 1", len(r.Dependencies))
	}
	d := r.Dependencies[0]
	if d.Ecosystem != "npm" || d.Name != "@sentry/node" || d.Version != "^8.0.0" || d.Dev {
		t.Errorf("Dependencies[0] = %+v, want {npm, @sentry/node, ^8.0.0, false}", d)
	}

	if len(r.Env) != 1 {
		t.Fatalf("len(Env) = %d, want 1", len(r.Env))
	}
	e := r.Env[0]
	if e.File != ".env.example" || e.Key != "SENTRY_DSN" || e.Value != "" {
		t.Errorf("Env[0] = %+v, want {.env.example, SENTRY_DSN, \"\"}", e)
	}

	if len(r.Hooks) != 1 || r.Hooks[0] != "npm install" {
		t.Errorf("Hooks = %v, want [npm install]", r.Hooks)
	}
}

func TestParseManifest_MinimalValid(t *testing.T) {
	data := `[recipe]
name = "docker"
version = "0.1.0"
`
	r, err := ParseManifest([]byte(data))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if r.Name != "docker" || r.Version != "0.1.0" {
		t.Errorf("r = %+v, want {Name: docker, Version: 0.1.0}", r)
	}
	if len(r.AppliesTo.Files) != 0 {
		t.Errorf("AppliesTo.Files = %v, want empty", r.AppliesTo.Files)
	}
}

func TestParseManifest_Errors(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{
			name: "missing name",
			toml: `[recipe]
version = "1.0.0"
`,
		},
		{
			name: "missing version",
			toml: `[recipe]
name = "sentry"
`,
		},
		{
			name: "invalid version",
			toml: `[recipe]
name = "sentry"
version = "not-semver"
`,
		},
		{
			name: "variable missing name",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[variables]]
prompt = "Service name"
`,
		},
		{
			name: "duplicate variable",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[variables]]
name = "service"

[[variables]]
name = "service"
`,
		},
		{
			name: "dependency missing ecosystem",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[dependencies]]
name = "@sentry/node"
version = "^8.0.0"
`,
		},
		{
			name: "dependency unknown ecosystem",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[dependencies]]
ecosystem = "yarn"
name = "@sentry/node"
`,
		},
		{
			name: "dependency missing name",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[dependencies]]
ecosystem = "npm"
version = "^8.0.0"
`,
		},
		{
			name: "env missing file",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[env]]
key = "SENTRY_DSN"
`,
		},
		{
			name: "env missing key",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[env]]
file = ".env.example"
`,
		},
		{
			name: "hook empty run",
			toml: `[recipe]
name = "sentry"
version = "1.0.0"

[[hooks.post]]
run = "   "
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseManifest([]byte(tt.toml)); err == nil {
				t.Fatalf("ParseManifest(%s): want error, got nil", tt.name)
			}
		})
	}
}

func TestParseManifest_InvalidTOMLSyntax(t *testing.T) {
	if _, err := ParseManifest([]byte("not valid toml {{{")); err == nil {
		t.Fatal("ParseManifest with invalid TOML syntax: want error, got nil")
	}
}

func TestTOML_RoundTrip(t *testing.T) {
	r := Recipe{
		Name:        "sentry",
		Version:     "1.0.0",
		Description: "Error tracking with Sentry",
		AppliesTo:   AppliesTo{Files: []string{"package.json"}},
		Variables: []Variable{
			{Name: "service", Prompt: "Service name", DefaultFrom: "project.name"},
		},
		Dependencies: []Dependency{
			{Ecosystem: "npm", Name: "@sentry/node", Version: "^8.0.0", Dev: false},
		},
		Env: []EnvEntry{
			{File: ".env.example", Key: "SENTRY_DSN", Value: ""},
		},
		Hooks: []string{"npm install"},
	}

	data, err := r.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}

	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(r.TOML()): %v", err)
	}

	if got.Name != r.Name || got.Version != r.Version || got.Description != r.Description {
		t.Errorf("round trip metadata = %+v, want %+v", got, r)
	}
	if len(got.AppliesTo.Files) != 1 || got.AppliesTo.Files[0] != "package.json" {
		t.Errorf("round trip AppliesTo = %+v, want %+v", got.AppliesTo, r.AppliesTo)
	}
	if len(got.Variables) != 1 || got.Variables[0] != r.Variables[0] {
		t.Errorf("round trip Variables = %+v, want %+v", got.Variables, r.Variables)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != r.Dependencies[0] {
		t.Errorf("round trip Dependencies = %+v, want %+v", got.Dependencies, r.Dependencies)
	}
	if len(got.Env) != 1 || got.Env[0] != r.Env[0] {
		t.Errorf("round trip Env = %+v, want %+v", got.Env, r.Env)
	}
	if len(got.Hooks) != 1 || got.Hooks[0] != r.Hooks[0] {
		t.Errorf("round trip Hooks = %v, want %v", got.Hooks, r.Hooks)
	}
}

func TestTOML_RoundTrip_Minimal(t *testing.T) {
	r := Recipe{Name: "docker", Version: "0.1.0"}

	data, err := r.TOML()
	if err != nil {
		t.Fatalf("TOML: %v", err)
	}
	got, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("ParseManifest(r.TOML()): %v", err)
	}
	if got.Name != r.Name || got.Version != r.Version {
		t.Errorf("round trip = %+v, want %+v", got, r)
	}
	if len(got.AppliesTo.Files) != 0 || len(got.Variables) != 0 || len(got.Dependencies) != 0 ||
		len(got.Env) != 0 || len(got.Hooks) != 0 {
		t.Errorf("round trip of a minimal recipe produced non-empty slices: %+v", got)
	}
}

func TestKnownEcosystems(t *testing.T) {
	for _, eco := range []string{"npm", "go", "pip", "cargo"} {
		toml := `[recipe]
name = "x"
version = "1.0.0"

[[dependencies]]
ecosystem = "` + eco + `"
name = "foo"
version = "1.0.0"
`
		if _, err := ParseManifest([]byte(toml)); err != nil {
			t.Errorf("ParseManifest with ecosystem %q: %v", eco, err)
		}
	}
}
