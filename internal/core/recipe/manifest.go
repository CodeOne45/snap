// Package recipe defines a recipe's manifest (recipe.toml): the metadata
// for a reusable change applied to an existing project. Parsing is pure —
// no I/O; the caller reads the bytes via a ports.FS and a ports.Store.
package recipe

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/CodeOne45/snap/internal/core/semver"
)

// ManifestFile is the name of a recipe's manifest at its root.
const ManifestFile = "recipe.toml"

// FilesDir is the name of a recipe's directory of new files, rendered with
// the same placeholder engine as a template's files.
const FilesDir = "files"

// knownEcosystems are the dependency ecosystems a recipe may declare.
var knownEcosystems = map[string]bool{
	"npm":   true,
	"go":    true,
	"pip":   true,
	"cargo": true,
}

// AppliesTo gates whether a recipe applies to a given project: every listed
// file must already exist in the target.
type AppliesTo struct {
	Files []string `json:"files,omitempty"`
}

// Variable is one entry from a recipe's [[variables]] table. Unlike a
// template variable, a recipe variable has no literal default — only
// DefaultFrom, a dotted path (e.g. "project.name") resolved against the
// target project's own recorded state.
type Variable struct {
	Name        string `json:"name"`
	Prompt      string `json:"prompt,omitempty"`
	DefaultFrom string `json:"default_from,omitempty"`
}

// Dependency is one entry from a recipe's [[dependencies]] table: a package
// to add via the matching internal/core/deps editor for Ecosystem.
type Dependency struct {
	Ecosystem string `json:"ecosystem"`
	Name      string `json:"name"`
	Version   string `json:"version,omitempty"`
	Dev       bool   `json:"dev,omitempty"`
}

// EnvEntry is one entry from a recipe's [[env]] table: a key to add to an
// env file (e.g. .env.example) if not already present.
type EnvEntry struct {
	File  string `json:"file"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Recipe is a recipe's parsed recipe.toml.
type Recipe struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`

	AppliesTo    AppliesTo    `json:"applies_to,omitempty"`
	Variables    []Variable   `json:"variables,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
	Env          []EnvEntry   `json:"env,omitempty"`
	// Hooks are post-apply shell commands, in declared order. Never run
	// without the calling adapter's own confirmation (CLI) or allow_hooks
	// (MCP) gate — parsing them here does not run anything.
	Hooks []string `json:"hooks,omitempty"`
}

// manifestTOML mirrors the on-disk shape of recipe.toml.
type manifestTOML struct {
	Recipe struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description,omitempty"`
	} `toml:"recipe"`
	AppliesTo struct {
		Files []string `toml:"files,omitempty"`
	} `toml:"applies_to"`
	Variables    []variableTOML   `toml:"variables"`
	Dependencies []dependencyTOML `toml:"dependencies"`
	Env          []envTOML        `toml:"env"`
	Hooks        struct {
		Post []hookTOML `toml:"post"`
	} `toml:"hooks"`
}

type variableTOML struct {
	Name        string `toml:"name"`
	Prompt      string `toml:"prompt,omitempty"`
	DefaultFrom string `toml:"default_from,omitempty"`
}

type dependencyTOML struct {
	Ecosystem string `toml:"ecosystem"`
	Name      string `toml:"name"`
	Version   string `toml:"version,omitempty"`
	Dev       bool   `toml:"dev,omitempty"`
}

type envTOML struct {
	File  string `toml:"file"`
	Key   string `toml:"key"`
	Value string `toml:"value,omitempty"`
}

type hookTOML struct {
	Run string `toml:"run"`
}

// ParseManifest parses raw recipe.toml bytes, validating the fields a
// recipe needs to apply cleanly so a broken recipe fails at load time
// rather than partway through `snap add`.
func ParseManifest(data []byte) (Recipe, error) {
	var raw manifestTOML
	if err := toml.Unmarshal(data, &raw); err != nil {
		return Recipe{}, fmt.Errorf("recipe: parse %s: %w", ManifestFile, err)
	}

	if raw.Recipe.Name == "" {
		return Recipe{}, fmt.Errorf("recipe: parse %s: [recipe].name is required", ManifestFile)
	}
	if _, err := semver.Parse(raw.Recipe.Version); err != nil {
		return Recipe{}, fmt.Errorf("recipe: parse %s: [recipe].version: %w", ManifestFile, err)
	}

	r := Recipe{
		Name:        raw.Recipe.Name,
		Version:     raw.Recipe.Version,
		Description: raw.Recipe.Description,
		AppliesTo:   AppliesTo{Files: raw.AppliesTo.Files},
	}

	seenVar := make(map[string]bool, len(raw.Variables))
	for _, rv := range raw.Variables {
		if rv.Name == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: a variable is missing a name", ManifestFile)
		}
		if seenVar[rv.Name] {
			return Recipe{}, fmt.Errorf("recipe: parse %s: duplicate variable %q", ManifestFile, rv.Name)
		}
		seenVar[rv.Name] = true
		r.Variables = append(r.Variables, Variable(rv))
	}

	for _, rd := range raw.Dependencies {
		if rd.Ecosystem == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: a dependency is missing an ecosystem", ManifestFile)
		}
		if !knownEcosystems[rd.Ecosystem] {
			return Recipe{}, fmt.Errorf("recipe: parse %s: dependency %q: unknown ecosystem %q", ManifestFile, rd.Name, rd.Ecosystem)
		}
		if rd.Name == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: a dependency is missing a name", ManifestFile)
		}
		r.Dependencies = append(r.Dependencies, Dependency(rd))
	}

	for _, re := range raw.Env {
		if re.File == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: an env entry is missing a file", ManifestFile)
		}
		if re.Key == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: an env entry is missing a key", ManifestFile)
		}
		r.Env = append(r.Env, EnvEntry(re))
	}

	for _, rh := range raw.Hooks.Post {
		if strings.TrimSpace(rh.Run) == "" {
			return Recipe{}, fmt.Errorf("recipe: parse %s: a [[hooks.post]] entry has an empty run command", ManifestFile)
		}
		r.Hooks = append(r.Hooks, rh.Run)
	}

	return r, nil
}

// TOML marshals r back to recipe.toml format.
func (r Recipe) TOML() ([]byte, error) {
	var raw manifestTOML
	raw.Recipe.Name = r.Name
	raw.Recipe.Version = r.Version
	raw.Recipe.Description = r.Description
	raw.AppliesTo.Files = r.AppliesTo.Files

	for _, v := range r.Variables {
		raw.Variables = append(raw.Variables, variableTOML(v))
	}
	for _, d := range r.Dependencies {
		raw.Dependencies = append(raw.Dependencies, dependencyTOML(d))
	}
	for _, e := range r.Env {
		raw.Env = append(raw.Env, envTOML(e))
	}
	for _, run := range r.Hooks {
		raw.Hooks.Post = append(raw.Hooks.Post, hookTOML{Run: run})
	}

	data, err := toml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("recipe: marshal %s: %w", ManifestFile, err)
	}
	return data, nil
}
