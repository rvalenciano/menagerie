// Package balancer holds the load-balancing algorithm: a pool of
// backends and the logic that picks which one serves the next request.
// It has no network I/O, which is what makes it testable in isolation
// from the HTTP proxy and the health checker.
package balancer

import (
	"errors"
	"net/url"
	"sync/atomic"
)

// ErrNoHealthyBackends is returned by Pool.Next when every backend in
// the pool is currently marked unhealthy.
var ErrNoHealthyBackends = errors.New("balancer: no healthy backends available")

// Backend is one upstream instance the load balancer can route to.
type Backend struct {
	URL   *url.URL
	alive atomic.Bool
}

// NewBackend parses rawURL and returns a Backend marked alive by default.
func NewBackend(rawURL string) (*Backend, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	b := &Backend{URL: u}
	b.alive.Store(true)
	return b, nil
}

// SetAlive updates the backend's health state. Safe for concurrent use.
func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

// IsAlive reports the backend's current health state. Safe for
// concurrent use.
func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

// Pool is a round-robin pool of backends. Safe for concurrent use.
type Pool struct {
	backends []*Backend
	next     atomic.Uint64
}

// NewPool builds a Pool from the given backends.
func NewPool(backends []*Backend) *Pool {
	return &Pool{backends: backends}
}

// Backends returns the pool's backends, in the order they were
// registered.
func (p *Pool) Backends() []*Backend {
	return p.backends
}

// Next returns the next healthy backend in round-robin order.
// It returns ErrNoHealthyBackends if every backend is currently
// unhealthy, or if the pool is empty.
//
// TODO(TDD): implement test-first against internal/balancer/pool_test.go
// (seam 1 in README.md §3). This is the shell's first red test.
func (p *Pool) Next() (*Backend, error) {
	return nil, errors.New("not implemented")
}
