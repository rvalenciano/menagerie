# go-load-balancer

A pure-Go Application Load Balancer (L7 / HTTP), built test-first. Part of a
personal portfolio ecosystem of small, focused apps in different languages.

## 1. Problem Statement

### 1.1 Goal

Build an HTTP Application Load Balancer, in pure Go (standard library only —
no third-party routing/proxy frameworks), that:

1. Accepts incoming HTTP requests on a single listen address.
2. Distributes each request to one of **N registered backend instances**
   using a **round-robin** strategy.
3. Detects unhealthy backends and stops routing traffic to them, using
   **both**:
   - **Active health checks** — a background goroutine periodically polls
     each backend's `/health` endpoint and marks it up/down.
   - **Passive health checks** — a backend that fails while actively
     proxying a live request is immediately marked down (don't wait for
     the next active probe).
4. Recovers automatically — a backend that starts passing active health
   checks again is returned to the rotation.
5. Survives and stays correct under concurrent load (many goroutines
   picking backends and proxying requests at once).
6. Can be benchmarked under heavy, sustained request pressure to produce
   throughput and latency numbers (p50/p95/p99), not just "it works."

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- TLS termination / HTTPS.
- Layer 4 (TCP) load balancing — HTTP only.
- Weighted or least-connections balancing (documented as a v2 extension —
  the `Balancer` picking logic is isolated behind a seam specifically so
  this can be swapped later without touching the proxy or health-check
  code).
- Dynamic service discovery (DNS-based, Consul, etc.) — the backend list
  is static, provided via config/env at startup.
- Sticky sessions, rate limiting, request retries/circuit breaking beyond
  "skip a known-dead backend."

### 1.3 Why this is a good TDD project

An ALB has a small number of genuinely interesting, independently testable
behaviors — that's what makes it a good exercise:

- A **pure, stateful algorithm** (round-robin over a mutable set of
  healthy/unhealthy backends) that has no network I/O at all, so it can be
  tested with plain Go structs and no server.
- An **HTTP boundary** (the proxy handler) that can be fully exercised in
  a test process using `net/http/httptest`, with fake backends standing in
  for real ones — no Docker required to test correctness.
- A **time-based background process** (the active health checker) that
  needs to be testable without actually sleeping for real intervals in
  the test suite.
- A genuine **performance question** ("does it fall over under load?")
  that unit tests cannot answer and that needs real, multi-process,
  networked load generation to answer honestly.

### 1.4 How we'll test each concern

