// Package usagelog records a local, private history of what snap has
// generated: one JSON line per successful apply, read back by `snap
// stats`. Entirely local — nothing here ever sends data anywhere — and
// disabled outright by setting SNAP_NO_USAGE_LOG.
package usagelog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CodeOne45/snap/internal/core/plan"
)

// DisableEnvVar is the environment variable that, when set to any
// non-empty value, makes Append a no-op — no directory or file is
// created.
const DisableEnvVar = "SNAP_NO_USAGE_LOG"

const fileName = "usage.jsonl"

// Entry is one logged generation or recipe application.
type Entry struct {
	Template string     `json:"template,omitempty"`
	Recipe   string     `json:"recipe,omitempty"`
	Stats    plan.Stats `json:"stats"`
	Time     time.Time  `json:"time"`
	// Source is "cli" or "mcp".
	Source string `json:"source"`
}

// Dir returns $XDG_STATE_HOME/snap, or ~/.local/state/snap if
// XDG_STATE_HOME isn't set.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "snap"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("usagelog: determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "snap"), nil
}

// Append adds e as one JSON line to the usage log, creating the log
// directory if needed. A no-op when DisableEnvVar is set.
func Append(e Entry) error {
	if os.Getenv(DisableEnvVar) != "" {
		return nil
	}

	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("usagelog: %w", err)
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("usagelog: marshal entry: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(dir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("usagelog: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("usagelog: write: %w", err)
	}
	return nil
}

// ReadAll returns every logged entry, oldest first. No log file yet is
// not an error — it returns an empty slice.
func ReadAll() ([]Entry, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("usagelog: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	entries := make([]Entry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("usagelog: parse entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
