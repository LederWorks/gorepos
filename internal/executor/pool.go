package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// Pool implements the Executor interface with a worker pool
type Pool struct {
	workerCount int
	manager     types.RepositoryManager
	mu          sync.RWMutex
	cancel      context.CancelFunc // tracks the most recent Execute context
}

// NewPool creates a new executor pool with the given repository manager
func NewPool(workerCount int, manager types.RepositoryManager) *Pool {
	return &Pool{
		workerCount: workerCount,
		manager:     manager,
	}
}

// Execute processes operations in parallel using the worker pool.
// It is safe to call Stop concurrently, but Execute itself must not be called
// concurrently — if a previous Execute is still in progress, this call returns
// a closed channel immediately. Create a new Pool for concurrent execution.
func (p *Pool) Execute(ctx context.Context, operations []types.Operation) <-chan types.Result {
	p.mu.Lock()
	if p.cancel != nil {
		// A previous Execute is still running. Return a closed channel so callers
		// drain immediately rather than blocking forever.
		p.mu.Unlock()
		ch := make(chan types.Result)
		close(ch)
		return ch
	}
	workerCount := p.workerCount
	execCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()

	results := make(chan types.Result, len(operations))

	go func() {
		defer close(results)
		defer func() {
			// Clear p.cancel once the goroutine finishes so Execute can be reused.
			p.mu.Lock()
			p.cancel = nil
			p.mu.Unlock()
		}()
		defer cancel()

		var wg sync.WaitGroup
		jobChan := make(chan types.Operation, len(operations))

		// Start workers
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go p.worker(execCtx, i, jobChan, results, &wg)
		}

		// Send operations to workers
		go func() {
			defer close(jobChan)
			for _, op := range operations {
				select {
				case jobChan <- op:
				case <-execCtx.Done():
					return
				}
			}
		}()

		// Wait for all workers to complete
		wg.Wait()
	}()

	return results
}

// worker processes operations from the job channel
func (p *Pool) worker(ctx context.Context, id int, jobs <-chan types.Operation, results chan<- types.Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return
			}

			result := p.executeOperation(ctx, &job)

			// The results channel is buffered to len(operations), so this send
			// never blocks — there is always a slot available. We do NOT select
			// on ctx.Done() here because that would silently drop a completed
			// result and cause callers to miss errors from finished work.
			results <- *result

		case <-ctx.Done():
			return
		}
	}
}

// executeOperation executes a single operation using the repository manager
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

// SetWorkerCount updates the number of workers
func (p *Pool) SetWorkerCount(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}

	p.workerCount = count
}

// Shutdown gracefully shuts down the executor pool
func (p *Pool) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	return nil
}

// GetWorkerCount returns the current worker count
func (p *Pool) GetWorkerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.workerCount
}
