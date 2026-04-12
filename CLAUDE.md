# CLAUDE.md — gorepos

Go CLI tool for managing multiple git repositories in parallel.

## Commands

All commands are run from inside `gorepos/`.

```bash
# Quick single binary (fast iteration)
go build -o gorepos cmd/gorepos/main.go

# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/graph/...
go test ./internal/config/...

# Run a single test by name
go test ./pkg/graph/... -run TestQueryGetNodesByType
```

### Build (Bash — macOS/Linux)

```bash
./scripts/build.sh                                                              # All platforms, all architectures
./scripts/build.sh --content-hash --target darwin --arch arm64                  # Local dev, content-based version
./scripts/build.sh --target all --arch all --clean --test --version "v1.0.0"    # Release build
```

### Build (PowerShell — Windows)

```powershell
.\scripts\build.ps1                                                          # All platforms, all architectures
.\scripts\build.ps1 -ContentHash -Target darwin                              # Local dev, content-based version
.\scripts\build.ps1 -Target all -Arch all -Clean -Test -Version "v1.0.0"     # Release build
```

Build output: `dist/<os>-<arch>/gorepos-<version>/gorepos[.exe]`

### Testing (PowerShell only)

The test framework lives in `scripts/` and has no bash equivalent yet.

```powershell
# Run the shared test framework against a config + base path
.\scripts\test_local.ps1 -ConfigFile <path-to-gorepos.yaml> -BasePath <repo-dir> -TestName "mytest"

# Key flags
#   -Setup           Run 'gorepos setup --force' first
#   -Groups          Test group operations
#   -Clone           Test clone operations
#   -Update          Test update operations
#   -SkipValidate    Skip config validation phase
#   -VerboseOutput   Show full command output
```

Pre-built test environments (Windows paths, wrappers around `test_local.ps1`):
- `scripts/local/grtest1.ps1` — default OneDrive config
- `scripts/local/grtest2.ps1` — custom config path
- `scripts/local/grtest3.ps1` — example template

---

## Architecture

The application is a 4-stage data pipeline:

1. **Config loading** (`internal/config/loader.go`) — reads `gorepos.yaml`, recursively resolves `includes:` (local paths and HTTP URLs), detects circular includes, merges global tags/labels down to repo level. Returns `ConfigLoadResult` with a `FileNode` hierarchy and a flat merged `Config`.

2. **Graph construction** (`pkg/graph/builder.go`) — converts the flat config into an in-memory graph of typed nodes (`Config`, `Repository`, `Group`, `Tag`, `Label`) connected by typed edges. This is the authoritative representation for querying and display.

3. **Context filtering** (`cmd/gorepos/main.go -> filterRepositoriesByContext`) — compares CWD against `basePath`; when CWD is inside a managed repo subtree, only that subtree's repos are visible. Not applied to `graph` or `validate`.

4. **Command execution** — each file in `internal/commands/` handles one CLI command, delegating to `internal/repository` (git ops) or `internal/display` (tree rendering).

### Package layout

| Path | Role |
|------|------|
| `cmd/gorepos/main.go` | Cobra wiring, global flags, context filtering, `runXxx` dispatch |
| `internal/commands/` | One file per CLI command (`status`, `update`, `clone`, `validate`, `graph`, `groups`, `repos`) |
| `internal/config/` | Config loading, validation, merging, setup wizard |
| `internal/commands/helpers.go` | Shared config loading (`LoadConfigWithVerbose`) and context filtering (`GetContextRepositoryNames`) |
| `internal/display/` | Tree renderers (`basic_tree`, `validation_tree`, `context_tree`, `groups_tree`) |
| `internal/executor/` | Channel-based worker pool for parallel git operations |
| `internal/repository/` | Git operations (clone, update, status) |
| `pkg/types/types.go` | Canonical data model shared across all packages |
| `pkg/graph/` | In-memory graph DB: `builder.go` (construction), `query.go` (traversal) |

### Key types

- **`Config`** — top-level YAML structure (version, includes, global, repositories, groups, templates)
- **`Repository`** — name, path, url, branch, tags (key-value map), labels (string list), disabled flag
- **`GlobalConfig`** — basePath, workers, timeout, global tags/labels, credentials
- **`Operation`** / **`Result`** — unit of work for the executor pool

**Graph node types:** `config`, `repository`, `group`, `tag`, `label`, `context`
**Graph relation types:** `parent_child`, `defines`, `includes`, `tagged_with`, `labeled_with`, `contained_in`
Nodes are classified as **explicit** (from YAML) or **derived** (computed, e.g. auto-populated groups).

---

## Key Conventions

### Command filtering rules

| Command | Context filtering |
|---------|------------------|
| `status`, `update`, `clone` | Only repos in CWD subtree |
| `validate` | All config files, no filtering |
| `graph` | Always full graph, no filtering |
| `groups`, `repos` | Context-aware |

### Config loading paths

Two paths exist and are both in active use:

- **`loader.LoadConfig(path)`** (graph path) — used by `update` and `clone` commands. Builds a full graph, calls `GetMergedConfig()`, applies `setDefaults`, validates. Returns a flat `*types.Config`. Remote `repo:` includes are fetched via `git clone --sparse`.
- **`loader.LoadConfigWithDetails(path)`** (flat merge path) — used by `status`, `validate`, `graph`, `groups`, and `repos` commands via `LoadConfigWithVerbose`. Returns `ConfigLoadResult` with `FileNode` hierarchy and `ProcessedFiles` list needed by display renderers.

The two paths now agree on defaults (`workers=10`, `timeout=5m`, `branch=main`). The graph path handles remote includes and duplicate repos better. Commands that need `FileHierarchy` for display must use `LoadConfigWithDetails`.

Always add new public entry points to `internal/config/config.go`, not `loader.go` directly.

### Tags vs. Labels

- **Tags** — `map[string]interface{}` key-value pairs (e.g. `platform: github`)
- **Labels** — `[]string` categorical strings (e.g. `["api", "backend"]`)

Both are inherited: global -> config-level -> repository-level. Repository values are merged with (not replaced by) parent values.

### Version embedding

`version` is injected by `-ldflags "-X main.version=<ver>"`. Never hardcode it. Use `--content-hash` for local development to avoid needing git commits.

### Display package conventions

Each module in `internal/display/` has a single responsibility and exposes a `printNodeXxx` function. Add a new module rather than adding complexity to an existing one. Shared types live in `display/types.go`.

### `Disabled` vs. absent repositories

`repo.Disabled = true` keeps the repo in config and graph but skips git operations and marks it `○` in the tree. Absent repos (not in YAML) are not represented at all.

---

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/go-playground/validator/v10` — struct-tag validation on config types

Go 1.24+ required.

---

## Workflow: Findings & TODOs

- **Code reviews, bugs, issues, and feature ideas** must be documented in `docs/FINDINGS.md` with full detail (file paths, line numbers, severity, suggested fix).
- **Actionable tasks** arising from findings must be added to `docs/TODOS.md`.
- When a TODO is completed, mark it done in `docs/TODOS.md` and ~~strike through~~ the corresponding section in `docs/FINDINGS.md`.
