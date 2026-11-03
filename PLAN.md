# Snap — Build Plan

> Snap turns any project you've already built into a reusable template in seconds, then lets you or your AI agent create new projects from it.
> "Snap" is a working name.

## 1. Vision and positioning

**One-liner:** Capture your best project once. Every new one, yours or your agent's, starts from it.

**Primary users (priority order):**
1. Developers using coding agents (Claude Code, Cursor, Codex) who want agents to follow their conventions.
2. Freelancers and agency devs who start a client project every few weeks on the same stack.
3. Teams with many services who want one enforced "how we build here" starting point.

**Differentiators vs cookiecutter / copier / Yeoman / template repos:**
- **Capture, not authoring.** Point it at a real project; it detects placeholders. No Jinja or config to write by hand.
- **Agent-native.** First-class MCP server and fully non-interactive CLI.
- **Single static binary.** No Python or Node runtime.
- **Update path.** Projects can pull later template improvements.

**Non-goals:** GUI, watched-folder automation, general build tool.

## 2. Use cases (what the product must make true)

### 2.1 Freelancer: new client site
Today: `cp -r acme-site northwind-site`, then grep for "acme" and fix 47 matches in different case forms by hand. Something always gets missed (e.g. OG image still says "Acme Corp").

With Snap:
```
$ snap capture ./acme-site --as client-site
Found 4 likely placeholders:
  "acme-site"   → {{name}}          12 places
  "AcmeCorp"    → {{name|pascal}}    8 places
  "ACME"        → {{name|upper}}     3 places
  "Acme Corp"   → {{company}}       24 places
Ignored: node_modules, .next, .git, .env
Accept all? [Y/n] y
✓ Template "client-site" saved

$ snap new client-site northwind-site --var company="Northwind Traders"
✓ 63 files created, git initialized, npm install done
```
**Acceptance:** zero occurrences of any original identifier (any case form) remain in the generated project.

### 2.2 Developer: AI agent follows conventions
Today: the agent improvises (Express instead of Fastify, Jest instead of Vitest, no Dockerfile).

With Snap MCP, the agent calls: `list_templates` → `describe_template` → asks user for missing vars → `plan_project` → `create_project`.
**Acceptance:** an agent can create a project from a template with no human typing beyond answering variable questions; output is byte-identical to `snap new` with the same inputs.

### 2.3 Team: services drift apart (v2)
Each generated project records `.snap/origin.toml`. When the template changes, `snap update` shows a reviewable three-way-merge diff.
**Acceptance:** a template change can be applied to an older generated project without overwriting local edits silently.

## 3. Feature roadmap

| Phase | Feature | What it does |
| --- | --- | --- |
| MVP | `snap capture <dir>` | Scans a project, ignores junk (.gitignore, node_modules, build output, secrets), proposes placeholders, writes a template |
| MVP | Placeholder detection | Finds identifiers in all case forms (kebab, snake, Pascal, camel, UPPER, Title) across file names, paths, contents |
| MVP | `snap new <template> <name>` | Generates a project; interactive prompts or `--var key=value` |
| MVP | Local template store | `~/.config/snap/templates/` as plain folders |
| MVP | `snap list`, `snap show` | List templates, preview resulting tree |
| MVP | `--dry-run`, `--json` everywhere | Plan preview and machine-readable output |
| v1 | `snap mcp` | MCP server over stdio |
| v1 | Post-generate hooks | `git init`, installs, custom commands, with confirmation |
| v1 | Optional blocks | Toggleable files/folders (Docker, CI, auth) |
| v1 | Remote templates | `gh:user/repo` or git URL, pinned by tag/commit |
| v1 | TUI | Browse templates, fill variables, live tree preview |
| v2 | `snap update` | Three-way merge of newer template version into existing project |
| v2 | `snap test` | Generate into temp dir and run a check command |
| v2 | Team registry (paid, separate service) | Private shared templates, versioning, audit |

Out of scope until proven: marketplace, plugin system, shell integration, conditional logic beyond toggles.

## 4. Architecture

Hexagonal (ports and adapters). Three thin front ends share one pure core; all I/O is behind interfaces. Only the executor writes to disk.

