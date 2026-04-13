# TODOs — gorepos

---

## Round 2 — SE Team Review (2026-04-12, PR #4 `arch-rev`)

### Security — Must Fix Before Merge

- [ ] **SEC-C1: Block dangerous env vars in `buildEnvironment`** — denylist `GIT_SSH_COMMAND`, `GIT_PROXY_COMMAND`, `GIT_EXEC_PATH`, `GIT_ASKPASS`, `LD_PRELOAD`, `DYLD_INSERT_LIBRARIES`, `PATH`. Add check to both `buildEnvironment` and `ValidateConfig`. (`manager.go:272-280`)

- [ ] **SEC-C2: Fix path traversal in `getRepoPath`** — after `filepath.Join`, verify resolved path starts with `basePath`. Reject absolute paths from remote includes or constrain to a permitted root. (`manager.go:259-268`, `validation.go:96-98`)

- [ ] **SEC-C3: Add URL scheme allowlist** — permit only `https://`, `ssh://`, SCP-syntax `git@`. Block `http://`, `file://`, `git://`, RFC-1918/link-local hosts. (`validation.go:91-93`)

- [ ] **SEC-H1: Validate `user`/`email` in config load path** — call `validateUserName`/`validateEmail` from `ValidateConfig` and `validatePartialConfig` for every identity field. Reject values starting with `-`. (`validation.go`, `setup.go`)

- [ ] **SEC-H1b: Surface `git config` identity errors** — replace `_ = exec.Command(...).Run()` with proper error handling in `Clone`. (`manager.go:64-69`)

- [ ] **SEC-H2: Disable redirect following in `LoadRemoteConfig`** — set `CheckRedirect` to return `http.ErrUseLastResponse`. Add private IP range blocklist. (`loader.go:298-333`)

### Architecture — Must Fix Before Merge

