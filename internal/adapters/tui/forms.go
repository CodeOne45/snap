package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/template"
)

// buildVariableForm builds one huh.Input per declared variable, pre-filled
// from defaults (falling back to the variable's own declared default). It
// returns the form and a map of pointers huh writes each field's value
// into directly — read after the form reaches huh.StateCompleted.
func buildVariableForm(vars []template.Variable, defaults map[string]string) (*huh.Form, map[string]*string) {
	values := make(map[string]*string, len(vars))
	fields := make([]huh.Field, 0, len(vars))
	for _, v := range vars {
		val := v.Default
		if d, ok := defaults[v.Name]; ok {
			val = d
		}
		values[v.Name] = &val

		title := v.Name
		if v.Prompt != "" {
			title = v.Prompt
		}
		fields = append(fields, huh.NewInput().
			Key(v.Name).
			Title(title).
			Value(&val).
			Validate(v.CheckValue))
	}
	return huh.NewForm(huh.NewGroup(fields...)), values
}

// buildToggleForm builds one huh.Confirm per declared [[optional]] block,
// pre-filled from its declared default.
func buildToggleForm(blocks []template.Optional) (*huh.Form, map[string]*bool) {
	values := make(map[string]*bool, len(blocks))
	fields := make([]huh.Field, 0, len(blocks))
	for _, o := range blocks {
		val := o.Default
		values[o.Name] = &val

		title := o.Name
		if o.Prompt != "" {
			title = o.Prompt
		}
		fields = append(fields, huh.NewConfirm().
			Key(o.Name).
			Title(title).
			Value(&val).
			Affirmative("Yes").
			Negative("No"))
	}
	return huh.NewForm(huh.NewGroup(fields...)), values
}

// buildHookConfirmForm builds a single confirm asking whether to run the
// plan's post-generate hooks, listing each command. It returns the form
// and the bool huh writes the answer into directly — read after the form
// reaches huh.StateCompleted.
func buildHookConfirmForm(hooks []plan.RunHookOp) (*huh.Form, *bool) {
	desc := fmt.Sprintf("This template has %d post-generate hook(s) that will run:\n", len(hooks))
	for _, h := range hooks {
		desc += "  " + h.Run + "\n"
	}
	var allow bool
	confirm := huh.NewConfirm().
		Key("run").
		Title("Run hooks?").
		Description(desc).
		Value(&allow).
		Affirmative("Yes").
		Negative("No")
	return huh.NewForm(huh.NewGroup(confirm)), &allow
}
