package usecase

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/CodeOne45/snap/internal/core/capture"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// InitialVersion is the version every freshly captured template starts at.
// Capture never bumps an existing template's version — that's
// snap template bump's job (M14) — so a caller computing TargetDir for a
// first capture joins Store.Root(), AsName, and InitialVersion.
const InitialVersion = "0.1.0"

// Accept decides whether to turn a detected candidate into a template
// variable, and what to name it. defaultName is "name" for the candidate
// sourced from the project's own folder name (matching snap new's
// auto-seeding convention), and a snake_case join of its words otherwise.
type Accept func(c capture.Candidate, defaultName string) (accept bool, name string, err error)

// CaptureParams are the inputs to Capture.
type CaptureParams struct {
	FS        ports.FS
	SourceDir string
	TargetDir string // where the new template is written, e.g. Store.Root()+"/"+AsName
	AsName    string // template name written into snap.toml
	Options   capture.Options
	Accept    Accept
}

// CaptureResult is everything Capture produced: the Plan to write the new
// template, the raw scan (Scan.Ignored is what a CLI prints as "Ignored:
// ..."), and the candidates that were accepted.
type CaptureResult struct {
	Plan       plan.Plan
	Scan       capture.Result
	Candidates []capture.Candidate // accepted only
}

// Capture scans SourceDir, proposes candidates via Accept, replaces every
// accepted candidate's case-forms (longest string first, to avoid a shorter
// form corrupting a longer one it's a substring of) in every file's path
// and (non-binary) content, and builds the Plan to write the resulting
// template — its files plus a generated snap.toml — to TargetDir. It
// performs no writes and no I/O beyond reading SourceDir.
func Capture(p CaptureParams) (CaptureResult, error) {
	result, err := capture.Scan(p.FS, p.SourceDir, p.Options)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
	}

	candidates, err := capture.BuildCandidates(p.FS, p.SourceDir, result)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
	}

	var accepted []capture.Candidate
	var variables []template.Variable
	replacements := make(map[string]string)
	usedNames := make(map[string]bool)

	for _, c := range candidates {
		ok, name, err := p.Accept(c, DefaultVariableName(c))
		if err != nil {
			return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
		}
		if !ok {
			continue
		}
		if !isValidVariableName(name) {
			return CaptureResult{}, fmt.Errorf("usecase: capture: invalid variable name %q", name)
		}
		if usedNames[name] {
			return CaptureResult{}, fmt.Errorf("usecase: capture: variable name %q used by more than one candidate", name)
		}
		usedNames[name] = true

		accepted = append(accepted, c)
		variables = append(variables, template.Variable{
			Name:   name,
			Prompt: render.Apply(c.Words, render.CaseTitle),
			Type:   template.TypeString,
		})
		registerReplacements(replacements, c.Words, name)
	}

	literals := make([]string, 0, len(replacements))
	for lit := range replacements {
		literals = append(literals, lit)
	}
	sort.Slice(literals, func(i, j int) bool { return len(literals[i]) > len(literals[j]) })

	b := plan.NewBuilder(p.TargetDir)
	for _, d := range result.Dirs {
		if err := b.CreateDir(replaceAll(d, literals, replacements)); err != nil {
			return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
		}
	}
	for _, f := range result.Files {
		content, err := p.FS.ReadFile(path.Join(p.SourceDir, f.Path))
		if err != nil {
			return CaptureResult{}, fmt.Errorf("usecase: capture: read %q: %w", f.Path, err)
		}
		newPath := replaceAll(f.Path, literals, replacements)
		newContent := content
		if !f.IsBinary {
			newContent = []byte(replaceAll(string(content), literals, replacements))
		}
		if err := b.WriteFile(newPath, newContent, f.Mode); err != nil {
			return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
		}
	}

	manifest := template.Manifest{Name: p.AsName, Version: InitialVersion, Variables: variables}
	manifestData, err := manifest.TOML()
	if err != nil {
		return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
	}
	if err := b.WriteFile(template.ManifestFile, manifestData, 0o644); err != nil {
		return CaptureResult{}, fmt.Errorf("usecase: capture: %w", err)
	}

	return CaptureResult{Plan: b.Build(), Scan: result, Candidates: accepted}, nil
}

// DefaultVariableName returns "name" for the candidate sourced from the
// project's own folder name (matching snap new's auto-seeding of "name"
// from the target directory), and a snake_case join of its words otherwise.
func DefaultVariableName(c capture.Candidate) string {
	for _, s := range c.Sources {
		if s == "folder-name" {
			return "name"
		}
	}
	return strings.Join(c.Words, "_")
}

// registerReplacements maps every case-form of words to a placeholder
// token for name: the canonical bare-kebab form to "{{name}}", every other
// form to "{{name|<case>}}". A form that's already registered (by an
// earlier-accepted candidate) is left alone.
func registerReplacements(replacements map[string]string, words []string, name string) {
	for i, f := range capture.CandidateForms(words) {
		if _, exists := replacements[f.Value]; exists {
			continue
		}
		if i == 0 {
			replacements[f.Value] = fmt.Sprintf("{{%s}}", name)
		} else {
			replacements[f.Value] = fmt.Sprintf("{{%s|%s}}", name, f.Case)
		}
	}
}

// replaceAll substitutes every literal in order (expected longest-first) so
// a shorter form never corrupts a longer form it's a substring of.
func replaceAll(s string, literals []string, replacements map[string]string) string {
	for _, lit := range literals {
		s = strings.ReplaceAll(s, lit, replacements[lit])
	}
	return s
}

func isValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}