- [ ] **C-1: Swap validate/defaults order in `LoadConfigWithDetails`** — move `setDefaults` before `ValidateConfig` in `loader.go:56-83`. This fixes `status`/`repos`/`validate`/`groups` failing for configs using default workers/timeout. (Also fixes Copilot comment #6)

- [ ] **C-2: Fix cycle detection key in graph builder** — use `include.Repo + "@" + include.Ref + ":" + include.GetFile()` instead of `include.String()` in `builder.go:145,189`. (Also fixes Copilot comment #7)

- [ ] **C-3: Remove `ctx.Done()` branch from executor result send** — always send the result; the channel is sized to `len(operations)`. (`pool.go:83-88`)

### Code Review — Bugs to Fix Before Merge

- [ ] **CR-H1: Fix `resolveGitHub` to use `u.Hostname()`** — replace hardcoded `raw.githubusercontent.com` with the hostname from the parsed URL (as `resolveGitLab` already does). Fixes GHES support. (`platform.go:110`)

- [ ] **CR-M1: Add context to `LoadRemoteConfigViaGit`** — add `ctx context.Context` parameter and use `exec.CommandContext` for all git calls inside. (`loader.go:340`)

- [ ] **CR-L1: Wire version variable into binary** — add `var version = "dev"` in `main.go` and assign to `rootCmd.Version`. (`build.sh:353`, `main.go`)

- [ ] **CR-L2: Fix build.sh debug echo to stderr** — add `>&2` to both verbose debug `echo` lines. (`build.sh:366`)

### Architecture — Fix Soon (High)

- [ ] **H-1: Consolidate two config loading paths** — align `LoadConfigWithDetails` to build on top of `BuildGraph` so all commands see the same repository precedence. Document the canonical merge rule.

- [ ] **H-2: Replace `git reset --hard` with `git pull --ff-only`** — or add `AheadBehind.Ahead > 0` guard before resetting in `Update`. Prevents data loss. (`manager.go:117-133`)

- [ ] **H-3: Honour or remove `Operation.Context`** — either use `op.Context` per worker or remove the field and document pool-wide cancellation only. (`types.go:174`, `pool.go:71`)

- [ ] **H-4: Implement or remove dead `CredentialConfig` fields** — SSHKeyPath/GitCredHelper/TokenEnvVar are accepted but never applied. Implement (`GIT_SSH_COMMAND` injection for SSH key) or remove before users build configs around them. (`types.go:161-167`)

- [ ] **H-5: Remove hardcoded `"configs"` sentinel in `extractHierarchyPath`** — derive hierarchy from include relationship or make base directory name configurable. (`builder.go:264-298`)

- [ ] **H-6: Split `GraphQuery` into `GraphReader` + `GraphMutator`** — 25-method interface with mutations blocks testability. Commands should accept the read-only interface. (`pkg/graph/types.go:142-188`)

### Architecture — Medium Priority

- [ ] **M-1: Make `GetMergedConfig` deterministic** — sort config nodes before iterating to prevent non-deterministic global settings from map iteration. (`builder.go:779`)

- [ ] **M-2: Fix `resolveAzureDevOps` `versionType`** — detect commit hashes (use existing `looksLikeCommitHash`) and tags to set correct `versionType`. (`platform.go:144-147`; Also Copilot CR-H2)

- [ ] **M-3: Raise `looksLikeCommitHash` minimum to 40 chars** — 7-char hex branch names are misidentified as commit hashes. (`loader.go:410-420`)

- [ ] **M-4: Add remote config caching in `Loader`** — `sync.Map` keyed on `repoURL+ref+filePath` to avoid N×clone on repeated includes. (`loader.go:340-407`)

- [ ] **M-5: Inject `RepositoryManager` into `ReposCommand`** — remove duplicated git execution in `repos.go:259-344`; call `manager.Status()` instead.

- [ ] **M-6: Delete `GetContextRepositoryNames`** — exported dead code with wrong bidirectional prefix logic. (`helpers.go:83-122`)

- [ ] **M-7: Remove `filterRepositoriesByContext` wrapper from `main.go`** — call `commands.FilterRepositoriesByContext` directly. (`main.go:179-181`)

- [ ] **SEC-M1: Add file size limit to local YAML reads** — use `io.LimitReader` (10 MB) for `os.ReadFile` on config files. (`loader.go:117`, `builder.go:303`)

- [ ] **SEC-M4: Guard `Pool.Execute` against concurrent calls** — document or enforce that `Execute` is not concurrency-safe; prevent `cancel` overwrite. (`pool.go:29-34`)

- [ ] **CR-L3: Clarify Gitea/Forgejo support in `PlatformEntry` docs** — either add `"gitea"` type (using GitLab-style resolver) or remove from comment. (`types.go:158`)

---

## Round 1 — Previous Findings (2026-04-09 / 2026-04-10)

## Critical

- [x] **Wire executor pool end-to-end** (Finding #5) — Fixed 2026-04-10

- [x] **Fix `GetMergedConfig()` — never sets Version field** (Finding #16) — Fixed 2026-04-10
- [x] **Fix `GetMergedConfig()` — Timeout: 300 nanoseconds** (Finding #17) — Fixed 2026-04-10

## High

- [x] **Split `setup` into `init` + `setup` wizard commands** (Finding #43) — Fixed 2026-04-10
- [x] **Add structured include entries with git ref support** (Finding #44) — Fixed 2026-04-10
- [x] **Git-native auth, git startup check, per-repo git identity** (Finding #45) — Fixed 2026-04-10

- [x] **Fix graph builder URL include handling** (Finding #18) — Fixed 2026-04-10
- [x] **Fix cross-file duplicate repo name handling in graph path** (Finding #19) — Fixed 2026-04-10

- [x] **Consolidate the two loading paths** (Finding #20) — Fixed 2026-04-10

- [x] **Remove hardcoded org names from context filtering** (Finding #21) — Fixed 2026-04-10 (removed with utils.go deletion)

- [x] **Fix `status` command `workers` flag guard** (Finding #22) — Fixed 2026-04-10
- [x] **Fix `setup` command ignoring `--dry-run` flag** (Finding #41) — Fixed 2026-04-10
- [x] **Change default config path to `~/.gorepos/`** (Finding #42) — Fixed 2026-04-10

## Medium

- [x] **Fix `setDefaults()` applied prematurely to included files** (Finding #23) — Fixed 2026-04-10
- [x] **Add `io.LimitReader` for remote config body** (Finding #24) — Fixed 2026-04-10

- [x] **Process `includes:` in remote configs** (Finding #25) — Fixed 2026-04-10

- [x] **Add bounds check before `FileHierarchy[0]`** (Finding #26) — Fixed 2026-04-10
- [x] **Handle errors from `createTagRelationship` / `createLabelRelationship`** (Finding #27) — Fixed 2026-04-10
- [x] **Fix `git status --porcelain` filename parsing for spaces** (Finding #28) — Fixed 2026-04-10
- [x] **Add context to git calls in `repos.go`** (Finding #29) — Fixed 2026-04-10
- [x] **Sort repositories after merge** (Finding #30) — Fixed 2026-04-10

- [x] **Unify `filterRepositoriesByContext` implementations** (Finding #31) — Fixed 2026-04-10

- [x] **Add test coverage for untested packages** (Finding #11) — Partial fix 2026-04-10
  - `pkg/types` — IncludeEntry YAML marshal/unmarshal + all methods ✓
  - `internal/commands` — FilterRepositoriesByContext + GetContextRepositoryNames ✓
  - `cmd/gorepos`, `internal/display` — deferred (require CLI/golden-file test infrastructure)

## Low

- [x] **Remove dead display functions from `main.go`** (Finding #32) — Fixed 2026-04-10
- [x] **Extract shared `loadConfigWithVerbose` helper** (Finding #33) — Fixed 2026-04-10
- [x] **Extract shared `getContextRepositoryNames`** (Finding #34) — Fixed 2026-04-10
- [x] **Fix variable shadowing of Cobra command vars** (Finding #35) — Fixed 2026-04-10
- [x] **Remove dead `internal/config/display.go`** (Finding #36) — Fixed 2026-04-10
- [x] **Fix nil error print in status** (Finding #37) — Fixed 2026-04-10
- [x] **Clean up double `os.Stat` in `Exists()`** (Finding #38) — Fixed 2026-04-10
- [x] **Sort group names before display** (Finding #39) — Fixed 2026-04-10
- [x] **Fix build.sh portability on macOS** (Findings #12, #40) — Fixed 2026-04-10
- [x] **Normalize path handling consistently** (Finding #13) — Fixed 2026-04-10

- [x] **Add CI/CD pipeline** (Finding #14) — Fixed 2026-04-10

- [x] **Harden circular include detection against symlinks** (Finding #15) — Fixed 2026-04-10