```
   ┌──────────────┐   ┌──────────────────┐   ┌────────────────────┐
   │ CLI (Cobra)  │   │ TUI (Bubble Tea) │   │ MCP server (stdio) │
   └──────┬───────┘   └────────┬─────────┘   └─────────┬──────────┘
          └────────────────────┼───────────────────────┘
                               ▼
 ┌──────────────────── Core: pure logic, no I/O ─────────────────────┐
 │  Use cases:   Capture   │   New   │   Update   │   List           │
 │  Detectors (strategies) │ Render pipeline │ Plan (CreateDir,      │
 │                         │                 │  WriteFile; RunHook   │
 │                         │                 │  added in v1)         │
 └───────────────────────────────┬───────────────────────────────────┘
                                 ▼
        Ports (interfaces): FS, Store, Prompter (+ HookRunner in v1)
                                 ▼
 ┌────────────┐ ┌────────────────┐ ┌─────────────┐ ┌──────────────────┐
 │ Filesystem │ │ Template store │ │ Hook runner │ │ Executor         │
 │ real + mem │ │ local + git    │ │ (v1)        │ │ temp dir, atomic │
 └────────────┘ └────────────────┘ └─────────────┘ └────────┬─────────┘
                                                             ▼
                                   Disk: new project + .snap/origin.toml
```

**Data flow for `snap new`:** load template → resolve variables (flags / prompts / MCP input) → drop disabled optional blocks → render paths and contents → build Plan → show or confirm → executor writes to temp dir and atomically moves into place → provenance file written → (v1) hooks run.

### Template format

A template is a normal folder with `snap.toml` at its root:

```toml
[template]
name = "api-starter"
version = "1.2.0"
description = "Go REST API with Docker and CI"

[[variables]]
name = "name"
prompt = "Project name"
type = "string"
validate = "^[a-z][a-z0-9-]+$"

[[variables]]
name = "port"
type = "int"
default = 8080

[[optional]]
name = "docker"
prompt = "Include Docker?"
default = true
paths = ["Dockerfile", "docker-compose.yml"]

[[hooks.post]]
run = "git init"

[[hooks.post]]
run = "go mod tidy"
```

Placeholders: `{{name}}` in file names, folder names and contents. Case filters: `{{name|kebab}}`, `{{name|snake}}`, `{{name|pascal}}`, `{{name|camel}}`, `{{name|upper}}`, `{{name|title}}`.

Provenance file written into every generated project, `.snap/origin.toml`:
```toml
template = "api-starter"
version = "1.2.0"
source = "local"            # or git URL + commit
generated_at = "2026-10-06T12:00:00Z"
[variables]
name = "payments-service"
port = 8080
```

## 5. Design patterns and key decisions

Guiding rule: **plan, then apply.** Every command builds an immutable Plan; one small component writes to disk.

| Pattern | Where | Why |
| --- | --- | --- |
| Hexagonal (ports & adapters) | Whole codebase | Core doesn't know if CLI, TUI or MCP called it |
| Command / Plan object | Generator, capture, update | Plan = list of ops; dry-run prints it, `--json` serializes it, executor applies it. MVP ops: `CreateDir`, `WriteFile`. `RunHook` is added in v1 with hooks |
| Strategy | Placeholder detectors | Each detector returns scored candidates; easy to add more |
| Chain of responsibility | Ignore rules | .gitignore → built-in junk list → `.snapignore` → CLI flags |
| Pipeline | Rendering | Discrete, testable stages |
| Repository | Template store | One interface for local, git, (later) registry |
| Adapter | Front ends | Thin translation to core use cases |
| Transactional executor | Apply | Temp dir + atomic rename; no half-written projects |

**Key decisions:**
- Templates are plain folders + one manifest; editable and git-versionable.
- Simple placeholder syntax with case filters; no Jinja-level logic.
- Binary files are copied, never rendered (detect by content sniffing).
- Hooks never run silently; MCP requires explicit `allow_hooks`.
- Generation is deterministic: same template version + same variables → byte-identical output.
- Never capture secrets: `.env`, `*.pem`, `*.key` and similar are excluded by default.

### Placeholder detection (the core algorithm, MVP)

1. Seed candidates: project folder name, `name` in package.json / go.mod module / Cargo.toml / pyproject.toml, git remote repo name, README H1.
2. Split each candidate into words (`acme-site` → [acme, site]) and generate all case forms.
3. Scan file names, paths and text contents for every form; count occurrences per form and per file.
4. Score: more files + appears in manifest + appears in paths → higher confidence.
5. Present grouped by logical variable; user accepts, renames variable, or rejects each.
6. Replace longest matches first to avoid partial overlaps (`AcmeCorp` before `Acme`).

## 6. MCP server design

