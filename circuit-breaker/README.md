# circuit-breaker

A reusable Circuit Breaker, built test-first in pure Go. Part of a
personal portfolio ecosystem of small, focused apps in different
languages.

## 1. Problem Statement

### 1.1 Goal

Build a reusable Circuit Breaker as a Go package (standard library only —
no third-party resilience frameworks), that:

1. Wraps calls to an unreliable dependency behind a single
   `Execute(fn func() error) error` seam.
2. Starts **closed** — calls pass through and failures are counted.
3. Trips to **open** after **N consecutive failures** (a configurable
   threshold) — while open, calls fail fast with a sentinel error
   without attempting the real call.
4. After a configurable cooldown elapses, transitions to **half-open**
   and allows exactly **one trial call** through.
5. A successful trial call closes the breaker (back to normal); a failed
   trial call reopens it and restarts the cooldown.
6. Ships with a small demo (`cmd/demo`) that wires an independent
   breaker around each of three real, separately-containerized upstream
   services with different failure profiles (`cmd/upstream`), so the
   state machine's behavior is visible end-to-end, not just in tests.

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- Distributed/shared breaker state across processes — this is a single
  in-process breaker, not a coordinated cluster-wide one.
- Failure-rate-based tripping (e.g. "50% of the last 20 calls failed") —
  v1 uses a consecutive-failure-count threshold only.
- Bulkheading / concurrency limiting.
- A metrics exporter (Prometheus or otherwise).

### 1.3 Why this is a good TDD project

A circuit breaker is a small, genuinely interesting exercise because:

- It's a **pure, enumerable state machine** (closed / open / half-open)
  with no network I/O in its core, so every transition can be tested
  with plain Go values and no server.
- Its one tricky dependency — **wall-clock time**, for the open-timeout
  cooldown — can be removed entirely by injecting a `Clock` instead of
  using real time, so tests are fast and deterministic instead of
  sleeping for real durations.
- It has a single, well-defined **call-wrapping seam** (`Execute`) that
  cleanly separates "does the state machine behave correctly" (no I/O)
  from "is it wired correctly around a real, unreliable dependency"
  (I/O, exercised optionally with `httptest`).

## 2. Architecture

```
                      ┌───────────────────────────┐
  caller  ──Execute─► │   breaker.Breaker          │
  (fn)                │                             │
                      │  state: closed/open/half-open
                      │  consecutive failure count │
                      │  Clock (injected)           │
                      └──────────────┬──────────────┘
                                     │ if not open: call fn(),
                                     │ record outcome
                                     ▼
                            ┌──────────────────┐
                            │   fn() error       │
                            │ (the real call,    │
                            │  e.g. HTTP request) │
                            └──────────────────┘
```

- **`internal/breaker`** — `State` (Closed/Open/HalfOpen), `Config`
  (`FailureThreshold`, `OpenTimeout`, injected `Clock`), and `Breaker`
  with its `Execute` seam. No network I/O — this package is the entire
  TDD-able core.
- **`cmd/upstream`** — a small, real HTTP server with a configurable
  failure profile (`CALLS_PER_CYCLE` / `FLAKY_WINDOW` env vars): a
  window of failing (500) requests followed by a window of succeeding
  (200) ones, repeating. `docker-compose.yml` runs three instances of
  it — `upstream-healthy` (never fails), `upstream-flaky` (fails 6 of
  every 10 requests, then recovers), and `upstream-down` (always
  fails) — as three genuinely separate containers, the same way
  go-load-balancer's `cmd/backend` stands in for a real backend.
- **`cmd/demo`** — builds one `Breaker` per upstream (from a
  `name=url` list, `-upstreams`/`UPSTREAMS`) and calls all three every
  round, logging each breaker's state independently
  (`upstream=<name> call=<n> state=<state> result=...`) — so a single
  run shows one breaker staying closed the whole time, one cycling
  through open/half-open/closed as its upstream recovers, and one
  tripping open and staying there.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

Per the project's TDD workflow, tests are only written at pre-agreed
seams. Proposed for this project:

1. **`breaker.Breaker` state transitions** (pure, no I/O — inject a
   `Clock` so tests don't sleep)
   Given a breaker in various states, asserts: closed→open after exactly
   N consecutive failures (not N-1, not N+1); open→half-open only once
   the injected clock has advanced past `OpenTimeout` (and not a moment
   before); half-open→closed on a successful trial call; half-open→open
   on a failed trial call, with the cooldown restarting from that point.

2. **`breaker.Breaker.Execute(fn func() error) error`** — the
   call-wrapping seam
   Asserts: `fn` is called and its outcome recorded when the breaker is
   closed or half-open; when the breaker is open, `Execute` returns
   `ErrCircuitOpen` immediately **without** calling `fn` (asserted via a
   call counter/flag on a fake `fn`); a half-open trial only ever calls
   `fn` once, even if additional calls arrive concurrently.

3. **(stretch, optional)** an HTTP-level integration seam — an
   `http.RoundTripper` or handler wrapper around `Execute`, exercised
   with `net/http/httptest` against a fake upstream that flips between
   success and failure. Useful for confirming the breaker composes
   cleanly with real HTTP code, but not required to validate the state
   machine itself (seams 1 and 2 already cover that in full).

Implementation status right now: **shell only**. Types and function
signatures exist; `Execute`'s body returns "not implemented" so the
first real test against each seam starts red, as it should.

## 4. How to run it

```bash
# unit tests (once seam 1+ is implemented)
go test ./...

# run the full stack: 3 upstream containers (healthy/flaky/down) + demo,
# each demo breaker calling its own upstream over real container
# networking — currently every call just logs "not implemented" until
# Execute is implemented, which is expected
docker compose up --build

# or run pieces locally without Docker, e.g. against one upstream:
go run ./cmd/upstream -port 8080 -flaky-window 6 &
go run ./cmd/demo -upstreams "flaky=http://localhost:8080"
```
