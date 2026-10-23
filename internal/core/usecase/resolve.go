// Package usecase orchestrates the core use cases (New, List, Show) by
// composing template parsing, variable resolution, rendering, and plan
// building. It depends only on ports, never on adapters.
package usecase

import (
	"fmt"
	"strings"

	"github.com/CodeOne45/snap/internal/core/render"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// ResolveVariables determines the final value for every declared variable.
// Precedence: an explicit value in provided, then an interactive prompt (if
// prompter is non-nil), then the variable's default, else an error. Every
// resolved value is validated against its declared type/pattern.
func ResolveVariables(vars []template.Variable, provided map[string]string, prompter ports.Prompter) (render.Vars, error) {
	out := make(render.Vars, len(vars))
	for _, v := range vars {
		value, err := resolveOne(v, provided, prompter)
		if err != nil {
			return nil, err
		}
		if err := v.CheckValue(value); err != nil {
			return nil, err
		}
		out[v.Name] = value
	}
	return out, nil
}

func resolveOne(v template.Variable, provided map[string]string, prompter ports.Prompter) (string, error) {
	if value, ok := provided[v.Name]; ok {
		return value, nil
	}
	if prompter != nil {
		message := v.Prompt
		if message == "" {
			message = v.Name
		}
		return prompter.Ask(message, v.Default)
	}
	if v.HasDefault {
		return v.Default, nil
	}
	return "", fmt.Errorf("usecase: variable %q has no value, default, or prompter", v.Name)
}

// ResolveToggles determines the on/off state of every declared [[optional]]
// block. Precedence: an explicit value in provided (e.g. --with/--without),
// then an interactive confirm (if prompter is non-nil, using the block's
// declared Default as the suggested answer), then the declared Default.
// provided naming a block the template doesn't declare is an error — most
// likely a typo in --with/--without.
func ResolveToggles(optional []template.Optional, provided map[string]bool, prompter ports.Prompter) (map[string]bool, error) {
	known := make(map[string]bool, len(optional))
	for _, o := range optional {
		known[o.Name] = true
	}
	for name := range provided {
		if !known[name] {
			return nil, fmt.Errorf("usecase: unknown optional block %q", name)
		}
	}

	out := make(map[string]bool, len(optional))
	for _, o := range optional {
		if v, ok := provided[o.Name]; ok {
			out[o.Name] = v
			continue
		}
		if prompter != nil {
			message := o.Prompt
			if message == "" {
				message = o.Name
			}
			ok, err := prompter.Confirm(message, o.Default)
			if err != nil {
				return nil, fmt.Errorf("usecase: optional block %q: %w", o.Name, err)
			}
			out[o.Name] = ok
			continue
		}
		out[o.Name] = o.Default
	}
	return out, nil
}

// filterOptionalPaths drops any path that an OFF optional block controls.
// A declared block path matches a walked path exactly, or as a directory
// prefix (e.g. "docker" matches "docker/Dockerfile").
func filterOptionalPaths(paths []string, optional []template.Optional, toggles map[string]bool) []string {
	kept := make([]string, 0, len(paths))
outer:
	for _, p := range paths {
		for _, o := range optional {
			if toggles[o.Name] {
				continue
			}
			if matchesOptionalPath(p, o.Paths) {
				continue outer
			}
		}
		kept = append(kept, p)
	}
	return kept
}

func matchesOptionalPath(p string, prefixes []string) bool {
	for _, prefix := range prefixes {
		prefix = strings.TrimSuffix(prefix, "/")
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}
