CC ?= gcc
CFLAGS ?= -std=c11 -Wall -Wextra -Iinclude
LDFLAGS ?=

.PHONY: build test clean

build: server

server: src/server.c src/cache.c include/cache.h
	$(CC) $(CFLAGS) -o server src/server.c src/cache.c $(LDFLAGS)

test: test_cache

test_cache: tests/test_cache.c src/cache.c include/cache.h
	$(CC) $(CFLAGS) -o test_cache tests/test_cache.c src/cache.c
	./test_cache

clean:
	rm -f server test_cache *.o
