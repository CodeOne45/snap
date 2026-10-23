package ports

// Prompter is the user-interaction port, used to ask for confirmation or
// variable values when a CLI/TUI session is interactive.
//
// Fleshed out alongside snap new / snap capture (M3/M5).
type Prompter interface {
	Confirm(message string, defaultYes bool) (bool, error)
	Ask(message, defaultValue string) (string, error)
}
