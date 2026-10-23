package mcp

import (
	"os"
	"testing"

	"github.com/CodeOne45/snap/internal/adapters/usagelog"
)

// TestMain disables the usage log for every test in this package by
// default (M19 wires it into create_project) — without this, any test
// exercising that tool would append a real entry to the developer
// machine's actual $XDG_STATE_HOME/snap/usage.jsonl. A test that
// specifically exercises usage logging re-enables it, sandboxed to its
// own t.TempDir(), via t.Setenv.
func TestMain(m *testing.M) {
	_ = os.Setenv(usagelog.DisableEnvVar, "1")
	os.Exit(m.Run())
}
