#include "cache.h"

#include <stdlib.h>

struct Cache {
    size_t capacity;
    /* TODO(TDD): hash table + LRU doubly-linked list fields go here,
     * added test-first as cache_set/cache_get/cache_delete are built out. */
};

Cache *cache_create(size_t capacity)
{
    Cache *cache = malloc(sizeof(Cache));
    if (cache == NULL) {
        return NULL;
    }

    cache->capacity = capacity;
    return cache;
}

void cache_destroy(Cache *cache)
{
    if (cache == NULL) {
        return;
    }

    free(cache);
}

int cache_set(Cache *cache, const char *key, const char *value, long ttl_ms)
{
    (void)cache;
    (void)key;
    (void)value;
    (void)ttl_ms;
    return -1; /* TODO(TDD): implement via TDD */
}

int cache_get(Cache *cache, const char *key, char *out_value, size_t out_size)
{
    (void)cache;
    (void)key;
    (void)out_value;
    (void)out_size;
    return -1; /* TODO(TDD): implement via TDD */
}

int cache_delete(Cache *cache, const char *key)
{
    (void)cache;
    (void)key;
    return -1; /* TODO(TDD): implement via TDD */
}
