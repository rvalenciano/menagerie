// Command demo enqueues a mix of jobs onto a WorkerPool and prints
// outcomes.
//
// Simulates three kinds of jobs:
//   - always-succeed: finishes on the first attempt.
//   - fail-then-succeed: fails a few times, then succeeds.
//   - always-fail: never succeeds, and should end up dead-lettered.
//
// Jobs are persisted to Postgres via JobQueue, using the connection
// string in the required DATABASE_URL environment variable (see
// docker-compose.yml, service `job-queue`).
//
// Until the seams in internal/queue, internal/retry, and internal/worker
// are implemented via TDD, this will surface "not implemented" errors
// (from JobQueue.Enqueue) and panics (from WorkerPool.Start) — that's
// expected, same as every other Go project in this repo before its
// seams are implemented.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"job-queue/internal/queue"
	"job-queue/internal/retry"
	"job-queue/internal/worker"
)

const (
	numWorkers  = 4
	maxAttempts = 5
)

// demoJob is the JSON payload shape enqueued for each simulated job. It
// mirrors the Python demo's fake Job class (name + fail_count), but
// here it's just data stored in the `jobs` table — how a *queue.Job
// gets turned back into actual work is up to WorkerPool.dispatch, once
// that seam is implemented via TDD.
type demoJob struct {
	Name      string `json:"name"`
	FailCount int    `json:"fail_count"`
}

func buildDemoJobs() []demoJob {
	var jobs []demoJob
	for i := 0; i < 3; i++ {
		jobs = append(jobs, demoJob{Name: fmt.Sprintf("always-succeed-%d", i), FailCount: 0})
	}
	for i := 0; i < 3; i++ {
		jobs = append(jobs, demoJob{Name: fmt.Sprintf("fail-then-succeed-%d", i), FailCount: 2})
	}
	for i := 0; i < 2; i++ {
		jobs = append(jobs, demoJob{Name: fmt.Sprintf("always-fail-%d", i), FailCount: maxAttempts + 1})
	}
	return jobs
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set; refusing to start without it")
	}

	ctx := context.Background()

	jobQueue, err := queue.NewJobQueue(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connecting to postgres: %v", err)
	}
	defer jobQueue.Close(ctx)

	retryPolicy := retry.NewRetryPolicy(maxAttempts, 100*time.Millisecond)
	pool := worker.NewWorkerPool(numWorkers, jobQueue, retryPolicy)

	for _, job := range buildDemoJobs() {
		payload, err := json.Marshal(job)
		if err != nil {
			log.Fatalf("marshaling job %q: %v", job.Name, err)
		}
		if _, err := jobQueue.Enqueue(ctx, payload); err != nil {
			log.Printf("Enqueue(%q): %v (expected until seam 1 is implemented test-first)", job.Name, err)
		}
	}

	startPool(ctx, pool)
	pool.Stop()

	fmt.Println()
	fmt.Printf("Dead-lettered jobs (%d):\n", len(pool.DeadLetters))
	for _, job := range pool.DeadLetters {
		fmt.Printf("  - %s\n", job.ID)
	}
}

// startPool calls pool.Start and recovers from the panic Start raises
// until seam 3 is implemented via TDD (Start/dispatch have no error
// return to surface "not implemented" through, so they panic instead —
// see internal/worker/worker.go).
func startPool(ctx context.Context, pool *worker.WorkerPool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("WorkerPool.Start: %v (expected until seam 3 is implemented test-first)", r)
		}
	}()
	pool.Start(ctx)
}
