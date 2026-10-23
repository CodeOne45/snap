// Package plan defines the immutable Plan that every command builds before
// touching anything: a Plan is a list of Ops, dry-run prints it, --json
// serializes it, and internal/executor is the only thing that applies it.
package plan

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// OpKind discriminates the concrete Op types for serialization and
// switches. The set grows additively — RunHookOp (M9) added a new OpKind
// and concrete type without touching CreateDirOp/WriteFileOp or their JSON.
type OpKind string

// Op kinds.
const (
	OpCreateDir  OpKind = "create_dir"
	OpWriteFile  OpKind = "write_file"
	OpRunHook    OpKind = "run_hook"
	OpModifyFile OpKind = "modify_file"
)

// Op is one step of a Plan. Concrete types are CreateDirOp, WriteFileOp,
// RunHookOp, and ModifyFileOp.
type Op interface {
	Kind() OpKind
}

// CreateDirOp creates an empty directory at Path (relative to the Plan's
// TargetDir).
type CreateDirOp struct {
	Path string
}

// Kind implements Op.
func (CreateDirOp) Kind() OpKind { return OpCreateDir }

// WriteFileOp writes Content to Path (relative to the Plan's TargetDir).
type WriteFileOp struct {
	Path    string
	Content []byte
	Mode    fs.FileMode
}

// Kind implements Op.
func (WriteFileOp) Kind() OpKind { return OpWriteFile }

// RunHookOp runs a shell command in the Plan's TargetDir. Hooks always run
// last, after every file has been written and the target directory
// committed (see internal/executor) — never silently, and never without
// the calling adapter's own confirmation/allow_hooks gate.
type RunHookOp struct {
	Run string
}

// Kind implements Op.
func (RunHookOp) Kind() OpKind { return OpRunHook }

// ModifyFileOp replaces an existing file's content with Content (not a
// new file — see WriteFileOp for that). ExpectedSHA256 is the hex-encoded
// SHA-256 of the file's content at planning time; the executor refuses to
// apply if the file's actual current content doesn't match it, so a Plan
// built from a stale snapshot never silently clobbers a newer edit.
type ModifyFileOp struct {
	Path           string
	ExpectedSHA256 string
	Content        []byte
	Mode           fs.FileMode
}

// Kind implements Op.
func (ModifyFileOp) Kind() OpKind { return OpModifyFile }

// Plan is an ordered, immutable list of Ops to apply under TargetDir.
type Plan struct {
	TargetDir string
	Ops       []Op
}

// Builder accumulates Ops for a Plan, validating paths as they're added.
type Builder struct {
	plan Plan
	seen map[string]bool
}

// NewBuilder starts a Builder for a plan targeting targetDir.
func NewBuilder(targetDir string) *Builder {
	return &Builder{plan: Plan{TargetDir: targetDir}, seen: make(map[string]bool)}
}

// CreateDir adds a CreateDirOp for p, which must be a clean, relative path
// that doesn't escape the target directory and hasn't already been used by
// another op in this Plan.
func (b *Builder) CreateDir(p string) error {
	cp, err := b.claim(p)
	if err != nil {
		return err
	}
	b.plan.Ops = append(b.plan.Ops, CreateDirOp{Path: cp})
	return nil
}

// WriteFile adds a WriteFileOp for p, which must be a clean, relative path
// that doesn't escape the target directory and hasn't already been used by
// another op in this Plan.
func (b *Builder) WriteFile(p string, content []byte, mode fs.FileMode) error {
	cp, err := b.claim(p)
	if err != nil {
		return err
	}
	b.plan.Ops = append(b.plan.Ops, WriteFileOp{Path: cp, Content: content, Mode: mode})
	return nil
}

// ModifyFile adds a ModifyFileOp for p, which must be a clean, relative
// path that doesn't escape the target directory and hasn't already been
// used by another op in this Plan.
func (b *Builder) ModifyFile(p string, expectedSHA256 string, content []byte, mode fs.FileMode) error {
	cp, err := b.claim(p)
	if err != nil {
		return err
	}
	b.plan.Ops = append(b.plan.Ops, ModifyFileOp{Path: cp, ExpectedSHA256: expectedSHA256, Content: content, Mode: mode})
	return nil
}

// RunHook adds a RunHookOp for the given shell command, in the order
// hooks should run. Unlike CreateDir/WriteFile, hooks don't claim a path:
// several hooks are expected (e.g. "git init" then "go mod tidy").
func (b *Builder) RunHook(run string) error {
	if strings.TrimSpace(run) == "" {
		return fmt.Errorf("plan: empty hook command")
	}
	b.plan.Ops = append(b.plan.Ops, RunHookOp{Run: run})
	return nil
}

// claim cleans p and reserves it for a new op, failing loudly on a
// collision instead of letting whichever write happens last silently win.
func (b *Builder) claim(p string) (string, error) {
	cp, err := cleanRelPath(p)
	if err != nil {
		return "", err
	}
	if b.seen[cp] {
		return "", fmt.Errorf("plan: duplicate path %q", cp)
	}
	b.seen[cp] = true
	return cp, nil
}

// Build returns the accumulated Plan.
func (b *Builder) Build() Plan {
	return b.plan
}

// WithoutHooks returns a copy of p with every RunHookOp removed — used
// when hooks weren't confirmed (CLI) or allowed (MCP's allow_hooks):
// the rest of the plan still applies.
func (p Plan) WithoutHooks() Plan {
	filtered := make([]Op, 0, len(p.Ops))
	for _, op := range p.Ops {
		if op.Kind() != OpRunHook {
			filtered = append(filtered, op)
		}
	}
	return Plan{TargetDir: p.TargetDir, Ops: filtered}
}

// Hooks returns every RunHookOp in p, in order.
func (p Plan) Hooks() []RunHookOp {
	var hooks []RunHookOp
	for _, op := range p.Ops {
		if h, ok := op.(RunHookOp); ok {
			hooks = append(hooks, h)
		}
	}
	return hooks
}

// cleanRelPath canonicalizes p and rejects anything that would escape the
// target directory: empty paths, absolute paths, and "../" escapes.
func cleanRelPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("plan: empty path")
	}
	norm := strings.ReplaceAll(p, `\`, "/")
	if path.IsAbs(norm) {
		return "", fmt.Errorf("plan: path %q must be relative", p)
	}
	cp := path.Clean(norm)
	if cp == "." || cp == ".." || strings.HasPrefix(cp, "../") {
		return "", fmt.Errorf("plan: path %q escapes the target directory", p)
	}
	return cp, nil
}
