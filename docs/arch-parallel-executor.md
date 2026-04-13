# Architecture: Wire Executor Pool End-to-End for True Parallel Repository Operations

## Decision

Inject `types.RepositoryManager` into `executor.Pool` as a constructor parameter so that `executeOperation` can delegate to the real `Clone`, `Update`, and `Status` manager methods instead of returning stub strings. A single new field `StatusData *types.RepoStatus` is added to `types.Result` so that `status` operation results carry their structured data through the fan-out channel to the display loop — eliminating the current pattern in `commands/status.go` that calls `repoManager.Status()` sequentially in the result-consumption loop. `runUpdate` and `runClone` in `cmd/gorepos/main.go` replace their sequential for-loops with `exec.Execute()` + channel drain + error aggregation using `errors.Join`. This approach follows the existing `RepositoryManager` interface injection pattern already used everywhere else in the codebase (config loader, graph builder), avoids any change to the `types.Executor` interface signature, and requires the minimum number of file changes to reach real parallelism.

---

## Component Design

### `pkg/types/types.go`
**Purpose**: Canonical data model — extend `Result` to carry status-operation output.
**Interface**:
```go
// CHANGE: add StatusData field to Result
type Result struct {
    Repository *Repository
    Operation  string
    Success    bool
    Output     string
    Error      error
    Duration   time.Duration
    StartTime  time.Time
    StatusData *RepoStatus // NEW: populated only for "status" operations
}
```
**Dependencies**: none (leaf package)

---

### `internal/executor/pool.go`
**Purpose**: Worker pool that dispatches operations to the real `RepositoryManager` in parallel.
**Interface**:
```go
type Pool struct {
    workerCount int
    manager     types.RepositoryManager // NEW field
    workers     []*worker
    mu          sync.RWMutex
    started     bool
}

// CHANGED: now requires a RepositoryManager
func NewPool(workerCount int, manager types.RepositoryManager) *Pool

// UNCHANGED public interface (satisfies types.Executor):
func (p *Pool) Execute(ctx context.Context, operations []types.Operation) <-chan types.Result
func (p *Pool) SetWorkerCount(count int)
func (p *Pool) Shutdown(ctx context.Context) error

// REPLACED stub with real dispatch:
func (p *Pool) executeOperation(ctx context.Context, op *types.Operation) *types.Result
```

**`executeOperation` implementation**:
```go
func (p *Pool) executeOperation(ctx context.Context, op *types.Operation) *types.Result {
    result := &types.Result{
        Repository: op.Repository,
        Operation:  op.Command,
        StartTime:  time.Now(),
    }

    if ctx.Err() != nil {
        result.Error = ctx.Err()
        result.Success = false
        result.Duration = time.Since(result.StartTime)
        return result
    }

    switch op.Command {
    case "clone":
        err := p.manager.Clone(ctx, op.Repository)
        if err != nil {
            result.Error = fmt.Errorf("clone %s: %w", op.Repository.Name, err)
            result.Success = false
        } else {
            result.Output = fmt.Sprintf("cloned %s to %s", op.Repository.URL, op.Repository.Path)
            result.Success = true
        }

    case "update":
        err := p.manager.Update(ctx, op.Repository)
        if err != nil {
            result.Error = fmt.Errorf("update %s: %w", op.Repository.Name, err)
            result.Success = false
        } else {
            result.Output = fmt.Sprintf("updated %s", op.Repository.Name)
            result.Success = true
        }

    case "status":
        status, err := p.manager.Status(ctx, op.Repository)
        if err != nil {
            result.Error = fmt.Errorf("status %s: %w", op.Repository.Name, err)
            result.Success = false
        } else {
            result.StatusData = status
            result.Output = fmt.Sprintf("status of %s retrieved", op.Repository.Name)
            result.Success = true
        }

    default:
        result.Error = fmt.Errorf("unknown operation: %s", op.Command)
        result.Success = false
    }

    result.Duration = time.Since(result.StartTime)
    return result
}
```
**Dependencies**: `pkg/types`, `context`, `fmt`, `sync`, `time`

