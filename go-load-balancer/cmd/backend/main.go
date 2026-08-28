// Command backend is a minimal "echo" HTTP server used as a load
// balancer target: in docker-compose it stands in for a real backend
// instance, identifying itself in every response so round-robin
// distribution is visible from the outside.
//
// Every setting is available as a flag or an environment variable;
// precedence is flag > env var > built-in default:
//
//	flag              env var       default
//	-port             PORT          "8080"
//	-instance-id      INSTANCE_ID   os.Hostname()
//	-unhealthy        UNHEALTHY     false (if set, /health returns 503)
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	fs := flag.NewFlagSet("backend", flag.ContinueOnError)

	port := fs.String("port", getEnv("PORT", "8080"), "port to listen on")
	instanceID := fs.String("instance-id", getEnv("INSTANCE_ID", hostname()), "identifier returned in responses")
	unhealthy := fs.Bool("unhealthy", os.Getenv("UNHEALTHY") == "true", "if set, /health returns 503")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		log.Fatalf("config: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if *unhealthy {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello from backend %s\n", *instanceID)
	})

	addr := ":" + *port
	log.Printf("backend %s listening on %s", *instanceID, addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
