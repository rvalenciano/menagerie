// Command server is a demo HTTP server: a stdlib net/http handler
// wrapped in a middleware that applies a single, process-wide
// TokenBucket to every request, returning 429 once it's empty.
package main

import (
	"log"
	"net/http"

	"rate-limiter/internal/ratelimit"
)

const (
	capacity         = 10
	refillRatePerSec = 5.0
)

// rateLimit wraps next with a middleware that consults bucket before
// every request. Since TokenBucket.TryAcquire is currently a stub
// (always returns false), every request will be rejected with 429
// until TryAcquire is implemented via TDD — that's expected and
// matches this repo's pattern of demos surfacing "not implemented"
// behavior cleanly rather than crashing.
func rateLimit(bucket *ratelimit.TokenBucket, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !bucket.TryAcquire() {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limit exceeded\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok\n"))
}

func main() {
	bucket := ratelimit.NewTokenBucket(capacity, refillRatePerSec)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	server := &http.Server{
		Addr:    "0.0.0.0:3000",
		Handler: rateLimit(bucket, mux),
	}

	log.Println("rate-limiter demo listening on 0.0.0.0:3000")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
