# ttl-cache

An in-memory LRU cache with per-key TTL expiration, in Go, built
test-first. Part of a personal portfolio ecosystem of small, focused
apps — originally written in C, ported to Go so the whole fleet shares
one language and toolchain (see the sibling projects `go-load-balancer`
and `consistent-hash-ring`).

## 1. Problem Statement

### 1.1 Goal

Build a tiny Redis-subset — an in-memory key/value cache with:

1. A **fixed capacity**. When the cache is full and a new key is set,
   evict the **least-recently-used** entry to make room.
2. A **per-key TTL** (time-to-live) set at write time. A key whose TTL
   has elapsed is treated as absent on `GET`, even if capacity-based
   eviction hasn't touched it yet (lazy expiry — no background sweep
   required).
3. Exposure over a minimal **line-based TCP protocol**:
   - `SET key value ttl_ms\n`
   - `GET key\n` — replies with the value, or a "not found" sentinel
     (covers both "never set" and "expired")
   - `DEL key\n`

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- Persistence (AOF/RDB-style durability) — the cache is purely in-memory
  and its contents are lost on restart.
- Replication.
- Eviction policies other than LRU (no LFU, no random eviction).
- Authentication.
- A concurrency-safe `Cache` — each TCP connection gets its own
  goroutine (see §2), but `internal/cache.Cache` itself has no locking
  yet. Making it safe for concurrent access (e.g. with a `sync.Mutex`)
  is future work, layered on top of the seams below once they exist.

### 1.3 Why this is a good TDD project

- A **pure data structure** (hash table + LRU doubly-linked list + lazy
  TTL check) that has zero network I/O, so it's testable with Go's
  stdlib `testing` package directly against `internal/cache` — no
  server, no sockets. This is a nice upgrade over the original C
  version's hand-rolled `assert()` tests: table-driven `testing.T` tests,
  `t.Run` subtests, and `go test -race` all come for free.
- A genuinely different **protocol seam**: the TCP line protocol is
  exercised a different way than the data-structure logic — with a real
  client hitting a real running `server` binary (e.g. `nc`, or a small
  script sending `SET`/`GET`/`DEL` lines and checking replies), not
  `testing.T` assertions linked into the same binary. Learning to reach
  for two different testing styles for two different kinds of seam is
  itself part of the exercise.
- A genuine **performance question** ("how many ops/sec under concurrent
  clients?") that unit tests can't answer and that needs a real,
  networked load client to answer honestly.

## 2. Architecture

```
                    ┌───────────────────────────┐
  client(s) ──────► │      cmd/server/main.go    │
  SET/GET/DEL       │  (TCP accept loop, parses   │
  over TCP          │   the line protocol)        │
                    └─────────────┬───────────────┘
                                  │ calls
                                  ▼
                    ┌───────────────────────────┐
                    │    internal/cache/cache.go │
                    │  hash table + LRU list +   │
                    │  lazy TTL check            │
                    │  (no network I/O)          │
                    └───────────────────────────┘
```

- **`internal/cache`** — the `Cache` type and `NewCache` / `Set` / `Get`
  / `Delete`. Pure logic, no sockets. This is the seam most unit testing
  happens against — it's imported directly into a `_test.go` file, no
  server required.
- **`cmd/server`** — a TCP server built on `net.Listen`. Accepts
  connections, reads newline-terminated commands, parses
  `SET`/`GET`/`DEL`, and calls into `internal/cache`. This is plumbing,
  not decision logic — it's fully wired up already (see "Implementation
  status" below), so it just gets error/not-found replies until the
  cache logic exists. Each connection runs in its own goroutine (a
  small, free improvement over the original C version's single
  connection at a time), but see the concurrency non-goal above — the
  `Cache` itself isn't safe for concurrent access yet.
- **`cmd/bench`** — a small concurrent Go load client used for
  throughput sanity-checking against the real running server. vegeta
  (used elsewhere in this monorepo) only speaks HTTP, and this fills
  the same role for ttl-cache's TCP line protocol.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`Cache.Set` / `Cache.Get` / `Cache.Delete` — pure data-structure
   logic** (hash table + LRU order)
   Tested with Go's stdlib `testing` package, directly against
   `internal/cache` — no network involved, no external test framework.

2. **Eviction seam** (capacity + TTL interplay)
   Once the cache is at capacity, `Set` of a new key evicts the
   least-recently-used entry. `Get` on an entry whose TTL has elapsed
   returns `ErrNotFound` even before any capacity-based eviction has
   run — lazy expiry, no background sweep needed. Same testing style as
   seam 1 (`testing.T` against `internal/cache` directly).

3. **TCP protocol seam** — a genuinely different kind of seam,
   **integration-level**, exercised with a real client connecting to a
   running `server` binary (e.g. `nc`, or a small shell/Python script
   sending `SET`/`GET`/`DEL` lines and checking replies) rather than a
   unit test linked into the same binary. Call this out explicitly:
   seams 1 and 2 are unit-level (no process boundary, no network); seam
   3 is integration-level (a real socket, a real running server
   process).

### Implementation status right now: shell only

`NewCache` is fully implemented (struct init only). `Set`/`Get`/`Delete`
are stubs that return a `not implemented` error — no hash table, no LRU
list, no TTL logic exists yet. `cmd/server` is fully wired up but will
only ever get error/not-found replies until the cache methods are
implemented via TDD. No test files exist yet — real tests are added
test-first, one seam at a time.

## 4. How to run it

No local Go toolchain is assumed — everything below can run in Docker.

```bash
# build both binaries (via Docker, matching CI/reproducible-build style)
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine sh -c "go build ./... && go vet ./..."

# run tests, once real ones exist (real tests replace this as they're written)
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine go test ./...

# build and run the containerized server, exposed on localhost:6380
docker build -t ttl-cache .
docker run --rm -p 6380:6380 ttl-cache

# talk to it by hand
printf 'SET foo bar 5000\r\nGET foo\r\n' | nc localhost 6380
```

## 5. Benchmarking

`cmd/bench` opens several concurrent TCP connections against a running
`server` and fires a SET/GET workload for a fixed duration, reporting
total ops and throughput. It's intentionally simple — good enough to
answer "does it fall over under concurrent connections," not a
replacement for a real tool like `wrk`.

```bash
# with the server already running (see above) on localhost:6380
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine \
  go run ./cmd/bench --host host.docker.internal --port 6380 --clients 20 --duration 5s

# or, from the monorepo root, as a compose service against the
# containerized server (see root README.md § Benchmarking):
#   docker compose -f docker-compose.yml -f docker-compose.bench.yml \
#     up --build --abort-on-container-exit bench-ttl-cache
```

Meaningful throughput numbers only make sense once `Set`/`Get` are
implemented — right now every op returns an error/not-found reply
almost instantly, so a benchmark run today measures socket/parsing
overhead only.

## 6. Future work (explicitly out of scope for v1)

- Making `internal/cache.Cache` safe for concurrent access (e.g. a
  `sync.Mutex`) now that each connection runs in its own goroutine.
- Persistence (append-only log or periodic snapshot) for restart
  durability.
- Additional eviction policies (LFU, random) behind the same
  `Set`/`Get` seam.
- A real benchmarking tool (`wrk` or similar) in place of the
  hand-rolled Go load client, if latency percentiles are ever needed.
