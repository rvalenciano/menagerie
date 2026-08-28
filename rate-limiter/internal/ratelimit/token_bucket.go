// Package ratelimit implements a token-bucket rate limiter. Pure logic,
// no I/O — time is injected via a Clock so tests can control the
// passage of time without sleeping.
package ratelimit

import (
	"sync"
	"time"
)

// Clock reports the current time. Production code uses time.Now via
// NewTokenBucket; tests inject a fake clock they can advance manually.
type Clock func() time.Time

// TokenBucket holds up to Capacity tokens, refilling at RefillRate
// tokens/second. TryAcquire consumes one token if available.
//
// Internal state is protected by a Mutex so TryAcquire can be called
// concurrently, which is what lets a single *TokenBucket be shared
// across goroutines/requests.
type TokenBucket struct {
	capacity   uint32
	refillRate float64
	clock      Clock

	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
}

// NewTokenBucket builds a bucket that starts full, using the real
// system clock (time.Now).
func NewTokenBucket(capacity uint32, refillRate float64) *TokenBucket {
	return NewTokenBucketWithClock(capacity, refillRate, time.Now)
}

// NewTokenBucketWithClock builds a bucket with an injected clock, for
// tests that need to control the passage of time.
func NewTokenBucketWithClock(capacity uint32, refillRate float64, clock Clock) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		refillRate: refillRate,
		clock:      clock,
		tokens:     float64(capacity),
		lastRefill: clock(),
	}
}

// TryAcquire attempts to consume one token. It returns true if a token
// was available (and consumes it), false otherwise.
//
// TODO(TDD): implement via TDD (proposed seam 1 in README.md §3):
// refill tokens based on elapsed time since lastRefill at refillRate
// tokens/sec, capped at capacity, then decide whether to consume a
// token.
//
// Expected shape: lock b.mu; compute elapsed := b.clock().Sub(b.lastRefill);
// add elapsed.Seconds() * b.refillRate to b.tokens, capped at
// float64(b.capacity); set b.lastRefill to the current clock reading;
// if b.tokens >= 1, subtract 1 and return true, otherwise return false
// without modifying b.tokens.
func (b *TokenBucket) TryAcquire() bool {
	return false
}
