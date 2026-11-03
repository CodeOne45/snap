
Snap is a Go CLI/TUI + MCP server that captures existing projects as templates and generates new projects from them.
The full product and technical plan is in @PLAN.md. Read it before starting any task.

## How to work on this repo

- Work one milestone at a time (see PLAN.md §8 "Milestone breakdown"). Don't start the next milestone until the current one's tests pass.
- Before writing code for a milestone, propose a short plan (files to create, types, tests) and wait for approval.
- After each milestone: run tests, summarize what was done, and update the checklist at the bottom of this file.

## Architecture rules (do not break)

- `internal/core/**` must never import `os`, `os/exec`, `net`, or any adapter package. All I/O goes through interfaces in `internal/ports`.
- Only `internal/executor` writes to the real filesystem.
- Every command builds a `plan.Plan` first; `--dry-run` prints it, `--json` serializes it.
- CLI, TUI and MCP are thin adapters that call `internal/core/usecase`. No business logic in adapters.
- Generation must be deterministic: no maps iterated without sorting, no timestamps in rendered output except `.snap/origin.toml`.

## Code conventions

- Go, latest stable version. `gofmt`, `go vet`, and `golangci-lint` must pass.
- Errors: wrap with context (`fmt.Errorf("capture: scan %s: %w", path, err)`); no panics outside `main`.
- Tests: table-driven unit tests next to code; golden tests in `testdata/` using the in-memory FS where possible.
- Update golden files only with an explicit `-update` flag, never silently.
- Keep dependencies minimal; ask before adding a new one.

## Commands

```
go build ./cmd/snap
go test ./...
go test ./... -run Golden -update   # regenerate golden files (only when intended)
golangci-lint run
```

Or via the Makefile (`make help` lists all targets): `make build`, `make test`, `make test-golden`, `make update-golden`, `make lint`, `make check` (everything CI runs).

## Safety rules

- Never capture secrets: exclude `.env*` (except `.env.example`), `*.pem`, `*.key`, `id_rsa*` by default.
- Never write outside the target directory; canonicalize paths and reject `..` escapes.
- Hooks never run without confirmation (CLI) or `allow_hooks: true` (MCP).

## Progress

- [x] M0 – Skeleton
- [x] M1 – Render engine
- [x] M2 – Plan + executor
- [x] M3 – Template store + `snap new`
- [x] M4 – Scanner
- [x] M5 – Detectors + `snap capture`
- [x] M6 – Golden tests (Gate 1)
- [x] M7 – CLI contract hardening
- [x] M8 – MCP server
- [x] M9 – Hooks
- [x] M10 – Optional blocks
- [x] M11 – Remote templates
- [x] M12 – TUI
- [~] M13 – Agent setup + release: CLAUDE.md/.claude//.mcp.json capture verified (regression test added), 4 example templates seeded under `examples/templates/`, `.goreleaser.yml` + release workflow written and validated locally (`goreleaser check`, `goreleaser build --snapshot`), minimal docs site scaffolded under `docs-site/` + Pages deploy workflow, Gate 2 passes (`internal/adapters/mcp/gate2_test.go`). Not done: no actual tagged release/publish (needs tap repos + secrets + a LICENSE file), no demo GIF.
- [x] M14 – Template versioning: `internal/core/semver` (hand-rolled, no pre-release/build metadata), `[template].version` validated at parse time, local store restructured to `<name>/<version>/` (`ports.Store` gained `Versions`/`PathVersion`), `snap template bump <name> <major|minor|patch>`, `name@version` pinning on `snap new`/`snap show` and MCP's `plan_project`/`create_project`/`describe_template`. `examples/templates/` restructured to match.
- [x] M15 – Base regeneration: `provenance.ParseOrigin` (inverse of `Origin.TOML`), `usecase.RegenerateBase` reconstructs a project's exact original Plan from its parsed `.snap/origin.toml`, pinned at `Template@Version` (M14) so a template bumped forward since still regenerates the old base. Core-only — no CLI/MCP surface yet (that's M16's `snap update`).
- [x] M16 – `snap update`: `internal/core/merge` (line-based diff3-style 3-way merge, `github.com/sergi/go-diff`), `executor.ApplyUpdate` (safe in-place swap for an already-populated directory — new since `Apply` refuses non-empty targets by design), `usecase.Update` orchestration (RegenerateBase for the old base + `New` for latest-version "theirs" + current on-disk "ours", exact-content rename detection, modify/delete conflict handling), `snap update <target-dir>` CLI command (`--dry-run`/`--json`/`--yes`, exit code 1 on unresolved conflicts like `git merge`). Tests per PLAN.md: clean update, conflicting local edit, file deleted locally, file renamed in template, plus added-in-template and user-added-unrelated-file.
- [x] M17 – `snap test`: optional `[test]` block in `snap.toml` (`check` command + `[test.vars]` sample values, same shape as `--var`), `usecase.Test` builds the generation plan into a caller-provided temp dir, `snap test <template>` CLI command generates into a real temp dir (always cleaned up) and runs `check` via the same `ports.HookRunner` post-generate hooks use, surfacing the check's exit code for CI (`--dry-run`/`--json` show the plan + check command without running anything). Added `[test]` blocks to the `go-cli-starter`/`api-service` example templates.
- [x] M18 – Local drift report: `usecase.Status` reads each given directory's `.snap/origin.toml` and compares against `Store.Versions`' latest (local sources only — remote sources report their version/source with no drift verdict, by design: no network access for a *local* report). `snap status <dir>...` CLI command (`--json`, exit code 1 if anything is behind or unreadable). Per-directory results, never a single hard failure across multiple dirs. All v2 milestones (M14-M18) are now complete.

