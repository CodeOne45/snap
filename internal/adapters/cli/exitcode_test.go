package cli

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil is success", nil, 0},
		{"plain error is user error", errors.New("bad input"), ExitUserError},
		{"wrapped plain error is user error", fmt.Errorf("snap new: %w", errors.New("missing var")), ExitUserError},
		{"internal error", internalErrorf("snap new: %w", errors.New("boom")), ExitInternalError},
		{"wrapped internal error stays internal", fmt.Errorf("outer: %w", internalErrorf("inner: %w", errors.New("boom"))), ExitInternalError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCodeFor(tt.err); got != tt.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestInternalErrorf_Unwraps(t *testing.T) {
	cause := errors.New("config dir unavailable")
	err := internalErrorf("snap new: %w", cause)
	if !errors.Is(err, cause) {
		t.Errorf("internalErrorf(%v) does not unwrap to the cause", err)
	}
}