---

### `internal/executor/pool_test.go`
**Purpose**: Unit tests for `Pool` using an inline mock `RepositoryManager`.
**Interface**:
```go
// Inline test helper — no exported types
type mockRepositoryManager struct {
    cloneErr   error
    updateErr  error
    statusResp *types.RepoStatus
    statusErr  error
    calls      []string   // records method calls for assertion
    mu         sync.Mutex
}

func (m *mockRepositoryManager) Clone(ctx context.Context, repo *types.Repository) error
func (m *mockRepositoryManager) Update(ctx context.Context, repo *types.Repository) error
func (m *mockRepositoryManager) Status(ctx context.Context, repo *types.Repository) (*types.RepoStatus, error)
func (m *mockRepositoryManager) Execute(ctx context.Context, repo *types.Repository, command string, args ...string) (*types.Result, error)
func (m *mockRepositoryManager) Exists(repo *types.Repository) bool

// All NewPool() calls updated to: NewPool(n, &mockRepositoryManager{})

// New tests added:
// TestExecuteOperation_Clone_CallsManager
// TestExecuteOperation_Update_CallsManager
// TestExecuteOperation_Status_PopulatesStatusData
// TestExecuteOperation_Clone_ManagerError_PropagatesError
// TestExecuteOperation_Update_ManagerError_PropagatesError
// TestExecuteOperation_Status_ManagerError_PropagatesError
// TestExecute_AllResultsHaveDuration
// TestExecute_Parallel_AllComplete (verifies N ops complete with N workers)
```
**Dependencies**: `pkg/types`, `internal/executor`, `context`, `sync`, `testing`, `time`

---

### `cmd/gorepos/main.go`
**Purpose**: Cobra dispatch — replace sequential loops in `runUpdate` and `runClone` with parallel channel consumption.

**`runUpdate` change** — replace the sequential loop block and the `exec.Shutdown` return:
```go
// BEFORE (lines ~280-290):
for _, repo := range updatedRepos {
    fmt.Printf("Updating %s...", repo.Name)
    err := repoManager.Update(ctx, repo)
    if err != nil {
        fmt.Printf(" ERROR: %v\n", err)
    } else {
        fmt.Printf(" OK\n")
    }
}
return exec.Shutdown(ctx)

// AFTER:
results := exec.Execute(ctx, operations)
var errs []error
for result := range results {
    if result.Success {
        fmt.Printf("[%s] OK (%s)\n", result.Repository.Name, result.Duration.Round(time.Millisecond))
    } else {
        fmt.Printf("[%s] ERROR: %v\n", result.Repository.Name, result.Error)
        errs = append(errs, result.Error)
    }
}
if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
    errs = append(errs, shutdownErr)
}
return errors.Join(errs...)
```

**`runClone` change** — identical pattern to `runUpdate` above.

**Constructor change** — both commands change pool creation:
```go
// BEFORE:
exec := executor.NewPool(cfg.Global.Workers)

// AFTER:
exec := executor.NewPool(cfg.Global.Workers, repoManager)
```

**New import**: `"errors"` (stdlib, Go 1.20+, safe with Go 1.24)

**Dependencies**: `internal/executor`, `internal/repository`, `pkg/types`, `errors`, `time`

---

### `internal/commands/status.go`
**Purpose**: Status command handler — consume real `StatusData` from results channel instead of calling manager sequentially.

**Constructor change**:
```go
// BEFORE:
exec := executor.NewPool(cfg.Global.Workers)

// AFTER:
exec := executor.NewPool(cfg.Global.Workers, repoManager)
```

