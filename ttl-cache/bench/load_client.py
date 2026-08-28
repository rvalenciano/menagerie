#!/usr/bin/env python3
"""
Small concurrent load client for ttl-cache.

Opens N concurrent connections against a running `server` instance and
fires a mix of SET/GET ops for a fixed duration, then reports total ops
and throughput (ops/sec). Not a substitute for a real benchmarking tool
(e.g. wrk/vegeta) — just enough to sanity-check "does it fall over under
concurrent connections" once the cache logic is implemented.

Usage:
    python3 bench/load_client.py --host 127.0.0.1 --port 6380 \
        --clients 20 --duration 5
"""

import argparse
import socket
import threading
import time


def worker(host, port, duration, counters, index):
    ops = 0
    try:
        with socket.create_connection((host, port), timeout=5) as sock:
            f = sock.makefile("rwb", buffering=0)
            deadline = time.monotonic() + duration
            i = 0
            while time.monotonic() < deadline:
                key = f"bench-key-{index}-{i % 100}"
                f.write(f"SET {key} value{i} 60000\n".encode())
                f.readline()
                f.write(f"GET {key}\n".encode())
                f.readline()
                ops += 2
                i += 1
    except OSError as exc:
        print(f"worker {index} error: {exc}")
    counters[index] = ops


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=6380)
    parser.add_argument("--clients", type=int, default=10)
    parser.add_argument("--duration", type=float, default=5.0)
    args = parser.parse_args()

    counters = [0] * args.clients
    threads = [
        threading.Thread(target=worker, args=(args.host, args.port, args.duration, counters, i))
        for i in range(args.clients)
    ]

    start = time.monotonic()
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    elapsed = time.monotonic() - start

    total_ops = sum(counters)
    print(f"clients={args.clients} duration={elapsed:.2f}s total_ops={total_ops} "
          f"throughput={total_ops / elapsed:.1f} ops/sec")


if __name__ == "__main__":
    main()
