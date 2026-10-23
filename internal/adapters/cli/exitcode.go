package cli

import (
	"errors"
	"fmt"
)

// Exit codes returned by cmd/snap's main. These are part of the CLI
// contract (documented in README.md) that the MCP server and scripts can
// rely on: 0 is success; anything else is from one of these two classes.
const (
	// ExitUserError is returned for anything traceable to the invocation
	// itself: bad arguments or flags, a missing/invalid template, a
	// validation failure, a missing required variable, a target directory
	// that already exists. The user can fix the input and retry.
	ExitUserError = 1
	// ExitInternalError is returned when snap's own environment lookup or
	// I/O failed unexpectedly (e.g. the config directory can't be
	// determined, an unrelated disk error) — not something fixable by
	// changing command-line input.
	ExitInternalError = 2
)

// internalError marks an error as internal (see ExitInternalError's doc for
// what qualifies), so ExitCodeFor classifies it correctly instead of
// defaulting to ExitUserError.
type internalError struct{ err error }

func (e *internalError) Error() string { return e.err.Error() }
func (e *internalError) Unwrap() error { return e.err }

// internalErrorf formats an internal error.
func internalErrorf(format string, args ...any) error {
	return &internalError{err: fmt.Errorf(format, args...)}
}

// ExitCodeFor maps an error returned by the root command's Execute to one
// of the exit codes above.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var ie *internalError
	if errors.As(err, &ie) {
		return ExitInternalError
	}
	return ExitUserError
}
