# wal

A crash-safe, append-only Write-Ahead Log, in Go, built test-first.
Part of the `menagerie` monorepo of small, focused apps.

## 1. Problem Statement

### 1.1 Goal

Build a Write-Ahead Log — the durability primitive underneath most
real databases (including the `postgres` this monorepo already runs),
message brokers, and AOF-style caches — as a small Go package (standard
library only), that:

1. `Append(record []byte) error` durably persists a record: once it
   returns successfully, the record must survive an immediate crash
   or power loss (not just a process exit — the bytes have to actually
   be on disk, not sitting in the OS page cache).
2. `Replay(fn func(record []byte) error) error` reads every record
   back, in append order, by calling `fn` for each one.
3. Survives a crash **mid-append**: if the process dies partway
   through writing a record (a torn/partial write), `Replay` on
   restart must recover every complete record before the crash and
   stop cleanly at the torn one — not error out on the whole file.

### 1.2 Non-goals (v1)

- Concurrent writers — one process appending at a time. Concurrent
  *readers* of an already-closed/rotated segment are a different,
  later concern.
- Segment rolling / compaction — one unbounded log file for v1.
  Splitting into fixed-size segments (so old ones can be deleted after
  a checkpoint) is real-world future work, deliberately deferred.
- Group commit (batching several `Append` calls behind one `fsync`) —
  a genuine v2 performance optimization once the naive one-`fsync`-
  per-`Append` version is green and benchmarked.
- Encryption, compression.

### 1.3 Why this is a good TDD project

A WAL splits cleanly into a **pure** half and a **real-I/O** half,
which is exactly the kind of seam boundary this whole monorepo is
built around:

- **Framing** (`internal/frame`) — how one record gets serialized into
  a self-describing chunk of bytes, and how a reader tells a complete,
  checksum-valid frame apart from a torn one. Zero file I/O — testable
  entirely with plain byte slices.
- **Durability** (`wal.Log.Append`) — the boundary where "return
  success" has to mean something real (bytes on disk, not just
  buffered), tested by injecting a fake `Syncer` so a test can assert
  `Sync` was actually called without touching a real disk.
- **Crash-safe recovery** (`wal.Log.Replay`) — the most interesting
  seam here: a test that appends N valid records, then appends a
  *deliberately truncated* (N+1)th frame's raw bytes (bypassing
  `Append` entirely, simulating exactly what a crash mid-write leaves
  behind), and asserts `Replay` recovers exactly the N complete
  records without erroring.

## 2. Architecture

```
  cmd/demo (a tiny KV store)
      │  put/delete → JSON-encode an op → Log.Append(bytes)
      │  startup    → Log.Replay(bytes) → decode op → rebuild map
      ▼
┌─────────────────────────┐
│   internal/wal.Log        │  Open / Append / Replay / Close
│   (file I/O, fsync)       │
└─────────────┬─────────────┘
              │ uses
              ▼
┌─────────────────────────┐
│   internal/frame          │  Encode / Decode
│   (pure bytes, no I/O)    │  length + CRC32 checksum framing
└─────────────────────────┘
```

- **`internal/frame`** — `Encode`/`Decode` a single record frame. Pure
  logic, no I/O. This is the seam most unit testing happens against.
- **`internal/wal`** — `Log` (`Open`, `Append`, `Replay`, `Close`) and
  the `Syncer` interface `Append` writes through. `Open`/`Close` are
  fully implemented plumbing; `Append`/`Replay` are the stubs.
- **`cmd/demo`** — a tiny in-memory key-value store layered on top of
  `wal.Log`: `put`/`delete` append-then-apply, startup replays to
  rebuild the map. This is what makes the WAL's guarantees observable
  — see §4 for the actual "kill -9 and recover" chaos test.

## 3. TDD plan — proposed seams (to confirm before writing the first test)

1. **`frame.Encode` / `frame.Decode`** (pure, no I/O)
   Round-trip a record through `Encode` then `Decode` and get the same
   bytes back. Separately: feed `Decode` a buffer that's too short for
   the header, one that has a valid header but a truncated body (the
   torn-write case), and one with a flipped bit in the payload (a
   checksum mismatch) — all three must return `frame.ErrCorrupt`, not
   panic or return a wrong length.

2. **`wal.Log.Append`** — the durability seam
   Inject a fake `Syncer` (implementing `Write`/`Sync`/`Close`) that
   records whether `Sync` was called. Assert `Append` calls `Write`
   then `Sync`, in that order, and that `Append` doesn't return until
   `Sync` has returned.

3. **`wal.Log.Replay`** — the crash-safety seam, the one worth the
   most attention
   Append N valid records via the real `Append`. Then, using a raw
   file handle (bypassing `Log` entirely), append a few extra bytes
   that look like the start of a frame but are cut short — exactly
   what a crash mid-`Append` would leave behind. `Replay` must yield
   exactly the N complete records, in order, and return `nil` — not an
   error.

4. **(stretch)** group commit — batching multiple pending `Append`
   calls behind a single `Sync`, and a benchmark showing the
   throughput difference vs. one-`Sync`-per-`Append`. Only worth doing
   once seams 1-3 are solid.

Implementation status right now: **shell only**. `Open`/`Close` on
`wal.Log` are real; `frame.Encode`/`Decode` and `wal.Log`'s
`Append`/`Replay` are stubs with hints — no framing format, no fsync
call, no replay loop exists yet.

## 4. How to run it

```bash
# build/vet (via Docker, no local Go toolchain assumed)
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine sh -c "go build ./... && go vet ./..."

# build and use the demo locally
docker build -t wal-demo .
docker run --rm -v "$PWD/data":/data wal-demo -log /data/demo.wal put foo bar
docker run --rm -v "$PWD/data":/data wal-demo -log /data/demo.wal get foo
docker run --rm -v "$PWD/data":/data wal-demo -log /data/demo.wal dump
```

### The actual point: crash it on purpose

Once `Append`/`Replay` are implemented, this is the demonstration that
matters — not "does it run," but "does it survive being killed
mid-write":

```bash
go build -o demo ./cmd/demo

./demo -log /tmp/demo.wal stress -n 2000000 &
PID=$!
sleep 0.2
kill -9 $PID                              # simulate a crash mid-append

./demo -log /tmp/demo.wal dump | wc -l    # some N <= 2,000,000 — and,
                                           # critically, no error and
                                           # no corrupted/garbage output
```

Right now this just surfaces "not implemented" at every step — that's
expected until the stubs above are filled in.

## 5. Benchmarking

The real performance question for a WAL is durable-append throughput —
how many `Append` calls/sec you can sustain when each one does a real
`fsync`. `stress -n <count>` (above) is the load generator: time it
(`time ./demo -log /tmp/bench.wal stress -n 100000`) once `Append` is
implemented. The natural v2 experiment is comparing that against a
group-commit design (seam 4) that batches several pending appends
behind one `fsync` — the throughput gap is usually dramatic, and it's
the same technique every real database uses.
