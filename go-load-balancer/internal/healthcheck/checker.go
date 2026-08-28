// Package healthcheck implements the active side of backend health
// detection: a background loop that polls each backend's health
// endpoint and updates its state in the pool.
package healthcheck

import (
	"context"
	"net/http"
	"time"

	"go-load-balancer/internal/balancer"
)

// Checker actively polls every backend in Pool on Path, every Interval,
// and updates each backend's alive state accordingly.
type Checker struct {
	Pool     *balancer.Pool
	Path     string
	Interval time.Duration
	Client   *http.Client
}

// NewChecker builds a Checker with a sane default HTTP client timeout.
func NewChecker(pool *balancer.Pool, path string, interval time.Duration) *Checker {
	return &Checker{
		Pool:     pool,
		Path:     path,
		Interval: interval,
		Client:   &http.Client{Timeout: 2 * time.Second},
	}
}

// Run polls every backend in the pool once per Interval, updating its
// alive state, until ctx is done. It blocks the calling goroutine —
// callers should run it in its own goroutine.
//
// TODO(TDD): implement test-first against
// internal/healthcheck/checker_test.go (seam 3 in README.md §3).
//
// Expected shape: a time.Ticker at c.Interval; on each tick (and
// probably once immediately, before the first tick), loop over
// c.Pool.Backends() and GET backend.URL joined with c.Path using
// c.Client — a 200 response means SetAlive(true), anything else
// (non-200, or the request erroring/timing out) means SetAlive(false).
// Doing each backend's check in its own goroutine (with a
// sync.WaitGroup per tick) keeps one slow/dead backend from delaying
// the check on the others. Select on ctx.Done() to exit the loop.
func (c *Checker) Run(ctx context.Context) {
}
