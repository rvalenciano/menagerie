# menagerie

A personal portfolio ecosystem of small, focused backend-systems
projects in different languages, built test-first. Each project is a
**shell**: types and function signatures exist, but every interesting
algorithmic body is a stub (`not implemented` / `todo!()` /
`NotImplementedError` / `return -1;`, whichever is idiomatic for that
language) — the actual logic is implemented test-first, by hand, one
TDD cycle at a time. This repo intentionally does not contain those
implementations.

## Projects

| Directory | Language | What it is |
|---|---|---|
| `go-load-balancer` | Go | L7 Application Load Balancer — round-robin over healthy backends, active + passive health checks |
| `circuit-breaker` | Go | Closed/open/half-open circuit breaker wrapping calls to an unreliable dependency |
| `consistent-hash-ring` | Go | Consistent-hashing ring with virtual nodes, minimizing remap on add/remove |
| `rate-limiter` | Rust | Token-bucket rate limiter as Axum middleware |
| `ttl-cache` | C | In-memory LRU cache with per-key TTL, served over a tiny line-based TCP protocol |
| `job-queue` | Python | Worker pool / job queue with retry + exponential backoff, dead-letter list |
| `exhibits` | Go | Shared "backend fleet" — one image, three health profiles (healthy/flaky/down), reused by `go-load-balancer` and `circuit-breaker` |

Each project's own `README.md` has its Problem Statement, non-goals,
architecture, and its proposed TDD seams (marked "to confirm before
writing the first test").

## Architecture-wide decisions

- **Go 1.26** for every Go module (`go-load-balancer`, `circuit-breaker`,
  `consistent-hash-ring`, `exhibits`), tied together with a root
  `go.work`.
- **`exhibits`** is a stateless, time-based fake backend (health is a
  pure function of wall-clock time, not request count) deployed three
  times — `backend-healthy`, `backend-flaky`, `backend-down` — so
  `go-load-balancer`'s failover and `circuit-breaker`'s state machine both
  have a real, shared, containerized dependency to react to instead of
  an in-process fake.
- **Shared PostgreSQL** (`postgres` service, database `systems_lab`)
  gives real persistence to the projects where it's a genuine
  improvement, not just observability:
  - `job-queue` — jobs (`jobs` table), dequeued via
    `SELECT ... FOR UPDATE SKIP LOCKED` for safe multi-worker pickup.
  - `consistent-hash-ring` — node membership (`ring_nodes`, append-only:
    add inserts a row, remove sets `removed_at`). The ring's hash math
    itself stays in-memory, rebuilt from the persisted active nodes.
  - `circuit-breaker` — an audit log of state transitions
    (`breaker_transitions`). The breaker's live decision-making
    (`Execute`, `State`) stays entirely in-memory for speed; this table
    is a side-effect log, not the source of truth for current state.
  - `rate-limiter` and `ttl-cache` deliberately stay **in-memory
    only** — a rate limiter or cache backed synchronously by the same
    database it's meant to sit in front of would defeat its own
    purpose.
  - Every table: `UUID` primary keys (`gen_random_uuid()`),
    `created_at`/`updated_at` columns with indexes, and a shared
    trigger (`set_updated_at()`, in `infra/postgres/init/00_common.sql`)
    that keeps `updated_at` current automatically.

## Running it

```bash
# Core long-running stack: postgres + the exhibits fleet + load-balancer
# + the standalone rate-limiter and ttl-cache servers
docker compose up --build postgres backend-healthy backend-flaky backend-down \
  load-balancer rate-limiter ttl-cache

# One-shot demos (make N calls / operations, then exit)
docker compose run --build circuit-breaker-demo
docker compose run --build consistent-hash-ring-demo
docker compose run --build job-queue-demo

# Build/vet every Go module at once from the workspace root — note
# plain `./...` doesn't work from the workspace root itself; list each
# module explicitly:
go build ./go-load-balancer/... ./circuit-breaker/... ./consistent-hash-ring/... ./exhibits/...
```

## Schema

See `infra/postgres/init/*.sql`, applied automatically the first time
the `postgres` container starts (via `/docker-entrypoint-initdb.d`).
