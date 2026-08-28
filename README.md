# menagerie

A personal portfolio ecosystem of small, focused backend-systems
projects, built test-first, all in Go. (It started as a
different-language-per-project experiment — `rate-limiter` was Rust,
`ttl-cache` was C, `job-queue` was Python — until a deliberate policy
change: the whole fleet moved to Go for one consistent toolchain. Each
project's own README still notes that history where relevant.) Each
project is a **shell**: types and function signatures exist, but every
interesting algorithmic body is a stub (`errors.New("not implemented")`,
or a doc-commented `panic` where the signature has no error return) —
the actual logic is implemented test-first, by hand, one TDD cycle at a
time. This repo intentionally does not contain those implementations.

## Projects

| Directory | What it is |
|---|---|
| `go-load-balancer` | L7 Application Load Balancer — round-robin over healthy backends, active + passive health checks |
| `circuit-breaker` | Closed/open/half-open circuit breaker wrapping calls to an unreliable dependency |
| `consistent-hash-ring` | Consistent-hashing ring with virtual nodes, minimizing remap on add/remove |
| `rate-limiter` | Token-bucket rate limiter as `net/http` middleware |
| `ttl-cache` | In-memory LRU cache with per-key TTL, served over a tiny line-based TCP protocol |
| `job-queue` | Worker pool / job queue with retry + exponential backoff, dead-letter list |
| `exhibits` | Shared "backend fleet" — one image, three health profiles (healthy/flaky/down), reused by `go-load-balancer` and `circuit-breaker` |
| `wal` | Crash-safe, append-only Write-Ahead Log — durable `Append`, crash-safe `Replay`, demoed via a tiny WAL-backed KV store |

Each project's own `README.md` has its Problem Statement, non-goals,
architecture, and its proposed TDD seams (marked "to confirm before
writing the first test").

## Architecture-wide decisions

- **Go 1.26** for every single module in this repo — `go-load-balancer`,
  `circuit-breaker`, `consistent-hash-ring`, `exhibits`, `rate-limiter`,
  `ttl-cache`, `job-queue`, `wal` — tied together with a root `go.work`. No
  third-party CLI or web framework anywhere (stdlib `net/http`, stdlib
  `flag`) — the one consistent exception is `github.com/jackc/pgx/v5`
  for the three Postgres-backed projects.
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

Bring up everything at once:

```bash
docker compose up --build postgres backend-healthy backend-flaky backend-down \
  load-balancer rate-limiter ttl-cache
docker compose run --build circuit-breaker
docker compose run --build consistent-hash-ring
docker compose run --build job-queue
docker compose run --build wal
```

Or bring up any one project on its own — `docker compose up --build <service>`
pulls in whatever it `depends_on` automatically (e.g. `load-balancer`
starts the `exhibits` fleet for you; the three `*-demo` services start
`postgres`). Every stub currently returns/raises/logs its language's
"not implemented" error — that's expected until you implement it.

| Project | Run | Talk to it / watch it | What it's exercising |
|---|---|---|---|
| `go-load-balancer` | `docker compose up --build load-balancer` | `curl http://localhost:8090/` a few times | round-robins over the `exhibits` fleet (healthy/flaky/down); once implemented, should skip `backend-down` and react live to `backend-flaky` |
| `circuit-breaker` | `docker compose run --build circuit-breaker` | one-shot — read its log output | one breaker per `exhibits` upstream; once implemented, each transition also gets written to `breaker_transitions` in Postgres |
| `consistent-hash-ring` | `docker compose run --build consistent-hash-ring` | one-shot — read its log output | node membership persisted to `ring_nodes`; the ring's hash math stays in-memory, rebuilt from the persisted active nodes |
| `rate-limiter` | `docker compose up --build rate-limiter` | `curl http://localhost:3000/` repeatedly, or bench it (below) | in-memory token bucket, no external dependencies |
| `ttl-cache` | `docker compose up --build ttl-cache` | `printf 'SET k v 5000\r\nGET k\r\n' \| nc localhost 6380` | in-memory LRU + TTL cache over a line protocol, no external dependencies |
| `job-queue` | `docker compose run --build job-queue` | one-shot — read its log output | jobs persisted to `jobs`; dequeue via `SELECT ... FOR UPDATE SKIP LOCKED` |
| `exhibits` | started automatically as a dependency (`backend-healthy`/`-flaky`/`-down`), not run standalone | `curl http://localhost:8090/` through the load balancer, or exec into another container to hit it directly | not a TDD exercise itself — pure shared infrastructure |
| `wal` | `docker compose run --build wal -log /data/demo.wal put foo bar` (state persists in the `waldata` volume) | `docker compose run --build wal -log /data/demo.wal dump` | durable `Append` + crash-safe `Replay`; the real test is the kill -9 chaos recipe in `wal/README.md` §4, once implemented |

**Ports:** `postgres` 5432 · `load-balancer` 8090→8080 · `rate-limiter`
3000 · `ttl-cache` 6380. The `*-demo` services and the `exhibits`
instances aren't published to the host — they're only reachable from
other containers on the `labnet` network.

### Benchmarking

All three load-testing tools are pure Go — `vegeta` for the two HTTP
services, a small hand-rolled TCP client for `ttl-cache` (which speaks
its own line protocol, not HTTP):

```bash
# rate=200 req/s, duration=30s by default
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit vegeta-load-balancer
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit vegeta-rate-limiter

# override rate/duration
RATE=1000 DURATION=1m docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit vegeta-load-balancer

# ttl-cache: clients=20, duration=5s by default
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit bench-ttl-cache
```

### Go workspace

```bash
# plain `./...` doesn't work from the workspace root itself — list each
# module explicitly:
go build ./go-load-balancer/... ./circuit-breaker/... ./consistent-hash-ring/... \
  ./exhibits/... ./rate-limiter/... ./ttl-cache/... ./job-queue/... ./wal/...
```

## Schema

See `infra/postgres/init/*.sql`, applied automatically the first time
the `postgres` container starts (via `/docker-entrypoint-initdb.d`).
