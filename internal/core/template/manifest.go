// Package template defines a template's manifest (snap.toml): its
// metadata and the variables it declares. Parsing is pure — no I/O; the
// caller reads the bytes via a ports.FS and a ports.Store.
package template

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/CodeOne45/snap/internal/core/semver"
)

// ManifestFile is the name of a template's manifest at its root. It is
// metadata for snap itself and is never copied into a generated project.
const ManifestFile = "snap.toml"

// VarType is the declared type of a template variable.
type VarType string

// Supported variable types. Unset in snap.toml defaults to TypeString.
const (
	TypeString VarType = "string"
	TypeInt    VarType = "int"
)

// Variable is one entry from a template's [[variables]] table.
type Variable struct {
	Name       string  `json:"name"`
	Prompt     string  `json:"prompt,omitempty"`
	Type       VarType `json:"type"`
	Pattern    string  `json:"pattern,omitempty"` // from the "validate" TOML key; string type only
	Default    string  `json:"default,omitempty"`
	HasDefault bool    `json:"has_default"`
}

// CheckValue validates value against v's type and, for string variables,
// its Pattern.
func (v Variable) CheckValue(value string) error {
	switch v.Type {
	case TypeInt:
		if !isInt(value) {
			return fmt.Errorf("template: variable %q: %q is not an integer", v.Name, value)
		}
	case TypeString:
		if v.Pattern != "" {
			re, err := regexp.Compile(v.Pattern)
			if err != nil {
				return fmt.Errorf("template: variable %q: invalid pattern %q: %w", v.Name, v.Pattern, err)
			}
			if !re.MatchString(value) {
				return fmt.Errorf("template: variable %q: %q does not match pattern %q", v.Name, value, v.Pattern)
			}
		}
	}
	return nil
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == '-' && i == 0 && len(s) > 1 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Manifest is a template's parsed snap.toml.
type Manifest struct {
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Description string     `json:"description,omitempty"`
	Variables   []Variable `json:"variables,omitempty"`
	// Hooks are post-generate shell commands, in declared order. Never
	// run without the calling adapter's own confirmation (CLI) or
	// allow_hooks (MCP) gate — parsing them here does not run anything.
	Hooks []string `json:"hooks,omitempty"`
	// Optional blocks are toggleable groups of template paths (e.g. a
	// Docker setup). Resolving them does not touch anything — a usecase
	// decides on/off (declared Default, or an explicit --with/--without
	// or MCP toggle) and filters the walked file list accordingly.
	Optional []Optional `json:"optional,omitempty"`
	// Test declares how `snap test` should exercise this template: sample
	// variable values and a check command to run against the generated
	// output. nil means the template has opted out of snap test (an
	// immediate error, not a silent no-op).
	Test *TestConfig `json:"test,omitempty"`
}

// Optional is one [[optional]] block: toggleable paths in the template.
type Optional struct {
	Name    string   `json:"name"`
	Prompt  string   `json:"prompt,omitempty"`
	Default bool     `json:"default"`
	Paths   []string `json:"paths"`
}

// TestConfig is a template's [test] block (M17): Check is a shell command
// `snap test` runs (e.g. "go build ./... && go test ./...") against the
// template generated with Vars, in a throwaway temp directory.
type TestConfig struct {
	Check string            `json:"check"`
	Vars  map[string]string `json:"vars,omitempty"`
}

// manifestTOML mirrors the on-disk shape of snap.toml.
type manifestTOML struct {
	Template struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description,omitempty"`
	} `toml:"template"`
	Variables []variableTOML `toml:"variables"`
	Hooks     struct {
		Post []hookTOML `toml:"post"`
	} `toml:"hooks"`
	Optional []optionalTOML `toml:"optional"`
	Test     *testTOML      `toml:"test"`
}

type testTOML struct {
	Check string            `toml:"check"`
	Vars  map[string]string `toml:"vars,omitempty"`
}

type hookTOML struct {
	Run string `toml:"run"`
}

type optionalTOML struct {
	Name    string   `toml:"name"`
	Prompt  string   `toml:"prompt,omitempty"`
	Default bool     `toml:"default"`
	Paths   []string `toml:"paths"`
}

type variableTOML struct {
	Name     string `toml:"name"`
	Prompt   string `toml:"prompt,omitempty"`
	Type     string `toml:"type"`
	Validate string `toml:"validate,omitempty"`
	Default  any    `toml:"default,omitempty"`
}

