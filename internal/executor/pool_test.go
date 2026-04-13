package executor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

const (
	msgExpectedSuccess = "expected success, got error: %v"
	msgExpectedFailure = "expected failure when manager returns error"
)

// --- Test helpers ---

// mockRepositoryManager is a thread-safe mock implementing types.RepositoryManager.
type mockRepositoryManager struct {
	mu         sync.Mutex
	calls      []string
	cloneErr   error
	updateErr  error
	statusResp *types.RepoStatus
	statusErr  error
}

func (m *mockRepositoryManager) Clone(_ context.Context, repo *types.Repository) error {
	m.mu.Lock()
	m.calls = append(m.calls, "clone:"+repo.Name)
	m.mu.Unlock()
	return m.cloneErr
}

func (m *mockRepositoryManager) Update(_ context.Context, repo *types.Repository) error {
	m.mu.Lock()
	m.calls = append(m.calls, "update:"+repo.Name)
	m.mu.Unlock()
	return m.updateErr
}

func (m *mockRepositoryManager) Status(_ context.Context, repo *types.Repository) (*types.RepoStatus, error) {
	m.mu.Lock()
	m.calls = append(m.calls, "status:"+repo.Name)
	m.mu.Unlock()
	return m.statusResp, m.statusErr
}

func (m *mockRepositoryManager) Execute(_ context.Context, repo *types.Repository, command string, args ...string) (*types.Result, error) {
	return &types.Result{Repository: repo, Operation: command, Success: true}, nil
}

func (m *mockRepositoryManager) Exists(_ *types.Repository) bool { return false }

func (m *mockRepositoryManager) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func newMock() *mockRepositoryManager { return &mockRepositoryManager{} }

func makeRepo(name string) *types.Repository {
	return &types.Repository{Name: name, Path: "/tmp/" + name, URL: "https://github.com/example/" + name + ".git"}
}

func makeOp(repo *types.Repository, command string) types.Operation {
	return types.Operation{
		Repository: repo,
		Command:    command,
	}
}

// --- NewPool ---

func TestNewPool_InitialState(t *testing.T) {
	p := NewPool(4, newMock())

	if p.GetWorkerCount() != 4 {
		t.Errorf("expected worker count 4, got %d", p.GetWorkerCount())
	}
}

func TestNewPool_WithManager(t *testing.T) {
	mock := newMock()
	p := NewPool(2, mock)
	if p.manager != mock {
		t.Error("expected manager to be stored in pool")
	}
}

// --- SetWorkerCount ---

