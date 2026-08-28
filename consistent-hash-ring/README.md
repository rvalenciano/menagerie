# consistent-hash-ring

A pure-Go consistent-hashing ring, built test-first. Part of a personal
portfolio ecosystem of small, focused apps in different languages.

## 1. Problem Statement

### 1.1 Goal

Build a reusable Go package implementing **consistent hashing with
virtual nodes** that:

1. Maps an arbitrary string key to one of **N registered node names**,
   deterministically — the same key always resolves to the same node
   for a fixed set of registered nodes.
2. Supports adding and removing nodes at runtime.
3. When a node is added or removed, remaps only a **small fraction of
   keys** (roughly `1/N`, where N is the resulting node count) — not
   the whole keyspace, which is the entire point of consistent hashing
   over naive `hash(key) % N` sharding.
4. Uses **virtual nodes** (multiple ring positions per physical node) so
   that key ownership is reasonably balanced across nodes even when N
   is small.

Plus a tiny CLI (`cmd/demo`) that takes a node list and a set of sample
keys, prints the key→node assignment, then adds/removes a node and
prints how many keys changed owner.

### 1.2 Non-goals (v1)

To keep the first slice honest and TDD-able, v1 deliberately excludes:

- Actual data migration or replication — this project only **computes**
  ownership (which node *would* own a key); it doesn't move any data
  anywhere.
- Weighted nodes — every node gets the same number of virtual nodes in
  v1 (documented as a v2 extension).
- Any network/HTTP surface — this is a pure library + CLI, not a
  service.

### 1.3 Why this is a good TDD project

Consistent hashing is almost entirely **pure logic** — hashing, sorting,
binary search over a ring — with zero I/O, which makes it a clean TDD
exercise:

- A **pure, deterministic algorithm** (`Ring.Get`) that can be tested
  with plain Go values and no server, mocks, or fixtures.
- A **genuinely interesting statistical property** to test, not just
  deterministic lookup correctness: after adding or removing a node,
  the fraction of keys that changed owner should be roughly `1/N`. This
  is a property/bound test, not an exact-value test — a good exercise
  in writing assertions that are honest about what's actually
  guaranteed.
- A **distribution property** for virtual nodes: with virtual nodes
  enabled, key ownership across physical nodes should be roughly
  balanced even for a small number of nodes — another bound-style
  assertion rather than an exact one.
- No network, no time, no concurrency to fake out — the whole thing is
  testable with `go test ./...` and nothing else.

## 2. Architecture

```
                     ┌───────────────────────────┐
  key ("user:42") ─► │        ring.Ring           │
                     │                            │
                     │  hash(key) ──► walk         │
                     │  clockwise on sorted ring   │
                     │  points to nearest node     │
                     │                            │
                     │  ┌──────────────────────┐  │
                     │  │ sortedHashes []uint32 │  │
                     │  │ hashToNode map        │  │
                     │  └──────────────────────┘  │
                     └─────────────┬──────────────┘
                                   │
                AddNode(ctx, name) / RemoveNode(ctx, name)
                adds/removes virtualNodesPerNode points
                per physical node (in-memory), AND
                persists the membership change to Postgres
                                   │
                     ┌─────────────▼──────────────┐
                     │   node-1  node-2  node-3    │
                     │  (each represented by many  │
                     │   virtual points on the ring)│
                     └─────────────┬──────────────┘
                                   │
                                   ▼
                     ┌───────────────────────────┐
                     │  Postgres: ring_nodes       │
                     │  (name, added_at, removed_at)│
                     │  append-only membership log │
                     └───────────────────────────┘
```

