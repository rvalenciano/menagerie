# job-queue

A Postgres-backed worker pool / job queue, built test-first in Go —
think a tiny Celery/Sidekiq. Jobs are persisted to a `jobs` table (see
`infra/postgres/init/01_job_queue.sql`) instead of living only in
memory. Part of a personal portfolio ecosystem of small, focused apps in
different languages (sibling project:
[go-load-balancer](../go-load-balancer)).

## 1. Problem Statement

### 1.1 Goal

Build a worker pool that:

1. Accepts jobs via `JobQueue.Enqueue(ctx, payload)` onto a
   Postgres-backed queue.
2. Runs **N worker goroutines** that pull jobs off the queue and execute
   them concurrently.
3. Automatically **retries** a job that fails, using **exponential
   backoff**, up to a configurable **max-attempts**.
4. Moves a job to a **dead-letter list** once it has exhausted its
   retries, instead of losing it silently.
5. Can be demonstrated end-to-end with a CLI (`cmd/demo`) that simulates
   a realistic mix of jobs: some succeed immediately, some fail a few
   times then succeed, some always fail (and end up dead-lettered).

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- **Distributed workers across machines** — one process, N goroutines
  pulling from a shared Postgres table (multiple processes are safe
  thanks to `FOR UPDATE SKIP LOCKED`, but the demo only runs one).
- **Scheduling / cron / delayed jobs** beyond the retry backoff itself —
  no "run this at 3am" or "run this every 5 minutes."
- **Priority queues** — jobs are processed in `next_retry_at` order;
  there is no notion of a job jumping the line.

### 1.3 Why this is a good TDD project

A worker pool has two genuinely different kinds of logic bundled
together, which makes it a nice contrast in testing technique within one
small project:

- A **pure decision-logic seam** (the retry/backoff policy) — given an
  attempt count and whether the job failed, decide whether to retry and
  compute the backoff delay. No I/O, no goroutines, no real sleeping
  required in tests — this is the same "pure function" flavor of test as
  `go-load-balancer`'s `Pool.Next()`.
- A **genuinely concurrent integration seam** (the worker pool and the
  queue itself) — real goroutines pulling from a real Postgres table,
  racing each other, needing to be correct under concurrent
  producers/consumers. This is the kind of test that can't be faked with
  a single-goroutine call and has to actually exercise concurrency
  against Postgres.

### 1.4 How we'll test each concern

| Concern | How it's exercised | Tooling |
|---|---|---|
| Safe concurrent dequeue against Postgres | Concurrent goroutines hammering a real `JobQueue` against a real Postgres instance, asserting no job id is ever claimed twice | `go test`, real goroutines, `docker-compose` postgres |
| Retry/backoff decision logic | Pure unit tests over attempt counts and outcomes, no real sleeping | `go test`, injected `Sleep` |
| Worker pool orchestration under retries | Real worker goroutines processing fake jobs that fail exactly K times, asserting eventual success or dead-letter | `go test`, real goroutines, short/injected delays |

## 2. Architecture

```
                    ┌───────────────────────────────────────┐
 Enqueue(payload) ─►│              WorkerPool                │
                    │                                         │
                    │   ┌──────────┐        ┌───────────────┐ │
                    │   │ JobQueue │◄──────►│ worker goroutine│ │
                    │   │ (Postgres│  pull   │      x N       │ │
                    │   │ `jobs`   │        └───────┬───────┘ │
                    │   │  table)  │                │          │
                    │   └──────────┘         dispatch(job)     │
                    │                                │          │
                    │                     ┌──────────▼────────┐ │
                    │                     │   RetryPolicy      │ │
                    │                     │ ShouldRetry(n)?    │ │
                    │                     │ NextDelay(n)       │ │
                    │                     └──────────┬────────┘ │
                    │                                │          │
                    │            success ◄───────────┼──► retry (re-enqueue after delay)
                    │                                │          │
                    │                     retries exhausted     │
                    │                                ▼          │
                    │                       DeadLetters slice   │
                    └───────────────────────────────────────┘
```

