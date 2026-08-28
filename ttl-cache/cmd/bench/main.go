// Command bench is a small concurrent load client for ttl-cache.
//
// Opens N concurrent TCP connections against a running server and
// fires a SET/GET loop for a fixed duration, then reports total ops
// and throughput (ops/sec). It's intentionally simple — good enough to
// answer "does it fall over under concurrent connections," not a
// replacement for a real HTTP load-test tool. vegeta (used elsewhere
// in this monorepo) only speaks HTTP, and ttl-cache's protocol is
// plain line-based TCP, so this fills that gap in the same language as
// everything else here.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	host := flag.String("host", "127.0.0.1", "ttl-cache server host")
	port := flag.Int("port", 6380, "ttl-cache server port")
	clients := flag.Int("clients", 10, "number of concurrent connections")
	duration := flag.Duration("duration", 5*time.Second, "how long to run")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	deadline := time.Now().Add(*duration)

	var totalOps int64
	var wg sync.WaitGroup
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ops, err := runClient(addr, idx, deadline)
			if err != nil {
				log.Printf("worker %d error: %v", idx, err)
			}
			atomic.AddInt64(&totalOps, ops)
		}(i)
	}

	start := time.Now()
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	fmt.Printf("clients=%d duration=%.2fs total_ops=%d throughput=%.1f ops/sec\n",
		*clients, elapsed, totalOps, float64(totalOps)/elapsed)
}

// runClient repeatedly does a SET followed by a GET on its own key
// until deadline, returning how many ops (each SET or GET counts as
// one) it completed.
func runClient(addr string, idx int, deadline time.Time) (int64, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	var ops int64
	for i := 0; time.Now().Before(deadline); i++ {
		key := fmt.Sprintf("bench-key-%d-%d", idx, i%100)

		if _, err := fmt.Fprintf(conn, "SET %s value%d 60000\n", key, i); err != nil {
			return ops, err
		}
		if _, err := reader.ReadString('\n'); err != nil {
			return ops, err
		}
		ops++

		if _, err := fmt.Fprintf(conn, "GET %s\n", key); err != nil {
			return ops, err
		}
		if _, err := reader.ReadString('\n'); err != nil {
			return ops, err
		}
		ops++
	}
	return ops, nil
}
