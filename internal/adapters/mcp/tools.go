package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/CodeOne45/snap/internal/adapters/gitcache"
	"github.com/CodeOne45/snap/internal/adapters/usagelog"
	"github.com/CodeOne45/snap/internal/core/capture"
	"github.com/CodeOne45/snap/internal/core/plan"
	"github.com/CodeOne45/snap/internal/core/remote"
	"github.com/CodeOne45/snap/internal/core/tokens"
	"github.com/CodeOne45/snap/internal/core/usecase"
	"github.com/CodeOne45/snap/internal/executor"
	"github.com/CodeOne45/snap/internal/ports"
)

// planProjectOut is plan_project's wire shape (M19): the Plan plus stats
// a bare Plan doesn't carry.
type planProjectOut struct {
	Plan  plan.Plan  `json:"plan"`
	Stats plan.Stats `json:"stats"`
}

// textResult wraps JSON bytes as a tool result. Every tool here returns its
// data as JSON text rather than a typed StructuredContent: several result
// shapes (Plan, Candidate) don't reflect cleanly into a JSON Schema (custom
// marshalers, an Op interface field), and returning the same bytes
// `--json` on the CLI produces keeps one single source of truth for the
// wire format (docs/cli-json-contract.md).
func textResult(data []byte) (*sdk.CallToolResult, any, error) {
	return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(data)}}}, nil, nil
}

func toolError(err error) (*sdk.CallToolResult, any, error) {
	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}},
		IsError: true,
	}, nil, nil
}

// --- list_templates ---

type listTemplatesIn struct {
	Tag string `json:"tag,omitempty" jsonschema:"optional substring filter on a template's name or description"`
}

