package usagelog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CodeOne45/snap/internal/core/plan"
)

func TestDir_UsesXDGStateHome(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != filepath.Join(state, "snap") {
		t.Errorf("Dir() = %q, want %q", dir, filepath.Join(state, "snap"))
	}
}

func TestAppend_WritesOneJSONLine(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	entry := Entry{
		Template: "api-starter",
		Stats:    plan.Stats{Files: 3, TextFiles: 3, Bytes: 120, ApproxTokens: 42},
		Time:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Source:   "cli",
	}
	if err := Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0] != entry {
		t.Errorf("entries[0] = %+v, want %+v", entries[0], entry)
	}
}

func TestAppend_MultipleEntriesInOrder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	for i, source := range []string{"cli", "mcp", "cli"} {
		e := Entry{Template: "t", Source: source, Time: time.Date(2026, 1, i+1, 0, 0, 0, 0, time.UTC)}
		if err := Append(e); err != nil {
			t.Fatalf("Append(%d): %v", i, err)
		}
	}

	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	for i, want := range []string{"cli", "mcp", "cli"} {
		if entries[i].Source != want {
			t.Errorf("entries[%d].Source = %q, want %q", i, entries[i].Source, want)
		}
	}
}

func TestAppend_DisabledByEnvVar(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("SNAP_NO_USAGE_LOG", "1")

	if err := Append(Entry{Template: "x", Source: "cli", Time: time.Now()}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if _, err := os.Stat(filepath.Join(state, "snap")); err == nil {
		t.Error("the usage log directory should not exist: SNAP_NO_USAGE_LOG was set")
	}
}

func TestReadAll_NoFileReturnsEmpty(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %+v, want empty", entries)
	}
}
