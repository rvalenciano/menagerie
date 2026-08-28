// Package queue implements a Postgres-backed queue that jobs move
// through on their way to a worker.
//
// No retry/backoff knowledge lives here — see internal/retry for that.
// This package only needs to move jobs in and out of the `jobs` table
// (see infra/postgres/init/01_job_queue.sql) safely under concurrent
// access from multiple worker goroutines/processes.
package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Job is a single row claimed from the `jobs` table.
type Job struct {
	ID           uuid.UUID
	Payload      []byte
	Status       string
	AttemptCount int
	MaxAttempts  int
	NextRetryAt  time.Time
	LastError    *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// JobQueue is a Postgres-backed queue of jobs, persisted in the `jobs`
// table.
//
// Unlike an in-memory design, jobs survive a process restart: they live
// in Postgres, not in local memory, so Enqueue and Dequeue are backed by
// real SQL against the `jobs` table rather than an in-memory channel or
// slice.
type JobQueue struct {
	conn *pgx.Conn
}

// NewJobQueue opens a single connection to databaseURL (a libpq-style
// connection string, e.g.
// "postgres://user:pass@host:5432/dbname?sslmode=disable") and returns a
// JobQueue wrapping it. If you'd rather pool connections across multiple
// worker goroutines, swap this for a pgxpool.Pool instead — that's a
// later refinement, not required for the first seam.
func NewJobQueue(ctx context.Context, databaseURL string) (*JobQueue, error) {
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	return &JobQueue{conn: conn}, nil
}

// Close closes the underlying connection.
func (q *JobQueue) Close(ctx context.Context) error {
	return q.conn.Close(ctx)
}

// Enqueue inserts a new job with the given payload, returning its id.
//
// TODO(TDD): implement test-first (README.md seam 1).
//
// Expected shape: `INSERT INTO jobs (payload) VALUES ($1) RETURNING id`,
// passing payload (already-marshaled JSON) as the parameter, then
// return the returned id (a UUID). Everything else (status,
// attempt_count, next_retry_at) should be left to the table's defaults.
func (q *JobQueue) Enqueue(ctx context.Context, payload []byte) (uuid.UUID, error) {
	return uuid.UUID{}, errors.New("not implemented")
}

// Dequeue atomically claims and returns the next ready job, or nil if
// none is ready.
//
// TODO(TDD): implement test-first (README.md seam 1).
//
// Expected shape, in one transaction: `SELECT id, payload,
// attempt_count, max_attempts, next_retry_at FROM jobs WHERE status =
// 'pending' AND next_retry_at <= now() ORDER BY next_retry_at FOR
// UPDATE SKIP LOCKED LIMIT 1`, then `UPDATE jobs SET status =
// 'in_progress', attempt_count = attempt_count + 1 WHERE id = $1`, then
// commit and return the claimed row as a *Job. `FOR UPDATE SKIP LOCKED`
// is the key concept here — research it: it's what lets multiple worker
// goroutines/processes call Dequeue concurrently against the same table
// without ever racing each other onto the same row.
func (q *JobQueue) Dequeue(ctx context.Context) (*Job, error) {
	return nil, errors.New("not implemented")
}
