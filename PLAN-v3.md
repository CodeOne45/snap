# Snap — v3 Plan: Recipes and Agent Efficiency

> Builds on PLAN.md. All architecture rules there still apply: pure core, plan-then-apply, only the executor writes, deterministic output, CLI/TUI/MCP are thin adapters.
> Start this plan only after v2 (M14–M18) is complete. Recipe updates (M23) depend on v2's versioning and base regeneration.

## 1. Goals

| Feature | Problem it solves | Why people adopt / pay |
| --- | --- | --- |
| Token receipts | Users can't see what Snap saves them | Concrete, honest number for the demo, README and team dashboard |
| Recipes | Projects are started rarely, but the same changes (Docker, CI, Sentry, auth) are added constantly | Turns Snap into a weekly tool; org recipe library is a strong paid feature |
| `snap lint --agent` | Agent context files are often bloated, which research links to higher cost and no better results | Unique, research-backed launch story; protects every project generated from a template |
| `snap check` | Agents drift from conventions over many sessions | Enforcement for teams; feeds the paid drift dashboard |
| `snap eval` (experimental) | Nobody can measure whether their agent setup helps | Differentiator; basis for a paid hosted eval later |

Non-goals: no LLM calls inside Snap's core (determinism is the product), no automatic rewriting of users' agent files, no network calls except git fetches and (for `snap eval`) the user's own agent CLI.

## 2. Milestone overview

| # | Milestone | Depends on | Size |
| --- | --- | --- | --- |
| M19 | Token receipts + `snap stats` | M8 (MCP) | Small |
| M20 | Recipe format + `snap add` (new files + dependencies) | M3, M9 | Medium |
| M21 | `snap capture --from-commit` | M5, M20 | Medium |
| M22 | Anchored edits to existing files + in-place transactional executor + recipe `[test]` verification | M20, M21 | Large |
| M23 | Recipe MCP tools + recipe updates | M8, M16, M22 | Medium |
| M24 | `snap lint --agent` | M19 (tokenizer) | Medium |
| M25 | `snap check` | M10, M20 | Medium |
| M26 | `snap eval` (experimental) | M24 | Large |

Recommended order: M19 → M20 → M21 → M24 → M22 → M23 → M25 → M26. M24 is placed early because it is cheap and gives a strong launch post.

---

## 3. M19 — Token receipts

**What:** every plan carries stats about what it writes. `create_project` (and later `apply_recipe`) returns them; `snap stats` shows totals.

```
✓ create_project: 31 files · 42 KB · ~11.8k tokens written by Snap instead of the agent
```

