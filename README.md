# Snap

Capture your best project once. Every new one, yours or your coding agent's, starts from it.

[Full docs](https://codeone45.github.io/snap/) (built from `docs-site/`) · [Example templates](examples/templates/)

Snap scans an existing project, detects the identifiers that make it "that specific project" (its name, in every case form it appears in — kebab, snake, PascalCase, camelCase, UPPER_CASE, Title Case), turns them into a reusable template, and lets you (or an agent) generate new projects from it with one command.

## Why

The usual way to start a new project from an old one is `cp -r acme-site northwind-site`, then grep for "acme" and fix the matches by hand across a dozen case forms and file names. Something always gets missed. Snap replaces that with a capture/generate pipeline:

```
$ snap capture ./acme-site --as client-site --yes
Ignored: node_modules, .next, .git, .env
✓ Template "client-site" saved (2 variables)

$ snap new client-site northwind-site --var name=northwind-site
```

- **Capture, not authoring.** Point it at a real project; it detects placeholders itself. No templating config to write by hand.
- **Single static binary.** No Python or Node runtime required to use it.
- **Deterministic.** Same template version + same variable values → byte-identical output, every time.

## Install / build

Requires Go 1.27+.

```
make build        # builds ./snap with the version stamped in
make install       # or: go install to $GOBIN / $GOPATH/bin
```

## Features (MVP)

| Command | What it does |
|---|---|
| `snap capture <dir> --as <name>` | Scans a project, applies the ignore chain (`.gitignore` → built-in junk list → `.snapignore` → `--exclude`), excludes secrets unconditionally, detects placeholder candidates, and saves a template |
| `snap new <template> <target>` | Generates a new project from a template, resolving variables from `--var`, interactive prompts, or declared defaults |
| `snap list` | Lists available templates |
| `snap show <template>` | Shows a template's declared variables and file tree |
| `snap add <recipe> [target]` | Applies a recipe (new files, dependency additions, env entries, hooks) to an existing project — defaults to the current directory |
| `snap recipes` | Lists available recipes (alias: `snap list --recipes`) |
| `snap mcp` | Runs snap as a Model Context Protocol server over stdio, for coding agents |
| `--dry-run` | Preview the write plan for `new`/`capture` without touching disk |
| `--json` | Machine-readable output, on every command that produces one — the exact shape is a committed, tested contract: see `docs/cli-json-contract.md` |
| `--yes` / `-y` | Never prompt: `new` errors immediately on a variable with no value or default; `capture` accepts every detected candidate under its default name |
| `--force` | On `new`/`capture`: remove an existing non-empty target directory first. Without it, a non-empty target is refused and left untouched; an empty one is always fine |
| `--help`, `--version` | Standard on the root command and every subcommand |

Exit codes: `0` success, `1` user error (bad input — fix the command and retry), `2` internal error (snap's own environment/I-O failed). See `docs/cli-json-contract.md`.

### Placeholder detection

Seed candidates come from the project's folder name, `package.json`/`go.mod`/`Cargo.toml`/`pyproject.toml`, the git remote, and the README's first heading. Each candidate is split into words and re-cased into all 6 forms (`{{name}}`, `{{name|kebab}}`, `{{name|snake}}`, `{{name|pascal}}`, `{{name|camel}}`, `{{name|upper}}`, `{{name|title}}`), scored by how often and where they appear, then presented for you to accept, rename, or reject — interactively, or all at once with `--yes`.

Existing `{{...}}` syntax that isn't one of Snap's own tokens (a Go template field reference, Jinja, Helm, Mustache — common in files a real project already ships) is left untouched rather than treated as an error.

### Safety

- `.env*` (except `.env.example`), `*.pem`, `*.key`, and `id_rsa*` are never captured — not even if a project's own `.gitignore` tries to un-ignore them.
- Generation writes to a temp directory and atomically renames into place; a failure partway through never leaves a half-written project, and a target directory that already exists is refused rather than overwritten.
- Every generated project gets `.snap/origin.toml`, recording the template, version, and the exact variable values used.

### Post-generate hooks

A template can declare shell commands to run after a project is generated:

```toml
[[hooks.post]]
run = "git init"

[[hooks.post]]
run = "go mod tidy"
```

Hooks never run silently: the CLI asks for confirmation the first time it generates from a given template *version* (remembered after that, so you're not asked again for the same template@version), and the MCP server's `create_project` only runs them when called with `allow_hooks: true`. Either way, hooks run last — after every file is written and the target directory committed — so a failing hook never rolls back the generated project; the failure is reported with its exit code and output, and snap exits non-zero.

### Optional blocks

A template can declare toggleable groups of files:

```toml
[[optional]]
name = "docker"
prompt = "Include Docker?"
default = true
paths = ["Dockerfile", "docker-compose.yml"]
```

Toggle one with `--with docker` / `--without docker` (repeatable; naming the same block in both is an error), or interactively if neither is given and there's a prompter. Unset blocks fall back to their declared default. `snap show`/`describe_template` list every block's name, prompt, default, and paths. Every block's resolved state — not just the ones you explicitly set — is recorded in `.snap/origin.toml`'s `[toggles]` table.

### Remote templates

`snap new` and `create_project`/`plan_project` accept a git-hosted template in place of a local template name:

```
snap new gh:acme/web-starter ~/code/northwind-site --var name=northwind-site
snap new gh:acme/web-starter@v1.2.0 ~/code/northwind-site        # pinned to a tag
snap new https://example.com/templates/api.git@abc1234 ~/code/api  # pinned to a commit
snap new git@github.com:acme/web-starter.git ~/code/site          # SCP-style URL (unpinned only)
```

`gh:user/repo` expands to `https://github.com/user/repo.git`; any other git URL (https/http/ssh/SCP-style) works directly. An unpinned reference resolves the remote's default branch on every run; `@tag` or `@commit` pins it, and once that exact commit has been fetched once it's reproducible fully offline from a local cache under `~/.config/snap/cache/` (`$XDG_CONFIG_HOME/snap/cache/` if set) — a separate cache from the template store, since a remote template isn't saved into it. `.snap/origin.toml`'s `source` field records the exact `<url>@<resolved commit>` used.

### Template versioning

`[template].version` in `snap.toml` must be a strict `MAJOR.MINOR.PATCH` semver string — `snap capture` always writes a fresh template at `0.1.0`. The local store keeps every version it's ever had, one directory per version, so an old base can still be regenerated later:

```
snap template bump client-site patch   # 1.0.0 -> 1.0.1, old version untouched
snap template bump client-site minor   # 1.0.1 -> 1.1.0
snap template bump client-site major   # 1.1.0 -> 2.0.0
snap template bump client-site patch --dry-run
snap template bump client-site patch --json
```

`snap new`/`snap show` (and MCP's `plan_project`/`create_project`/`describe_template`) resolve a bare template name to its latest (highest semver) version, or pin an exact one with `name@version`, mirroring remote templates' `@tag`/`@commit` syntax:

```
snap new client-site ~/code/site --var name=site              # latest version
snap new client-site@1.0.0 ~/code/site --var name=site        # pinned
snap show client-site@1.0.0
```

### Updating a generated project

`snap update <target-dir>` three-way merges a project's current files against its own history: the project's recorded *old* base (reconstructed from `.snap/origin.toml`, byte-for-byte, even if the template has moved on since) and the template's *latest* version, both resolved with the exact same variables and toggles the project was generated with.

```
snap update ~/code/northwind-site
snap update ~/code/northwind-site --dry-run
snap update ~/code/northwind-site --json
```

A change only the template made, or only you made, applies cleanly. A change both sides made identically is a no-op. A line both sides changed *differently* becomes a conflict, with `git merge`-style markers written right into the file:

```
<<<<<<< ours
your version of the line
=======
the template's version of the line
>>>>>>> theirs
```

A file you deleted stays deleted unless the template also changed it (flagged as a conflict either way); a file the template renamed is detected by exact content match and silently adopted, unless you'd edited the old file first (in which case both the old, edited file and the new, template-provided one are kept, flagged as a conflict). `.snap/origin.toml` is rewritten to the new version only when the merge completes with zero conflicts — exactly like `git merge`, a conflict is a non-zero exit (`$?`), not a failure that left nothing written: everything that merged cleanly is already on disk, and the listed paths are what's left to resolve by hand before re-running.

Unlike `snap new`/`snap capture`, `snap update` modifies an already-populated directory in place rather than refusing a non-empty target — it builds the complete merged tree in a temp directory first, then swaps it in for the old one (keeping a backup until the swap succeeds), so a failure partway through never touches your project.

### Testing a template

A template can declare how to sanity-check itself, so its author catches a broken template before anyone else generates from it:

```toml
[test]
check = "go build ./... && go test ./..."

[test.vars]
name = "sample-project"
port = "8080"
```

```
snap test client-site
snap test client-site --dry-run   # show the plan and the check command, run nothing
```

`snap test` generates the template into a throwaway temp directory using the `[test.vars]` sample values (the same shape as `--var`), then runs `check` there as a real shell command — the exact same mechanism as a post-generate hook, just without needing confirmation (nothing it touches survives past the command). The temp directory is always cleaned up afterward. The check command's own exit code becomes `snap test`'s exit code, so it's usable as a CI step. A template with no `[test]` block can't be tested this way — `snap test` is an immediate error naming the template, not a silent no-op.

### Checking for drift

`snap status` reports which template (and version) each of your generated projects came from, and whether that's still the template's latest local version — the local groundwork for a future team drift dashboard:

```
snap status ~/code/northwind-site ~/code/other-site
# ~/code/northwind-site   client-site 1.0.0 -> 1.2.0   (behind)
# ~/code/other-site       client-site 1.2.0            (up to date)

snap status ~/code/northwind-site --json
```

Each directory is checked independently — one that isn't a generated project (or whose `.snap/origin.toml` can't be read) doesn't stop the others from being reported, it's just shown with its own error instead. A remote-sourced project is reported with its recorded version but no drift verdict (checking a remote's latest would need network access, out of scope for a *local* drift report). The exit code is non-zero whenever anything was reported behind or unreadable, so it's usable as a CI/cron check across many projects at once.

### Token receipts

Every generation prints what it saved an agent from writing itself:

```
snap new client-site ~/code/northwind-site --var name=northwind-site
# ✓ 31 files · 42.0 KB · ~11.8k tokens written by Snap instead of the agent
```

The count is always an *approximation* (there's no universal offline tokenizer for every model), shown with `~`. A private, local usage log records one line per generation — never sent anywhere, and disabled entirely with `SNAP_NO_USAGE_LOG=1`:

```
snap stats                  # totals by template and by source (cli/mcp)
snap stats --since 30d
snap stats --json
```

### Recipes

A template creates a new project; a **recipe** applies a reusable change to an *existing* one — new files, dependency additions, env entries, and post-apply hooks:

```toml
[recipe]
name = "sentry"
version = "1.0.0"
description = "Error tracking with Sentry"

[applies_to]
files = ["package.json"]

[[dependencies]]
ecosystem = "npm"   # npm | go | pip | cargo
name = "@sentry/node"
version = "^8.0.0"

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""
```

```
snap add sentry                        # apply to the current directory
snap add sentry ~/code/northwind-site --var service=payments
snap add sentry --dry-run
snap recipes                            # list available recipes
```

Dependency edits are format-preserving (`internal/core/deps`): `package.json`/`go.mod`/`Cargo.toml`/`requirements.txt`/`pyproject.toml` are edited in place, keeping existing key order and indentation — never by shelling out to a package manager. Applying a recipe is all-or-nothing: any conflict (a file already there with different content, a dependency already pinned to a different version, `applies_to` not satisfied) fails the whole plan with nothing written; re-applying an already-applied recipe is a no-op. Every successful apply is recorded as a `[[recipes]]` entry in `.snap/origin.toml`, alongside (or, for a project Snap didn't generate, instead of) its template provenance.

### Interactive TUI

Run `snap` with no arguments to launch an interactive wizard: pick a template, enter a target directory, fill in variables (pre-filled with declared defaults), toggle any `[[optional]]` blocks, preview the generated file tree, and confirm. It's built entirely on the same `internal/core/usecase` calls the CLI and MCP server use — no separate logic, no separate safety guarantees. `ctrl+c` quits at any point without writing anything; nothing is written until you confirm the preview.

## Usage

```
# Capture an existing project as a template
snap capture ~/code/acme-site --as client-site --dry-run   # preview first
snap capture ~/code/acme-site --as client-site --yes       # accept-all, non-interactive
snap capture ~/code/acme-site --as client-site             # interactive accept/rename/reject

# Inspect what's available
snap list
snap list --json
snap show client-site
snap show client-site --json

# Generate a new project
snap new client-site ~/code/northwind-site --var name=northwind-site
snap new client-site ~/code/northwind-site --dry-run
snap new client-site ~/code/northwind-site --yes   # error instead of prompting
snap new client-site ~/code/northwind-site --force  # overwrite an existing non-empty target
snap new client-site ~/code/northwind-site --without docker --with ci  # toggle [[optional]] blocks

# Update a project to its template's latest version
snap update ~/code/northwind-site
snap update ~/code/northwind-site --dry-run

# Sanity-check a template against its own declared [test] block
snap test client-site

# Check which generated projects are behind their template
snap status ~/code/northwind-site ~/code/other-site

# Totals of what snap has generated so far
snap stats

# Apply a recipe to an existing project
snap add sentry
snap add sentry ~/code/northwind-site --var service=payments
snap add sentry --dry-run
snap recipes
```

Templates live as plain, hand-editable folders under `~/.config/snap/templates/` (or `$XDG_CONFIG_HOME/snap/templates/`), each with a `snap.toml` manifest and the captured files. Recipes live the same way, under `~/.config/snap/recipes/` (or `$XDG_CONFIG_HOME/snap/recipes/`), each with a `recipe.toml` manifest and an optional `files/` directory.

## Development

```
make build          # build ./snap
make test           # full test suite
make test-golden    # just the M6 round-trip tests (capture -> new -> diff), verbosely
make update-golden  # regenerate committed golden output after an intentional change
make fmt            # gofmt -w .
make fmt-check      # fail if anything isn't gofmt-formatted
make vet            # go vet
make lint           # golangci-lint
make check          # everything CI runs: fmt-check, vet, test, lint
```

See `PLAN.md` for the full product/technical plan and architecture, and `CLAUDE.md` for repo-specific conventions and the milestone checklist.

## Releasing

`.goreleaser.yml` builds linux/darwin/windows × amd64/arm64 archives, a Homebrew cask (`CodeOne45/homebrew-tap`), and a Scoop bucket (`CodeOne45/scoop-bucket`), triggered by pushing a `v*` tag (`.github/workflows/release.yml`). Validate the config locally without publishing anything:

```
goreleaser check
goreleaser build --snapshot --clean --single-target
```

A real release needs `HOMEBREW_TAP_GITHUB_TOKEN` and `SCOOP_BUCKET_GITHUB_TOKEN` repo secrets (tokens with push access to those two tap/bucket repos — the default `GITHUB_TOKEN` can't push to another repo), and a `LICENSE` file at the repo root (not yet added — pick MIT or Apache-2.0 per `PLAN.md`).

## Roadmap

Not built yet:

- **v1 (remaining):** a demo GIF; an actual tagged release.
- **v2 (remaining):** a paid team registry (hosted, out of scope for this repo) — everything else in `PLAN.md`'s v2 scope is built.
- **v3 (in progress, see `PLAN-v3.md`):** recipes (`snap add`), `snap capture --from-commit`, `snap lint --agent`, `snap check`, `snap eval`. Token receipts (M19) are done.

## MCP server

`snap mcp` runs over stdio and exposes the same core use cases as the CLI to an agent — same safety guarantees (path-escape rejection, non-empty-target refusal), same JSON shapes as `--json` (see `docs/cli-json-contract.md`).

| Tool | What it does | Side effects |
|---|---|---|
| `list_templates` | List templates (their latest version), optionally filtered by a substring of name/description | None |
| `describe_template` | A template's variables and raw file tree. `name` may be `name@version` to pin a specific local version | None |
| `plan_project` | Resolve variables + `[[optional]]` toggles and build the write plan. `template` may be `name@version` (local pin) or a remote reference (`gh:user/repo[@pin]`, a git URL). Response includes `stats` (M19: file/byte/approx-token counts) | None locally; fetches/caches a remote template's commit if given |
| `create_project` | Generate a project. `allow_hooks: true` required to actually run any hooks the template declares (otherwise they're skipped). `template` accepts the same pinning as `plan_project`. Response includes `stats` | Writes files; runs hooks only if allowed; appends one local usage-log entry (source `"mcp"`) unless `SNAP_NO_USAGE_LOG` is set |
| `capture_template` | Scan a project, propose scored placeholder candidates | None |
| `save_template` | Re-scan and save only the accepted candidates as a template | Writes files |

Resources: `template://<name>/snap.toml` and `template://<name>/README.md`. Prompt: `new-project` (describe → ask for missing variables → plan → confirm → create).

`create_project`/`save_template` never prompt (a missing required variable is an immediate error) and, like the CLI, refuse to write into an existing non-empty directory — there is no agent-facing `--force` equivalent in M8.
