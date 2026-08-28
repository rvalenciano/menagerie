# rate-limiter

A token-bucket rate limiter, built test-first in Go, plus a small
stdlib `net/http` demo server that applies it as middleware. Part of a
personal portfolio ecosystem of small, focused apps — now an all-Go
fleet (sibling projects: [go-load-balancer](../go-load-balancer),
[circuit-breaker](../circuit-breaker)).

## 1. Problem Statement

### 1.1 Goal

Implement a token-bucket rate limiter as a reusable Go package, plus a
small `net/http`-based HTTP demo server that applies it as middleware,
such that:

1. Each caller draws from a bucket of up to `capacity` tokens.
2. Tokens replenish continuously over time at a configured `refillRate`
   (tokens/second), rather than resetting in discrete steps.
3. A request that finds the bucket empty is rejected with HTTP `429 Too
   Many Requests`; a request that finds a token available is let through
   and consumes one token.
4. The bucket stays correct under concurrent access — many requests (or
   goroutines) acquiring at once must never let more than `capacity`
   tokens be handed out in a burst.
5. Can be benchmarked under heavy, sustained request pressure to show
   the success rate actually collapsing to the configured cap once the
   bucket is saturated, not just "it works."

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- Distributed/shared rate limiting across multiple processes or
  instances — this is a single-process, in-memory limiter only (no
  Redis, no shared store).
- The sliding-window-log variant (or any algorithm other than token
  bucket) — documented as future work.
- Per-IP / per-user partitioning — v1 is a single global bucket shared
  by every request. Partitioning is a natural v2 extension once the
  single-bucket seam is solid.

### 1.3 Why this is a good TDD project

A token-bucket limiter has a small number of genuinely interesting,
independently testable behaviors — that's what makes it a good exercise:

- A **pure, stateful algorithm** (refill-over-time math plus the
  acquire/deny decision) that has no I/O at all once time is injected
  via a clock abstraction, so it can be tested with plain Go structs
  and no server, and without sleeping in the test suite.
- An **HTTP boundary** (the middleware) that can be fully exercised
  in-process using `net/http/httptest` against the handler — no real
  network socket required to test correctness.
- A genuine **concurrency-correctness question**: many goroutines
  calling `TryAcquire` at once must never collectively pull more tokens
  than the bucket ever held.
