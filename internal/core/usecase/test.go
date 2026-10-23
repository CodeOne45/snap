package usecase

import (
	"fmt"
	"path"
	"time"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/ports"
)

// TestParams are the inputs to Test.
type TestParams struct {
	Store ports.Store
	FS    ports.FS
	// Template is a bare name (latest version) or "name@version" (M14's
	// local pinning), same as New.
	Template string
	// TargetDir is a throwaway directory the caller creates (a real temp
	// dir for a real `snap test` run) to generate the sample project into.
	TargetDir string
}

// TestResult is the Plan to generate Template's sample project, plus the
// check command its [test] block declares to run against it.
type TestResult struct {
	Plan  plan.Plan
	Check string
}

// Test builds the Plan for Template's declared [test] block: generating
// it with the template author's own sample variable values, into
// TargetDir. It performs no writes and runs nothing — the caller applies
// the Plan (executor.Apply) and runs Check itself (ports.HookRunner, the
// same mechanism as post-generate hooks).
func Test(p TestParams) (TestResult, error) {
	root, err := resolveStoreRoot(p.Store, p.Template)
	if err != nil {
		return TestResult{}, fmt.Errorf("usecase: test: %w", err)
	}
	data, err := p.FS.ReadFile(path.Join(root, template.ManifestFile))
	if err != nil {
		return TestResult{}, fmt.Errorf("usecase: test: read manifest: %w", err)
	}
	manifest, err := template.ParseManifest(data)
	if err != nil {
		return TestResult{}, fmt.Errorf("usecase: test: %w", err)
	}
	if manifest.Test == nil {
		return TestResult{}, fmt.Errorf("usecase: test: template %q has no [test] block declared", p.Template)
	}

	result, err := New(NewParams{
		Store:       p.Store,
		FS:          p.FS,
		Template:    p.Template,
		TargetDir:   p.TargetDir,
		Vars:        manifest.Test.Vars,
		GeneratedAt: time.Now().UTC(),
	})
	if err != nil {
		return TestResult{}, fmt.Errorf("usecase: test: %w", err)
	}

	return TestResult{Plan: result, Check: manifest.Test.Check}, nil
}