**Result loop change**:
```go
// BEFORE (~lines 91-128):
results := exec.Execute(ctx, operations)
for result := range results {
    fmt.Printf("\n%s:\n", result.Repository.Name)
    status, err := repoManager.Status(ctx, result.Repository) // sequential call
    if err != nil { ... }
    // display status fields
}

// AFTER:
results := exec.Execute(ctx, operations)
var errs []error
for result := range results {
    fmt.Printf("\n%s:\n", result.Repository.Name)
    if !result.Success || result.StatusData == nil {
        fmt.Printf("  Error: %v\n", result.Error)
        errs = append(errs, result.Error)
        continue
    }
    status := result.StatusData
    fmt.Printf("  Path: %s\n", status.Path)
    fmt.Printf("  Branch: %s\n", status.CurrentBranch)
    if status.IsClean {
        fmt.Printf("  Status: Clean\n")
    } else {
        fmt.Printf("  Status: %d uncommitted files\n", len(status.UncommittedFiles))
        if s.verbose {
            for _, file := range status.UncommittedFiles {
                fmt.Printf("    - %s\n", file)
            }
        }
    }
    if status.AheadBehind != nil {
        if status.AheadBehind.Ahead > 0 || status.AheadBehind.Behind > 0 {
            fmt.Printf("  Sync: %d ahead, %d behind\n", status.AheadBehind.Ahead, status.AheadBehind.Behind)
        } else {
            fmt.Printf("  Sync: Up to date\n")
        }
    }
}
if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
    errs = append(errs, shutdownErr)
}
return errors.Join(errs...)
```

**Remove** the now-unused `repoManager` reference in the result loop (the variable is still needed for pool construction).

**New import**: `"errors"`

**Dependencies**: `internal/executor`, `internal/repository`, `pkg/types`, `errors`

---

## Data Flow

```
CLI invocation: gorepos clone|update|status --parallel N
        │
        ▼
  main.go / commands/status.go
        │  loadConfig()
        │  repoManager := repository.NewManager(basePath)   ← real git executor
        │  exec := executor.NewPool(N, repoManager)          ← manager injected
        │  build []types.Operation{repo, command, ctx}
        │  [dry-run guard: print plan and return early]
        │
        ▼
  exec.Execute(ctx, operations)
        │  returns <-chan types.Result  (buffered, len=ops)
        │
        ├─ goroutine: feeds ops → jobChan (respects ctx.Done)
        │
        ├─ worker 0 ──┐
        ├─ worker 1 ──┤  each worker:
        ├─ ...         │    job := <-jobChan
        └─ worker N-1 ┘    result := pool.executeOperation(ctx, &job)
                            │
                            ├─ "clone"  → manager.Clone(ctx, repo)
                            ├─ "update" → manager.Update(ctx, repo)
                            └─ "status" → manager.Status(ctx, repo)
                                           └─ result.StatusData = *RepoStatus
                            results <- result
        │
        ▼
  caller: for result := range results
        │  result.Success == true  → print "[name] OK (Xms)"
        │  result.Success == false → print "[name] ERROR: ..." + collect error
        │  result.StatusData != nil (status cmd) → print branch/clean/ahead-behind
        │
        ▼
  exec.Shutdown(ctx)
        │  waits for any in-flight workers (none, channel already closed)
        │  returns nil or ctx.Err() on timeout
        ▼
  errors.Join(errs...)
        │  nil if all succeeded
        └─ non-nil → Cobra prints to stderr, exits with code 1

ERROR PATH:
  - ctx cancelled mid-flight: workers observe ctx.Done, send result{Error: ctx.Err}
  - manager returns error: result.Success=false, error wrapped with repo name
  - all errors collected into []error, joined and returned to Cobra
```

---

## Implementation Map

