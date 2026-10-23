// Package tui implements "snap" run with no arguments: an interactive
// wizard — browse templates, fill variables, toggle optional blocks,
// preview the generated file tree, confirm — built entirely on top of the
// same internal/core/usecase calls the CLI and MCP server use. No business
// logic lives here.
package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/template"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

type step int

const (
	stepPickTemplate step = iota
	stepTargetDir
	stepVariables
	stepToggles
	stepPreview
	stepHookConfirm
	stepDone
	stepError
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// templateItem adapts a template for bubbles' list component. name is the
// store's own key for it (store.List()'s entry), kept separate from
// manifest.Name since Store.Path takes that key, not the declared name.
type templateItem struct {
	name     string
	manifest template.Manifest
}

func (i templateItem) Title() string       { return i.name }
func (i templateItem) Description() string { return i.manifest.Description }
func (i templateItem) FilterValue() string { return i.name }

// Model is the wizard's state machine.
type Model struct {
	fs         ports.FS
	store      ports.Store
	hookRunner ports.HookRunner

	step step
	err  error

	list list.Model

	targetInput textinput.Model
	targetDir   string

	templateName string
	manifest     template.Manifest

	varValues map[string]*string
	varForm   *huh.Form

	toggleValues map[string]*bool
	toggleForm   *huh.Form

	plan    plan.Plan
	preview viewport.Model

	hookConfirm   *huh.Form
	hookRunChoice *bool

	summary string

	width, height int
}

// New builds the wizard's initial model: the template list, loaded eagerly
// since it's local, fast I/O — there's no need for an async loading state.
func New(fs ports.FS, store ports.Store, hookRunner ports.HookRunner) Model {
	m := Model{fs: fs, store: store, hookRunner: hookRunner, step: stepPickTemplate}

	names, err := store.List()
	if err != nil {
		return m.withErr(fmt.Errorf("tui: list templates: %w", err))
	}
	if len(names) == 0 {
		return m.withErr(fmt.Errorf("tui: no templates available yet — capture one first with 'snap capture'"))
	}
	manifests, err := usecase.List(store, fs)
	if err != nil {
		return m.withErr(fmt.Errorf("tui: list templates: %w", err))
	}

	items := make([]list.Item, len(names))
	for i, name := range names {
		items[i] = templateItem{name: name, manifest: manifests[i]}
	}
	m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
	m.list.Title = "Select a template"
	return m
}

func (m Model) withErr(err error) Model {
	m.err = err
	m.step = stepError
	return m
}

// Init implements tea.Model. There's nothing to do asynchronously: the
// template list is already loaded by New.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model, dispatching to the handler for the current
// step.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
		m.preview = viewport.New(msg.Width, max(msg.Height-4, 3))
		m.preview.SetContent(plan.Tree(m.plan))
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepPickTemplate:
		return m.updatePickTemplate(msg)
	case stepTargetDir:
		return m.updateTargetDir(msg)
	case stepVariables:
		return m.updateVariables(msg)
	case stepToggles:
		return m.updateToggles(msg)
	case stepPreview:
		return m.updatePreview(msg)
	case stepHookConfirm:
		return m.updateHookConfirm(msg)
	default: // stepDone, stepError
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}
}

func (m Model) updatePickTemplate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q":
			if !m.list.SettingFilter() {
				return m, tea.Quit
			}
		case "enter":
			if item, ok := m.list.SelectedItem().(templateItem); ok {
				m.templateName = item.name
				m.manifest = item.manifest
				m.targetInput = textinput.New()
				m.targetInput.Placeholder = "path/to/new-project"
				m.targetInput.Focus()
				m.step = stepTargetDir
				return m, textinput.Blink
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) updateTargetDir(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, tea.Quit
		case "enter":
			dir := strings.TrimSpace(m.targetInput.Value())
			if dir == "" {
				return m, nil
			}
			m.targetDir = dir
			return m.enterVariables()
		}
	}
	var cmd tea.Cmd
	m.targetInput, cmd = m.targetInput.Update(msg)
	return m, cmd
}

func (m Model) enterVariables() (tea.Model, tea.Cmd) {
	defaults := map[string]string{"name": filepath.Base(m.targetDir)}
	m.varForm, m.varValues = buildVariableForm(m.manifest.Variables, defaults)
	m.step = stepVariables
	return m, m.varForm.Init()
}