func (h *handlers) listTemplates(_ context.Context, _ *sdk.CallToolRequest, in listTemplatesIn) (*sdk.CallToolResult, any, error) {
	manifests, err := usecase.List(h.store, h.fs)
	if err != nil {
		return toolError(err)
	}
	if in.Tag != "" {
		filtered := manifests[:0]
		for _, m := range manifests {
			if strings.Contains(m.Name, in.Tag) || strings.Contains(m.Description, in.Tag) {
				filtered = append(filtered, m)
			}
		}
		manifests = filtered
	}
	data, err := jsonIndent(manifests)
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

// --- describe_template ---

type describeTemplateIn struct {
	Name string `json:"name" jsonschema:"the template's name"`
}

func (h *handlers) describeTemplate(_ context.Context, _ *sdk.CallToolRequest, in describeTemplateIn) (*sdk.CallToolResult, any, error) {
	result, err := usecase.Show(h.store, h.fs, in.Name)
	if err != nil {
		return toolError(err)
	}
	data, err := jsonIndent(result)
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

// --- plan_project / create_project ---

type planProjectIn struct {
	Template  string            `json:"template" jsonschema:"the template's name"`
	Variables map[string]string `json:"variables,omitempty" jsonschema:"variable name -> value"`
	Toggles   map[string]bool   `json:"toggles,omitempty" jsonschema:"[[optional]] block name -> on/off; a block not listed here uses its declared default"`
	TargetDir string            `json:"target_dir" jsonschema:"where the project would be generated"`
}

func (h *handlers) buildPlan(in planProjectIn) (plan.Plan, error) {
	store, source, err := h.resolveTemplateStore(in.Template)
	if err != nil {
		return plan.Plan{}, err
	}
	return usecase.New(usecase.NewParams{
		Store:       store,
		FS:          h.fs,
		Template:    in.Template,
		TargetDir:   in.TargetDir,
		Vars:        in.Variables,
		Toggles:     in.Toggles,
		Prompter:    nil, // MCP never prompts: a missing required variable is an error
		GeneratedAt: time.Now().UTC(),
		Source:      source,
	})
}

// resolveTemplateStore checks whether templateName is a remote reference
// (gh:user/repo, a git URL, optionally @pinned). If so, it ensures the
// pinned (or latest) commit is cached locally, returning a store scoped to
// that one directory and the "<url>@<resolved commit>" string for
// .snap/origin.toml's source field, instead of h.store (left untouched —
// handlers is shared across concurrent calls, so this never mutates it).
// Returns h.store and an empty source unchanged for a plain local template
// name.
func (h *handlers) resolveTemplateStore(templateName string) (ports.Store, string, error) {
	ref, ok := remote.ParseRef(templateName)
	if !ok {
		return h.store, "", nil
	}
	cacheRoot := path.Join(path.Dir(h.store.Root()), "cache")
	dir, commit, err := gitcache.New(cacheRoot).Resolve(ref)
	if err != nil {
		return nil, "", fmt.Errorf("resolve %s: %w", templateName, err)
	}
	return gitcache.SingleTemplateStore{Dir: dir}, ref.URL + "@" + commit, nil
}

func (h *handlers) planProject(_ context.Context, _ *sdk.CallToolRequest, in planProjectIn) (*sdk.CallToolResult, any, error) {
	p, err := h.buildPlan(in)
	if err != nil {
		return toolError(err)
	}
	data, err := jsonIndent(planProjectOut{Plan: p, Stats: plan.ComputeStats(p, tokens.Default)})
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

type createProjectIn struct {
	planProjectIn
	// AllowHooks must be true for any post-generate hooks the template
	// declares to actually run. Without it, hooks are silently stripped
	// from the plan before applying — the rest of the plan still applies.
	// There is no persisted "remembered approval" for MCP, unlike the CLI:
	// every call states its own intent.
	AllowHooks bool `json:"allow_hooks,omitempty" jsonschema:"must be true to run any post-generate hooks the template declares; otherwise hooks are skipped"`
}

type createProjectOut struct {
	TargetDir    string     `json:"target_dir"`
	FilesWritten int        `json:"files_written"`
	DirsCreated  int        `json:"dirs_created"`
	HooksRun     int        `json:"hooks_run,omitempty"`
	Stats        plan.Stats `json:"stats"`
}

func (h *handlers) createProject(_ context.Context, _ *sdk.CallToolRequest, in createProjectIn) (*sdk.CallToolResult, any, error) {
	p, err := h.buildPlan(in.planProjectIn)
	if err != nil {
		return toolError(err)
	}

	hooksRun := 0
	ex := executor.New(h.fs)
	if in.AllowHooks && len(p.Hooks()) > 0 {
		ex = executor.NewWithHooks(h.fs, h.hookRunner)
		hooksRun = len(p.Hooks())
	} else {
		p = p.WithoutHooks()
	}
	if err := ex.Apply(p); err != nil {
		return toolError(err)
	}

	st := plan.ComputeStats(p, tokens.Default)
	out := createProjectOut{TargetDir: p.TargetDir, HooksRun: hooksRun, Stats: st}
	for _, op := range p.Ops {
		switch op.(type) {
		case plan.WriteFileOp:
			out.FilesWritten++
		case plan.CreateDirOp:
			out.DirsCreated++
		}
	}
	// Usage logging is best-effort telemetry, never a reason to fail a
	// generation that already succeeded.
	_ = usagelog.Append(usagelog.Entry{Template: in.Template, Stats: st, Time: time.Now().UTC(), Source: "mcp"})

	data, err := jsonIndent(out)
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

// --- capture_template ---

type captureTemplateIn struct {
	SourceDir string   `json:"source_dir" jsonschema:"the project directory to scan"`
	Name      string   `json:"name,omitempty" jsonschema:"the intended template name (for display only — save_template is what actually saves it)"`
	Exclude   []string `json:"exclude,omitempty" jsonschema:"additional gitignore-style exclude patterns"`
}

type candidateDTO struct {
	Key         string   `json:"key"`
	Words       []string `json:"words"`
	RawForms    []string `json:"raw_forms"`
	Sources     []string `json:"sources"`
	FileCount   int      `json:"file_count"`
	Occurrences int      `json:"occurrences"`
	Score       int      `json:"score"`
	DefaultName string   `json:"default_name"`
}

type captureTemplateOut struct {
	Ignored    []string       `json:"ignored,omitempty"`
	Candidates []candidateDTO `json:"candidates"`
}

func (h *handlers) scanCandidates(sourceDir string, exclude []string) (capture.Result, []capture.Candidate, error) {
	result, err := capture.Scan(h.fs, sourceDir, capture.Options{ExtraExcludes: exclude})
	if err != nil {
		return capture.Result{}, nil, err
	}
	candidates, err := capture.BuildCandidates(h.fs, sourceDir, result)
	if err != nil {
		return capture.Result{}, nil, err
	}
	return result, candidates, nil
}

func (h *handlers) captureTemplate(_ context.Context, _ *sdk.CallToolRequest, in captureTemplateIn) (*sdk.CallToolResult, any, error) {
	result, candidates, err := h.scanCandidates(in.SourceDir, in.Exclude)
	if err != nil {
		return toolError(err)
	}

	out := captureTemplateOut{Ignored: result.Ignored, Candidates: make([]candidateDTO, len(candidates))}
	for i, c := range candidates {
		out.Candidates[i] = candidateDTO{
			Key:         c.Key(),
			Words:       c.Words,
			RawForms:    c.RawForms,
			Sources:     c.Sources,
			FileCount:   c.FileCount,
			Occurrences: c.PathHits + c.ContentHits,
			Score:       c.Score,
			DefaultName: usecase.DefaultVariableName(c),
		}
	}
	data, err := jsonIndent(out)
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

// --- save_template ---

type saveTemplateIn struct {
	SourceDir string            `json:"source_dir" jsonschema:"the project directory to scan (same as capture_template)"`
	Name      string            `json:"name" jsonschema:"the template name to save as"`
	Accepted  map[string]string `json:"accepted" jsonschema:"candidate key (from capture_template's output) -> chosen variable name; any candidate not listed here is rejected"`
	Exclude   []string          `json:"exclude,omitempty" jsonschema:"additional gitignore-style exclude patterns — must match capture_template's, if any were used"`
}

type saveTemplateOut struct {
	Template     string   `json:"template"`
	Variables    []string `json:"variables"`
	FilesWritten int      `json:"files_written"`
}

func (h *handlers) saveTemplate(_ context.Context, _ *sdk.CallToolRequest, in saveTemplateIn) (*sdk.CallToolResult, any, error) {
	targetDir := path.Join(h.store.Root(), in.Name, usecase.InitialVersion)

	accept := func(c capture.Candidate, defaultName string) (bool, string, error) {
		name, ok := in.Accepted[c.Key()]
		if !ok {
			return false, "", nil
		}
		if name == "" {
			name = defaultName
		}
		return true, name, nil
	}

	res, err := usecase.Capture(usecase.CaptureParams{
		FS:        h.fs,
		SourceDir: in.SourceDir,
		TargetDir: targetDir,
		AsName:    in.Name,
		Options:   capture.Options{ExtraExcludes: in.Exclude},
		Accept:    accept,
	})
	if err != nil {
		return toolError(err)
	}
	if err := executor.New(h.fs).Apply(res.Plan); err != nil {
		return toolError(err)
	}

	out := saveTemplateOut{Template: in.Name}
	for _, c := range res.Candidates {
		out.Variables = append(out.Variables, in.Accepted[c.Key()])
	}
	sort.Strings(out.Variables)
	for _, op := range res.Plan.Ops {
		if _, ok := op.(plan.WriteFileOp); ok {
			out.FilesWritten++
		}
	}

	data, err := jsonIndent(out)
	if err != nil {
		return toolError(err)
	}
	return textResult(data)
}

func jsonIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