- A genuine **performance question** ("does it actually reject once
  saturated, under real concurrent HTTP traffic?") that unit tests
  cannot answer and that needs real, networked load generation to answer
  honestly.

### 1.4 How we'll test each concern

| Concern | How it's exercised | Tooling |
|---|---|---|
| Refill/acquire math over time | Unit tests inject a fake clock and advance it manually — no real sleeps | plain `testing`, injected `Clock` |
| HTTP 429 behavior at the middleware boundary | In-process requests against the real handler, no socket | `net/http/httptest`, injected clock |
| Correctness under concurrent acquires | N goroutines hammering one shared bucket; total successful acquires asserted `<= capacity` | `sync.WaitGroup`, shared `*TokenBucket` |
| Behavior under real, sustained HTTP pressure | Real load fired at the real, containerized demo server | [vegeta](https://github.com/tsenart/vegeta) (pure Go load-testing tool) via `benchmark/` |

## 2. Architecture

```
                    ┌───────────────────────────────┐
   client  ───────► │  net/http demo server (main.go)│
   requests         │                                │
                    │  ┌──────────────┐  ┌─────────┐ │
                    │  │ rateLimit    │─►│ handler │ │
                    │  │ middleware   │  │  "ok"   │ │
                    │  └──────┬───────┘  └─────────┘ │
                    └─────────┼──────────────────────┘
                               │ TryAcquire()
                               ▼
                      ┌──────────────────┐
                      │   TokenBucket     │
                      │  capacity         │
                      │  tokens           │
                      │  refillRate       │
                      │  lastRefill       │◄── Clock (injected)
                      └──────────────────┘

  TryAcquire() == true  → pass request through, 200
  TryAcquire() == false → short-circuit, 429
```

- **`internal/ratelimit/token_bucket.go`** — `TokenBucket`: capacity,
  current tokens, refill rate, last-refill timestamp, and a pluggable
  `Clock` type (instead of calling `time.Now()` directly) so tests can
  control the passage of time without sleeping. This is the seam every
  other layer sits on top of; it has no knowledge of HTTP.
- **`cmd/server/main.go`** — wires a stdlib `net/http` handler with a
  middleware (`rateLimit`) that calls `TryAcquire()` on a shared
  `*ratelimit.TokenBucket` for every incoming request, returning `429`
  on rejection and passing through to the handler otherwise.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`TokenBucket.TryAcquire() bool`** (pure, no I/O)
   Refill-over-time math and the acquire/deny decision. Time comes from
   an injected `Clock` rather than `time.Now()` directly, so tests
   advance a fake clock instead of sleeping. Candidate cases: a full
   bucket allows `capacity` acquires then denies; waiting for
   `1/refillRate` seconds (on the fake clock) frees up exactly one more
   token; the bucket never holds more than `capacity` tokens no matter
   how long it sits idle.

2. **HTTP middleware seam**
   Wraps requests, returns `429` when `TryAcquire` fails and passes
   through otherwise. Exercised in-process using Go's test utilities
   (`net/http/httptest` against the handler, no real network socket
   needed) with a fake/injected clock, so the whole request/deny cycle
   is deterministic and fast.

3. **Concurrency seam**
   Bucket correctness under many concurrent `TryAcquire` calls from
   multiple goroutines (the bucket's internal state is behind a
   `sync.Mutex`, so `TryAcquire` can be called safely from a single
   shared `*TokenBucket`) — asserted by a burst test that spawns N
   goroutines all calling `TryAcquire` on one shared bucket at once, and
   checks the total number of successful acquires never exceeds
   `capacity`.

Implementation status right now: **shell only**. `NewTokenBucket` /
`NewTokenBucketWithClock` are implemented (they're just field
initialization), but `TryAcquire` is a stub that always returns `false`
— the first real test against seam 1 should start red.

## 4. How to run it

There's no local Go toolchain assumed; everything below runs through
Docker. (If you do have `go` installed locally, `go test ./...`,
`go build ./...`, and `go run ./cmd/server` all work directly too.)

```bash
# build the shell (the stub compiles fine — it just always returns false)
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go build ./...

# once TryAcquire is implemented, unit tests run the same way:
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./...

# build and run the demo server
docker compose up --build

# hit it a few times by hand
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000/; done

docker compose down
```

## 5. Benchmarking under heavy pressure

`benchmark/` builds a small pure-Go [vegeta](https://github.com/tsenart/vegeta)
image and fires sustained load at the real, containerized demo server.

```bash
# rate=200 req/s, duration=30s, target=http://rate-limiter:3000/ (defaults)
docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit vegeta

# custom rate/duration
RATE=1000 DURATION=1m docker compose -f docker-compose.yml -f docker-compose.bench.yml \
  up --build --abort-on-container-exit vegeta

docker compose -f docker-compose.yml -f docker-compose.bench.yml down
```

This produces a `vegeta report` with request rate achieved, success
ratio, and latency percentiles (p50/p95/p99/max). The interesting number
here isn't throughput — it's the **success ratio**: once the sustained
attack rate exceeds `capacity + refillRate` sustained tokens/second,
the success ratio should collapse from ~100% down to roughly
`refillRate / attackRate`, which is the honest, load-tested proof that
the limiter is actually capping traffic at its configured rate rather
than just "working" in a unit test.

## 6. Future work (explicitly out of scope for v1)

- Sliding-window-log (or other) rate-limiting algorithms behind the same
  middleware seam.
- Per-IP / per-user bucket partitioning instead of one global bucket.
- A distributed/shared limiter (e.g. Redis-backed) for multi-instance
  deployments.
- Configurable capacity/refill-rate via CLI flags or env vars (currently
  hardcoded constants in `cmd/server/main.go`).
- Structured metrics (current/available tokens, rejection rate) beyond
  what a single `vegeta` run reports.
