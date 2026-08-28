// Command backend is the shared "backend fleet" used across this
// monorepo's projects: a small real HTTP server whose health cycles
// over wall-clock time (unhealthy for UNHEALTHY_SECONDS out of every
// CYCLE_PERIOD_SECONDS), not per-request count — so every caller
// (the load balancer's health poller, its proxy, the circuit
// breaker's demo) sees a consistent, stateless answer regardless of
// how often or how many of them are asking. docker-compose.yml runs
// three instances of this same binary with different env vars:
//
//	backend-healthy   UNHEALTHY_SECONDS=0   (never unhealthy)
//	backend-flaky     UNHEALTHY_SECONDS=6, CYCLE_PERIOD_SECONDS=10 (default)
//	backend-down      UNHEALTHY_SECONDS=10, CYCLE_PERIOD_SECONDS=10 (always unhealthy)
//
// Both "/" and "/health" report the same underlying state.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	fs := flag.NewFlagSet("backend", flag.ContinueOnError)
	port := fs.String("port", getEnv("PORT", "8080"), "port to listen on")
	name := fs.String("name", getEnv("NAME", "backend"), "identifier returned in responses")
	cyclePeriod := fs.Int("cycle-period-seconds", getEnvInt("CYCLE_PERIOD_SECONDS", 10), "length of the failure/success cycle, in seconds")
	unhealthyFor := fs.Int("unhealthy-seconds", getEnvInt("UNHEALTHY_SECONDS", 6), "how many seconds of each cycle report unhealthy")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Fatalf("config: %v", err)
	}

	healthy := func() bool {
		if *unhealthyFor <= 0 {
			return true
		}
		return int(time.Now().Unix())%*cyclePeriod >= *unhealthyFor
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !healthy() {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !healthy() {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s: simulated failure\n", *name)
			return
		}
		fmt.Fprintf(w, "%s: ok\n", *name)
	})

	addr := ":" + *port
	log.Printf("backend %q listening on %s (unhealthy %ds of every %ds)", *name, addr, *unhealthyFor, *cyclePeriod)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