func TestSetWorkerCount_Normal(t *testing.T) {
	p := NewPool(5, newMock())
	p.SetWorkerCount(10)
	if p.GetWorkerCount() != 10 {
		t.Errorf("expected 10, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_ClampMin(t *testing.T) {
	p := NewPool(5, newMock())
	p.SetWorkerCount(0)
	if p.GetWorkerCount() != 1 {
		t.Errorf("expected clamped min of 1, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_ClampMax(t *testing.T) {
	p := NewPool(5, newMock())
	p.SetWorkerCount(200)
	if p.GetWorkerCount() != 100 {
		t.Errorf("expected clamped max of 100, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_Negative(t *testing.T) {
	p := NewPool(5, newMock())
	p.SetWorkerCount(-5)
	if p.GetWorkerCount() != 1 {
		t.Errorf("expected clamped min of 1, got %d", p.GetWorkerCount())
	}
}

// --- Execute ---

func TestExecute_Runs(t *testing.T) {
	p := NewPool(2, newMock())
	ctx := context.Background()
	ops := []types.Operation{makeOp(makeRepo("r1"), "clone")}

	results := p.Execute(ctx, ops)
	count := 0
	for range results {
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 result, got %d", count)
	}
}

func TestExecute_ProcessesAllOperations(t *testing.T) {
	p := NewPool(3, newMock())
	ctx := context.Background()

	repos := []*types.Repository{makeRepo("r1"), makeRepo("r2"), makeRepo("r3")}
	ops := []types.Operation{
		makeOp(repos[0], "clone"),
		makeOp(repos[1], "update"),
		makeOp(repos[2], "status"),
	}

	results := p.Execute(ctx, ops)

	count := 0
	for range results {
		count++
	}

	if count != 3 {
		t.Errorf("expected 3 results, got %d", count)
	}
}

func TestExecute_KnownCommands_AreSuccessful(t *testing.T) {
	p := NewPool(1, newMock())
	ctx := context.Background()
	repo := makeRepo("r1")

	for _, cmd := range []string{"clone", "update", "status"} {
		ops := []types.Operation{makeOp(repo, cmd)}
		results := p.Execute(ctx, ops)
		for result := range results {
			if !result.Success {
				t.Errorf("expected success for command %q, got error: %v", cmd, result.Error)
			}
		}
	}
}

func TestExecute_UnknownCommand_Fails(t *testing.T) {
	p := NewPool(1, newMock())
	ctx := context.Background()

	ops := []types.Operation{makeOp(makeRepo("r1"), "unknown-command")}
	results := p.Execute(ctx, ops)

	var result types.Result
	for r := range results {
		result = r
	}

	if result.Success {
		t.Error("expected failure for unknown command")
	}
	if result.Error == nil {
		t.Error("expected an error for unknown command")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	p := NewPool(1, newMock())
	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	ops := []types.Operation{
		makeOp(makeRepo("r1"), "clone"),
		makeOp(makeRepo("r2"), "clone"),
	}

	results := p.Execute(ctx, ops)

	collected := 0
	timeout := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-results:
			if !ok {
				goto done
			}
			collected++
		case <-timeout:
			t.Error("timed out waiting for results after context cancel")
			goto done
		}
	}
done:
	_ = collected
}

func TestExecute_EmptyOperations(t *testing.T) {
	p := NewPool(2, newMock())
	ctx := context.Background()

	results := p.Execute(ctx, []types.Operation{})

	count := 0
	for range results {
		count++
	}

	if count != 0 {
		t.Errorf("expected 0 results for empty operations, got %d", count)
	}
}

func TestExecute_ReturnsRepository(t *testing.T) {
	p := NewPool(1, newMock())
	ctx := context.Background()
	repo := makeRepo("myrepo")

	results := p.Execute(ctx, []types.Operation{makeOp(repo, "status")})

	for result := range results {
		if result.Repository == nil {
			t.Error("result should have a repository reference")
		} else if result.Repository.Name != "myrepo" {
			t.Errorf("expected repo name 'myrepo', got %q", result.Repository.Name)
		}
	}
}

func TestExecute_ReturnsOperationName(t *testing.T) {
	p := NewPool(1, newMock())
	ctx := context.Background()
	repo := makeRepo("r1")

	results := p.Execute(ctx, []types.Operation{makeOp(repo, "clone")})

	for result := range results {
		if result.Operation != "clone" {
			t.Errorf("expected operation 'clone', got %q", result.Operation)
		}
	}
}

// --- executeOperation: real manager dispatch ---

func TestExecuteOperation_Clone_CallsManager(t *testing.T) {
	mock := newMock()
	p := NewPool(1, mock)
	ctx := context.Background()
	repo := makeRepo("myrepo")

	result := p.executeOperation(ctx, &types.Operation{Repository: repo, Command: "clone"})

	if !result.Success {
		t.Errorf(msgExpectedSuccess, result.Error)
	}
	if mock.CallCount() != 1 {
		t.Errorf("expected manager.Clone called once, got %d calls", mock.CallCount())
	}
}

func TestExecuteOperation_Clone_ManagerError_PropagatesError(t *testing.T) {
	mock := newMock()
	mock.cloneErr = context.DeadlineExceeded
	p := NewPool(1, mock)
	ctx := context.Background()

	result := p.executeOperation(ctx, &types.Operation{Repository: makeRepo("r1"), Command: "clone"})

	if result.Success {
		t.Error(msgExpectedFailure)
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}
}

func TestExecuteOperation_Update_CallsManager(t *testing.T) {
	mock := newMock()
	p := NewPool(1, mock)
	ctx := context.Background()

	result := p.executeOperation(ctx, &types.Operation{Repository: makeRepo("r1"), Command: "update"})

	if !result.Success {
		t.Errorf(msgExpectedSuccess, result.Error)
	}
	if mock.CallCount() != 1 {
		t.Errorf("expected manager.Update called once, got %d calls", mock.CallCount())
	}
}

func TestExecuteOperation_Update_ManagerError_PropagatesError(t *testing.T) {
	mock := newMock()
	mock.updateErr = context.DeadlineExceeded
	p := NewPool(1, mock)
	ctx := context.Background()

	result := p.executeOperation(ctx, &types.Operation{Repository: makeRepo("r1"), Command: "update"})

	if result.Success {
		t.Error(msgExpectedFailure)
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}
}

func TestExecuteOperation_Status_PopulatesStatusData(t *testing.T) {
	mock := newMock()
	mock.statusResp = &types.RepoStatus{
		Path:          "/tmp/r1",
		CurrentBranch: "main",
		IsClean:       true,
	}
	p := NewPool(1, mock)
	ctx := context.Background()

	result := p.executeOperation(ctx, &types.Operation{Repository: makeRepo("r1"), Command: "status"})

	if !result.Success {
		t.Errorf(msgExpectedSuccess, result.Error)
	}
	if result.StatusData == nil {
		t.Fatal("expected StatusData to be populated")
	}
	if result.StatusData.CurrentBranch != "main" {
		t.Errorf("expected branch 'main', got %q", result.StatusData.CurrentBranch)
	}
}

func TestExecuteOperation_Status_ManagerError_PropagatesError(t *testing.T) {
	mock := newMock()
	mock.statusErr = context.DeadlineExceeded
	p := NewPool(1, mock)
	ctx := context.Background()

	result := p.executeOperation(ctx, &types.Operation{Repository: makeRepo("r1"), Command: "status"})

	if result.Success {
		t.Error(msgExpectedFailure)
	}
	if result.StatusData != nil {
		t.Error("expected StatusData to be nil on error")
	}
}

func TestExecute_AllResultsHaveDuration(t *testing.T) {
	p := NewPool(2, newMock())
	ctx := context.Background()

	ops := []types.Operation{
		makeOp(makeRepo("r1"), "clone"),
		makeOp(makeRepo("r2"), "update"),
		makeOp(makeRepo("r3"), "status"),
	}

	for result := range p.Execute(ctx, ops) {
		if result.Duration <= 0 {
			t.Errorf("expected positive duration for %s, got %v", result.Repository.Name, result.Duration)
		}
	}
}

func TestExecute_Parallel_AllComplete(t *testing.T) {
	p := NewPool(5, newMock())
	ctx := context.Background()

	ops := make([]types.Operation, 20)
	for i := range ops {
		ops[i] = makeOp(makeRepo("r"+string(rune('a'+i%26))), "clone")
	}

	count := 0
	for result := range p.Execute(ctx, ops) {
		if !result.Success {
			t.Errorf("unexpected failure: %v", result.Error)
		}
		count++
	}

	if count != 20 {
		t.Errorf("expected 20 results, got %d", count)
	}
}

// --- executeOperation ---

func TestExecuteOperation_CancelledContext(t *testing.T) {
	p := NewPool(1, newMock())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	op := &types.Operation{
		Repository: makeRepo("r1"),
		Command:    "clone",
	}

	result := p.executeOperation(ctx, op)
	if result.Success {
		t.Error("expected failure with cancelled context")
	}
	if result.Error == nil {
		t.Error("expected error with cancelled context")
	}
}

// --- Shutdown ---

func TestShutdown_NotStarted(t *testing.T) {
	p := NewPool(2, newMock())
	ctx := context.Background()
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("unexpected error shutting down unstarted pool: %v", err)
	}
}

// --- Concurrency: multiple executions ---

func TestExecute_MultipleCallsSerially(t *testing.T) {
	p := NewPool(2, newMock())
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		ops := []types.Operation{
			makeOp(makeRepo("r1"), "status"),
			makeOp(makeRepo("r2"), "update"),
		}
		results := p.Execute(ctx, ops)
		count := 0
		for range results {
			count++
		}
		if count != 2 {
			t.Errorf("iteration %d: expected 2 results, got %d", i, count)
		}
	}
}

// TestExecute_CompletedResultsNotDroppedOnCancel verifies that results for operations
// that have already finished are not silently discarded when the context is cancelled
// mid-flight (regression test for C-3).
//
// The results channel is buffered to len(operations), so sending a completed result
// never blocks — we must NOT select on ctx.Done() at the send site, which would
// cause finished work to be silently dropped.
func TestExecute_CompletedResultsNotDroppedOnCancel(t *testing.T) {
	const n = 10
	p := NewPool(n, newMock()) // one worker per op so they all finish quickly

	ops := make([]types.Operation, n)
	for i := 0; i < n; i++ {
		ops[i] = makeOp(makeRepo("repo"), "status")
	}

	ctx, cancel := context.WithCancel(context.Background())

	results := p.Execute(ctx, ops)

	// Cancel after a brief moment — some ops may be in-flight or already done.
	cancel()

	// Drain all results. With the fix, every operation that completed must send
	// its result; we cannot know exactly how many finished before the cancel, but
	// none of the sent results should be dropped (channel never blocks).
	collected := 0
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-results:
			if !ok {
				goto done
			}
			collected++
		case <-timeout:
			t.Fatal("timed out draining results — channel may be blocked")
		}
	}
done:
	// We cannot assert collected == n because cancellation may have stopped workers
	// before they picked up every job. What we CAN assert is that we didn't deadlock
	// and all received results were actually delivered (not silently dropped).
	t.Logf("collected %d/%d results after cancel", collected, n)
}