func (m Model) updateVariables(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.varForm.Update(msg)
	m.varForm = form.(*huh.Form)
	if m.varForm.State == huh.StateCompleted {
		return m.afterVariables()
	}
	return m, cmd
}

func (m Model) afterVariables() (tea.Model, tea.Cmd) {
	if len(m.manifest.Optional) == 0 {
		return m.buildPreview()
	}
	m.toggleForm, m.toggleValues = buildToggleForm(m.manifest.Optional)
	m.step = stepToggles
	return m, m.toggleForm.Init()
}

func (m Model) updateToggles(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.toggleForm.Update(msg)
	m.toggleForm = form.(*huh.Form)
	if m.toggleForm.State == huh.StateCompleted {
		return m.buildPreview()
	}
	return m, cmd
}

// buildPreview resolves every collected value through usecase.New — the
// exact same call the CLI's --dry-run and MCP's plan_project make — and
// renders the resulting Plan as a file tree.
func (m Model) buildPreview() (tea.Model, tea.Cmd) {
	vars := make(map[string]string, len(m.varValues))
	for k, v := range m.varValues {
		vars[k] = *v
	}
	toggles := make(map[string]bool, len(m.toggleValues))
	for k, v := range m.toggleValues {
		toggles[k] = *v
	}
	p, err := usecase.New(usecase.NewParams{
		Store:       m.store,
		FS:          m.fs,
		Template:    m.templateName,
		TargetDir:   m.targetDir,
		Vars:        vars,
		Toggles:     toggles,
		GeneratedAt: time.Now().UTC(),
	})
	if err != nil {
		return m.withErr(fmt.Errorf("tui: %w", err)), nil
	}
	m.plan = p
	m.preview = viewport.New(m.width, max(m.height-4, 3))
	m.preview.SetContent(plan.Tree(p))
	m.step = stepPreview
	return m, nil
}

func (m Model) updatePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "y", "enter":
			if hooks := m.plan.Hooks(); len(hooks) > 0 {
				m.hookConfirm, m.hookRunChoice = buildHookConfirmForm(hooks)
				m.step = stepHookConfirm
				return m, m.hookConfirm.Init()
			}
			return m.apply(false)
		case "q", "esc":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.preview, cmd = m.preview.Update(msg)
	return m, cmd
}

func (m Model) updateHookConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.hookConfirm.Update(msg)
	m.hookConfirm = form.(*huh.Form)
	if m.hookConfirm.State == huh.StateCompleted {
		return m.apply(*m.hookRunChoice)
	}
	return m, cmd
}

// apply writes the plan to disk, running hooks only if allowHooks is true
// — mirroring the CLI's own hook-confirmation gate.
func (m Model) apply(allowHooks bool) (tea.Model, tea.Cmd) {
	p := m.plan
	ex := executor.New(m.fs)
	if allowHooks && len(p.Hooks()) > 0 {
		ex = executor.NewWithHooks(m.fs, m.hookRunner)
	} else {
		p = p.WithoutHooks()
	}
	if err := ex.Apply(p); err != nil {
		return m.withErr(fmt.Errorf("tui: %w", err)), nil
	}
	m.summary = fmt.Sprintf("Generated %s from %s.", p.TargetDir, m.templateName)
	m.step = stepDone
	return m, nil
}

// View implements tea.Model, rendering the current step.
func (m Model) View() string {
	switch m.step {
	case stepPickTemplate:
		return m.list.View()
	case stepTargetDir:
		return fmt.Sprintf("%s\n\n%s\n\n%s",
			titleStyle.Render("Target directory"),
			m.targetInput.View(),
			helpStyle.Render("enter: continue • esc: quit"))
	case stepVariables:
		return m.varForm.View()
	case stepToggles:
		return m.toggleForm.View()
	case stepPreview:
		return fmt.Sprintf("%s\n\n%s\n\n%s",
			titleStyle.Render("Preview"),
			m.preview.View(),
			helpStyle.Render("y/enter: generate • q/esc: quit"))
	case stepHookConfirm:
		return m.hookConfirm.View()
	case stepDone:
		return fmt.Sprintf("%s\n\n%s\n\n%s", successStyle.Render("✓ Done"), m.summary, helpStyle.Render("press any key to exit"))
	case stepError:
		return fmt.Sprintf("%s\n\n%s", errorStyle.Render("✗ Error"), m.err.Error())
	default:
		return ""
	}
}