**Design:**
- New package `internal/core/tokens` with a `Counter` interface. Default implementation: an offline BPE tokenizer used as an approximation (Claude's exact tokenizer is not available offline). Always display with `~` and the word "approximately" in docs. Keep the implementation swappable.
- `plan.Stats{Files, TextFiles, Bytes, ApproxTokens}` computed from the Plan; binary files count toward files and bytes, never tokens.
- Stats live in the Plan and in `--json` output only. Never in generated files (determinism).
- Usage log: append one JSON line per successful apply to `$XDG_STATE_HOME/snap/usage.jsonl` (template/recipe id, stats, timestamp, source = cli|mcp). Local only, never sent anywhere. `SNAP_NO_USAGE_LOG=1` disables it.
- `snap stats [--since 30d] [--json]`: totals by template/recipe and by source.

**Acceptance:**
- R1: Generating a fixture reports file and byte counts that match `find`/`du` on the output.
- R2: Binary files contribute 0 tokens.
- R3: Same inputs give the same stats (deterministic).
- R4: With `SNAP_NO_USAGE_LOG=1`, no usage file is created or modified.

---

## 4. Recipes (M20–M23)

### 4.1 Concept

A **template** creates a new project. A **recipe** applies a reusable change to an existing project: new files, dependency additions, environment variables, and (from M22) small anchored edits to existing files.

```bash
snap add sentry                    # apply to the current project
snap add sentry --dry-run          # show the plan
snap add sentry --var service=payments
snap recipes                       # list recipes (alias: snap list --recipes)
```

### 4.2 Recipe format

A recipe is a folder:

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

# Recipe applies only if all of these match
[applies_to]
files = ["package.json"]

[[variables]]
name = "service"
prompt = "Service name"
default_from = "project.name"     # resolved from the target project

[[dependencies]]
ecosystem = "npm"                 # npm | go | pip | cargo
name = "@sentry/node"
version = "^8.0.0"
dev = false

[[env]]
file = ".env.example"
key = "SENTRY_DSN"
value = ""

# M22+
[[edits]]
file = "src/main.ts"
kind = "insert_after"             # insert_after | insert_before | append | prepend
anchor = "import express from 'express'"
content = "import { initSentry } from './lib/sentry'"

[[hooks.post]]
run = "npm install"
```

### 4.3 New plan op

Recipes modify existing files, so add one op kind:

- `ModifyFile{path, expected_sha256, content}`: the planner reads the current file, computes the full new content in memory, and records the hash of the current content. The executor refuses to apply if the file changed since planning.

This keeps all edit logic in the pure core (computing new content) and keeps the executor simple (verify hash, write). Add it with a new `kind` value in the JSON plan; existing kinds are unchanged.

### 4.4 Provenance

Every applied recipe is recorded in `.snap/origin.toml`:

```toml
[[recipes]]
name = "sentry"
version = "1.0.0"
source = "local"
applied_at = 2026-11-02T10:00:00Z
[recipes.variables]
service = "payments"
```

Projects not created by Snap get a `.snap/origin.toml` with only `[[recipes]]` entries.

### 4.5 M20 — Recipe format + `snap add` (new files and dependencies only)

**Scope:** hand-written recipes; new files, dependency additions, env entries, hooks. No edits to arbitrary files yet.

**Dependency editors** (in core, pure, format-preserving):
- npm: edit `package.json`, preserving key order and indentation.
- go: `go.mod` via `golang.org/x/mod/modfile`.
- pip: `requirements.txt` (line add) and `pyproject.toml` `[project].dependencies`.
- cargo: `Cargo.toml` `[dependencies]`.
- Never run a package manager from core; installs happen only through hooks with the usual confirmation rules.

**Rules:**
- All-or-nothing by default: if any step can't apply (file exists with different content, `applies_to` fails), the whole plan fails with a clear conflict list.
- Idempotent: re-applying a recipe whose files and deps are already present produces an empty plan and says so.
- A dependency already present at a different version is a conflict, not an overwrite.

**Acceptance:**
- R5: Applying `sentry` to a clean fixture adds the files, the dependency and the env key; the project builds.
- R6: Applying it twice: second run is a no-op with a clear message.
- R7: `package.json` formatting (indentation, key order) is unchanged except for the added line.
- R8: Target already has `src/lib/sentry.ts` with different content → plan fails, nothing written.
- R9: `applies_to` not met (no `package.json`) → clear error, nothing written.

### 4.6 M21 — `snap capture --from-commit`

```bash
snap capture --from-commit a1b2c3 --as sentry     # one commit vs its parent
snap capture --from-range main..feature-sentry --as sentry
snap capture --from-commit a1b2c3 --as sentry --test "npm run build"
```

**How it works:**
1. Read the diff via git (go-git, shell-out fallback).
2. Added files → `files/`, with placeholder detection reused from M5 using the source project's identity (name, package name).
3. Changes to known manifests (`package.json`, `go.mod`, `requirements.txt`, `pyproject.toml`, `Cargo.toml`) → structured `[[dependencies]]` by comparing parsed before/after, not text diffs.
4. Added lines in `.env.example` → `[[env]]`.
5. Other modified files → proposed `[[edits]]` with an anchor taken from the nearest stable context line; marked "needs review" until M22 exists (before M22: reported as unsupported and skipped with a warning).
6. Deleted and renamed files → reported as unsupported, skipped with a warning.
7. Same secret and ignore rules as project capture.

**Recipe verification (folded in from the M17 `[test]` pattern, no separate milestone):** `recipe.Recipe` gains an optional `Test *TestConfig` field, same shape as a template's (`check` command + `[test.vars]` sample values), written as recipe.toml's own `[test]`/`[test.vars]` block. Nothing infers a check command from the diff — exactly like a template's `[test]` block, it's opt-in and hand-authored, but `--test "<command>"` lets `snap capture --from-commit` seed it directly at capture time, since the author already knows what proves the commit's effect (and is usually about to run that command anyway). M21 only adds the schema and capture-time seeding; M22 is where it actually runs (see below), so R10's "and B builds" becomes something `snap add` itself can assert, not just a fact the test suite happens to check by hand.

**Acceptance:**
- R10: A commit that adds Sentry to fixture A becomes a recipe that applies cleanly to fixture B (same stack, different name), and B builds.
- R11: Project-specific names from A do not appear anywhere in B after applying.
- R12: Secrets in the commit (a real `.env`) are never captured.

### 4.7 M22 — Anchored edits + in-place transactional executor

**Edits:** `insert_after`, `insert_before`, `append`, `prepend`. The anchor must match exactly one line (after trimming whitespace); zero or multiple matches is a conflict. Inserted content is indented to match the anchor line.

**Executor for in-place changes:** creating a new project used temp-dir + rename. Modifying an existing project needs a different strategy:
1. Verify every `expected_sha256`.
2. Back up each file to be modified into `.snap/tx/<id>/`.
3. Apply all writes.
4. If the recipe declares `[test]` (M21) and `--skip-test` wasn't given, run `check` in TargetDir via the same `ports.HookRunner` `snap test`/post-apply hooks already use. A non-zero exit is treated exactly like a write failure (step 5) — the check's output is reported as the reason.
5. On any failure (a write error or a failing `[test].check`), restore from backup; on success, delete the backup.
6. Refuse to run if the git working tree has uncommitted changes in affected files, unless `--allow-dirty`. Recommend committing first, so `git diff` shows exactly what the recipe did.

**Recipe verification (folded in from the M17 `[test]` pattern, no separate milestone):** this is the only place a recipe's `[test]` block (M21) actually runs — M21 added just the schema and capture-time seeding. `snap add` runs `[test].check` automatically whenever the recipe declares one (an unverified apply is the whole gap this closes), with a `--skip-test` opt-out mirroring `snap new`'s `--yes` precedent for a declared-but-unwanted step. A recipe with no `[test]` block behaves exactly as M20 already does — step 4 is simply skipped.

**Acceptance:**
- R13: Anchor found once → edit applied with correct indentation.
- R14: Anchor missing or ambiguous → conflict listed, nothing written.
- R15: File changed between plan and apply → refused.
- R16: Injected write failure mid-apply → all files restored byte-identically.
- R17: Dirty working tree → refused without `--allow-dirty`.
- R18: A recipe with a `[test]` block whose `check` fails after applying → all files restored byte-identically, same as R16, and the check's output is reported.
- R19: A recipe with a `[test]` block whose `check` passes → the backup is deleted and the apply succeeds normally (no behavior change from a recipe with no `[test]` block).

### 4.8 M23 — Recipe MCP tools + recipe updates

**MCP tools:**

| Tool | Side effects |
| --- | --- |
| `list_recipes` (optionally filtered by what applies to the current project) | None |
| `describe_recipe` | None |
| `plan_recipe` (target dir, variables) → plan + conflicts + stats | None |
| `apply_recipe` (+ `allow_hooks`) | Writes files; hooks only if allowed |
| `capture_recipe` (from commit/range) → proposal | None until saved |

Tool descriptions must tell the agent to call `plan_recipe` first, show the plan, and report conflicts to the user instead of working around them by hand.

**Recipe updates:** reuse v2's machinery. For each `[[recipes]]` entry, regenerate the old recipe output (M15) and three-way merge with the new version (M16). `snap update --recipes` and `snap status` include recipe versions.

**Acceptance:**
- R20: An agent applies a recipe via MCP; the result is byte-identical to `snap add` with the same inputs.
- R21: Updating a recipe from 1.0.0 to 1.1.0 applies cleanly when the user hasn't touched the recipe's files, and reports a conflict when they have.

---

## 5. M24 — `snap lint --agent`

### 5.1 Why

A 2026 study from ETH Zurich ("Evaluating AGENTS.md: Are Repository-Level Context Files Helpful for Coding Agents?", Gloaguen et al., arXiv:2602.11988) found that repository context files tended not to improve task success, and in some settings reduced it, while increasing inference cost by over 20%. Agents followed the instructions, which led to broader exploration (more file reads and test runs). The authors recommend that human-written context files describe only minimal requirements. Codebase overviews in particular did not help agents locate relevant files faster. Separately, Chroma's "Context Rot" research shows model performance becoming less reliable as input length grows.

Caveat for docs and marketing: the study covers specific agents and benchmarks (largely Python). Present the feature as "keep agent context minimal and measure it," not "context files are bad."

### 5.2 What it checks

Scans `CLAUDE.md`, `AGENTS.md`, nested ones, and `.claude/` (commands, agents, settings).

| Rule | Detects | Severity |
| --- | --- | --- |
| AG001 size | Total approx tokens of always-loaded agent files above a budget (default configurable, e.g. 1,500) | warning |
| AG002 overview | Sections like "Overview", "Architecture", "Project structure", and directory trees | info |
| AG003 duplicate | Paragraphs highly similar to README content (shingle similarity) | warning |
| AG004 linter-enforced | Style rules (indentation, quotes, import order) when a formatter/linter config exists | info |
| AG005 dead reference | Commands, scripts or paths mentioned that don't exist (`npm run lint` with no `lint` script, missing files) | error |
| AG006 leftover | Unrendered `{{placeholders}}` or the source project's name after generation | error |
| AG007 conflicting | Contradictory instructions across nested files (e.g. two different test commands) | warning |

AG005 and AG006 are objectively wrong and are the most valuable rules; keep heuristic rules (AG002–AG004) at lower severity.

### 5.3 Behavior

```bash
snap lint --agent              # current project
snap lint --agent --json
snap lint --agent --template client-site   # lint a template's agent files
```

- Output: rule id, file, line, message, suggestion, plus total approx tokens.
- Never rewrites files. Suggestions only.
- Exit code non-zero only for `error` severity (CI-friendly).
- `snap capture` runs the linter on captured agent files and shows warnings (non-blocking).
- Config in `snap.toml` or `.snap/lint.toml`: budget, disabled rules.

**Acceptance:**
- R22: A `CLAUDE.md` referencing a non-existent `npm run e2e` → AG005 error with line number.
- R23: Generated project containing the source name in `CLAUDE.md` → AG006 error.
- R24: A minimal, correct `CLAUDE.md` → no warnings.
- R25: Token total matches M19's counter for the same files.

---

## 6. M25 — `snap check`

### 6.1 What

Templates and recipes declare invariants; `snap check` verifies a project still satisfies them. Useful after long agent sessions, in CI, and as an agent hook.

```toml
# in snap.toml (template) or recipe.toml
[check]
require_files = ["Dockerfile", "test/"]
forbid_files = ["__tests__/"]
require_dependencies = [{ ecosystem = "npm", name = "pino" }]
forbid_dependencies = [{ ecosystem = "npm", name = "winston" }]
commands = ["npm run lint"]          # optional, run only with --run-commands
```

### 6.2 Behavior

- Finds applicable rules from `.snap/origin.toml` (template + all recipes).
- Output per violation: rule, explanation, and a remediation hint the agent can act on (e.g. "missing Dockerfile: run `snap add docker`" when a matching recipe exists).
- `--json` for agents and CI; non-zero exit on violations.
- Commands run only with `--run-commands` (same safety stance as hooks).
- MCP tool: `check_project` (read-only unless commands are explicitly allowed).
- Document how to wire it as a Claude Code hook (verify the current hook configuration format in Claude Code's docs when implementing) and as a CI step.

**Acceptance:**
- R26: Project generated from a template passes `snap check`.
- R27: Delete the Dockerfile and add `winston` → two violations with hints; exit non-zero.
- R28: Project with no `.snap/origin.toml` → clear message, exit code distinct from "violations found."

---

## 7. M26 — `snap eval` (experimental)

### 7.1 What

Measure whether an agent setup actually helps, on the user's own projects, with the user's own agent and account.

```toml
# .snap/eval.toml
agent = "claude"                 # adapter name
repeats = 3

[[tasks]]
name = "add-endpoint"
prompt = "Add a GET /health endpoint that returns {status: 'ok'} with a test."
verify = "npm test"

[[variants]]
name = "current"                 # the project's agent files as-is

[[variants]]
name = "minimal"
claude_md = ".snap/eval/minimal-CLAUDE.md"
```

### 7.2 How it runs

1. For each variant × task × repeat: create a fresh isolated copy (generate from the project's template, or a git worktree).
2. Swap in the variant's agent files.
3. Run the agent headless through an adapter (for Claude Code, its non-interactive print mode with machine-readable output; verify flags and the usage/cost fields when implementing).
4. Run `verify`; record pass/fail, tokens/cost if the agent reports them, duration.
5. Report per variant: success rate, median tokens, with the sample size shown and a warning that small samples are noisy.

### 7.3 Safety and cost

- Always print an estimate of runs (variants × tasks × repeats) and require confirmation; `--yes` for CI.
- Runs only in isolated copies, never the user's working tree.
- Snap never stores API keys; the agent CLI uses its own auth.
- Mark the command experimental; the adapter interface should support other agents later.

**Acceptance:**
- R29: Dry run prints the full run matrix and the number of agent invocations without running anything.
- R30: A failing `verify` is recorded as a failure, not an error of Snap.
- R31: The user's working tree is unchanged after an eval run.

---

## 8. Changes to existing components

| Component | Change |
| --- | --- |
| `core/plan` | Add `ModifyFile` op kind and `Stats`; JSON schema version bump (additive) |
| `executor` | Add in-place transactional mode (backup/restore) alongside temp-dir mode |
| `core/recipe` (new, M20) | `Recipe` type and `recipe.toml` parsing; gains an optional `Test` field (M21) |
| `core/template` | `[check]` section in manifests (M25) |
| `core/capture` | Commit/range source in addition to directory source |
| `core/deps` (new) | Format-preserving dependency editors per ecosystem |
| `core/tokens` (new) | Token counter interface + default approximation |
| `core/lint` (new) | Agent-file rules as strategies (one type per rule) |
| `core/check` (new) | Invariant evaluation |
| `adapters/mcp` | Recipe tools, `check_project`, stats in results |
| `adapters/agent` (new, M26) | Headless agent runners |
| Provenance | `[[recipes]]` entries in `.snap/origin.toml` |

## 9. Business mapping

| Free (open source) | Paid team tier (later, separate service) |
| --- | --- |
| Recipes, `snap add`, capture from commit | Org recipe library with approval and versioning |
| Token receipts, local `snap stats` | Team-wide savings dashboard |
| `snap lint --agent` | Org-wide lint policies and reports |
| `snap check` locally and in CI | Drift dashboard across all projects, update PRs |
| `snap eval` with your own agent | Hosted eval runs and history |

## 10. Launch hooks

- M19: add the token receipt line to the demo video.
- M21: demo "turn yesterday's commit into a recipe, apply it to another service" (strong 30-second clip).
- M24: blog post: "Your CLAUDE.md might be costing you tokens. Here's how to check," citing the research with its caveats.

## References

- Gloaguen, T., Mündler, N., Mueller, M., Raychev, V., Vechev, M. "Evaluating AGENTS.md: Are Repository-Level Context Files Helpful for Coding Agents?" arXiv:2602.11988, February 2026. https://arxiv.org/abs/2602.11988
- Chroma Research. "Context Rot: How Increasing Input Tokens Impacts LLM Performance." 2025. https://research.trychroma.com/context-rot