## Progress (`PLAN-v3.md` — recipes and agent efficiency)

- [x] M19 – Token receipts + `snap stats`: `internal/core/tokens` (`Counter` interface, `HeuristicCounter` default — a deterministic word/punctuation-boundary approximation, no external tokenizer/vocabulary, by explicit choice over a real BPE tokenizer to avoid two new dependencies and embedded vocab data). `plan.Stats`/`ComputeStats` (binary files count toward `files`/`bytes`, never `text_files`/`approx_tokens`). `internal/adapters/usagelog` (local-only `$XDG_STATE_HOME/snap/usage.jsonl`, `SNAP_NO_USAGE_LOG` disables it outright). `snap new`'s default path prints a receipt line and logs usage (source `cli`); `snap new --json` now wraps `{plan, stats}` (no byte-pinned schema test existed for it, unlike `list`/`show`, so this was safe to evolve); new `snap stats [--since Nd|duration] [--json]`. MCP's `create_project`/`plan_project` gained an additive `stats` field; `create_project` logs usage (source `mcp`). Added `TestMain` safety nets in `cli`/`mcp` test packages disabling the usage log by default, so no test touches the real developer machine's log file.
- [x] M20 – Recipe format + `snap add`: `plan.ModifyFileOp` (new op kind: `{path, expected_sha256, content, mode}`, additive JSON schema). `internal/core/deps` (pure, format-preserving dependency/env editors, hand-rolled text surgery by explicit choice over a new parser dependency — `AddRequirementsTxt`/`AddGoMod` (`golang.org/x/mod/modfile`)/`AddCargoToml`/`AddPyprojectToml`/`AddPackageJSON`/`AddEnvEntry`, all idempotent-or-`*ConflictError`, never silently overwriting a different version/value). `internal/core/recipe` (`Recipe`/`AppliesTo`/`Variable`/`Dependency`/`EnvEntry` + `recipe.toml` parse/marshal, mirroring `template.Manifest`). `provenance.Origin` gained `Recipes []RecipeEntry` (`[[recipes]]`, `omitempty` on every template-provenance field so a project not generated by Snap gets a `.snap/origin.toml` with only recipe entries). `usecase.Add` orchestration: `applies_to` check, variable resolution (`default_from = "project.<key>"` reads the target's own recorded origin variables), renders `files/` same as a template, dispatches dependency edits by ecosystem (pip prefers `pyproject.toml` over `requirements.txt` when both exist), all-or-nothing conflict collection, empty-Plan idempotency, `[[recipes]]` provenance append. `executor.ApplyRecipe`: pre-flight-only conflict check (hash mismatch on `modify_file`, differing content on `write_file`) before any write, then direct per-file writes — no full transactional backup/restore yet, that's M22. `snap add <recipe> [target-dir]` (`--dry-run`/`--json`/`--var`/`--yes`, defaults target to `.`, hook gating mirrors `snap new`'s, reports an empty plan as "already applied; nothing to do", logs usage with the `recipe` field set instead of `template`) and `snap recipes` (alias `snap list --recipes`) CLI commands; `RecipeStore` added to `Deps`, rooted at `<config dir>/snap/recipes`. Fixed a pre-existing gap discovered along the way: `plan.ComputeStats` only counted `WriteFileOp`, silently excluding every `ModifyFileOp` from `snap add`'s receipt/`snap stats` totals — now counts both.
- [ ] M21 – `snap capture --from-commit`
- [ ] M22 – Anchored edits + in-place transactional executor
- [ ] M23 – Recipe MCP tools + recipe updates
- [ ] M24 – `snap lint --agent`
- [ ] M25 – `snap check`
- [ ] M26 – `snap eval` (experimental)

## Known gaps (M13, deliberately left open)

- **No `LICENSE` file.** `PLAN.md` §9 says "MIT/Apache-2.0" without picking one — needs a decision before any real release.
- **No tagged release yet.** `.goreleaser.yml` + `.github/workflows/release.yml` are written and validated locally (`goreleaser check`, `goreleaser build --snapshot --clean`), but nothing has been published. A real release additionally needs `HOMEBREW_TAP_GITHUB_TOKEN` and `SCOOP_BUCKET_GITHUB_TOKEN` repo secrets (tokens with push access to `CodeOne45/homebrew-tap` and `CodeOne45/scoop-bucket`, which don't exist yet either).
- **No demo GIF.** Needs a real terminal recording tool; not producible headlessly.
- **`docs-site/` isn't live.** The README's link to it 404s until GitHub Pages is enabled on the repo and something is pushed to `main` (the deploy workflow only triggers on push/dispatch).
- **Nothing in this repo has been committed.** `git init` was run but there are zero commits — by the user's standing preference, nothing gets committed without an explicit ask.