| Action | File | Description |
|--------|------|-------------|
| MODIFY | `pkg/types/types.go` | Add `StatusData *RepoStatus` field to `Result` struct |
| MODIFY | `internal/executor/pool.go` | Add `manager` field; update `NewPool` signature; replace `executeOperation` stub with real dispatch |
| MODIFY | `internal/executor/pool_test.go` | Add `mockRepositoryManager`; update all `NewPool(n)` → `NewPool(n, mock)`; add new tests for real dispatch and error propagation |
| MODIFY | `internal/commands/status.go` | Update `NewPool` call to pass `repoManager`; replace sequential `repoManager.Status()` loop with channel-driven `result.StatusData` consumption; add error aggregation |
| MODIFY | `cmd/gorepos/main.go` | Update `NewPool` calls in `runUpdate` and `runClone`; replace sequential for-loops with `exec.Execute()` + channel drain + `errors.Join` |

---

## Build Sequence

1. **`pkg/types/types.go`** — Add `StatusData *RepoStatus` to `Result`. No dependencies on other changed files. All other files compile against this type.

2. **`internal/executor/pool.go`** — Update `NewPool(workerCount int, manager types.RepositoryManager) *Pool`; add `manager types.RepositoryManager` field to `Pool` struct; remove stub body of `executeOperation` and replace with real `switch op.Command` dispatching to `p.manager.Clone/Update/Status`. This file now compiles against the updated `types.Result` (step 1).

3. **`internal/commands/status.go`** — Update `executor.NewPool` call to pass `repoManager` as second argument; rewrite result-consumption loop to read `result.StatusData` instead of calling `repoManager.Status()`; add `errors.Join` aggregation. Depends on step 1 (StatusData field) and step 2 (NewPool signature).

4. **`cmd/gorepos/main.go`** — Update `executor.NewPool` calls in `runUpdate` and `runClone` to pass `repoManager`; replace sequential for-loops with channel drain; add `errors.Join`. Depends on step 2 (NewPool signature). Import `"errors"` and `"time"`.

5. **`internal/executor/pool_test.go`** — Add `mockRepositoryManager` struct implementing `types.RepositoryManager`; update every `NewPool(n)` call to `NewPool(n, &mockRepositoryManager{})`; add table-driven tests for clone/update/status dispatch, error propagation, StatusData population, and true parallelism. Depends on steps 1 and 2.

---

## Test Strategy

### `internal/executor/pool_test.go` — Unit tests (mock manager, no git binary needed)

**New `mockRepositoryManager`** (implement all 5 interface methods; configurable error/response fields; thread-safe call recorder):
```go
type mockRepositoryManager struct {
    mu         sync.Mutex
    calls      []string
    cloneErr   error
    updateErr  error
    statusResp *types.RepoStatus
    statusErr  error
}
```

**Critical scenarios to add/update**:

| Test | Verifies |
|------|----------|
| `TestNewPool_WithManager` | Constructor stores manager, worker count |
| `TestExecuteOperation_Clone_CallsManager` | mock.Clone called exactly once; result.Success=true |
| `TestExecuteOperation_Clone_ManagerError_PropagatesError` | mock.Clone returns error → result.Success=false, result.Error non-nil, name in message |
| `TestExecuteOperation_Update_CallsManager` | mock.Update called; result.Success=true |
| `TestExecuteOperation_Update_ManagerError_PropagatesError` | error wrapping |
| `TestExecuteOperation_Status_PopulatesStatusData` | mock.Status returns RepoStatus → result.StatusData equals that value |
| `TestExecuteOperation_Status_ManagerError_PropagatesError` | result.StatusData=nil, error non-nil |
| `TestExecute_AllResultsHaveDuration` | result.Duration > 0 for all results |
| `TestExecute_Parallel_AllComplete` | 20 ops, 5 workers → exactly 20 results, all success |
| `TestExecute_ContextCancellation` (update) | still must not hang; some results may carry ctx.Err |
| `TestExecuteOperation_CancelledContext` (keep) | still valid, no change needed beyond mock injection |