- **`internal/queue`** — `JobQueue`, backed by the Postgres `jobs` table
  (see `infra/postgres/init/01_job_queue.sql`). `Enqueue` inserts a row;
  `Dequeue` claims one using the `SELECT ... FOR UPDATE SKIP LOCKED`
  pattern, so multiple worker goroutines/processes can call `Dequeue`
  concurrently against the same table without ever picking up the same
  row twice. Because jobs live in Postgres rather than local memory, the
  queue (and the dead-letter state on each row) survives a process
  restart. No retry/backoff decision logic lives here; it just moves
  jobs in and out safely under concurrent access.
- **`internal/retry`** — `RetryPolicy`, pure decision logic. Given the
  current attempt count, decides whether a failed job should be retried
  (`ShouldRetry`) and how long to wait before the next attempt
  (`NextDelay`, exponential backoff). Takes an injectable `Sleep`
  function so tests never actually wait for real backoff durations.
- **`internal/worker`** — `WorkerPool`, the orchestration seam. Owns N
  worker goroutines, a `*queue.JobQueue`, a `*retry.RetryPolicy`, and a
  dead-letter slice (guarded by a mutex, since multiple workers may
  append to it concurrently). Each worker pulls a job, runs it, and on
  failure consults the `RetryPolicy` to decide whether to re-enqueue
  (after backoff) or move the job to the dead-letter list.
- **`cmd/demo`** — wires the above together, enqueues a mix of
  always-succeed / fail-then-succeed / always-fail fake jobs, and prints
  the outcomes plus the final dead-letter list.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **Queue behavior** — `JobQueue.Enqueue` / `JobQueue.Dequeue`
   Correctness against a real Postgres instance (the docker-compose
   `postgres` service, or a locally-run one): `Enqueue` inserts a row
   into the `jobs` table and returns its id; `Dequeue` must safely claim
   a single pending, ready job even under concurrent callers. In
   particular, concurrent `Dequeue` calls from multiple workers must
   never return the same job twice — this is what `SELECT ... FOR UPDATE
   SKIP LOCKED` gives you, and is the property the tests need to
   exercise directly (e.g. multiple goroutines dequeuing concurrently
   against the same table and asserting no job id is claimed more than
   once).

2. **Retry/backoff policy** — `RetryPolicy.ShouldRetry` /
   `RetryPolicy.NextDelay`
   A pure function seam: given the current attempt count, decide whether
   to retry and compute the backoff delay. Inject a `Sleep` function so
   tests never actually sleep for real backoff durations.

3. **`WorkerPool` integration seam**
   N real worker goroutines pulling from a real `JobQueue`, processing
   fake jobs (some always succeed, some fail exactly K times then
   succeed, some always fail). Asserts a job eventually succeeds within
   `MaxAttempts`, or lands in the dead-letter list once retries are
   exhausted — exercised with real goroutines but short/injected delays
   so the test suite stays fast.

Implementation status right now: **shell only**. Types and method
signatures exist; the seam methods above return/panic with "not
implemented" so the first real test against each seam starts red, as it
should. `NewJobQueue`, `NewRetryPolicy`, `NewWorkerPool`, and `Stop` are
already implemented for real — they're infrastructure, not the
algorithm under test.

## 4. How to run it

```bash
# once the seams above are implemented
go test ./...

# build and run the CLI demo directly (requires DATABASE_URL, e.g.
# against the docker-compose postgres service or a local Postgres with
# the jobs table from infra/postgres/init/01_job_queue.sql)
DATABASE_URL="postgres://systems_lab:systems_lab@localhost:5432/systems_lab?sslmode=disable" \
  go run ./cmd/demo

# or via Docker
docker build -t job-queue .
docker run --rm -e DATABASE_URL="postgres://systems_lab:systems_lab@host.docker.internal:5432/systems_lab?sslmode=disable" \
  job-queue

# or via docker-compose from the repo root (wires DATABASE_URL for you)
docker compose run --build job-queue
```

Currently the demo surfaces "not implemented" errors/panics from the
stubs — expected until the seams above are implemented via TDD.

## 5. Benchmarking (optional, secondary)

This project's main interest is **concurrency correctness**, not raw
throughput, so benchmarking is a nice-to-have rather than a core
deliverable: enqueue N jobs and measure throughput/completion time with
varying worker-pool sizes once the seams above are implemented.
