package executor

import (
	"context"
	"testing"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

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
	p := NewPool(4)

	if p.GetWorkerCount() != 4 {
		t.Errorf("expected worker count 4, got %d", p.GetWorkerCount())
	}
	if p.IsStarted() {
		t.Error("pool should not be started on creation")
	}
}

// --- SetWorkerCount ---

func TestSetWorkerCount_Normal(t *testing.T) {
	p := NewPool(5)
	p.SetWorkerCount(10)
	if p.GetWorkerCount() != 10 {
		t.Errorf("expected 10, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_ClampMin(t *testing.T) {
	p := NewPool(5)
	p.SetWorkerCount(0)
	if p.GetWorkerCount() != 1 {
		t.Errorf("expected clamped min of 1, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_ClampMax(t *testing.T) {
	p := NewPool(5)
	p.SetWorkerCount(200)
	if p.GetWorkerCount() != 100 {
		t.Errorf("expected clamped max of 100, got %d", p.GetWorkerCount())
	}
}

func TestSetWorkerCount_Negative(t *testing.T) {
	p := NewPool(5)
	p.SetWorkerCount(-5)
	if p.GetWorkerCount() != 1 {
		t.Errorf("expected clamped min of 1, got %d", p.GetWorkerCount())
	}
}

// --- Execute ---

func TestExecute_StartsPool(t *testing.T) {
	p := NewPool(2)
	ctx := context.Background()
	ops := []types.Operation{makeOp(makeRepo("r1"), "clone")}

	results := p.Execute(ctx, ops)
	// Drain results
	for range results {
	}

	if !p.IsStarted() {
		t.Error("pool should be started after Execute")
	}
}

func TestExecute_ProcessesAllOperations(t *testing.T) {
	p := NewPool(3)
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
	p := NewPool(1)
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
	p := NewPool(1)
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
	p := NewPool(1)
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately before sending operations
	cancel()

	ops := []types.Operation{
		makeOp(makeRepo("r1"), "clone"),
		makeOp(makeRepo("r2"), "clone"),
	}

	results := p.Execute(ctx, ops)

	// Just drain — we mainly verify this doesn't hang or panic
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
	p := NewPool(2)
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
	p := NewPool(1)
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
	p := NewPool(1)
	ctx := context.Background()
	repo := makeRepo("r1")

	results := p.Execute(ctx, []types.Operation{makeOp(repo, "clone")})

	for result := range results {
		if result.Operation != "clone" {
			t.Errorf("expected operation 'clone', got %q", result.Operation)
		}
	}
}

// --- executeOperation ---

func TestExecuteOperation_CancelledContext(t *testing.T) {
	p := NewPool(1)
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
	p := NewPool(2)
	ctx := context.Background()
	// Should not error when pool hasn't been started
	if err := p.Shutdown(ctx); err != nil {
		t.Errorf("unexpected error shutting down unstarted pool: %v", err)
	}
}

// --- Concurrency: multiple executions ---

func TestExecute_MultipleCallsSerially(t *testing.T) {
	p := NewPool(2)
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
