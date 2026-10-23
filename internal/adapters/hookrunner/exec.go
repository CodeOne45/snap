// Package hookrunner implements ports.HookRunner by shelling out to the
// host's shell. This is the only adapter that spawns a subprocess; it
// exists purely to run a template-declared command, never anything core
// decides on its own.
package hookrunner

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/CodeOne45/snap/internal/ports"
)

// Exec runs hooks via the host shell ("sh -c" on Unix, "cmd /C" on
// Windows).
type Exec struct{}

// New returns an Exec hook runner.
func New() Exec { return Exec{} }

// Run implements ports.HookRunner.
func (Exec) Run(hook ports.Hook) (ports.HookResult, error) {
	cmd := shellCommand(hook.Run)
	cmd.Dir = hook.Dir

	output, runErr := cmd.CombinedOutput()
	result := ports.HookResult{Output: string(output)}

	if runErr == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("hookrunner: %q exited %d", hook.Run, result.ExitCode)
	}
	return result, fmt.Errorf("hookrunner: run %q: %w", hook.Run, runErr)
}

func shellCommand(run string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", run)
	}
	return exec.Command("sh", "-c", run)
}

var _ ports.HookRunner = Exec{}
