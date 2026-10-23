package executor

import (
	"fmt"

	"github.com/CodeOne45/snap/internal/core/plan"
)

// ApplyUpdate applies p (M16's snap update) to an already-populated
// TargetDir, in place — unlike Apply, which only ever targets a fresh or
// empty directory. p is expected to represent the project's complete
// desired post-update state: every file that should exist afterward,
// changed or not. A file that existed before but isn't in p is simply
// gone afterward — there's no dedicated "remove" op; the caller tracks
// deleted paths separately, for reporting, if it wants to.
//
// Safety: p is first built in full in a fresh temp directory (exactly as
// Apply does), so a failure while building it never touches TargetDir at
// all. Only once that succeeds does ApplyUpdate touch the real project:
// TargetDir is renamed aside to a backup directory, then the temp
// directory is renamed into TargetDir's place. If that second rename
// fails, ApplyUpdate restores the backup back to TargetDir before
// returning the error — the backup itself is only ever removed after a
// fully successful swap. This is weaker than Apply's single-rename
// atomicity (there's a brief window where neither rename has happened yet
// a second time), but touching an existing, already-populated directory
// in place has no fully atomic option in the first place.
func (e *Executor) ApplyUpdate(p plan.Plan) error {
	fi, err := e.fs.Stat(p.TargetDir)
	if err != nil {
		return fmt.Errorf("executor: apply update: target %q does not exist: %w", p.TargetDir, err)
	}
	if !fi.IsDir {
		return fmt.Errorf("executor: apply update: target %q is not a directory", p.TargetDir)
	}

	tmp, err := e.tempDir(p.TargetDir)
	if err != nil {
		return fmt.Errorf("executor: apply update: create temp dir: %w", err)
	}
	if err := e.applyOps(tmp, p.Ops); err != nil {
		_ = e.fs.RemoveAll(tmp)
		return err
	}

	backup, err := e.backupDir(p.TargetDir)
	if err != nil {
		_ = e.fs.RemoveAll(tmp)
		return fmt.Errorf("executor: apply update: pick backup dir: %w", err)
	}
	if err := e.fs.Rename(p.TargetDir, backup); err != nil {
		_ = e.fs.RemoveAll(tmp)
		return fmt.Errorf("executor: apply update: move %q aside: %w", p.TargetDir, err)
	}

	if err := e.fs.Rename(tmp, p.TargetDir); err != nil {
		if restoreErr := e.fs.Rename(backup, p.TargetDir); restoreErr != nil {
			return fmt.Errorf("executor: apply update: swap failed (%w) AND restoring the original from %q failed (%w) — the original project is preserved there", err, backup, restoreErr)
		}
		return fmt.Errorf("executor: apply update: swap failed, original restored: %w", err)
	}

	return e.fs.RemoveAll(backup)
}

// backupDir picks a sibling-of-target backup directory name. It doesn't
// create it — Rename does that by moving targetDir there directly.
func (e *Executor) backupDir(targetDir string) (string, error) {
	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	return targetDir + ".snap-backup-" + suffix, nil
}
