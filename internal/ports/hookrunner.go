package ports

// Hook is a single post-generate command to run in a project directory.
type Hook struct {
	Run string
	Dir string
}

// HookResult carries a hook's output and exit code, whether it succeeded
// or not — so a failure can be reported with full context, per
// CLAUDE.md's safety rule that a failed hook is reported with its exit
// code and output, not just a bare error.
type HookResult struct {
	Output   string
	ExitCode int
}

// HookRunner is the port for executing template hooks (e.g. "git init").
// Callers must gate this behind explicit confirmation (CLI) or
// allow_hooks (MCP); the port itself does not enforce that.
type HookRunner interface {
	// Run executes hook.Run in hook.Dir. A non-zero exit code, or a
	// failure to even start the command, is reported via a non-nil error;
	// HookResult is always populated with whatever output/exit code is
	// available, even on error.
	Run(hook Hook) (HookResult, error)
}
