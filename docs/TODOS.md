# TODOs — gorepos

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
