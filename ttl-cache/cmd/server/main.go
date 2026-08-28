// Command server runs the ttl-cache TCP server: it listens for
// connections, parses newline-terminated commands, and calls into the
// (currently stubbed) internal/cache package.
//
//	SET key value ttl_ms\n   -> "OK\n" | "ERR\n"
//	GET key\n                -> "VALUE <value>\n" | "NOT_FOUND\n"
//	DEL key\n                -> "OK\n" | "NOT_FOUND\n"
//
// This file is networking/parsing plumbing only; all cache decision
// logic lives behind internal/cache and is currently stubbed, so every
// command below will return an error/not-found reply until cache.go is
// implemented via TDD. That's expected and fine for v1 scaffolding.
//
// Unlike the original C version's single-threaded accept loop (one
// connection fully served before the next was accepted, documented
// there as a v1 limitation), each connection here gets its own
// goroutine — a small, essentially free improvement in Go. That does
// NOT make the Cache itself safe for concurrent access, though:
// internal/cache.Cache has no locking yet. Guarding it (e.g. with a
// sync.Mutex) is part of what implementing Set/Get/Delete test-first
// should address, not something bolted on here in the plumbing layer.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"ttl-cache/internal/cache"
)

func main() {
	port := flag.Int("port", 6380, "port to listen on")
	capacity := flag.Int("capacity", 1024, "maximum number of cache entries")
	flag.Parse()

	c := cache.NewCache(*capacity)

	addr := fmt.Sprintf(":%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	log.Printf("ttl-cache listening on port %d (capacity=%d)", *port, *capacity)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go serveClient(conn, c)
	}
}

// serveClient reads newline-terminated commands from conn, one at a
// time, until the client disconnects or a read error occurs.
func serveClient(conn net.Conn, c *cache.Cache) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimRight(line, "\r\n")
		reply := handleCommand(c, line)
		if reply == "" {
			continue
		}
		if _, err := conn.Write([]byte(reply)); err != nil {
			return
		}
	}
}

// handleCommand parses a single command line and dispatches it to the
// cache, returning the newline-terminated protocol reply. An empty
// line produces no reply, matching the C original's behavior of simply
// reading the next line when a command is blank.
func handleCommand(c *cache.Cache, line string) string {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}

	switch parts[0] {
	case "SET":
		if len(parts) < 4 {
			return "ERR\n"
		}
		key, value, ttlStr := parts[1], parts[2], parts[3]

		ttlMs, err := strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			return "ERR\n"
		}

		if err := c.Set(key, value, time.Duration(ttlMs)*time.Millisecond); err != nil {
			return "ERR\n"
		}
		return "OK\n"

	case "GET":
		if len(parts) < 2 {
			return "ERR\n"
		}

		value, err := c.Get(parts[1])
		if err != nil {
			return "NOT_FOUND\n"
		}
		return fmt.Sprintf("VALUE %s\n", value)

	case "DEL":
		if len(parts) < 2 {
			return "ERR\n"
		}

		if err := c.Delete(parts[1]); err != nil {
			return "NOT_FOUND\n"
		}
		return "OK\n"

	default:
		return "ERR unknown command\n"
	}
}
