// Package prompt implements ports.Prompter by reading lines from an
// injected io.Reader and writing prompts to an injected io.Writer, so it
// works against a real terminal or, in tests, in-memory buffers.
package prompt

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/CodeOne45/snap/internal/ports"
)

// Stdin is a line-based ports.Prompter.
type Stdin struct {
	r *bufio.Reader
	w io.Writer
}

// New returns a Stdin prompter reading from r and writing prompts to w.
func New(r io.Reader, w io.Writer) *Stdin {
	return &Stdin{r: bufio.NewReader(r), w: w}
}

// Ask implements ports.Prompter: it prints message (with defaultValue shown
// in brackets, if any) and returns the trimmed line read, or defaultValue if
// the line is empty.
func (p *Stdin) Ask(message, defaultValue string) (string, error) {
	var promptErr error
	if defaultValue != "" {
		_, promptErr = fmt.Fprintf(p.w, "%s [%s]: ", message, defaultValue)
	} else {
		_, promptErr = fmt.Fprintf(p.w, "%s: ", message)
	}
	if promptErr != nil {
		return "", fmt.Errorf("prompt: ask %q: %w", message, promptErr)
	}
	line, err := p.readLine()
	if err != nil {
		return "", fmt.Errorf("prompt: ask %q: %w", message, err)
	}
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

// Confirm implements ports.Prompter: it prints message with a [Y/n] or
// [y/N] hint matching defaultYes, and returns true for any input starting
// with 'y'/'Y', false for 'n'/'N', and defaultYes for an empty line.
func (p *Stdin) Confirm(message string, defaultYes bool) (bool, error) {
	hint := "[y/N]"
	if defaultYes {
		hint = "[Y/n]"
	}
	if _, err := fmt.Fprintf(p.w, "%s %s: ", message, hint); err != nil {
		return false, fmt.Errorf("prompt: confirm %q: %w", message, err)
	}
	line, err := p.readLine()
	if err != nil {
		return false, fmt.Errorf("prompt: confirm %q: %w", message, err)
	}
	switch strings.ToLower(line) {
	case "":
		return defaultYes, nil
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	default:
		return false, fmt.Errorf("prompt: confirm %q: unrecognized answer %q", message, line)
	}
}

func (p *Stdin) readLine() (string, error) {
	line, err := p.r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

var _ ports.Prompter = (*Stdin)(nil)
