package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CodeOne45/snap/internal/ports"
)

// Run starts the wizard and blocks until the user quits or completes it.
func Run(fs ports.FS, store ports.Store, hookRunner ports.HookRunner) error {
	m := New(fs, store, hookRunner)
	final, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	if err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	if fm, ok := final.(Model); ok && fm.step == stepError && fm.err != nil {
		return fm.err
	}
	return nil
}
