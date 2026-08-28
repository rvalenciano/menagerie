# ttl-cache

An in-memory LRU cache with per-key TTL expiration, in C, built test-first.
Part of a personal portfolio ecosystem of small, focused apps in different
languages (see the sibling project `go-load-balancer`).

## 1. Problem Statement

### 1.1 Goal

Build a tiny Redis-subset — an in-memory key/value cache with:

1. A **fixed capacity**. When the cache is full and a new key is set,
   evict the **least-recently-used** entry to make room.
2. A **per-key TTL** (time-to-live, in milliseconds) set at write time. A
   key whose TTL has elapsed is treated as absent on `GET`, even if
   capacity-based eviction hasn't touched it yet (lazy expiry — no
   background sweep required).
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
- Concurrent multi-client scaling — a single-threaded accept loop
  (handling one connection fully before accepting the next) is
  acceptable for v1; documented as future work.

### 1.3 Why this is a good TDD project

- A **pure data structure** (hash table + LRU doubly-linked list + lazy
  TTL check) that has zero network I/O, so it can be tested by linking
  `cache.c` directly into a small test binary — no server, no sockets.
- A genuinely different **protocol seam**: C doesn't have a
  batteries-included test framework, so the TCP line protocol is
  exercised a different way than the data-structure logic — with a real
  socket client against a real running `server` binary, not asserts
  linked into a test binary. Learning to reach for two different testing
  styles for two different kinds of seam is itself part of the exercise.
- A genuine **performance question** ("how many ops/sec under concurrent
  clients?") that unit tests can't answer and that needs a real,
  networked load client to answer honestly.

## 2. Architecture

```
                    ┌───────────────────────────┐
  client(s) ──────► │        src/server.c        │
  SET/GET/DEL       │  (TCP accept loop, parses   │
  over TCP          │   the line protocol)        │
                    └─────────────┬───────────────┘
                                  │ calls
                                  ▼
                    ┌───────────────────────────┐
                    │        src/cache.c         │
                    │  hash table + LRU list +   │
                    │  lazy TTL check            │
                    │  (no network I/O)          │
                    └───────────────────────────┘
```

- **`include/cache.h` / `src/cache.c`** — the opaque `Cache` type and
  `cache_create` / `cache_destroy` / `cache_set` / `cache_get` /
  `cache_delete`. Pure logic, no sockets. This is the seam most unit
  testing happens against — it's compiled directly into a test binary,
  no server required.
- **`src/server.c`** — a minimal single-threaded POSIX-socket TCP server.
  Accepts one connection at a time, reads newline-terminated commands,
  parses `SET`/`GET`/`DEL`, and calls into `cache.c`. This is plumbing,
  not decision logic — it's fully wired up already (see "Implementation
  status" below), so it just gets error/not-found replies until the
  cache logic exists.
- **`tests/test_cache.c`** — hand-rolled `assert()`-based unit tests,
  compiled and linked directly against `src/cache.c`.
- **`bench/load_client.py`** — a small concurrent load client used for
  throughput sanity-checking against the real running server.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`cache_set` / `cache_get` / `cache_delete` — pure data-structure
   logic** (hash table + LRU order)
   Tested with plain `assert()`-based tests compiled directly against
   `cache.c`/`cache.h` (`tests/test_cache.c`) — no network involved, no
   external test framework dependency. Hand-rolled asserts are idiomatic
   enough for a C project this size; [Unity](https://github.com/ThrowTheSwitch/Unity)
   or [munit](https://nemequ.github.io/munit/) are options if the test
   file grows unwieldy, but neither is required to start.

2. **Eviction seam** (capacity + TTL interplay)
   Once the cache is at capacity, `cache_set` of a new key evicts the
   least-recently-used entry. `cache_get` on an entry whose TTL has
   elapsed returns "not found" even before any capacity-based eviction
   has run — lazy expiry, no background sweep needed. Same testing style
   as seam 1 (plain asserts against `cache.c` directly).

3. **TCP protocol seam** — a genuinely different kind of seam,
   **integration-level**, exercised with a real client connecting to a
   running `server` binary (e.g. `nc`, or a small shell/Python script
   sending `SET`/`GET`/`DEL` lines and checking replies) rather than a
   unit test binary linked against `cache.c`. Call this out explicitly:
   seams 1 and 2 are unit-level (no process boundary, no network); seam 3
   is integration-level (a real socket, a real running server process).

### Implementation status right now: shell only

`cache_create`/`cache_destroy` are fully implemented (allocation only).
`cache_set`/`cache_get`/`cache_delete` are stubs that return `-1` —
no hash table, no LRU list, no TTL logic exists yet. `server.c` is fully
wired up but will only ever get error/not-found replies until the cache
functions are implemented via TDD. `tests/test_cache.c` is a placeholder
(`int main(void) { return 0; }`) — real tests are added test-first, one
seam at a time.

## 4. How to run it

No local C toolchain is assumed — everything below can run in Docker.

```bash
# build the server binary (via Docker, matching CI/reproducible-build style)
docker run --rm -v "$PWD":/src -w /src gcc:13 make build

# run the placeholder test binary (real tests replace this as they're written)
docker run --rm -v "$PWD":/src -w /src gcc:13 make test

# build and run the containerized server, exposed on localhost:6380
docker build -t ttl-cache .
docker run --rm -p 6380:6380 ttl-cache

# talk to it by hand
printf 'SET foo bar 5000\r\nGET foo\r\n' | nc localhost 6380
```

## 5. Benchmarking

`bench/load_client.py` opens several concurrent TCP connections against a
running `server` and fires a SET/GET workload for a fixed duration,
reporting total ops and throughput. It's intentionally simple — good
enough to answer "does it fall over under concurrent connections," not a
replacement for a real tool like `wrk`/`vegeta`.

```bash
# with the server already running (see above) on localhost:6380
python3 bench/load_client.py --host 127.0.0.1 --port 6380 --clients 20 --duration 5
```

Meaningful throughput numbers only make sense once `cache_set`/`cache_get`
are implemented — right now every op returns an error/not-found reply
almost instantly, so a benchmark run today measures socket/parsing
overhead only.

## 6. Future work (explicitly out of scope for v1)

- Multi-client concurrency (thread-per-connection or an event loop)
  instead of the single-threaded accept loop.
- Persistence (append-only log or periodic snapshot) for restart
  durability.
- Additional eviction policies (LFU, random) behind the same
  `cache_set`/`cache_get` seam.
- A real benchmarking tool (`wrk`, `vegeta`, or similar) in place of the
  hand-rolled Python load client.