`snap mcp` runs over stdio as a thin adapter over the core.

| Tool | Input | Returns | Side effects |
| --- | --- | --- | --- |
| `list_templates` | optional tag/search | id, description, variables, tags | None |
| `describe_template` | template id | variables, optional blocks, tree preview, hooks | None |
| `plan_project` | template id, variables, target dir | full Plan as JSON | None |
| `create_project` | plan inputs + `allow_hooks` | created paths, hook results | Writes files; hooks only if allowed |
| `capture_template` | source dir, name | proposed placeholders with scores | None |
| `save_template` | capture id, accepted placeholders | template id | Writes to store |

Resources: each template's `snap.toml` and README. Prompt: `new-project` (describe → ask missing vars → plan → create).

Safety: writes restricted to target dir (canonicalize paths, block `../`), refuse non-empty dir without `force`, hooks off by default for agents, every tool description states its side effects.

## 7. Tech stack and repo layout

Go (single static binary, fast iteration, mature TUI ecosystem). Verify current library versions and the MCP Go SDK API before starting.

| Concern | Choice |
| --- | --- |
| CLI | Cobra |
| TUI | Bubble Tea + Lip Gloss + Huh |
| MCP | Official MCP Go SDK |
| Config | TOML (BurntSushi/toml) |
| Git | go-git, shell-out fallback |
| Ignore rules | a gitignore-matching library |
| Release | GoReleaser → Homebrew, Scoop, GitHub releases |

```text
snap/
├── cmd/snap/              # main.go, wires adapters to core
├── internal/
│   ├── core/
│   │   ├── template/      # Template, Manifest, Variable types
│   │   ├── capture/       # scanner + detector strategies
│   │   ├── render/        # placeholder engine, case filters
│   │   ├── plan/          # Plan, Op types, diff, serialization
│   │   └── usecase/       # Capture, New, List, Update orchestration
│   ├── ports/             # interfaces: FS, Store, Prompter (HookRunner in v1)
│   ├── adapters/
│   │   ├── cli/           # Cobra commands
│   │   ├── tui/           # Bubble Tea models
│   │   ├── mcp/           # MCP tools, resources, prompts
│   │   ├── fs/            # real + in-memory filesystem
│   │   └── store/         # local and git template stores
│   └── executor/          # transactional plan apply
├── testdata/              # golden fixtures
└── docs/
```

**Testing:** golden tests are the backbone. Each fixture = a real-world project (Next.js, Go API, Python package, Rust crate). Capture → generate with new name → diff against committed expected output. Unit tests use the in-memory FS.

## 8. Execution plan

```
 Weeks 1–4          Weeks 5–8            Weeks 9–12           Months 4–6
 MVP core      ──►  Agent-ready     ──►  Public launch   ──►  Monetize
 capture/new/list   MCP server           TUI + preview        snap update
 case detection     hooks                docs, demo GIF       team registry beta
 golden tests       remote templates     Show HN, MCP lists   paid pilots
        ▲                  ▲                    ▲
  Gate: 5 real      Gate: agent demo     Gate: teams ask for
  projects, no      works end to end     shared templates
  manual fixes
```

Do not start a phase until its gate passes.

### Milestone breakdown (MVP, ordered)

1. **M0 – Skeleton:** Go module, Cobra root command, `ports` interfaces, in-memory FS, CI (lint + test).
2. **M1 – Render engine:** placeholder parser, case filters, path + content rendering, binary detection. Unit tests.
3. **M2 – Plan + executor:** Plan/Op types with only `CreateDir` and `WriteFile`, dry-run printer, JSON serialization, transactional executor. No `RunHook` op, no `HookRunner` port, and no hook stubs: those are added together in v1 when hooks and their safety rules (confirmation, `allow_hooks`) are built. Design `Op` as a closed, extensible type (an interface or tagged union with a `kind` field in JSON) so adding `RunHook` later doesn't change existing ops or the JSON format.
4. **M3 – Template store + `snap new`:** load `snap.toml`, resolve variables (flags + prompts), validation, `snap list`, `snap show`.
5. **M4 – Scanner:** walk dir, ignore chain, secret exclusion, binary detection.
6. **M5 – Detectors + `snap capture`:** seed candidates, case-form expansion, scoring, interactive accept, write template.
7. **M6 – Golden tests:** 5 fixtures, round-trip capture → new → diff. Gate 1.

### Post-MVP test plan (Gate 1 validation)

