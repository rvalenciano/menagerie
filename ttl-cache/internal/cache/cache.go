// Package cache implements an in-memory key/value cache with a fixed
// capacity and per-key TTL expiration: a hash table for O(1) lookup
// plus an LRU doubly-linked list for O(1) least-recently-used eviction.
// It has zero network I/O, which is what makes it testable with plain
// Go stdlib testing — no server, no sockets.
package cache

import (
	"errors"
	"time"
)

// ErrNotFound is returned by Get and Delete when key is not present —
// either it was never set, or (for Get only) it was set but its TTL has
// elapsed. Callers cannot tell the two cases apart, matching the line
// protocol's single NOT_FOUND reply for both.
var ErrNotFound = errors.New("cache: not found")

// Cache is an in-memory key/value store with a fixed maximum number of
// entries ("capacity"). When full and a new key is set, the
// least-recently-used entry is evicted to make room. Each entry also
// carries its own TTL, checked lazily on Get.
//
// Not yet safe for concurrent use — see cmd/server/main.go, which hands
// each connection its own goroutine but does no locking of its own.
type Cache struct {
	capacity int
	// TODO(TDD): hash table + LRU doubly-linked list fields go here,
	// added test-first as Set/Get/Delete are built out. You may also
	// want an injectable clock (e.g. a `now func() time.Time` field,
	// defaulting to time.Now) so Get's TTL check is testable without
	// real sleeps — your call when you design this.
}

// NewCache creates a cache that holds at most capacity entries.
func NewCache(capacity int) *Cache {
	return &Cache{capacity: capacity}
}

// Set inserts or overwrites key with value, expiring ttl after now.
//
// TODO(TDD): implement test-first (seams 1-2 in README.md §3). Expected
// shape: look up key in the hash table.
//   - If found: update its value and set its expiry to
//     time.Now().Add(ttl), then move it to the front of the LRU list
//     (it's now the most-recently-used entry).
//   - If not found and the cache is at capacity: evict the LRU list's
//     tail entry (the least-recently-used one) first, freeing its slot,
//     before inserting the new one at the front.
//   - If not found and there's room: insert a new entry at the front.
func (c *Cache) Set(key, value string, ttl time.Duration) error {
	return errors.New("not implemented")
}

// Get looks up key. If found and not expired, it returns the value and
// moves the entry to the front of the LRU list (it was just used).
//
// TODO(TDD): implement test-first (seams 1-2 in README.md §3). Expected
// shape: look up key.
//   - Not found, OR found but time.Now() is at or after its stored
//     expiry (TTL elapsed): treat as a miss — return ErrNotFound, and
//     while you're there, actually remove the expired entry (lazy
//     expiry: nothing sweeps the table in the background, so a stale
//     entry only ever gets cleaned up the next time something touches
//     it).
//   - Found and not expired: move it to the front of the LRU list and
//     return its value.
func (c *Cache) Get(key string) (string, error) {
	return "", errors.New("not implemented")
}

// Delete removes key if present, from both the hash table and the LRU
// list.
//
// TODO(TDD): implement test-first (seams 1-2 in README.md §3). Expected
// shape: if key is present, remove it from the hash table and unlink it
// from the LRU list, then return nil. If it isn't present, this is a
// no-op — return ErrNotFound.
func (c *Cache) Delete(key string) error {
	return errors.New("not implemented")
}
