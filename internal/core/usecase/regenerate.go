package usecase

import (
	"fmt"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/provenance"
	"github.com/CodeOne45/snap/internal/ports"
)

// RegenerateParams are the inputs to RegenerateBase.
type RegenerateParams struct {
	Store ports.Store
	FS    ports.FS
	// Origin is a generated project's parsed .snap/origin.toml
	// (provenance.ParseOrigin), naming exactly which template, version,
	// variables and toggles produced it.
	Origin provenance.Origin
	// TargetDir roots the resulting Plan. It's never applied to disk by
	// RegenerateBase itself — the base is reconstructed "in memory" for a
	// caller (M16's three-way merge) to diff against, so this is just a
	// label, not read back from anywhere.
	TargetDir string
}

// RegenerateBase reconstructs, in memory, exactly the Plan that originally
// produced a project — from nothing but its own recorded provenance. It
// pins the exact original version (Origin.Template + "@" + Origin.Version,
// M14's local pinning syntax) so a template that has since been bumped
// forward still regenerates the old base unchanged.
//
// For a remote-sourced origin, Store must already be scoped to that exact
// resolved commit (a gitcache.SingleTemplateStore, which ignores the
// template/version string entirely) — the same way New's callers resolve a
// remote reference before calling it; RegenerateBase itself does no
// network or git I/O.
func RegenerateBase(p RegenerateParams) (plan.Plan, error) {
	vars := make(map[string]string, len(p.Origin.Variables))
	for k, v := range p.Origin.Variables {
		vars[k] = fmt.Sprintf("%v", v)
	}

	template := p.Origin.Template + "@" + p.Origin.Version
	result, err := New(NewParams{
		Store:       p.Store,
		FS:          p.FS,
		Template:    template,
		TargetDir:   p.TargetDir,
		Vars:        vars,
		Toggles:     p.Origin.Toggles,
		GeneratedAt: p.Origin.GeneratedAt,
		Source:      p.Origin.Source,
	})
	if err != nil {
		return plan.Plan{}, fmt.Errorf("usecase: regenerate base: %w", err)
	}
	return result, nil
}
