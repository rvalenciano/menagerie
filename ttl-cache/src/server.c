/*
 * Minimal single-threaded TCP server for the ttl-cache line protocol:
 *
 *   SET key value ttl_ms\n   -> "OK\n" | "ERR\n"
 *   GET key\n                -> "VALUE <value>\n" | "NOT_FOUND\n"
 *   DEL key\n                -> "OK\n" | "NOT_FOUND\n"
 *
 * This file is networking/parsing plumbing only; all cache decision logic
 * lives behind cache.h and is currently stubbed, so every command below
 * will return an error/not-found reply until cache.c is implemented via
 * TDD. That's expected and fine for v1 scaffolding.
 */

#define _POSIX_C_SOURCE 200809L

#include <arpa/inet.h>
#include <errno.h>
#include <netinet/in.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

#include "cache.h"

#define DEFAULT_PORT 6380
#define DEFAULT_CAPACITY 1024
#define LINE_BUF_SIZE 4096
#define VALUE_BUF_SIZE 2048

static int start_listener(int port)
{
    int fd = socket(AF_INET, SOCK_STREAM, 0);
    if (fd < 0) {
        perror("socket");
        return -1;
    }

    int opt = 1;
    setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_ANY);
    addr.sin_port = htons((uint16_t)port);

    if (bind(fd, (struct sockaddr *)&addr, sizeof(addr)) < 0) {
        perror("bind");
        close(fd);
        return -1;
    }

    if (listen(fd, 16) < 0) {
        perror("listen");
        close(fd);
        return -1;
    }

    return fd;
}

/* Reads a single '\n'-terminated line (NUL-terminated, newline stripped)
 * from a connected socket. Returns line length on success, 0 on clean
 * EOF/disconnect, -1 on error or if the line would overflow buf. */
static ssize_t read_line(int client_fd, char *buf, size_t buf_size)
{
    size_t len = 0;

    while (len + 1 < buf_size) {
        char c;
        ssize_t n = recv(client_fd, &c, 1, 0);
        if (n == 0) {
            return len > 0 ? (ssize_t)len : 0;
        }
        if (n < 0) {
            if (errno == EINTR) {
                continue;
            }
            return -1;
        }
        if (c == '\n') {
            buf[len] = '\0';
            return (ssize_t)len;
        }
        if (c != '\r') {
            buf[len++] = c;
        }
    }

    return -1; /* line too long */
}

static void handle_command(int client_fd, Cache *cache, char *line)
{
    char reply[VALUE_BUF_SIZE + 32];
    char *saveptr = NULL;
    char *cmd = strtok_r(line, " ", &saveptr);

    if (cmd == NULL) {
        return;
    }

    if (strcmp(cmd, "SET") == 0) {
        char *key = strtok_r(NULL, " ", &saveptr);
        char *value = strtok_r(NULL, " ", &saveptr);
        char *ttl_str = strtok_r(NULL, " ", &saveptr);

        if (key == NULL || value == NULL || ttl_str == NULL) {
            snprintf(reply, sizeof(reply), "ERR\n");
        } else {
            long ttl_ms = strtol(ttl_str, NULL, 10);
            int rc = cache_set(cache, key, value, ttl_ms);
            snprintf(reply, sizeof(reply), rc == 0 ? "OK\n" : "ERR\n");
        }
    } else if (strcmp(cmd, "GET") == 0) {
        char *key = strtok_r(NULL, " ", &saveptr);
        char value[VALUE_BUF_SIZE];

        if (key == NULL) {
            snprintf(reply, sizeof(reply), "ERR\n");
        } else {
            int rc = cache_get(cache, key, value, sizeof(value));
            if (rc == 0) {
                snprintf(reply, sizeof(reply), "VALUE %s\n", value);
            } else {
                snprintf(reply, sizeof(reply), "NOT_FOUND\n");
            }
        }
    } else if (strcmp(cmd, "DEL") == 0) {
        char *key = strtok_r(NULL, " ", &saveptr);

        if (key == NULL) {
            snprintf(reply, sizeof(reply), "ERR\n");
        } else {
            int rc = cache_delete(cache, key);
            snprintf(reply, sizeof(reply), rc == 0 ? "OK\n" : "NOT_FOUND\n");
        }
    } else {
        snprintf(reply, sizeof(reply), "ERR unknown command\n");
    }

    send(client_fd, reply, strlen(reply), 0);
}

static void serve_client(int client_fd, Cache *cache)
{
    char line[LINE_BUF_SIZE];

    for (;;) {
        ssize_t n = read_line(client_fd, line, sizeof(line));
        if (n <= 0) {
            break;
        }
        handle_command(client_fd, cache, line);
    }

    close(client_fd);
}

int main(int argc, char **argv)
{
    int port = DEFAULT_PORT;
    size_t capacity = DEFAULT_CAPACITY;

    if (argc > 1) {
        port = atoi(argv[1]);
    }
    if (argc > 2) {
        capacity = (size_t)strtoul(argv[2], NULL, 10);
    }

    Cache *cache = cache_create(capacity);
    if (cache == NULL) {
        fprintf(stderr, "failed to create cache\n");
        return 1;
    }

    int listen_fd = start_listener(port);
    if (listen_fd < 0) {
        cache_destroy(cache);
        return 1;
    }

    printf("ttl-cache listening on port %d (capacity=%zu)\n", port, capacity);

    for (;;) {
        struct sockaddr_in client_addr;
        socklen_t client_len = sizeof(client_addr);
        int client_fd = accept(listen_fd, (struct sockaddr *)&client_addr, &client_len);
        if (client_fd < 0) {
            if (errno == EINTR) {
                continue;
            }
            perror("accept");
            continue;
        }

        /* Single-threaded, one connection at a time — acceptable for v1;
         * concurrency is future work (see README non-goals). */
        serve_client(client_fd, cache);
    }

    close(listen_fd);
    cache_destroy(cache);
    return 0;
}
