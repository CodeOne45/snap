package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeOne45/snap/internal/adapters/osfs"
	"github.com/CodeOne45/snap/internal/adapters/store/local"
)

// TestGate2_AgentCreatesProjectEndToEnd is PLAN.md's M13 Gate 2: an agent
// (standing in for a Claude Code session) drives the real, shipped
// examples/templates/go-cli-starter template end to end purely through the
// MCP tool surface — list, describe, plan, create — with no manual fixes
// to the generated output. It runs against the real filesystem (not
// memfs): the example templates are the actual files this repo ships, read
// from their real on-disk location.
func TestGate2_AgentCreatesProjectEndToEnd(t *testing.T) {
	examplesDir, err := filepath.Abs("../../../examples/templates")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(examplesDir, "go-cli-starter", "1.0.0", "snap.toml")); err != nil {
		t.Fatalf("examples/templates/go-cli-starter not found at %s: %v", examplesDir, err)
	}

	fsys := osfs.New()
	store := local.New(fsys, filepath.ToSlash(examplesDir))
	server := NewServer(fsys, store, &fakeHookRunner{}, "test")

	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "gate2-agent"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	call := func(name string, args any) *sdk.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", name, err)
		}
		if res.IsError {
			t.Fatalf("CallTool(%s): tool returned an error: %s", name, textOf(t, res))
		}
		return res
	}

	// 1. Discover what's available, as an agent would before knowing the
	// template's name or shape.
	listRes := call("list_templates", listTemplatesIn{})
	var manifests []map[string]any
	if err := json.Unmarshal([]byte(textOf(t, listRes)), &manifests); err != nil {
		t.Fatalf("Unmarshal list_templates: %v", err)
	}
	found := false
	for _, m := range manifests {
		if m["name"] == "go-cli-starter" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list_templates = %v, want go-cli-starter among them", manifests)
	}

	// 2. Learn what variables it needs.
	call("describe_template", describeTemplateIn{Name: "go-cli-starter"})

	target := filepath.ToSlash(filepath.Join(t.TempDir(), "mycli"))
	params := planProjectIn{
		Template:  "go-cli-starter",
		Variables: map[string]string{"name": "mycli", "module": "github.com/gate2/mycli"},
		TargetDir: target,
	}

	// 3. Preview the plan without writing anything.
	call("plan_project", params)
	if _, err := os.Stat(target); err == nil {
		t.Fatal("plan_project must not write anything, but the target dir exists")
	}

	// 4. Generate for real.
	call("create_project", createProjectIn{planProjectIn: params})

	// Gate 2's bar is "no manual fixes": the generated files must already
	// be exactly right, not need hand-editing afterward.
	mainGo, err := os.ReadFile(filepath.Join(target, "cmd", "mycli", "main.go"))
	if err != nil {
		t.Fatalf("ReadFile(main.go): %v", err)
	}
	if want := "fmt.Println(\"mycli is running\")"; !strings.Contains(string(mainGo), want) {
		t.Errorf("main.go = %q, want it to contain %q", mainGo, want)
	}
	goMod, err := os.ReadFile(filepath.Join(target, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile(go.mod): %v", err)
	}
	if want := "module github.com/gate2/mycli"; !strings.Contains(string(goMod), want) {
		t.Errorf("go.mod = %q, want it to contain %q", goMod, want)
	}
	if _, err := os.Stat(filepath.Join(target, ".snap", "origin.toml")); err != nil {
		t.Errorf("origin.toml should have been written: %v", err)
	}
}

func textOf(t *testing.T, res *sdk.CallToolResult) string {
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
