// Package proxy contains the HTTP boundary of the load balancer: the
// handler that picks a backend from the pool and reverse-proxies the
// request to it.
package proxy

import (
	"net/http"

	"go-load-balancer/internal/balancer"
)

// Handler is an http.Handler that reverse-proxies each request to the
// next healthy backend chosen by Pool. A backend that fails while
// serving a proxied request is marked dead in the Pool (passive health
// check) so it's skipped on subsequent requests until an active health
// check marks it alive again.
type Handler struct {
	Pool *balancer.Pool
}

// NewHandler builds a Handler backed by pool.
func NewHandler(pool *balancer.Pool) *Handler {
	return &Handler{Pool: pool}
}

// ServeHTTP implements http.Handler.
//
// TODO(TDD): implement test-first against internal/proxy/proxy_test.go
// (seam 2 in README.md §3). This is the shell's second red test, after
// balancer.Pool.Next is implemented.
//
// Expected shape: call h.Pool.Next() — on ErrNoHealthyBackends, write a
// 502/503 and return. Otherwise build an httputil.ReverseProxy for the
// chosen backend's URL (httputil.NewSingleHostReverseProxy is the
// standard-library tool for this) and call its ServeHTTP. Set the
// proxy's ErrorHandler to mark the backend dead (backend.SetAlive(false)
// — this is the "passive" health check) and write an error response,
// rather than letting a broken backend hang the client.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
