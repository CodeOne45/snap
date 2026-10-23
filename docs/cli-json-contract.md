# CLI contract (M7)

This is the stable contract the MCP server (M8) and any script wrapping
`snap` can rely on. A test pins each JSON shape exactly
(`internal/core/plan/json_test.go`'s `TestJSON_SchemaStability`,
`internal/adapters/cli/list_test.go`'s `TestListCmd_JSONSchemaStability`,
`internal/adapters/cli/show_test.go`'s `TestShowCmd_JSONSchemaStability`) —
a deliberate format change must update both the test and this file in the
same commit.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | User error: bad arguments/flags, an unknown or invalid template, a validation failure, a missing required variable, a target directory that already exists and isn't empty. Also returned by `snap update` when the merge completes with one or more unresolved conflicts — those files are still written to disk with conflict markers; this is "needs manual resolution", not "nothing happened", mirroring `git merge`'s own exit convention. Also returned by `snap test` when its `[test]` check command exits non-zero (the check's own exit code is not passed through verbatim — `snap test` always normalizes to 1, same as every other user-facing failure). Also returned by `snap status` when any checked directory is reported behind or couldn't be read. |
| 2 | Internal error: snap's own environment lookup or I/O failed unexpectedly (e.g. the config directory couldn't be determined). Not fixable by changing command-line input. |

(`internal/adapters/cli/exitcode.go`.)

## `--json` output shapes

Every JSON-producing command indents with two spaces and ends the stream
with a trailing newline (`fmt.Fprintln`).

### `snap capture` / `snap template bump` with `--json`: a Plan

```json
{
  "target_dir": "myproj",
  "ops": [
    { "kind": "create_dir", "path": "src" },
    { "kind": "write_file", "path": "src/main.go", "content": "<base64>", "mode": 420 },
    { "kind": "run_hook", "run": "git init" }
  ]
}
```

- `ops` is a **tagged union**: every element has a `"kind"` discriminator (`"create_dir"`, `"write_file"`, `"run_hook"` as of M9, or `"modify_file"` as of M20 — added purely additively, with no change to the existing kinds' shape).
- `write_file`'s `content` is standard base64 (Go's `encoding/json` default for `[]byte`); `mode` is a plain decimal Unix file-mode integer (`420` = `0o644`).
- `run_hook`'s `run` is the literal shell command, never rendered through the placeholder engine.
- `modify_file` (M20, recipes): `{ "kind": "modify_file", "path": "package.json", "expected_sha256": "<hex>", "content": "<base64>", "mode": 420 }`. `expected_sha256` is the hex-encoded SHA-256 of `path`'s content *at planning time*; `executor.ApplyRecipe` refuses the whole Plan (nothing written) if the file's actual current content no longer matches it — see **Recipes and `snap add`** below.
- Fields are only present when non-empty (`omitempty`), except `kind` and `path`/`run` (whichever the kind uses), which are always present.

### `snap new` with `--json` (M19: wraps the Plan with stats)

```json
{
  "plan": { "target_dir": "myproj", "ops": [ "... same shape as above ..." ] },
  "stats": { "files": 31, "text_files": 30, "bytes": 43008, "approx_tokens": 11800 }
}
```

`stats` is **always** present (unlike `capture`/`template bump`, which stay a bare Plan — there's no byte-pinned schema test for `new --json`'s shape, so this evolved deliberately for M19; see `internal/core/plan/stats.go`'s `Stats`/`ComputeStats`). `approx_tokens` is always an *approximation* — there's no universal offline tokenizer for every model — a binary file (`render.IsBinary`) counts toward `files`/`bytes` but never `text_files`/`approx_tokens`. MCP's `plan_project`/`create_project` carry the same `stats` field (see **MCP** below).
- A Plan may contain hooks whether or not they'll actually run — `plan_project`/`--dry-run`/`--json` always show the full plan as the template declares it. Whether hooks run is a separate, later decision: CLI confirmation (remembered per template+version) or MCP's `allow_hooks: true` on `create_project`. A Plan with hooks stripped (declined/not allowed) is `plan.Plan.WithoutHooks()`.

### `snap list` with `--json`: an array of manifests

```json
[
  {
    "name": "api-starter",
    "version": "1.0.0",
    "description": "test fixture",
    "variables": [
      { "name": "name", "type": "string", "pattern": "^[a-z][a-z0-9-]*$", "has_default": false },
      { "name": "author", "prompt": "Author name", "type": "string", "default": "Anonymous", "has_default": true }
    ]
  }
]
```

- `variables[].type` is `"string"` or `"int"`.
- `pattern` is only present for a string variable with a `validate` regex declared in `snap.toml`.
- `has_default` is always present (`true`/`false`); `default` is only present when `has_default` is `true`, and is always a string (even for an `int` variable — this is the declared default's literal text, not a resolved value; resolved values in `.snap/origin.toml` are typed, see below).

### `snap show <template>` with `--json`: one manifest plus its file tree

```json
{
  "manifest": { "...": "same shape as one element of snap list --json" },
  "files": ["README.md", "src/{{name|pascal}}.go"],
  "dirs": ["empty-dir"]
}
```

- `files`/`dirs` are the template's own, unrendered paths — placeholders are shown literally, not substituted.
- `dirs` is omitted if there are no tracked empty directories.

## `--toggles` / `--with` / `--without` ([[optional]] blocks, M10)

`plan_project`/`create_project` take a `toggles` field: `{"block name": true|false}`. A block not listed resolves to its `snap.toml`-declared default (CLI: `--with <name>` / `--without <name>`, repeatable, each overriding one block; the same name in both is an error). An unknown block name (not declared by the template) is always an error. `describe_template`/`snap show` list every declared `[[optional]]` block (name, prompt, default, paths) alongside variables.

## `.snap/origin.toml` (not JSON, but part of the same stability contract)

Written by `snap new` into every generated project. TOML, not JSON — see
`internal/core/provenance/origin.go`. Variable values are **typed**
(`port = 8080`, not `port = "8080"`), and `generated_at` is a native TOML
datetime (unquoted), reproducible across runs when `GeneratedAt` is fixed
(tests do this; the CLI uses `time.Now().UTC()`).

```toml
template = "api-starter"
version = "1.0.0"
source = "local"
generated_at = 2026-01-01T00:00:00Z

[variables]
  name = "payments-api"
  port = 8080

[toggles]
  docker = true
```

`[toggles]` records every declared `[[optional]]` block's *resolved* state, not just the ones explicitly overridden — v2's `snap update` needs the full picture to reproduce what was generated. Omitted entirely for a template with no optional blocks.

A variable's value here is never passed through the placeholder engine —
`--var description="uses {{handlebars}}"` is stored as that literal string.

`source` is `"local"` for a template resolved from the local template store. For a remote reference (`gh:user/repo[@pin]`, any git URL, optionally `@pin`), it is instead `"<url>@<resolved commit>"` — the exact commit used, even when the reference itself was unpinned — e.g. `source = "https://github.com/acme/web-starter.git@a1b2c3d4e5f6..."`.

`.snap/origin.toml` is also read back, not just written: `provenance.ParseOrigin` is `Origin.TOML`'s inverse (M15), used by `usecase.RegenerateBase` to reconstruct — in memory, byte-for-byte — exactly the project a `template`/`version`/`variables`/`toggles` combination originally produced, even after the template has since been bumped to a newer version (M14's `name@version` pinning is how it finds the old one). This is the building block M16's `snap update` diffs against; neither has a CLI/MCP surface of its own yet.

## Remote templates

`template` (CLI: the `<template>` argument; MCP: `plan_project`/`create_project`'s `template` field) accepts a remote git reference in place of a local template name: `gh:user/repo[@pin]`, or any git URL (`https://`, `http://`, `ssh://`, or SCP-style `user@host:path`), optionally followed by `@pin` (a tag or commit — not supported after SCP-style syntax, since that `@` already separates user from host). Unpinned, it resolves the remote's current default branch on every call; pinned, it resolves once and is then reproducible fully offline from `internal/adapters/gitcache`'s local cache (one subdirectory per unique URL, under the user's config dir, separate from the local template store — a remote template is never saved into it). A remote reference is never a browsable/listable template: `list_templates`/`snap list` never include it.

## Template versioning (M14)

`[template].version` in `snap.toml` must parse as strict `MAJOR.MINOR.PATCH` (no pre-release/build metadata, no `v` prefix) — `ParseManifest` rejects anything else, including an empty/missing version. `snap capture` always writes a fresh template at `0.1.0` (`usecase.InitialVersion`).

The local template store keeps one directory per version: `<store root>/<name>/<version>/snap.toml`, instead of MVP/v1's flat `<store root>/<name>/snap.toml`. `ports.Store` gained two methods to go with it:

- `Versions(name) ([]string, error)` — every version, sorted ascending (oldest first).
- `PathVersion(name, version) (string, error)` — that exact version's directory.

`Path(name)` (used by every existing caller) keeps resolving to the *latest* (highest semver) version transparently — no caller needed to change for this. A remote reference's `SingleTemplateStore` implements both new methods by ignoring their arguments and always returning its one resolved directory (versioning for remote templates happens at the git level, via M11's `@tag`/`@commit` pins, not through this interface).

**`snap template bump <name> <major|minor|patch>`** (CLI only, no MCP tool) copies the latest version's files verbatim — no placeholder rendering, unlike `new` — into a new version directory, with only the manifest's `[template].version` field rewritten. The old version is left untouched (needed so M15's base regeneration can still find it). Supports `--dry-run`/`--json` like every other command; the plan it builds targets the new version's directory, so applying it twice is refused by the same non-empty-target safety rule as `new`/`capture`.

**`name@version` pinning**, mirroring remote templates' `@tag`/`@commit` syntax, works on `snap new`/`snap show` and MCP's `plan_project`/`create_project`/`describe_template`: a bare name resolves to the latest version; `name@version` resolves to exactly that version (an unknown version is an error). A template whose own name happens to contain `@` is tried as a literal name first, so this is backward compatible.

## Base regeneration (M15, core-only — no CLI/MCP surface of its own)

`provenance.ParseOrigin` is `Origin.TOML`'s inverse. `usecase.RegenerateBase` uses it plus M14's `name@version` pinning to reconstruct — in memory, byte-for-byte — exactly the Plan a project was originally generated from, pinned at the *recorded* version even if the template has since moved on. This is the building block `snap update` (M16) is built on.

## `snap update <target-dir>` (M16)

Three-way merges `target-dir`'s current files against its recorded old base (M15, pinned at the version `.snap/origin.toml` names) and the template's current latest version, both resolved with the same variables/toggles the project already recorded. Post-generate hooks are never re-run by an update (stripped from the "theirs" plan before merging).

Per file: only the template changed → take theirs; only the user changed → keep ours; both changed identically → either; both changed differently → a **line-based three-way merge** (`internal/core/merge`, diff3-style, built on `github.com/sergi/go-diff`), conflict markers written only around the lines that actually conflict:

```
<<<<<<< ours
...current project's lines...
=======
...template's new lines...
>>>>>>> theirs
```

A binary file (`render.IsBinary`) that conflicts is never run through the text merge — ours is kept unchanged and the path is still listed as a conflict. A file present in the old base and the latest template but missing locally (the user deleted it) stays deleted — flagged as a conflict only if the template also changed that file's content (so the deletion is silently respected when nothing else changed). A rename is detected by an exact content match between a path that disappeared and one that appeared between the old base and the latest template; a clean rename (the user never touched the old path) moves the file, while an edited old path keeps **both** the user's edit at the old path and the template's new path, flagged as a conflict.

`.snap/origin.toml` is rewritten to the latest version **only** when zero conflicts remain; otherwise it's carried forward byte-for-byte unchanged, so a subsequent `snap update` (after the conflicts are resolved by hand) starts from the same old base again rather than silently losing track of what's still pending.

`--json`'s shape adds the merge outcome a bare Plan can't express:

```json
{
  "plan": { "target_dir": "myproj", "ops": [ { "kind": "write_file", "...": "..." } ] },
  "conflicts": ["README.md"],
  "deleted": ["old-file.txt"],
  "old_version": "1.0.0",
  "new_version": "1.1.0"
}
```

`conflicts`/`deleted` are omitted (not `null`/`[]`) when empty. Exit code 1 (not 0) whenever `conflicts` is non-empty, in every mode (`--dry-run`, `--json`, and the default apply) — see **Exit codes** above.

Unlike `new`/`capture`, `update` targets an already-populated directory on purpose — it never refuses a non-empty target. Applying is `executor.ApplyUpdate`, not `Apply`: it builds the complete merged tree in a temp directory, then does a two-step safe swap (the existing directory is renamed aside to a backup, the temp directory renamed into its place, and the backup deleted only once that second rename succeeds — a failed swap restores the backup), rather than `Apply`'s single-rename-into-empty-space model. This is weaker than `Apply`'s single-rename atomicity but is the only way to replace an already-populated directory's contents at all.

## `snap test <template>` (M17)

A template opts in with an optional `[test]` block in `snap.toml`:

```toml
[test]
check = "go build ./... && go test ./..."

[test.vars]
name = "sample-project"
port = "8080"
```

`check` is required if `[test]` is present at all (an empty or missing `check` is a parse error); `[test.vars]` is the same shape as `--var` (string-valued, even for an `int` variable — `usecase.New`'s own validation still applies, so `port = "not-a-number"` fails exactly like `--var port=not-a-number` would). A template with no `[test]` block can't be tested: `usecase.Test` returns an immediate error naming the template, not a silent no-op.

`snap test` generates the template (using `[test.vars]`, same codepath as `new`) into a real temp directory it creates and always removes afterward, then runs `check` there via `ports.HookRunner` — the exact same mechanism `[[hooks.post]]` already uses, just without needing confirmation (there's no persistent project for an unconfirmed hook to leak into). The check command's own exit code is surfaced as `snap test`'s exit code (1 on failure, via a plain error — see **Exit codes**), so it's usable as a CI step; its captured output is printed either way.

`--json`'s shape:

```json
{
  "plan": { "target_dir": "/tmp/snap-test-.../project", "ops": [ "..." ] },
  "check": "go build ./... && go test ./..."
}
```

`--dry-run` and `--json` both only print; neither generates anything nor runs `check`.

## `snap status <dir>...` (M18)

Reads each given directory's `.snap/origin.toml` and compares its recorded version against the template's current latest local version (`ports.Store.Versions`). Takes one or more directories; each is checked independently — one that isn't a generated project, or whose `.snap/origin.toml` can't be read or parse, never stops the others from being checked, it's just reported with its own error instead.

A remote-sourced project (`source` isn't `"local"`) is reported with its recorded version and source, but no drift verdict: `latest_version` is empty and `up_to_date` is `false` (meaning "unknown", not "confirmed behind") — determining a remote template's actual latest would need network access, out of scope for a *local* drift report.

`--json`'s shape is an array, one entry per directory, in the order given:

```json
[
  { "dir": "/code/northwind-site", "template": "client-site", "current_version": "1.0.0", "latest_version": "1.2.0", "up_to_date": false, "source": "local" },
  { "dir": "/code/other-site", "template": "client-site", "current_version": "1.2.0", "latest_version": "1.2.0", "up_to_date": true, "source": "local" },
  { "dir": "/code/not-a-project", "up_to_date": false, "error": "usecase: status: .../not-a-project: read .snap/origin.toml: ..." }
]
```

Fields are omitted (not `""`/`false`) only via Go's `omitempty` on strings — `up_to_date` is always present, since `false` is itself meaningful (behind, unknown, or errored all read as "not confirmed up to date"). Exit code 1 (not 0) whenever any directory was reported behind (local source only) or had an error — same non-zero-means-"needs attention" convention as `snap update`.

## Token receipts and `snap stats` (M19, `PLAN-v3.md`)

Every Plan's `Stats` (`internal/core/plan/stats.go`) is a pure, deterministic function of its `WriteFileOp`s: `files`/`bytes` count every one (binary or text, per `render.IsBinary`); `text_files`/`approx_tokens` only count non-binary ones. `approx_tokens` comes from `internal/core/tokens.Default` (`HeuristicCounter`): a deterministic word/punctuation-boundary approximation, no external tokenizer or vocabulary — there is no universal offline tokenizer for every model (and Claude's exact one isn't available offline at all), so this is always displayed with `~`, never as an exact count.

`snap new`'s default (apply) path prints one receipt line on success:

```
✓ 31 files · 42.0 KB · ~11.8k tokens written by Snap instead of the agent
```

Every successful `snap new` (CLI) and `create_project` (MCP) call also appends one JSON line to a local, private usage log at `$XDG_STATE_HOME/snap/usage.jsonl` (`~/.local/state/snap/usage.jsonl` if `XDG_STATE_HOME` is unset) — `internal/adapters/usagelog`. Never sent anywhere; set `SNAP_NO_USAGE_LOG` to any non-empty value to disable it outright (no file or directory is even created). A logging failure is never a reason to fail a generation that already succeeded. Each line:

```json
{"template":"api-starter","stats":{"files":31,"text_files":30,"bytes":43008,"approx_tokens":11800},"time":"2026-11-02T10:00:00Z","source":"cli"}
```

`source` is `"cli"` or `"mcp"`. `--dry-run` and `--json` never log (nothing was actually written).

`snap stats [--since <duration>] [--json]` reads the log back and reports totals, by template/recipe and by source. `--since` accepts anything `time.ParseDuration` does, plus a `d` (days) suffix snap adds (`30d`, `24h`, `90m`). `--json`:

```json
{
  "total_files": 31,
  "total_bytes": 43008,
  "total_approx_tokens": 11800,
  "by_template": { "api-starter": { "files": 31, "text_files": 30, "bytes": 43008, "approx_tokens": 11800 } },
  "by_source": { "cli": { "files": 31, "text_files": 30, "bytes": 43008, "approx_tokens": 11800 } }
}
```

MCP's `plan_project`/`create_project` responses each carry a `stats` field in this same `plan.Stats` shape (see **`--json` output shapes** above).

## Recipes and `snap add` (M20, `PLAN-v3.md`)

A **template** creates a new project; a **recipe** applies a reusable change to an *existing* one: new files (rendered with the same placeholder engine as a template), dependency additions, env entries, and post-apply hooks. A recipe is a folder, resolved through its own `ports.Store` (the CLI's `RecipeStore`, a `local.Store` rooted at `<config dir>/snap/recipes`, sibling to the template store):

```text
sentry/
├── recipe.toml
└── files/                 # new files, rendered with the same placeholder engine
    └── src/lib/sentry.ts
```

```toml
[recipe]
name = "sentry"
version = "1.0.0"
description = "Error tracking with Sentry"

[applies_to]
files = ["package.json"]   # recipe applies only if every listed file already exists

[[variables]]
name = "service"
prompt = "Service name"
default_from = "project.name"   # resolved from the target's own .snap/origin.toml

[[dependencies]]
ecosystem = "npm"                # npm | go | pip | cargo
name = "@sentry/node"
version = "^8.0.0"
dev = false

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""

[[hooks.post]]
run = "npm install"
```

```bash
snap add sentry                    # apply to the current directory
snap add sentry some/project        # or an explicit target
snap add sentry --dry-run
snap add sentry --json
snap add sentry --var service=payments
snap recipes                        # list recipes (alias: snap list --recipes)
```

**Dependency editors** (`internal/core/deps`, pure, format-preserving — never run a package manager, installs happen only through `[[hooks.post]]`, gated the same as a template's hooks): `npm` edits `package.json`; `go` edits `go.mod` via `golang.org/x/mod/modfile`; `cargo` edits `Cargo.toml`'s `[dependencies]`/`[dev-dependencies]`; `pip` edits whichever of `pyproject.toml`'s `[project].dependencies` array or `requirements.txt` is present in the target (`pyproject.toml` preferred when both exist). `[[env]]` entries are added to the named file the same way, creating it if missing (unlike a dependency's manifest, which must already exist).

**All-or-nothing**: `usecase.Add` collects every conflict — `applies_to` not satisfied, a new file already present with different content, a dependency already present at a different version, an env key already present with a different value, or a dependency's manifest file missing entirely — and fails with the full list rather than applying part of the recipe. Nothing is written in that case (exit code 1, same convention as `snap update`'s unresolved-conflict case).

**Idempotent**: re-running `snap add` for a recipe whose files, dependencies, and env entries are already exactly present produces an empty Plan; the CLI prints `<recipe> is already applied to <dir>; nothing to do.` and exits 0 — no hook re-run, no `.snap/origin.toml` rewrite.

**Provenance**: a successful apply appends one `[[recipes]]` entry to the target's `.snap/origin.toml` (created fresh, with *only* this field, if the project wasn't generated by Snap at all):

```toml
[[recipes]]
name = "sentry"
version = "1.0.0"
source = "local"
applied_at = 2026-11-02T10:00:00Z
[recipes.variables]
  service = "payments"
```

**`snap add --json`** wraps the Plan with stats, identically to `snap new --json`: `{ "plan": {...}, "stats": {...} }`. `plan.ComputeStats` counts both `write_file` and `modify_file` ops toward `files`/`bytes`/`text_files`/`approx_tokens` (a recipe's dependency/env edits are `modify_file`, not `write_file`, but they write real content just the same). A successful (non-empty-plan) `snap add` logs to the usage log exactly like `snap new`, with `"recipe"` set instead of `"template"`.

**Applying**: `executor.ApplyRecipe` (not `Apply`/`ApplyUpdate`) writes each op directly to its final path under the target directory — pre-flight only, not transactional: every `modify_file`'s `expected_sha256` and every `write_file`'s target (if it already exists) are checked against the target's current content *before anything is written*, so a stale Plan is refused in full. Once writing starts, a failure partway through is not rolled back — full transactional backup/restore for in-place changes is M22's "in-place transactional executor", not M20's.

## `--yes` / `--force` / `--dry-run`

- `--yes` (`-y`, on `new`, `capture`, `update`, and `add`): never prompts. On
  `new`, a variable with no `--var` and no declared default is an immediate
  error naming the variable, instead of blocking on stdin. On `capture`,
  every detected candidate is accepted under its default name. On `update`,
  a variable the template's latest version newly introduces (one
  `.snap/origin.toml` has no recorded value for) is an immediate error
  instead of prompting. On `add`, a recipe variable with no `--var` and no
  resolvable `default_from` is an immediate error; on all four, `--yes`
  also skips hook confirmation (hooks are stripped from the Plan, not run).
- `--force` (on both `new` and `capture`): removes an existing non-empty
  target directory before writing. Without it, a non-empty existing target
  is refused with exit code 1 and nothing is touched; an *empty* existing
  target directory is always fine, `--force` or not. `add` has no
  `--force`: it always targets an already-existing, already-populated
  directory on purpose.
- `--dry-run` (on `new`/`capture`/`add`): prints the plan (`plan.DryRun`'s
  human format) and writes nothing. Mutually exclusive in effect with
  `--json`; when both are passed, `--json` wins (checked first).
