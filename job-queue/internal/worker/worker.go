// Package worker implements the orchestration seam: N worker goroutines
// pulling from a JobQueue and retrying failed jobs according to a
// RetryPolicy, dead-lettering jobs that exhaust their retries.
package worker

import (
	"context"
	"errors"
	"sync"

	"job-queue/internal/queue"
	"job-queue/internal/retry"
)

// WorkerPool owns N worker goroutines, a JobQueue, a RetryPolicy, and a
// dead-letter list.
//
// Each worker pulls a job, runs it, and on failure consults the
// RetryPolicy to decide whether to re-enqueue (after backoff) or move
// the job to the dead-letter list.
type WorkerPool struct {
	numWorkers  int
	jobQueue    *queue.JobQueue
	retryPolicy *retry.RetryPolicy

	deadLettersMu sync.Mutex
	DeadLetters   []*queue.Job

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWorkerPool returns a WorkerPool with numWorkers workers, backed by
// jobQueue and retryPolicy.
func NewWorkerPool(numWorkers int, jobQueue *queue.JobQueue, retryPolicy *retry.RetryPolicy) *WorkerPool {
	return &WorkerPool{
		numWorkers:  numWorkers,
		jobQueue:    jobQueue,
		retryPolicy: retryPolicy,
	}
}

// Start spins up numWorkers worker goroutines that pull jobs from
// jobQueue and dispatch them (with retry-on-failure) until the pool is
// stopped.
//
// TODO(TDD): implement test-first (README.md seam 3).
//
// Expected shape: derive a cancellable context from ctx and stash its
// CancelFunc in wp.cancel (so Stop can cancel it). Launch wp.numWorkers
// goroutines, each added to wp.wg, that loop: call wp.jobQueue.Dequeue
// with a short per-iteration timeout (e.g. via context.WithTimeout on
// the worker's context) and, for whatever comes back, call
// wp.dispatch(ctx, job, 1) — looping until the derived context is
// Done() (check ctx.Err() each iteration). A short dequeue timeout
// matters here: without one, a worker blocked in Dequeue on an empty
// queue would never notice cancellation.
func (wp *WorkerPool) Start(ctx context.Context) {
	panic(errors.New("not implemented"))
}

// dispatch runs a single job attempt, and on failure consults
// retryPolicy to either re-enqueue the job (after backoff) or append it
// to DeadLetters once retries are exhausted.
//
// TODO(TDD): implement test-first (README.md seam 3).
//
// Expected shape: run the job (however "running" a *queue.Job is
// modeled — check the interface once Dequeue is implemented; likely
// invoking a caller-supplied handler function). On success, mark it
// done (however jobQueue models "done" — check its interface once it's
// implemented). On failure, ask wp.retryPolicy.ShouldRetry(attempt) —
// if false, append the job to wp.DeadLetters (guarded by
// wp.deadLettersMu, since multiple worker goroutines may append
// concurrently); if true, wait wp.retryPolicy.NextDelay(attempt) (via
// wp.retryPolicy.Sleep, so tests can skip the real wait) and then
// retry with attempt+1.
func (wp *WorkerPool) dispatch(ctx context.Context, job *queue.Job, attempt int) {
	panic(errors.New("not implemented"))
}

// Stop signals worker goroutines to stop pulling new jobs and waits for
// them to finish.
func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()
}