Run these before starting v1. Each scenario is Given / When / Then. "Fixture" means a real project from `testdata/` (Next.js app, Go API, Python package, Rust crate, plus one of your own messy projects).

**A. Capture correctness**

| # | Scenario | Given | When | Then |
| --- | --- | --- | --- | --- |
| A1 | Every case form renamed | Next.js fixture `acme-site` containing `acme-site`, `AcmeSite`, `acme_site`, `ACME_SITE`, `Acme Site` | `snap capture` then `snap new … northwind-site` | `grep -ri "acme"` in the output returns 0 matches; each form became the matching `northwind` form |
| A2 | Output really works | Any fixture | Capture, then generate with a new name | The project's own build and tests pass (`npm run build`, `go build ./... && go test ./...`, `pytest`, `cargo test`) |
| A3 | Longest match first | Project containing both `AcmeCorp` and `Acme` | Capture with `{{name}}` and `{{company}}` | `AcmeCorp` is replaced as one unit, never as `Acme` + `Corp` |
| A4 | No false positives | Project named `app` (short, generic word) | `snap capture` | Detector marks it low confidence and does not auto-accept; words like `application` and `apply` are untouched |
| A5 | Word boundaries | Name `site`, file containing `website` and `site_config` | Capture and generate | `website` unchanged; `site_config` renamed only if the user accepted it |
| A6 | Names in paths | `src/acme_site/__init__.py`, `cmd/acme-site/main.go` | Capture and generate | Folders and files are renamed; imports that reference them still resolve (A2 passes) |

**B. Safety and exclusions**

| # | Scenario | Given | When | Then |
| --- | --- | --- | --- | --- |
| B1 | Secrets never captured | `.env` with a fake key, `server.key`, `id_rsa` | `snap capture` | None are in the template; `.env.example` is kept; output lists them as ignored |
| B2 | Junk ignored | `node_modules/`, `.git/`, `dist/`, entries from `.gitignore` | `snap capture` | None are in the template |
| B3 | Re-capture a generated project | A project created by `snap new` (has `.snap/origin.toml`) | `snap capture` on it | `.snap/` is not in the new template |
| B4 | Duplicate paths rejected | A template whose rendering produces the same path twice | `snap new` | Fails with a clear error before writing anything |
| B5 | Path escape blocked | `--var name=../../etc` or a template file path containing `..` | `snap new` | Rejected by validation; nothing written outside the target directory |
| B6 | Non-empty target refused | Target directory already has files | `snap new` without `--force` | Refuses with a clear message; existing files untouched |
| B7 | No half-written project | Write fails mid-way (read-only parent dir, or an injected FS error in tests) | `snap new` | Target directory does not exist afterward; temp dir cleaned up |

**C. Determinism and provenance**

| # | Scenario | Given | When | Then |
| --- | --- | --- | --- | --- |
| C1 | Byte-identical output | Same template, same variables, fixed `GeneratedAt` | Generate twice into two folders | `diff -r` shows no differences |
| C2 | Provenance written | Any generation | `snap new` | `.snap/origin.toml` exists with template, version, source, `generated_at` (UTC), and typed variables (`port = 8080`, not `"8080"`) |
| C3 | Values aren't rendered | `--var description="uses {{handlebars}}"` | `snap new` | `origin.toml` stores the literal string unchanged |

**D. Files and platforms**