// ParseManifest parses raw snap.toml bytes, validating variable names,
// types, and regex patterns so a broken template fails at load time rather
// than at first use.
func ParseManifest(data []byte) (Manifest, error) {
	var raw manifestTOML
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Manifest{}, fmt.Errorf("template: parse %s: %w", ManifestFile, err)
	}

	if raw.Template.Name == "" {
		return Manifest{}, fmt.Errorf("template: parse %s: [template].name is required", ManifestFile)
	}
	if _, err := semver.Parse(raw.Template.Version); err != nil {
		return Manifest{}, fmt.Errorf("template: parse %s: [template].version: %w", ManifestFile, err)
	}

	m := Manifest{
		Name:        raw.Template.Name,
		Version:     raw.Template.Version,
		Description: raw.Template.Description,
	}

	seen := make(map[string]bool, len(raw.Variables))
	for _, rv := range raw.Variables {
		if rv.Name == "" {
			return Manifest{}, fmt.Errorf("template: parse %s: a variable is missing a name", ManifestFile)
		}
		if seen[rv.Name] {
			return Manifest{}, fmt.Errorf("template: parse %s: duplicate variable %q", ManifestFile, rv.Name)
		}
		seen[rv.Name] = true

		vt := VarType(rv.Type)
		if vt == "" {
			vt = TypeString
		}
		if vt != TypeString && vt != TypeInt {
			return Manifest{}, fmt.Errorf("template: parse %s: variable %q: unknown type %q", ManifestFile, rv.Name, rv.Type)
		}

		if rv.Validate != "" {
			if _, err := regexp.Compile(rv.Validate); err != nil {
				return Manifest{}, fmt.Errorf("template: parse %s: variable %q: invalid pattern %q: %w", ManifestFile, rv.Name, rv.Validate, err)
			}
		}

		v := Variable{Name: rv.Name, Prompt: rv.Prompt, Type: vt, Pattern: rv.Validate}
		if rv.Default != nil {
			v.Default = fmt.Sprintf("%v", rv.Default)
			v.HasDefault = true
			if err := v.CheckValue(v.Default); err != nil {
				return Manifest{}, fmt.Errorf("template: parse %s: variable %q: default: %w", ManifestFile, rv.Name, err)
			}
		}
		m.Variables = append(m.Variables, v)
	}

	for _, rh := range raw.Hooks.Post {
		if strings.TrimSpace(rh.Run) == "" {
			return Manifest{}, fmt.Errorf("template: parse %s: a [[hooks.post]] entry has an empty run command", ManifestFile)
		}
		m.Hooks = append(m.Hooks, rh.Run)
	}

	seenOptional := make(map[string]bool, len(raw.Optional))
	for _, ro := range raw.Optional {
		if ro.Name == "" {
			return Manifest{}, fmt.Errorf("template: parse %s: an [[optional]] block is missing a name", ManifestFile)
		}
		if seenOptional[ro.Name] {
			return Manifest{}, fmt.Errorf("template: parse %s: duplicate optional block %q", ManifestFile, ro.Name)
		}
		seenOptional[ro.Name] = true
		if len(ro.Paths) == 0 {
			return Manifest{}, fmt.Errorf("template: parse %s: optional block %q has no paths", ManifestFile, ro.Name)
		}
		m.Optional = append(m.Optional, Optional(ro))
	}

	if raw.Test != nil {
		if strings.TrimSpace(raw.Test.Check) == "" {
			return Manifest{}, fmt.Errorf("template: parse %s: [test].check is required when a [test] block is present", ManifestFile)
		}
		m.Test = &TestConfig{Check: raw.Test.Check, Vars: raw.Test.Vars}
	}

	return m, nil
}

// TOML marshals m back to snap.toml format. A variable's Default is written
// typed (e.g. port = 8080, not port = "8080") so a round trip through
// ParseManifest(m.TOML()) reproduces m.
func (m Manifest) TOML() ([]byte, error) {
	var raw manifestTOML
	raw.Template.Name = m.Name
	raw.Template.Version = m.Version
	raw.Template.Description = m.Description

	for _, v := range m.Variables {
		rv := variableTOML{Name: v.Name, Prompt: v.Prompt, Type: string(v.Type), Validate: v.Pattern}
		if v.HasDefault {
			rv.Default = typedDefault(v)
		}
		raw.Variables = append(raw.Variables, rv)
	}
	for _, run := range m.Hooks {
		raw.Hooks.Post = append(raw.Hooks.Post, hookTOML{Run: run})
	}
	for _, o := range m.Optional {
		raw.Optional = append(raw.Optional, optionalTOML(o))
	}
	if m.Test != nil {
		raw.Test = &testTOML{Check: m.Test.Check, Vars: m.Test.Vars}
	}

	data, err := toml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("template: marshal %s: %w", ManifestFile, err)
	}
	return data, nil
}

func typedDefault(v Variable) any {
	if v.Type == TypeInt {
		if n, err := strconv.Atoi(v.Default); err == nil {
			return int64(n)
		}
	}
	return v.Default
}
