package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func addPrompts(server *sdk.Server) {
	server.AddPrompt(&sdk.Prompt{
		Name:        "new-project",
		Description: "Guides creating a new project from a snap template: describe it, ask the user for any missing variables, plan, review, then create.",
		Arguments: []*sdk.PromptArgument{
			{Name: "template", Description: "The template's name, if already known", Required: false},
		},
	}, getNewProjectPrompt)
}

func getNewProjectPrompt(_ context.Context, req *sdk.GetPromptRequest) (*sdk.GetPromptResult, error) {
	templateName := req.Params.Arguments["template"]

	var text string
	if templateName == "" {
		text = "Call list_templates to see what's available, then ask the user which one to use. " +
			"Once chosen, follow the same steps as if the template name were already known."
	} else {
		text = fmt.Sprintf(
			"1. Call describe_template with name=%q to see its declared variables.\n"+
				"2. For each variable without a default, ask the user for a value.\n"+
				"3. Call plan_project with the template, the gathered variables, and a target_dir; show the plan to the user.\n"+
				"4. Once the user confirms, call create_project with the same arguments.\n"+
				"create_project refuses if target_dir already exists and is not empty — ask the user for a different target_dir in that case.",
			templateName,
		)
	}

	return &sdk.GetPromptResult{
		Description: "Create a new project from a snap template.",
		Messages: []*sdk.PromptMessage{
			{Role: "user", Content: &sdk.TextContent{Text: text}},
		},
	}, nil
}
