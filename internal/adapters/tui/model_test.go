package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/ports"
)

const simpleManifest = `
[template]
name = "simple"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"
`

const dockerManifest = `
[template]
name = "withdocker"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[optional]]
name = "docker"
default = true
paths = ["Dockerfile"]
`

const hookedManifest = `
[template]
name = "hooked"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[hooks.post]]
run = "touch hook-ran.txt"
`

// fakeHookRunner records calls instead of spawning a real subprocess.
type fakeHookRunner struct {
	calls []ports.Hook
}

func (f *fakeHookRunner) Run(hook ports.Hook) (ports.HookResult, error) {
	f.calls = append(f.calls, hook)
	return ports.HookResult{}, nil
}

func newTestFixture(t *testing.T, name, manifest string, extraFiles map[string]string) (ports.FS, ports.Store) {
	t.Helper()
	fsys := memfs.New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	must(fsys.MkdirAll("templates/"+name+"/1.0.0", 0o755))
	must(fsys.WriteFile("templates/"+name+"/1.0.0/snap.toml", []byte(manifest), 0o644))
	must(fsys.WriteFile("templates/"+name+"/1.0.0/README.md", []byte("# {{name}}"), 0o644))
	for path, content := range extraFiles {
		must(fsys.WriteFile("templates/"+name+"/1.0.0/"+path, []byte(content), 0o644))
	}
	return fsys, local.New(fsys, "templates")
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// pickAndSetTarget drives the model through stepPickTemplate and
// stepTargetDir (both plain bubbles components, safe to drive with real
// keystrokes) and lands on stepVariables.
func pickAndSetTarget(t *testing.T, m Model, targetDir string) Model {
	t.Helper()
	resized, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = resized.(Model)
	newM, cmd := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepTargetDir {
		t.Fatalf("after picking template: step = %v, want stepTargetDir", m.step)
	}
	if cmd == nil {
		t.Error("entering stepTargetDir: want a blink cmd, got nil")
	}

	for _, r := range targetDir {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(Model)
	}
	newM, _ = m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepVariables {
		t.Fatalf("after target dir: step = %v, want stepVariables", m.step)
	}
	if m.targetDir != targetDir {
		t.Errorf("targetDir = %q, want %q", m.targetDir, targetDir)
	}
	return m
}

// completeVariables sets every collected variable pointer directly (as if
// huh had collected it from the user) and marks the form completed — huh's
// own keystroke handling is out of scope here; what this test verifies is
// the wizard's own step transitions and plan-building logic.
func completeVariables(t *testing.T, m Model, values map[string]string) Model {
	t.Helper()
	for k, v := range values {
		ptr, ok := m.varValues[k]
		if !ok {
			t.Fatalf("no variable field %q in form", k)
		}
		*ptr = v
	}
	m.varForm.State = huh.StateCompleted
	newM, _ := m.Update(key(tea.KeyRunes))
	return newM.(Model)
}

func completeToggles(t *testing.T, m Model, values map[string]bool) Model {
	t.Helper()
	for k, v := range values {
		ptr, ok := m.toggleValues[k]
		if !ok {
			t.Fatalf("no toggle field %q in form", k)
		}
		*ptr = v
	}
	m.toggleForm.State = huh.StateCompleted
	newM, _ := m.Update(key(tea.KeyRunes))
	return newM.(Model)
}

func TestWizard_NoTemplatesAvailable(t *testing.T) {
	fsys := memfs.New()
	if err := fsys.MkdirAll("templates", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	store := local.New(fsys, "templates")
	m := New(fsys, store, &fakeHookRunner{})
	if m.step != stepError {
		t.Fatalf("step = %v, want stepError", m.step)
	}
}

func TestWizard_HappyPath_NoOptionalNoHooks(t *testing.T) {
	fsys, store := newTestFixture(t, "simple", simpleManifest, nil)
	m := New(fsys, store, &fakeHookRunner{})
	if m.step != stepPickTemplate {
		t.Fatalf("initial step = %v, want stepPickTemplate", m.step)
	}

	m = pickAndSetTarget(t, m, "out")
	if got := *m.varValues["name"]; got != "out" {
		t.Errorf("name default = %q, want %q (the target dir's basename)", got, "out")
	}

	m = completeVariables(t, m, map[string]string{"name": "acme"})
	if m.step != stepPreview {
		t.Fatalf("step after variables (no optional blocks) = %v, want stepPreview", m.step)
	}
	if m.plan.TargetDir != "out" {
		t.Errorf("plan.TargetDir = %q, want %q", m.plan.TargetDir, "out")
	}

	newM, _ := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepDone {
		t.Fatalf("step after confirming preview = %v, want stepDone", m.step)
	}

	content, err := fsys.ReadFile("out/README.md")
	if err != nil || string(content) != "# acme" {
		t.Errorf("out/README.md = %q, %v, want %q", content, err, "# acme")
	}
}

func TestWizard_OptionalToggleOff(t *testing.T) {
	fsys, store := newTestFixture(t, "withdocker", dockerManifest, map[string]string{"Dockerfile": "FROM scratch"})
	m := New(fsys, store, &fakeHookRunner{})

	m = pickAndSetTarget(t, m, "out")
	m = completeVariables(t, m, map[string]string{"name": "acme"})
	if m.step != stepToggles {
		t.Fatalf("step after variables (has an optional block) = %v, want stepToggles", m.step)
	}

	m = completeToggles(t, m, map[string]bool{"docker": false})
	if m.step != stepPreview {
		t.Fatalf("step after toggles = %v, want stepPreview", m.step)
	}

	newM, _ := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepDone {
		t.Fatalf("step = %v, want stepDone", m.step)
	}
	if _, err := fsys.Stat("out/Dockerfile"); err == nil {
		t.Error("Dockerfile should not exist: docker was toggled off")
	}
}

func TestWizard_OptionalToggleOn(t *testing.T) {
	fsys, store := newTestFixture(t, "withdocker", dockerManifest, map[string]string{"Dockerfile": "FROM scratch"})
	m := New(fsys, store, &fakeHookRunner{})

	m = pickAndSetTarget(t, m, "out")
	m = completeVariables(t, m, map[string]string{"name": "acme"})
	m = completeToggles(t, m, map[string]bool{"docker": true})

	newM, _ := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepDone {
		t.Fatalf("step = %v, want stepDone", m.step)
	}
	content, err := fsys.ReadFile("out/Dockerfile")
	if err != nil || string(content) != "FROM scratch" {
		t.Errorf("out/Dockerfile = %q, %v, want it present", content, err)
	}
}

func TestWizard_HooksDeclined(t *testing.T) {
	fsys, store := newTestFixture(t, "hooked", hookedManifest, nil)
	hooks := &fakeHookRunner{}
	m := New(fsys, store, hooks)

	m = pickAndSetTarget(t, m, "out")
	m = completeVariables(t, m, map[string]string{"name": "acme"})
	if m.step != stepPreview {
		t.Fatalf("step = %v, want stepPreview", m.step)
	}

	newM, _ := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepHookConfirm {
		t.Fatalf("step after confirming a plan with hooks = %v, want stepHookConfirm", m.step)
	}

	// hookRunChoice defaults to false (never toggled) — hooks declined.
	m.hookConfirm.State = huh.StateCompleted
	newM, _ = m.Update(key(tea.KeyRunes))
	m = newM.(Model)
	if m.step != stepDone {
		t.Fatalf("step = %v, want stepDone", m.step)
	}
	if len(hooks.calls) != 0 {
		t.Errorf("hook calls = %+v, want none: hooks were declined", hooks.calls)
	}
	if _, err := fsys.Stat("out/README.md"); err != nil {
		t.Errorf("README.md should still be generated even when hooks are declined: %v", err)
	}
}

func TestWizard_HooksApproved(t *testing.T) {
	fsys, store := newTestFixture(t, "hooked", hookedManifest, nil)
	hooks := &fakeHookRunner{}
	m := New(fsys, store, hooks)

	m = pickAndSetTarget(t, m, "out")
	m = completeVariables(t, m, map[string]string{"name": "acme"})
	newM, _ := m.Update(key(tea.KeyEnter))
	m = newM.(Model)
	if m.step != stepHookConfirm {
		t.Fatalf("step = %v, want stepHookConfirm", m.step)
	}

	*m.hookRunChoice = true
	m.hookConfirm.State = huh.StateCompleted
	newM, _ = m.Update(key(tea.KeyRunes))
	m = newM.(Model)
	if m.step != stepDone {
		t.Fatalf("step = %v, want stepDone", m.step)
	}
	if len(hooks.calls) != 1 || hooks.calls[0].Run != "touch hook-ran.txt" {
		t.Errorf("hook calls = %+v, want exactly one [touch hook-ran.txt]", hooks.calls)
	}
}

func TestWizard_CtrlCQuitsImmediately(t *testing.T) {
	fsys, store := newTestFixture(t, "simple", simpleManifest, nil)
	m := New(fsys, store, &fakeHookRunner{})

	_, cmd := m.Update(key(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("ctrl+c: want a quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c cmd() = %#v, want tea.QuitMsg", cmd())
	}
}

func TestWizard_QuitFromPreviewDoesNotWrite(t *testing.T) {
	fsys, store := newTestFixture(t, "simple", simpleManifest, nil)
	m := New(fsys, store, &fakeHookRunner{})

	m = pickAndSetTarget(t, m, "out")
	m = completeVariables(t, m, map[string]string{"name": "acme"})
	if m.step != stepPreview {
		t.Fatalf("step = %v, want stepPreview", m.step)
	}

	_, cmd := m.Update(key(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("esc from preview: want a quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("esc cmd() = %#v, want tea.QuitMsg", cmd())
	}
	if _, err := fsys.Stat("out"); err == nil {
		t.Error("quitting from the preview must not write anything, but \"out\" exists")
	}
}
