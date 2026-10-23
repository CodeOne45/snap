package mcp

import (
	"context"
	"path"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeOne45/snap/internal/core/template"
)

const templateResourceURITemplate = "template://{name}/{file}"

// allowedResourceFiles are the only files exposed per template, per
// PLAN.md §6: "each template's snap.toml and README".
var allowedResourceFiles = map[string]string{
	template.ManifestFile: "application/toml",
	"README.md":           "text/markdown",
}

func addResources(server *sdk.Server, h *handlers) {
	server.AddResourceTemplate(&sdk.ResourceTemplate{
		Name:        "template-file",
		Description: "A template's snap.toml or README.md.",
		URITemplate: templateResourceURITemplate,
	}, h.readTemplateResource)
}

func (h *handlers) readTemplateResource(_ context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	name, file, ok := parseTemplateURI(req.Params.URI)
	if !ok {
		return nil, sdk.ResourceNotFoundError(req.Params.URI)
	}
	mimeType, allowed := allowedResourceFiles[file]
	if !allowed {
		return nil, sdk.ResourceNotFoundError(req.Params.URI)
	}

	root, err := h.store.Path(name)
	if err != nil {
		return nil, sdk.ResourceNotFoundError(req.Params.URI)
	}
	data, err := h.fs.ReadFile(path.Join(root, file))
	if err != nil {
		return nil, sdk.ResourceNotFoundError(req.Params.URI)
	}

	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      req.Params.URI,
			MIMEType: mimeType,
			Text:     string(data),
		}},
	}, nil
}

// parseTemplateURI parses "template://<name>/<file>", rejecting anything
// that doesn't match that exact two-segment shape (no "..", no extra
// segments) — the resource surface is read-only and deliberately narrow.
func parseTemplateURI(uri string) (name, file string, ok bool) {
	rest, found := strings.CutPrefix(uri, "template://")
	if !found {
		return "", "", false
	}
	segments := strings.Split(rest, "/")
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return "", "", false
	}
	for _, s := range segments {
		if s == ".." || s == "." {
			return "", "", false
		}
	}
	return segments[0], segments[1], true
}
