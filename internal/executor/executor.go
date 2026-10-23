// Package executor is the only package that applies a plan.Plan to a
// filesystem. It is transactional: every Op is written into a temporary
// directory first, then moved into place with a single atomic rename, so a
// failure mid-apply never leaves a half-written project at TargetDir.
package executor

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/ports"
)

const defaultFileMode fs.FileMode = 0o644

// Executor applies plans through an FS port (real or in-memory).
type Executor struct {
	fs         ports.FS
	hookRunner ports.HookRunner
}

// New returns an Executor backed by fs, with no HookRunner: a Plan
// containing a RunHookOp will fail rather than silently skip it. Use this
// whenever the Plan is known never to contain hooks (e.g. snap capture),
// or after calling plan.Plan.WithoutHooks on a Plan whose hooks weren't
// confirmed/allowed.
func New(fs ports.FS) *Executor {
	return &Executor{fs: fs}
}

// NewWithHooks returns an Executor that also runs RunHookOps via hr, once
// the rest of the Plan has been committed to TargetDir. Only use this once
// the caller has confirmed (CLI) or allowed (MCP's allow_hooks) running
// hooks — the executor itself enforces no such gate.
func NewWithHooks(fs ports.FS, hr ports.HookRunner) *Executor {
	return &Executor{fs: fs, hookRunner: hr}
}

// Apply executes p transactionally. p.TargetDir may not exist, or may exist
// as an empty directory; a file or a non-empty directory there blocks the
// apply. Deciding whether to remove an existing non-empty target first
// (--force), prompt, or refuse is the calling use case's job, not the
// executor's.
//
// Hooks run last, after the target directory is committed (renamed into
// place) — so a failing hook never rolls back the files that were already
// written; it's reported as an error, with the files left exactly as
// generated.
func (e *Executor) Apply(p plan.Plan) error {
	blocked, err := e.targetBlocked(p.TargetDir)
	if err != nil {
		return fmt.Errorf("executor: apply: check target %q: %w", p.TargetDir, err)
	}
	if blocked {
		return fmt.Errorf("executor: apply: target %q already exists and is not an empty directory", p.TargetDir)
	}

	tmp, err := e.tempDir(p.TargetDir)
	if err != nil {
		return fmt.Errorf("executor: apply: create temp dir: %w", err)
	}

	if err := e.applyOps(tmp, p.Ops); err != nil {
		_ = e.fs.RemoveAll(tmp)
		return err
	}

	if err := e.fs.Rename(tmp, p.TargetDir); err != nil {
		_ = e.fs.RemoveAll(tmp)
		return fmt.Errorf("executor: apply: finalize %q: %w", p.TargetDir, err)
	}

	return e.runHooks(p.TargetDir, p.Hooks())
}

// runHooks runs every hook in order, stopping at the first failure. It is
// called only after TargetDir is already committed, so a failure here
// never removes or rolls back the generated project.
func (e *Executor) runHooks(targetDir string, hooks []plan.RunHookOp) error {
	for _, h := range hooks {
		if e.hookRunner == nil {
			return fmt.Errorf("executor: apply: plan has hook %q but no HookRunner is configured", h.Run)
		}
		result, err := e.hookRunner.Run(ports.Hook{Run: h.Run, Dir: targetDir})
		if err != nil {
			return fmt.Errorf("executor: apply: hook %q failed (exit %d): %w\noutput:\n%s", h.Run, result.ExitCode, err, result.Output)
		}
	}
	return nil
}

// targetBlocked reports whether dir already exists as a file or a
// non-empty directory, in which case Apply must refuse rather than clobber
// something.
func (e *Executor) targetBlocked(dir string) (bool, error) {
	fi, err := e.fs.Stat(dir)
	if err != nil {
		return false, nil // doesn't exist: not blocked
	}
	if !fi.IsDir {
		return true, nil
	}
	entries, err := e.fs.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

func (e *Executor) applyOps(tmp string, ops []plan.Op) error {
	for _, op := range ops {
		switch o := op.(type) {
		case plan.CreateDirOp:
			if err := e.fs.MkdirAll(path.Join(tmp, o.Path), 0o755); err != nil {
				return fmt.Errorf("executor: apply: create dir %q: %w", o.Path, err)
			}
		case plan.WriteFileOp:
			dest := path.Join(tmp, o.Path)
			if err := e.fs.MkdirAll(path.Dir(dest), 0o755); err != nil {
				return fmt.Errorf("executor: apply: create parent of %q: %w", o.Path, err)
			}
			mode := o.Mode
			if mode == 0 {
				mode = defaultFileMode
			}
			if err := e.fs.WriteFile(dest, o.Content, mode); err != nil {
				return fmt.Errorf("executor: apply: write %q: %w", o.Path, err)
			}
		case plan.RunHookOp:
			continue // hooks run after rename, via runHooks — not during the file-writing pass
		default:
			return fmt.Errorf("executor: apply: unsupported op type %T", op)
		}
	}
	return nil
}

// tempDir picks a sibling-of-target temp directory name and creates it.
func (e *Executor) tempDir(targetDir string) (string, error) {
	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	tmp := targetDir + ".snap-tmp-" + suffix
	if err := e.fs.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}
	return tmp, nil
}

func randomSuffix() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("executor: generate temp suffix: %w", err)
	}
	return hex.EncodeToString(b), nil
}
