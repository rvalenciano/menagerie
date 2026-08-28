# job-queue

A Postgres-backed worker pool / job queue, built test-first in Python —
think a tiny Celery/Sidekiq. Jobs are persisted to a `jobs` table (see
`infra/postgres/init/01_job_queue.sql`) instead of living only in
memory — a deliberate upgrade from the project's original in-memory
design. Part of a personal portfolio ecosystem of small, focused apps in
different languages (sibling project:
[go-load-balancer](../go-load-balancer)).

## 1. Problem Statement

### 1.1 Goal

Build an in-memory worker pool that:

1. Accepts jobs via `enqueue(job)` onto a thread-safe queue.
2. Runs **N worker threads** that pull jobs off the queue and execute
   them concurrently.
3. Automatically **retries** a job that raises an exception, using
   **exponential backoff**, up to a configurable **max-attempts**.
4. Moves a job to a **dead-letter list** once it has exhausted its
   retries, instead of losing it silently.
5. Can be demonstrated end-to-end with a script that simulates a
   realistic mix of jobs: some succeed immediately, some fail a few
   times then succeed, some always fail (and end up dead-lettered).

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- **Distributed workers** — no workers across multiple machines or
  processes. One process, N threads.
- **Scheduling / cron / delayed jobs** beyond the retry backoff itself —
  no "run this at 3am" or "run this every 5 minutes."
- **Priority queues** — jobs are processed FIFO; there is no notion of
  a job jumping the line.

### 1.3 Why this is a good TDD project

A worker pool has two genuinely different kinds of logic bundled
together, which makes it a nice contrast in testing technique within one
small project:

- A **pure decision-logic seam** (the retry/backoff policy) — given an
  attempt count and whether the job raised, decide whether to retry and
  compute the backoff delay. No I/O, no threads, no real sleeping
  required in tests — this is the same "pure function" flavor of test as
  `go-load-balancer`'s `Pool.Next()`.
- A **genuinely concurrent integration seam** (the worker pool itself) —
  real threads pulling from a real thread-safe queue, racing each other,
  needing to be correct under concurrent producers/consumers. This is
  the kind of test that can't be faked with a single-threaded call and
  has to actually exercise `threading`.

### 1.4 How we'll test each concern

| Concern | How it's exercised | Tooling |
|---|---|---|
| FIFO ordering + thread-safety of the queue | Concurrent producer/consumer threads hammering a real `JobQueue` | stdlib `threading`, `pytest` |
| Retry/backoff decision logic | Pure unit tests over attempt counts and outcomes, no real sleeping | `pytest`, injected clock/sleep |
| Worker pool orchestration under retries | Real worker threads processing fake jobs that fail exactly K times, asserting eventual success or dead-letter | `pytest`, real `threading`, short/injected delays |

## 2. Architecture

```
                    ┌───────────────────────────────────────┐
 enqueue(job) ────► │              WorkerPool                │
                    │                                         │
                    │   ┌──────────┐        ┌───────────────┐ │
                    │   │ JobQueue │◄──────►│  worker thread │ │
                    │   │ (Postgres│  pull   │      x N       │ │
                    │   │ `jobs`   │        └───────┬───────┘ │
                    │   │  table)  │                │          │
                    │   └──────────┘         run job()          │
                    │                                │          │
                    │                     ┌──────────▼────────┐ │
                    │                     │   RetryPolicy      │ │
                    │                     │ should_retry(n)?   │ │
                    │                     │ next_delay(n)      │ │
                    │                     └──────────┬────────┘ │
                    │                                │          │
                    │            success ◄───────────┼──► retry (re-enqueue after delay)
                    │                                │          │
                    │                     retries exhausted     │
                    │                                ▼          │
                    │                       dead_letter list    │
                    └───────────────────────────────────────┘
```

- **`job_queue/queue.py`** — `JobQueue`, backed by the Postgres `jobs`
  table (see `infra/postgres/init/01_job_queue.sql`) instead of an
  in-memory structure. `enqueue` inserts a row; `dequeue` claims one
  using the `SELECT ... FOR UPDATE SKIP LOCKED` pattern, so multiple
  worker processes/threads can call `dequeue` concurrently against the
  same table without ever picking up the same row twice. Because jobs
  live in Postgres rather than local memory, the queue (and the
  dead-letter state on each row) survives a process restart — a
  deliberate upgrade from the project's original in-memory design. No
  retry/backoff decision logic lives here; it just moves jobs in and
  out safely under concurrent access.
- **`job_queue/retry.py`** — `RetryPolicy`, pure decision logic. Given
  the current attempt count, decides whether a failed job should be
  retried (`should_retry`) and how long to wait before the next attempt
  (`next_delay`, exponential backoff). Takes an injected `clock`/`sleep`
  callable so tests never actually wait for real backoff durations.
- **`job_queue/worker.py`** — `WorkerPool`, the orchestration seam. Owns
  N worker threads, a `JobQueue`, a `RetryPolicy`, and a dead-letter
  list. Each worker pulls a job, runs it, and on failure consults the
  `RetryPolicy` to decide whether to re-enqueue (after backoff) or move
  the job to the dead-letter list.
- **`demo.py`** — wires the above together, enqueues a mix of
  always-succeed / fail-then-succeed / always-fail fake jobs, and prints
  the outcomes plus the final dead-letter list.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **Queue behavior** — `JobQueue.enqueue` / `JobQueue.dequeue`
   Correctness against a real Postgres instance (the docker-compose
   `postgres` service, or a locally-run one) rather than an in-memory
   structure: `enqueue` inserts a row into the `jobs` table and returns
   its id; `dequeue` must safely claim a single pending, ready job even
   under concurrent callers. In particular, concurrent `dequeue` calls
   from multiple workers must never return the same job twice — this is
   what `SELECT ... FOR UPDATE SKIP LOCKED` gives you, and is the
   property the tests need to exercise directly (e.g. multiple
   threads/processes dequeuing concurrently against the same table and
   asserting no job id is claimed more than once).

2. **Retry/backoff policy** — `RetryPolicy.should_retry` /
   `RetryPolicy.next_delay`
   A pure function seam: given the current attempt count and whether the
   job raised, decide whether to retry and compute the backoff delay.
   Inject a clock/sleep function so tests never actually sleep for real
   backoff durations.

3. **`WorkerPool` integration seam**
   N real worker threads pulling from a real `JobQueue`, processing fake
   jobs (some always succeed, some fail exactly K times then succeed,
   some always fail). Asserts a job eventually succeeds within
   `max_attempts`, or lands in the dead-letter list once retries are
   exhausted — exercised with real threads but short/injected delays so
   the test suite stays fast.

Implementation status right now: **shell only**. Classes and method
signatures exist; the seam methods above raise `NotImplementedError` so
the first real test against each seam starts red, as it should.

## 4. How to run it

```bash
# install (editable, with dev deps)
pip install -e ".[dev]"

# run tests (once seams above are implemented)
pytest

# run the demo (currently surfaces NotImplementedError from the stubs —
# expected until the seams above are implemented via TDD)
python demo.py

# or via Docker
docker build -t job-queue .
docker run --rm job-queue
```

## 5. Benchmarking (optional, secondary)

This project's main interest is **concurrency correctness**, not raw
throughput, so benchmarking is a nice-to-have rather than a core
deliverable: enqueue N jobs and measure throughput/completion time with
varying worker-pool sizes (e.g. `time python demo.py --jobs 1000
--workers 4` vs. `--workers 16`) once the seams above are implemented.
