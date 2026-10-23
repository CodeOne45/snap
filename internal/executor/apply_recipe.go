package executor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/CodeOne45/snap/internal/core/plan"
)

// ApplyRecipe applies p (M20's snap add) to an already-populated TargetDir,
// writing each op directly to its final path rather than staging a full
// temp directory first: unlike Apply/ApplyUpdate, a recipe only ever
// touches a handful of specific files, not the whole tree.
//
// Safety is pre-flight only, not transactional: every ModifyFileOp's
// recorded hash and every WriteFileOp's target (if it already exists) are
// checked against TargetDir's current content before anything is written,
// so a Plan built from a stale snapshot is refused in full rather than
// partially applied. But once writing starts, a failure partway through
// (e.g. a disk error on the third of five files) leaves TargetDir exactly
// as far as it got — there is no backup/restore here. That's M22's "in-
// place transactional executor" to add.
func (e *Executor) ApplyRecipe(p plan.Plan) error {
	fi, err := e.fs.Stat(p.TargetDir)
	if err != nil {
		return fmt.Errorf("executor: apply recipe: target %q does not exist: %w", p.TargetDir, err)
	}
	if !fi.IsDir {
		return fmt.Errorf("executor: apply recipe: target %q is not a directory", p.TargetDir)
	}

	if err := e.checkRecipeConflicts(p); err != nil {
		return err
	}
	if err := e.applyRecipeOps(p.TargetDir, p.Ops); err != nil {
		return err
	}

	return e.runHooks(p.TargetDir, p.Hooks())
}

// checkRecipeConflicts verifies every op's assumption about TargetDir's
// current content before anything is written. Every conflict is collected
// so the caller sees the full picture in one refusal instead of failing
// one file at a time partway through.
func (e *Executor) checkRecipeConflicts(p plan.Plan) error {
	var conflicts []string
	for _, op := range p.Ops {
		switch o := op.(type) {
		case plan.ModifyFileOp:
			current, err := e.fs.ReadFile(path.Join(p.TargetDir, o.Path))
			if err != nil {
				conflicts = append(conflicts, fmt.Sprintf("%s: expected to already exist: %v", o.Path, err))
				continue
			}
			if sha256Hex(current) != o.ExpectedSHA256 {
				conflicts = append(conflicts, fmt.Sprintf("%s: changed since the plan was built", o.Path))
			}
		case plan.WriteFileOp:
			current, err := e.fs.ReadFile(path.Join(p.TargetDir, o.Path))
			if err != nil {
				continue // doesn't exist yet, as the plan expects
			}
			if !bytes.Equal(current, o.Content) {
				conflicts = append(conflicts, fmt.Sprintf("%s: already exists with different content", o.Path))
			}
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("executor: apply recipe: conflicts:\n  %s", strings.Join(conflicts, "\n  "))
	}
	return nil
}

func (e *Executor) applyRecipeOps(targetDir string, ops []plan.Op) error {
	for _, op := range ops {
		switch o := op.(type) {
		case plan.CreateDirOp:
			if err := e.fs.MkdirAll(path.Join(targetDir, o.Path), 0o755); err != nil {
				return fmt.Errorf("executor: apply recipe: create dir %q: %w", o.Path, err)
			}
		case plan.WriteFileOp:
			if err := e.writeRecipeFile(targetDir, o.Path, o.Content, o.Mode); err != nil {
				return fmt.Errorf("executor: apply recipe: write %q: %w", o.Path, err)
			}
		case plan.ModifyFileOp:
			if err := e.writeRecipeFile(targetDir, o.Path, o.Content, o.Mode); err != nil {
				return fmt.Errorf("executor: apply recipe: modify %q: %w", o.Path, err)
			}
		case plan.RunHookOp:
			continue // hooks run last, via runHooks
		default:
			return fmt.Errorf("executor: apply recipe: unsupported op type %T", op)
		}
	}
	return nil
}

// writeRecipeFile writes content to targetDir/p, creating parent
// directories as needed (a recipe's new file may be the first thing under
// a directory the target doesn't have yet).
func (e *Executor) writeRecipeFile(targetDir, p string, content []byte, mode fs.FileMode) error {
	dest := path.Join(targetDir, p)
	if err := e.fs.MkdirAll(path.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	if mode == 0 {
		mode = defaultFileMode
	}
	return e.fs.WriteFile(dest, content, mode)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
