package balancer_test

import (
	"testing"

	"go-load-balancer/internal/balancer"
)

func TestPool_Next_RoundRobinsHealthyBackends(t *testing.T) {
	b1, _ := balancer.NewBackend("http://backend-1:8080")
	b2, _ := balancer.NewBackend("http://backend-2:8080")
	b3, _ := balancer.NewBackend("http://backend-3:8080")
	pool := balancer.NewPool([]*balancer.Backend{b1, b2, b3})

	// 5 calls on a 3-backend pool: this checks ordering AND that it
	// wraps back around to b1 after b3, in one sequence.
	want := []*balancer.Backend{b1, b2, b3, b1, b2}

	for i, w := range want {
		got, err := pool.Next()
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if got != w {
			t.Errorf("call %d: got %s, want %s", i, got.URL, w.URL)
		}
	}
}
