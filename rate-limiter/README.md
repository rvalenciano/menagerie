# rate-limiter

A token-bucket rate limiter, built test-first in Rust, plus a small Axum
HTTP demo server that applies it as middleware. Part of a personal
portfolio ecosystem of small, focused apps in different languages
(sibling project: [go-load-balancer](../go-load-balancer), in Go).

## 1. Problem Statement

### 1.1 Goal

Implement a token-bucket rate limiter as a reusable Rust module, plus a
small Axum-based HTTP demo server that applies it as middleware, such
that:

1. Each caller draws from a bucket of up to `capacity` tokens.
2. Tokens replenish continuously over time at a configured `refill_rate`
   (tokens/second), rather than resetting in discrete steps.
3. A request that finds the bucket empty is rejected with HTTP `429 Too
   Many Requests`; a request that finds a token available is let through
   and consumes one token.
4. The bucket stays correct under concurrent access — many requests (or
   threads) acquiring at once must never let more than `capacity` tokens
   be handed out in a burst.
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
  via a clock abstraction, so it can be tested with plain Rust structs
  and no server, and without sleeping in the test suite.
- An **HTTP boundary** (the Axum middleware) that can be fully exercised
  in-process using Tower's test utilities (`ServiceExt::oneshot` against
  the router) — no real network socket required to test correctness.
- A genuine **concurrency-correctness question**: many threads calling
  `try_acquire` at once must never collectively pull more tokens than
  the bucket ever held.
- A genuine **performance question** ("does it actually reject once
  saturated, under real concurrent HTTP traffic?") that unit tests
  cannot answer and that needs real, networked load generation to answer
  honestly.

### 1.4 How we'll test each concern

| Concern | How it's exercised | Tooling |
|---|---|---|
| Refill/acquire math over time | Unit tests inject a fake clock and advance it manually — no real sleeps | plain `#[test]`, injected `Clock` |
| HTTP 429 behavior at the middleware boundary | In-process requests against the real Axum router, no socket | `tower::ServiceExt::oneshot`, injected clock |
| Correctness under concurrent acquires | N threads hammering one shared bucket; total successful acquires asserted `<= capacity` | `std::thread`, `Arc` |
| Behavior under real, sustained HTTP pressure | Real load fired at the real, containerized demo server | [vegeta](https://github.com/tsenart/vegeta) (pure Go load-testing tool) via `benchmark/` |

## 2. Architecture

```
                    ┌───────────────────────────────┐
   client  ───────► │   Axum demo server (main.rs)   │
   requests         │                                │
                    │  ┌──────────────┐  ┌─────────┐ │
                    │  │ rate_limit   │─►│ handler │ │
                    │  │ middleware   │  │  "ok"   │ │
                    │  └──────┬───────┘  └─────────┘ │
                    └─────────┼──────────────────────┘
                               │ try_acquire()
                               ▼
                      ┌──────────────────┐
                      │   TokenBucket     │
                      │  capacity         │
                      │  tokens           │
                      │  refill_rate      │
                      │  last_refill      │◄── Clock (injected)
                      └──────────────────┘

  try_acquire() == true  → pass request through, 200
  try_acquire() == false → short-circuit, 429
```

- **`src/token_bucket.rs`** — `TokenBucket`: capacity, current tokens,
  refill rate, last-refill timestamp, and a pluggable `Clock` trait
  (instead of calling `Instant::now()` directly) so tests can control
  the passage of time without sleeping. This is the seam every other
  layer sits on top of; it has no knowledge of HTTP.
- **`src/main.rs`** — wires an Axum `Router` with a middleware layer
  (`axum::middleware::from_fn_with_state`) that calls `try_acquire()` on
  a shared `Arc<TokenBucket>` for every incoming request, returning
  `429` on rejection and passing through to the handler otherwise.
- **`src/lib.rs`** — exposes `token_bucket` as a library target
  (`rate_limiter`) separate from the `main.rs` binary, so the bucket can
  be unit-tested on its own without pulling in Axum/Tokio.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`TokenBucket::try_acquire(&self) -> bool`** (pure, no I/O)
   Refill-over-time math and the acquire/deny decision. Time comes from
   an injected `Clock` trait rather than `Instant::now()` directly, so
   tests advance a fake clock instead of sleeping. Candidate cases: a
   full bucket allows `capacity` acquires then denies; waiting for
   `1/refill_rate` seconds (on the fake clock) frees up exactly one more
   token; the bucket never holds more than `capacity` tokens no matter
   how long it sits idle.

2. **HTTP middleware seam**
   Wraps requests, returns `429` when `try_acquire` fails and passes
   through otherwise. Exercised in-process using Axum's test utilities
   (`tower::ServiceExt::oneshot` against the router, no real network
   socket needed) with a fake/injected clock, so the whole request/deny
   cycle is deterministic and fast.

3. **Concurrency seam**
   Bucket correctness under many concurrent `try_acquire` calls from
   multiple threads (the bucket's internal state is behind a `Mutex`, or
   built from atomics, so `try_acquire` can take `&self` and be called
   from an `Arc<TokenBucket>` shared across threads) — asserted by a
   burst test that spawns N threads all calling `try_acquire` on one
   shared bucket at once, and checks the total number of successful
   acquires never exceeds `capacity`.

Implementation status right now: **shell only**. `TokenBucket::new` is
implemented (it's just field initialization), but `try_acquire` is
`todo!()` — the first real test against seam 1 should start red.

## 4. How to run it

There's no local Rust toolchain assumed; everything below runs through
Docker. (If you do have `cargo` installed locally, `cargo test`,
`cargo build`, and `cargo run` all work directly too.)

```bash
# build the shell (todo!() bodies compile fine — they only panic if hit)
docker run --rm -v "$PWD":/src -w /src rust:1-slim cargo build

# once try_acquire is implemented, unit tests run the same way:
docker run --rm -v "$PWD":/src -w /src rust:1-slim cargo test

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
attack rate exceeds `capacity + refill_rate` sustained tokens/second,
the success ratio should collapse from ~100% down to roughly
`refill_rate / attack_rate`, which is the honest, load-tested proof that
the limiter is actually capping traffic at its configured rate rather
than just "working" in a unit test.

## 6. Future work (explicitly out of scope for v1)

- Sliding-window-log (or other) rate-limiting algorithms behind the same
  middleware seam.
- Per-IP / per-user bucket partitioning instead of one global bucket.
- A distributed/shared limiter (e.g. Redis-backed) for multi-instance
  deployments.
- Configurable capacity/refill-rate via CLI flags or env vars (currently
  hardcoded constants in `main.rs`).
- Structured metrics (current/available tokens, rejection rate) beyond
  what a single `vegeta` run reports.