**Existing passing tests** that only need `NewPool(n)` → `NewPool(n, &mockRepositoryManager{})`:
- `TestNewPool_InitialState`, `TestSetWorkerCount_*`, `TestExecute_StartsPool`, `TestExecute_ProcessesAllOperations`, `TestExecute_EmptyOperations`, `TestExecute_ReturnsRepository`, `TestExecute_ReturnsOperationName`, `TestShutdown_NotStarted`, `TestExecute_MultipleCallsSerially`

> **Note on `TestExecute_KnownCommands_AreSuccessful`**: this test currently passes because the stub always sets `Success=true`. After the change it will still pass because `mockRepositoryManager` defaults to `nil` errors (i.e., success). Keep it as-is after updating the constructor call.

### Integration test approach
The existing `internal/repository/manager_test.go` already uses `t.TempDir()` + real `git` binary for integration testing of the manager. End-to-end parallel testing of `Pool` + real `Manager` belongs in a new `internal/executor/pool_integration_test.go` (build tag `//go:build integration`) to keep CI fast. That file is **out of scope** for this feature but is the right next step.

### `go.mod` broken replace directive
The `replace github.com/gabriel-vasile/mimetype => /home/user/mimetype-stub` directive will cause `go test` to fail on any machine that does not have `/home/user/mimetype-stub`. None of the new or modified files import `mimetype` or its transitive parents directly, so this issue pre-exists and is out of scope. However, if CI is broken for unrelated reasons, remove or stub the replace directive before running tests.

---

## Critical Notes

### Breaking change to `NewPool` constructor
`NewPool` gains a required second argument. Every call site must be updated in the same PR:
- `cmd/gorepos/main.go`: `runUpdate` (line ~307) and `runClone` (line ~307)
- `internal/commands/status.go`: `Execute` method (line ~52)
- `internal/executor/pool_test.go`: all test functions

The `types.Executor` interface (`Execute`, `SetWorkerCount`, `Shutdown`) is **not changed**. Any future caller that constructs a `Pool` via the interface will need to use the concrete constructor, which is already the existing pattern.

### `errors.Join` requires Go 1.20+
The module declares `go 1.24.7` — `errors.Join` is safe to use.

### Thread safety of progress output
`fmt.Printf` calls in the result-consumption loop in `main.go` and `status.go` execute in the **caller goroutine** (the `for result := range results` loop is single-threaded). Workers only write to the buffered `results` channel. There is no concurrent `fmt.Printf` — no mutex needed around progress output.

### Result channel buffer size
The existing `Execute` pre-allocates `make(chan types.Result, len(operations))`. This means workers never block on sending — they complete and release their goroutine as soon as the manager call returns. This is correct and intentional: it prevents deadlock if the caller drains slowly.

### `Shutdown` after channel drain
`exec.Shutdown(ctx)` is called **after** `for result := range results` completes (channel closed). By the time the channel closes, all worker goroutines have exited the `worker()` function. `Shutdown` in the current implementation iterates `p.workers` which are the pre-allocated structs from `start()` — their `wg.Wait()` will return immediately. This is a no-op in practice but is kept for lifecycle correctness and future pool reuse patterns.

### Error aggregation semantics
`errors.Join` returns nil if all errs are nil. Cobra's `RunE` returns this directly. A single repository failure causes exit code 1 while all other repositories have already printed their results (streaming progress output, not buffered). This matches the design constraint §8.

### Dry-run guard (constraint §6)
Both `runUpdate` and `runClone` already return before constructing operations when `dryRun` is true. The `executor.NewPool` call is placed **before** the dry-run guard in the current code; it should remain there (pool construction is cheap — no goroutines are started until `Execute` is called, due to the `!p.started` lazy-init in `Execute`).

### `manager.Clone` called on already-existing repo returns error
`repository.Manager.Clone` returns `fmt.Errorf("repository already exists at %s", ...)` if the repo exists. `runClone` already guards with `if repoManager.Exists(repo) { continue }` before building operations, so this error path is not reached in practice. The guard must be preserved.