| # | Scenario | Given | When | Then |
| --- | --- | --- | --- | --- |
| D1 | Binaries untouched | `logo.png`, a `.woff2` font, a `.sqlite` file | Capture and generate | Output files are byte-identical to the originals |
| D2 | Permissions kept | Executable `scripts/dev.sh` | Capture and generate | Output script is still executable |
| D3 | Line endings kept | A file with CRLF endings | Capture and generate | Still CRLF |
| D4 | Symlinks handled | A symlink inside the project | `snap capture` | Either preserved or skipped with a warning; never followed outside the project |
| D5 | Windows | Run A1, B1, D2 on Windows in CI | — | Same results; paths use the correct separator |
| D6 | Large repo | A repo with thousands of files | `snap capture` | Finishes in a reasonable time (set a target once you've measured a baseline) and shows progress |

**E. CLI contract (needed before MCP)**

| # | Scenario | Given | When | Then |
| --- | --- | --- | --- | --- |
| E1 | Dry run writes nothing | Any template | `snap new … --dry-run` | Plan printed; filesystem unchanged |
| E2 | JSON is valid and stable | Any command with `--json` | Parse output | Valid JSON matching a committed schema; every op has a `kind` field |
| E3 | No prompt hangs | Required variable missing | `snap new … --yes` (no TTY) | Exits non-zero immediately with an error naming the missing variable |
| E4 | Exit codes | Success, user error, internal error | Run each | Distinct, documented exit codes |

**F. Human test (not automated)**

Give the MVP to 3 developers. Ask each to capture one of their own real projects without help, then generate a new one. Watch silently and note every placeholder they had to fix by hand and every moment of confusion. Gate 1 passes when your 5 fixtures need zero manual fixes and testers fix at most one or two placeholders each.

### v1 milestones (agent-ready + launch)

8. **M7 – CLI contract hardening:** stable `--json` schema committed as a file, `--yes` mode, documented exit codes and error codes, no TTY assumptions. Everything the MCP server will rely on. Tests: section E.
9. **M8 – MCP server:** `snap mcp` over stdio with `list_templates`, `describe_template`, `plan_project`, `create_project` (no hooks yet), `capture_template`, `save_template`; template resources; `new-project` prompt; path-safety checks. Tests: an MCP client test harness calling every tool; `create_project` output byte-identical to `snap new` with the same inputs.
10. **M9 – Hooks:** add the `RunHook` op, the `HookRunner` port and adapter together. CLI asks for confirmation the first time a template version's hooks run (remember the approval per template + version); MCP requires `allow_hooks: true`. Define failure behavior: files stay, failed hook is reported with its exit code and output, Snap exits non-zero. Tests: hook succeeds, hook fails, hooks declined, MCP call without `allow_hooks` never runs them.
11. **M10 – Optional blocks:** `[[optional]]` in `snap.toml`, a filter stage in the render pipeline, `--with docker` / `--without docker` flags, toggles shown in `describe_template`. Record the chosen toggles in `.snap/origin.toml` (required later by `snap update`). Tests: each toggle on/off produces exactly the expected file set.
12. **M11 – Remote templates:** `snap new gh:user/repo`, any git URL, `@tag` or `@commit` pinning, a local cache, `origin.toml` records the URL + resolved commit. Tests: pinned version stays reproducible after the remote changes; offline use works from cache.
13. **M12 – TUI:** run `snap` with no arguments: browse templates, fill variables, toggle optional blocks, live file-tree preview, confirm. Built only on existing use cases (no new core logic). Tests: model-level unit tests of the Bubble Tea state machine.
14. **M13 – Agent setup + release:** make sure `CLAUDE.md`, `.claude/` and `.mcp.json` are captured and renamed like normal files; seed 3–5 example templates; GoReleaser with Homebrew and Scoop; docs site; demo GIF. **Gate 2:** a Claude Code session creates a project from a template end to end via MCP with no manual fixes.

### v2 milestones (update and quality)

15. **M14 – Template versioning:** semver in `snap.toml`, `snap template bump`, the local store keeps previous versions (remote templates use git tags). Needed so an old base can be regenerated.
16. **M15 – Base regeneration:** from a project's `origin.toml` (template, version, variables, toggles), regenerate exactly what was originally created, in memory. Tests: regenerated base equals the original output byte for byte.
17. **M16 – `snap update`:** three-way merge between the old base (M15), the new template version, and the current project. Handles added, deleted and renamed files; conflicts are written as conflict markers and listed; `--dry-run` shows the diff; `origin.toml` is updated only on success. Tests: clean update, conflicting local edit, file deleted locally, file renamed in template.
18. **M17 – `snap test`:** generate a template into a temp dir with sample variables and run a check command declared in `snap.toml` (for example `go test ./...`). Exit code usable in CI. Lets template authors catch broken templates before users do.
19. **M18 – Local drift report:** `snap status <dirs…>` lists which projects came from which template and version and which are behind. This is the local groundwork for the paid drift dashboard; the hosted registry itself stays out of this repo.

### Success metrics (3 months after launch, hypotheses)

| Metric | Target |
| --- | --- |
| GitHub stars | 500+ |
| Weekly active installs | 200+ |
| Templates captured per active user | 2+ |
| Inbound team-sharing requests | 5+ |

## 9. Business model

Open-core: CLI fully open source (MIT/Apache-2.0) forever, including capture, new, TUI, MCP, git sharing and update. Later paid hosted Team registry (private org templates, versioning, drift dashboard, SSO, audit). Never move free features behind a paywall. The paid registry is a separate service and is out of scope for this repo for now.