- **`internal/ring`** — the whole algorithm, plus a thin persistence
  seam for membership. `Ring` holds a sorted list of virtual-node hash
  positions (`sortedHashes`) and a lookup from position to owning node
  name (`hashToNode`) — this part is pure, in-memory, and unaffected by
  persistence. `AddNode`/`RemoveNode` mutate both the in-memory
  structure **and**, when the `Ring` was built with `NewRingWithDB`,
  persist the membership change to the `ring_nodes` table (an insert for
  add, a `removed_at` update for remove); `Get` hashes the key and finds
  the nearest position clockwise (wrapping around at the end of the
  ring) via binary search — `Get` has no persistence involved at all,
  it's a pure in-memory lookup. `LoadActiveNodeNames` reads the
  currently-active rows from `ring_nodes` so a process can rebuild its
  in-memory ring on startup; it only returns names, it never touches
  `sortedHashes`/`hashToNode` directly — wiring the names back in via
  `AddNode` is the caller's job. In short: **membership is persisted,
  the derived ring math is not** — the hash structure is always rebuilt
  in memory from the persisted node list, never stored itself.
- **`cmd/demo`** — wires a `Ring` to a CLI: reads `DATABASE_URL` from the
  environment (fails loudly if unset) and connects via `pgx.Connect`,
  calls `LoadActiveNodeNames` to show what's currently persisted, parses
  `-nodes`/`-keys` flags (stdlib `flag` package only, no CLI framework),
  prints the resulting key→node assignment, then adds and removes a node
  and reports how many keys changed owner, so the `1/N` property is
  visible by eye as well as by test.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`Ring.Get(key string) (node string, err error)`** — deterministic
   lookup. Given a fixed set of registered nodes, the same key always
   maps to the same node across repeated calls. Returns the sentinel
   `ring.ErrNoNodes` when the ring has no nodes registered. This is the
   first red test — everything else builds on it.

2. **`Ring.AddNode` / `Ring.RemoveNode` — remapping-fraction test, now
   against a real Postgres.** Hash a large sample of keys (e.g. a few
   thousand) against the ring before and after adding (or removing) one
   node out of N. Assert the fraction of keys that changed owner is
   roughly proportional to `1/N`. This is a **property/statistical
   test**, not an exact-value test — assert the observed fraction falls
   within a reasonable bound around the expected `1/N`, not a hardcoded
   percentage. Since `AddNode`/`RemoveNode` now take a `context.Context`
   and persist membership via a `*pgx.Conn`, this seam needs a real
   Postgres to test against — the docker-compose `postgres` service (db
   `systems_lab`), or one run locally — rather than being purely
   in-memory. Also worth a dedicated test: re-adding a previously-removed
   node name should succeed (a fresh row, per the partial unique index on
   `ring_nodes(name) WHERE removed_at IS NULL`), not conflict with its
   old removed row.

3. **Virtual-node distribution seam.** With virtual nodes enabled,
   assert that key ownership across nodes is roughly balanced — e.g.
   each node owns somewhere near `total_keys / N`, within a reasonable
   bound — even for a small number of physical nodes (2-5). Worth
   testing at more than one `virtualNodesPerNode` value to see the
   bound tighten as virtual nodes increase.

Implementation status right now: **shell only**. Types and function
signatures exist in `internal/ring/ring.go`; bodies are stubbed (`Get`,
`AddNode`, `RemoveNode` all return a plain `"not implemented"` error) so
the first real test against seam 1 starts red, as it should.
`LoadActiveNodeNames` and the `pgx` connection plumbing (`NewRingWithDB`,
`cmd/demo`'s `DATABASE_URL` wiring) are already implemented for
real — they're infrastructure, not the algorithm under test.

## 4. Running it

```bash
# once seam 1+ is implemented
go test ./...

# build and run the CLI demo directly (requires DATABASE_URL, e.g. against
# the docker-compose postgres service or a local Postgres with the
# ring_nodes table from infra/postgres/init/02_consistent_hash_ring.sql)
DATABASE_URL="postgres://systems_lab:systems_lab@localhost:5432/systems_lab?sslmode=disable" \
  go run ./cmd/demo -nodes=node-1,node-2,node-3 -sample=1000

# or via Docker
docker build -t consistent-hash-ring .
docker run --rm -e DATABASE_URL="postgres://systems_lab:systems_lab@host.docker.internal:5432/systems_lab?sslmode=disable" \
  consistent-hash-ring -nodes=node-1,node-2,node-3

# or via docker-compose from the repo root (wires DATABASE_URL for you)
docker compose run --build consistent-hash-ring-demo
```
