// Package mcp implements "snap mcp": a Model Context Protocol server over
// stdio, exposing the same core use cases as the CLI to an agent. It is a
// thin adapter — no business logic lives here, and it inherits the exact
// same safety guarantees (path-escape rejection in internal/core/plan,
// non-empty-target refusal in internal/executor) that internal/adapters/cli
// gets from calling the same use cases.
package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeOne45/snap/internal/ports"
)

// handlers holds the dependencies every tool handler needs.
type handlers struct {
	fs         ports.FS
	store      ports.Store
	hookRunner ports.HookRunner
}

// NewServer builds the snap MCP server: list_templates, describe_template,
// plan_project, create_project, capture_template, save_template; a
// template:// resource template for each template's snap.toml and README;
// and the new-project prompt. create_project only runs a template's hooks,
// via hookRunner, when called with allow_hooks: true. Production code
// passes a real hookrunner.Exec; tests can pass a fake.
func NewServer(fs ports.FS, store ports.Store, hookRunner ports.HookRunner, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "snap", Version: version}, nil)
	h := &handlers{fs: fs, store: store, hookRunner: hookRunner}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_templates",
		Description: "List available templates, optionally filtered by a substring of their name or description. No side effects.",
	}, h.listTemplates)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "describe_template",
		Description: "Describe one template: its declared variables and its raw (unrendered) file tree. No side effects.",
	}, h.describeTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "plan_project",
		Description: "Resolve variables for a template and build the write plan for target_dir, without writing anything. Returns the same JSON shape as `snap new --json`.",
	}, h.planProject)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_project",
		Description: "Generate a new project from a template into target_dir. Refuses if target_dir already exists and is not empty. Writes files.",
	}, h.createProject)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "capture_template",
		Description: "Scan a project and propose placeholder candidates, scored, without writing anything or saving a template.",
	}, h.captureTemplate)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_template",
		Description: "Re-scan source_dir, accept only the candidates listed in `accepted` under the given variable names, and save the resulting template. Writes files.",
	}, h.saveTemplate)

	addResources(server, h)
	addPrompts(server)

	return server
}
