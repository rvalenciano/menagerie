#ifndef CACHE_H
#define CACHE_H

#include <stddef.h>

/*
 * Opaque cache handle. Callers never see the internal hash table / LRU
 * list layout — that's the whole point of the seam: cache.c can change
 * its data structures freely as long as this header's contract holds.
 */
typedef struct Cache Cache;

/*
 * Create a cache with a fixed maximum number of entries ("capacity").
 * Returns NULL on allocation failure.
 */
Cache *cache_create(size_t capacity);

/*
 * Free a cache and everything it owns. Safe to call with NULL.
 */
void cache_destroy(Cache *cache);

/*
 * Insert or overwrite `key` with `value`, expiring after `ttl_ms`
 * milliseconds from now. If the cache is at capacity and `key` is not
 * already present, the least-recently-used entry must be evicted first.
 *
 * Returns 0 on success, -1 on failure (e.g. allocation failure).
 */
int cache_set(Cache *cache, const char *key, const char *value, long ttl_ms);

/*
 * Look up `key`. If found and not expired, copies the value (including
 * the terminating NUL, truncated to fit) into `out_value` (a buffer of
 * `out_size` bytes) and marks the entry as most-recently-used.
 *
 * Returns 0 on hit, -1 on miss (not found, or found but TTL elapsed —
 * callers cannot tell the difference, matching the protocol's single
 * "not found" reply for both cases).
 */
int cache_get(Cache *cache, const char *key, char *out_value, size_t out_size);

/*
 * Remove `key` if present.
 *
 * Returns 0 if the key was found and removed, -1 if it was not present.
 */
int cache_delete(Cache *cache, const char *key);

#endif /* CACHE_H */