| Concern | How it's exercised | Tooling |
|---|---|---|
| N instances of a service to distribute traffic to | Unit/integration tests spin up N `httptest.Server` fakes in-process; `docker-compose.yml` spins up N real `cmd/backend` containers | `net/http/httptest`, Docker Compose |
| Correctness under heavy pressure | Real load fired at the real, containerized load balancer + real backend containers | [vegeta](https://github.com/tsenart/vegeta) (pure Go load-testing tool) via `benchmark/` |

## 2. Architecture

```
                 ┌─────────────────────────┐
  client  ─────► │   cmd/loadbalancer       │
  requests       │                          │
                 │  ┌────────┐  ┌─────────┐ │      ┌───────────────┐
                 │  │ proxy  │─►│  pool   │◄┼──────┤ healthcheck    │
                 │  │handler │  │(balancer)│ │      │ (active poll)  │
                 │  └────────┘  └─────────┘ │      └───────┬───────┘
                 └─────────────┬────────────┘              │
                                │  round-robin over          │ GET /health
                                │  healthy backends           │
                 ┌──────────────┼──────────────┬─────────────▼──┐
                 ▼              ▼              ▼                │
           backend-1      backend-2      backend-3   ◄──────────┘
         (cmd/backend)  (cmd/backend)  (cmd/backend)
```

- **`internal/balancer`** — `Backend` (URL + alive flag) and `Pool`
  (round-robin picker over healthy backends). No network I/O. This is the
  seam most future balancing strategies (least-connections, weighted,
  random) would plug into.
- **`internal/proxy`** — an `http.Handler` that asks the `Pool` for the
  next healthy backend, reverse-proxies the request to it
  (`httputil.ReverseProxy`), and marks the backend dead on failure
  (passive health check).
- **`internal/healthcheck`** — a `Checker` that, on a fixed interval,
  concurrently `GET`s each backend's health path and updates its alive
  state in the `Pool` (active health check).
- **`internal/config`** — parses backend list / ports / intervals from
  CLI flags and environment variables (stdlib `flag` package only — no
  CLI framework). Precedence: flag > env var > built-in default. Run
  `./loadbalancer -help` / `./backend -help` for the full flag list.
- **`cmd/loadbalancer`** — wires the above into a running HTTP server.
- **`cmd/backend`** — a minimal "echo" HTTP server used both as the fake
  backend in integration tests conceptually, and as the real container
  image in `docker-compose.yml`. Returns its own instance ID so you can
  visually confirm round-robin distribution, and exposes `/health` (with
  an optional env-var toggle to simulate failure for manual chaos
  testing).

## 3. TDD plan — confirmed seams

Per the project's TDD workflow, tests are only written at pre-agreed
seams. For this project those are:

1. **`balancer.Pool.Next()`** (pure logic, no network)
   Given a set of backends in various alive/dead states, asserts:
   round-robin order among healthy backends, dead backends are skipped,
   a backend recovering rejoins rotation, and `ErrNoHealthyBackends` is
   returned when none are alive. Also exercised under `-race` with
   concurrent callers.

2. **`proxy.Handler` — the HTTP seam** (`net/http/httptest`)
   N fake backends (`httptest.Server`) registered in a real `Pool`, real
   HTTP requests fired at a real `Handler` wrapped in `httptest.Server`.
   Asserts: requests are distributed round-robin, a failing backend gets
   marked dead after a failed proxy attempt (passive check) and is no
   longer selected, and the client gets a sane error (not a hang) when
   every backend is down.

3. **`healthcheck.Checker` — the active-polling seam**
   A fake backend that flips between 200 and 500 on `/health`. Asserts
   the `Pool`'s view of that backend's alive state converges to match,
   using an injected clock/short interval rather than real sleeps, so the
   test is fast and deterministic.

`internal/config` is intentionally **not** a formal TDD seam — it's
env-var parsing glue, not decision logic worth the ceremony.

Implementation status right now: **shell only**. Types and function
signatures exist; bodies return "not implemented" so the first real test
against each seam starts red, as it should.

## 4. Running it

```bash
# unit/integration tests (once seam 1+ is implemented)
make test

# build and run the whole stack: 1 load balancer + 3 backends
make up

# hit it a few times by hand and watch the instance ID rotate
for i in $(seq 1 6); do curl -s http://localhost:8080/; echo; done

make down
```

## 5. Benchmarking under heavy pressure

`benchmark/` builds a small pure-Go [vegeta](https://github.com/tsenart/vegeta)
image and fires sustained load at the real, containerized load balancer
sitting in front of the real backend containers.

```bash
# rate=200 req/s, duration=30s, target=http://loadbalancer:8080/ (defaults)
make bench

# custom rate/duration
make bench RATE=1000 DURATION=1m
```

This produces a `vegeta report` with request rate achieved, success ratio,
and latency percentiles (p50/p95/p99/max) — the numbers that answer "does
it stand heavy pressure," which no unit test can.

## 6. Future work (explicitly out of scope for v1)

- Additional `Balancer` strategies behind the same `Pool` seam
  (least-connections, weighted round-robin).
- DNS-based dynamic backend discovery (Docker Compose's embedded DNS
  round-robins A records for a scaled service — `docker compose up
  --scale backend=N` — which would let N be chosen at runtime instead of
  hardcoded in `docker-compose.yml`).
- TLS termination.
- Structured metrics (Prometheus `/metrics`) — currently only `vegeta`
  reports give performance numbers, and only per-run, not continuously.
