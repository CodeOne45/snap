package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeOne45/snap/internal/adapters/memfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

const testManifest = `
[template]
name = "api-starter"
version = "1.0.0"
description = "test fixture"

[[variables]]
name = "name"
type = "string"
`

const hookedManifest = `
[template]
name = "hooked"
version = "1.0.0"

[[variables]]
name = "name"
type = "string"

[[hooks.post]]
run = "git init"
`

const optionalManifest = `
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

// fakeHookRunner records calls instead of spawning a real subprocess — the
// MCP test harness runs entirely against memfs, which has no real
// directory for a real command to chdir into.
type fakeHookRunner struct {
	calls []ports.Hook
}

func (f *fakeHookRunner) Run(hook ports.Hook) (ports.HookResult, error) {
	f.calls = append(f.calls, hook)
	return ports.HookResult{}, nil
}

// testServer wires a hermetic in-memory template store and connects a
// client/server pair over an in-memory transport, per
// mcp.NewInMemoryTransports's doc: the server must connect first.
type testServer struct {
	fs    ports.FS
	store ports.Store
	hooks *fakeHookRunner
	cs    *sdk.ClientSession
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	fsys := memfs.New()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	must(fsys.MkdirAll("templates/api-starter/1.0.0/src", 0o755))
	must(fsys.WriteFile("templates/api-starter/1.0.0/snap.toml", []byte(testManifest), 0o644))
	must(fsys.WriteFile("templates/api-starter/1.0.0/README.md", []byte("# {{name|title}}"), 0o644))
	must(fsys.WriteFile("templates/api-starter/1.0.0/src/{{name|pascal}}.go", []byte("package main // {{name}}"), 0o644))
	must(fsys.MkdirAll("templates/hooked/1.0.0", 0o755))
	must(fsys.WriteFile("templates/hooked/1.0.0/snap.toml", []byte(hookedManifest), 0o644))
	must(fsys.WriteFile("templates/hooked/1.0.0/README.md", []byte("# {{name}}"), 0o644))
	must(fsys.MkdirAll("templates/withdocker/1.0.0", 0o755))
	must(fsys.WriteFile("templates/withdocker/1.0.0/snap.toml", []byte(optionalManifest), 0o644))
	must(fsys.WriteFile("templates/withdocker/1.0.0/README.md", []byte("# {{name}}"), 0o644))
	must(fsys.WriteFile("templates/withdocker/1.0.0/Dockerfile", []byte("FROM scratch"), 0o644))

	store := local.New(fsys, "templates")
	hooks := &fakeHookRunner{}
	server := NewServer(fsys, store, hooks, "test")

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	ctx := context.Background()

	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test-client"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return &testServer{fs: fsys, store: store, hooks: hooks, cs: cs}
}

func (s *testServer) callTool(t *testing.T, name string, args any) *sdk.CallToolResult {
	t.Helper()
	res, err := s.cs.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func (s *testServer) text(t *testing.T, res *sdk.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatalf("CallTool result has no content: %+v", res)
	}
	tc, ok := res.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("CallTool result content is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestListTemplates(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "list_templates", listTemplatesIn{})
	if res.IsError {
		t.Fatalf("list_templates returned an error: %s", s.text(t, res))
	}
	got := s.text(t, res)
	if !strings.Contains(got, `"name": "api-starter"`) {
		t.Errorf("list_templates = %s, want it to contain api-starter", got)
	}
}

func TestListTemplates_TagFilter(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "list_templates", listTemplatesIn{Tag: "nope-does-not-match"})
	var manifests []map[string]any
	if err := json.Unmarshal([]byte(s.text(t, res)), &manifests); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("list_templates with non-matching tag = %+v, want empty", manifests)
	}
}

func TestDescribeTemplate(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "describe_template", describeTemplateIn{Name: "api-starter"})
	if res.IsError {
		t.Fatalf("describe_template returned an error: %s", s.text(t, res))
	}
	got := s.text(t, res)
	for _, want := range []string{`"name": "api-starter"`, `"files"`, `src/{{name|pascal}}.go`} {
		if !strings.Contains(got, want) {
			t.Errorf("describe_template = %s, want it to contain %q", got, want)
		}
	}
}

func TestDescribeTemplate_Unknown(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "describe_template", describeTemplateIn{Name: "nope"})
	if !res.IsError {
		t.Error("describe_template(nope): want IsError, got false")
	}
}

func TestPlanProject(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "plan_project", planProjectIn{
		Template:  "api-starter",
		Variables: map[string]string{"name": "acme-site"},
		TargetDir: "out",
	})
	if res.IsError {
		t.Fatalf("plan_project returned an error: %s", s.text(t, res))
	}
	got := s.text(t, res)
	if !strings.Contains(got, `"target_dir": "out"`) {
		t.Errorf("plan_project = %s, want it to contain target_dir", got)
	}
	if !strings.Contains(got, `"stats"`) || !strings.Contains(got, `"files"`) {
		t.Errorf("plan_project = %s, want a stats object (M19)", got)
	}
	// No side effects.
	if _, err := s.fs.Stat("out"); err == nil {
		t.Error("plan_project must not write anything, but \"out\" exists")
	}
}

func TestCreateProject_IncludesStatsAndLogsUsage(t *testing.T) {
	t.Setenv(usagelog.DisableEnvVar, "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	s := newTestServer(t)
	res := s.callTool(t, "create_project", planProjectIn{
		Template:  "api-starter",
		Variables: map[string]string{"name": "acme-site"},
		TargetDir: "out",
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}
	got := s.text(t, res)
	if !strings.Contains(got, `"stats"`) || !strings.Contains(got, `"files"`) {
		t.Errorf("create_project = %s, want a stats object (M19)", got)
	}

	entries, err := usagelog.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Template != "api-starter" || entries[0].Source != "mcp" {
		t.Errorf("entries[0] = %+v, want Template=api-starter Source=mcp", entries[0])
	}
}

// TestCreateProject_ByteIdenticalToDirectUsecase is M8's explicit
// requirement: create_project's output must be byte-identical to calling
// usecase.New + executor.Apply directly with the same inputs.
func TestCreateProject_ByteIdenticalToDirectUsecase(t *testing.T) {
	s := newTestServer(t)

	res := s.callTool(t, "create_project", planProjectIn{
		Template:  "api-starter",
		Variables: map[string]string{"name": "acme-site"},
		TargetDir: "via-mcp",
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}

	direct, err := usecase.New(usecase.NewParams{
		Store:       s.store,
		FS:          s.fs,
		Template:    "api-starter",
		TargetDir:   "via-direct",
		Vars:        map[string]string{"name": "acme-site"},
		GeneratedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("usecase.New: %v", err)
	}
	if err := executor.New(s.fs).Apply(direct); err != nil {
		t.Fatalf("executor.Apply: %v", err)
	}

	for _, rel := range []string{"README.md", "src/AcmeSite.go"} {
		viaMCP, err := s.fs.ReadFile("via-mcp/" + rel)
		if err != nil {
			t.Fatalf("ReadFile(via-mcp/%s): %v", rel, err)
		}
		viaDirect, err := s.fs.ReadFile("via-direct/" + rel)
		if err != nil {
			t.Fatalf("ReadFile(via-direct/%s): %v", rel, err)
		}
		if string(viaMCP) != string(viaDirect) {
			t.Errorf("%s differs: via-mcp=%q via-direct=%q", rel, viaMCP, viaDirect)
		}
	}
}

func TestCreateProject_RefusesNonEmptyTarget(t *testing.T) {
	s := newTestServer(t)
	if err := s.fs.MkdirAll("taken", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := s.fs.WriteFile("taken/existing.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res := s.callTool(t, "create_project", planProjectIn{
		Template:  "api-starter",
		Variables: map[string]string{"name": "acme-site"},
		TargetDir: "taken",
	})
	if !res.IsError {
		t.Error("create_project onto a non-empty target: want IsError, got false")
	}
}

func TestCaptureTemplateAndSaveTemplate(t *testing.T) {
	s := newTestServer(t)
	if err := s.fs.MkdirAll("src/widgets-app", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := s.fs.WriteFile("src/widgets-app/go.mod", []byte("module github.com/someuser/widgets-app\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := s.fs.WriteFile("src/widgets-app/main.go", []byte("package main // widgets-app"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	captureRes := s.callTool(t, "capture_template", captureTemplateIn{SourceDir: "src/widgets-app", Name: "widgets-template"})
	if captureRes.IsError {
		t.Fatalf("capture_template returned an error: %s", s.text(t, captureRes))
	}
	var out captureTemplateOut
	if err := json.Unmarshal([]byte(s.text(t, captureRes)), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly 1", out.Candidates)
	}
	candidate := out.Candidates[0]
	if candidate.Key != "widgets-app" {
		t.Fatalf("candidate.Key = %q, want %q", candidate.Key, "widgets-app")
	}

	// capture_template must not have written anything.
	if _, err := s.fs.Stat("templates/widgets-template"); err == nil {
		t.Error("capture_template must not write anything, but the template directory exists")
	}

	saveRes := s.callTool(t, "save_template", saveTemplateIn{
		SourceDir: "src/widgets-app",
		Name:      "widgets-template",
		Accepted:  map[string]string{candidate.Key: "name"},
	})
	if saveRes.IsError {
		t.Fatalf("save_template returned an error: %s", s.text(t, saveRes))
	}

	content, err := s.fs.ReadFile("templates/widgets-template/0.1.0/main.go")
	if err != nil {
		t.Fatalf("ReadFile(templates/widgets-template/0.1.0/main.go): %v", err)
	}
	if string(content) != "package main // {{name}}" {
		t.Errorf("main.go = %q, want the accepted candidate replaced with {{name}}", content)
	}
}

func TestReadTemplateResource(t *testing.T) {
	s := newTestServer(t)
	res, err := s.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "template://api-starter/snap.toml"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, `name = "api-starter"`) {
		t.Errorf("ReadResource(snap.toml) = %+v, want it to contain the manifest", res.Contents)
	}

	res, err = s.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "template://api-starter/README.md"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 || res.Contents[0].Text != "# {{name|title}}" {
		t.Errorf("ReadResource(README.md) = %+v, want the README content", res.Contents)
	}
}

func TestReadTemplateResource_DisallowedFile(t *testing.T) {
	s := newTestServer(t)
	_, err := s.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "template://api-starter/src/main.go"})
	if err == nil {
		t.Error("ReadResource(a file other than snap.toml/README.md): want error, got nil")
	}
}

func TestReadTemplateResource_PathEscape(t *testing.T) {
	s := newTestServer(t)
	_, err := s.cs.ReadResource(context.Background(), &sdk.ReadResourceParams{URI: "template://api-starter/../secret"})
	if err == nil {
		t.Error("ReadResource with a \"..\" segment: want error, got nil")
	}
}

func TestNewProjectPrompt(t *testing.T) {
	s := newTestServer(t)
	res, err := s.cs.GetPrompt(context.Background(), &sdk.GetPromptParams{
		Name:      "new-project",
		Arguments: map[string]string{"template": "api-starter"},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatal("GetPrompt returned no messages")
	}
	tc, ok := res.Messages[0].Content.(*sdk.TextContent)
	if !ok {
		t.Fatalf("message content is %T, want *mcp.TextContent", res.Messages[0].Content)
	}
	for _, want := range []string{"describe_template", "plan_project", "create_project"} {
		if !strings.Contains(tc.Text, want) {
			t.Errorf("new-project prompt = %q, want it to mention %q", tc.Text, want)
		}
	}
}

func TestCreateProject_HooksSkippedWithoutAllowHooks(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "create_project", createProjectIn{
		planProjectIn: planProjectIn{
			Template:  "hooked",
			Variables: map[string]string{"name": "acme-site"},
			TargetDir: "out",
		},
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}
	if len(s.hooks.calls) != 0 {
		t.Errorf("hook calls = %+v, want none: allow_hooks was not set", s.hooks.calls)
	}
	if strings.Contains(s.text(t, res), `"hooks_run"`) {
		t.Errorf("result = %s, want hooks_run omitted (zero value)", s.text(t, res))
	}
}

func TestCreateProject_HooksRunWithAllowHooks(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "create_project", createProjectIn{
		planProjectIn: planProjectIn{
			Template:  "hooked",
			Variables: map[string]string{"name": "acme-site"},
			TargetDir: "out",
		},
		AllowHooks: true,
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}
	if len(s.hooks.calls) != 1 || s.hooks.calls[0].Run != "git init" {
		t.Errorf("hook calls = %+v, want exactly [git init]", s.hooks.calls)
	}
	if s.hooks.calls[0].Dir != "out" {
		t.Errorf("hook ran with Dir = %q, want %q", s.hooks.calls[0].Dir, "out")
	}
}

func TestCreateProject_ToggleOff(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "create_project", createProjectIn{
		planProjectIn: planProjectIn{
			Template:  "withdocker",
			Variables: map[string]string{"name": "acme-site"},
			Toggles:   map[string]bool{"docker": false},
			TargetDir: "out",
		},
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}
	if _, err := s.fs.ReadFile("out/Dockerfile"); err == nil {
		t.Error("Dockerfile should not exist: docker was toggled off")
	}
}

func TestCreateProject_ToggleOn(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "create_project", createProjectIn{
		planProjectIn: planProjectIn{
			Template:  "withdocker",
			Variables: map[string]string{"name": "acme-site"},
			Toggles:   map[string]bool{"docker": true},
			TargetDir: "out",
		},
	})
	if res.IsError {
		t.Fatalf("create_project returned an error: %s", s.text(t, res))
	}
	content, err := s.fs.ReadFile("out/Dockerfile")
	if err != nil || string(content) != "FROM scratch" {
		t.Errorf("Dockerfile = %q, %v, want it present", content, err)
	}
}

func TestDescribeTemplate_IncludesOptionalBlocks(t *testing.T) {
	s := newTestServer(t)
	res := s.callTool(t, "describe_template", describeTemplateIn{Name: "withdocker"})
	if res.IsError {
		t.Fatalf("describe_template returned an error: %s", s.text(t, res))
	}
	got := s.text(t, res)
	for _, want := range []string{`"name": "docker"`, `"paths"`, `"Dockerfile"`} {
		if !strings.Contains(got, want) {
			t.Errorf("describe_template = %s, want it to contain %q", got, want)
		}
	}
}
