# GoRepos – Copilot Instructions

## Build, Test & Run

All commands are run from inside `gorepos/`.

```bash
# Quick single binary (fast iteration)
go build -o gorepos cmd/gorepos/main.go

# Full build (current platform, all architectures)
./scripts/build.sh

# Build with content-based versioning (no git commits needed)
./scripts/build.sh --content-hash --target darwin

# Release build (all platforms, with tests)
./scripts/build.sh --target all --arch all --clean --test --version "v1.0.0"

# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/graph/...
go test ./internal/config/...

# Run a single test by name
go test ./pkg/graph/... -run TestQueryGetNodesByType
```

Build output: `dist/<os>-<arch>/gorepos-<version>/gorepos[.exe]`

On Windows, use `.\scripts\build.ps1` with `-Target`, `-Arch`, `-Clean`, `-Test`, `-Version` flags.

## Architecture Overview

The application has a layered data pipeline:

1. **Config loading** (`internal/config/loader.go`) — reads `gorepos.yaml`, recursively resolves `includes:` (local paths and HTTP URLs), detects circular includes, merges global tags/labels down to repo level. Produces a `ConfigLoadResult` with a `FileNode` hierarchy and a flat merged `Config`.

2. **Graph construction** (`pkg/graph/builder.go`) — converts the flat config into an in-memory graph of typed nodes (`Config`, `Repository`, `Group`, `Tag`, `Label`) connected by typed edges (`parent_child`, `defines`, `includes`, `tagged_with`, `labeled_with`). This is the authoritative representation used for querying and display.

3. **Context filtering** (`cmd/gorepos/main.go → filterRepositoriesByContext`) — compares CWD against `basePath`; when CWD is inside a managed repo subtree, only that subtree's repos are visible to the command. This filtering is *not* applied to `graph` or `validate`.

4. **Command execution** — each file in `internal/commands/` handles one CLI command, receiving the filtered repo set and delegating to `internal/repository` (git ops) or `internal/display` (tree rendering).

### Package layout

| Path | Role |
|------|------|
| `cmd/gorepos/main.go` | Cobra wiring, global flags, context filtering, `runXxx` dispatch |
| `internal/commands/` | One file per CLI command (`status`, `validate`, `graph`, `groups`, `repos`) |
| `internal/config/` | Config loading, validation, merging, setup, display helpers |
| `internal/display/` | Tree renderers (`basic_tree`, `validation_tree`, `context_tree`, `groups_tree`) |
| `internal/executor/` | Worker pool for parallel repo operations |
| `internal/repository/` | Git operations (clone, update, status) |
| `pkg/types/types.go` | Canonical data model shared across all packages |
| `pkg/graph/` | In-memory graph DB (`builder.go`, `query.go`) |

### Key types (`pkg/types/types.go`)

- `Config` — top-level YAML structure (version, includes, global, repositories, groups, templates)
- `Repository` — single repo entry with name, path, url, branch, tags (key-value), labels (string list), disabled
- `GlobalConfig` — basePath, workers, timeout, global tags/labels, credentials
- `Operation` / `Result` — unit of work for the executor pool

### Graph node and edge types (`pkg/graph/`)

**Node types:** `config`, `repository`, `group`, `tag`, `label`, `context`  
**Relation types:** `parent_child`, `defines`, `includes`, `tagged_with`, `labeled_with`, `contained_in`

Nodes are classified as **explicit** (from YAML) or **derived** (computed, e.g. groups auto-populated from includes).

## Key Conventions

### Command-specific filtering rules

| Command | Context filtering |
|---------|------------------|
| `status`, `update`, `clone` | Only repos in CWD subtree |
| `validate` | All config files, including invalid ones |
| `graph` | No filtering — always full graph |
| `groups`, `repos` | Context-aware |

### Config loading paths

Two paths exist and are both in active use:
- `loader.LoadConfig(path)` (graph path) — used by `update` and `clone` via `loadConfig()` in `main.go`. Builds full graph, applies `setDefaults`, validates. Returns flat `*types.Config`. Handles private remote `repo:` includes via `git clone --sparse`.
- `loader.LoadConfigWithDetails(path)` (flat merge path) — used by `status`, `validate`, `graph`, `groups`, `repos` via `commands.LoadConfigWithVerbose(cfgFile, verbose)`. Returns `ConfigLoadResult` with `FileNode` hierarchy and `ProcessedFiles` needed by display renderers.

Both paths now agree on defaults. Commands needing `FileHierarchy` for display must use `LoadConfigWithDetails`.

### Two config APIs, one public surface

`internal/config/config.go` exposes the public API. `loader.go` contains implementation. Always add new public entry points to `config.go`, not `loader.go` directly.

### Tags vs. Labels

- **Tags** — `map[string]interface{}` key-value pairs for metadata (e.g., `platform: github`)
- **Labels** — `[]string` simple categorical values (e.g., `["api", "backend"]`)

Both are inherited: global → config-level → repository-level. Repository-level values are merged with (not replaced by) parent values.

### Version embedding

Version is injected at link time by `-ldflags "-X main.version=<ver>"`. Never hardcode it. The build scripts handle detection automatically (git tag → commit+timestamp → `dev-<date>`). Use `--content-hash` during local development to avoid needing git commits. Note: `var version string` is not declared in source; the ldflags injection is handled entirely by the build scripts.

### Display package conventions

Each display module in `internal/display/` has a single responsibility and exposes a `printNodeXxx` function. Prefer adding a new module over adding complexity to an existing one. Shared types live in `display/types.go`.

### `Disabled` vs. absent repositories

`repo.Disabled = true` keeps the repo in the config and graph but skips git operations and marks it with `○` in the tree. Absent repos (not in YAML) are not represented at all.

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/go-playground/validator/v10` — struct-tag validation on config types

Go 1.24+ required (see `go.mod`).

## Tools & CI

### CI Workflow

- **Lint**: `golangci-lint-action@v9` — uses golangci-lint **v2**, which enables both `errcheck` and `staticcheck` by default
- **Build matrix**: only runs after `test` and `lint` succeed (`needs: [test, lint]`)
- **Actions versions**: `checkout@v6`, `setup-go@v6`, `golangci-lint-action@v9` (Node.js 24 compatible)

### Lint Conventions (golangci-lint v2)

| Rule | Pattern |
|------|---------|
| `errcheck` — unchecked inline return | `_ = f.Close()` |
| `errcheck` — unchecked deferred return | `defer func() { _ = f.Close() }()` — **not** `defer f.Close()` |
| `errcheck` — multi-return | `_, _ = fmt.Sscanf(...)` |
| `ST1005` (staticcheck) — error string casing | Error strings must be lowercase: `"azure devops..."` not `"Azure DevOps..."` |
| `SA9003` (staticcheck) — empty branch | Never leave `if` / `else` bodies empty |
| `QF1001` (staticcheck) — De Morgan's law | Apply automatically when flagged |
| `QF1002` (staticcheck) — tagged switch | Use `switch x { case val: }` not `switch { case x == val: }` |
| `unused` | No unexported functions or variables that are never called |
