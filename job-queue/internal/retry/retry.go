// Package retry implements retry/backoff decision logic.
//
// Pure decision logic, no I/O: given an attempt count, decide whether a
// failed job should be retried and how long to wait before the next
// attempt. Sleep is injectable so tests never have to wait for real
// backoff durations.
package retry

import (
	"errors"
	"time"
)

// RetryPolicy decides whether to retry a failed job, and how long to
// wait.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts allowed (including
	// the first, non-retry attempt) before a job is considered
	// exhausted.
	MaxAttempts int
	// BaseDelay is the base delay used by the backoff calculation in
	// NextDelay.
	BaseDelay time.Duration
	// Sleep is an injectable sleep function, defaulting to time.Sleep,
	// so tests can avoid real delays.
	Sleep func(time.Duration)
}

// NewRetryPolicy returns a RetryPolicy with the given maxAttempts and
// baseDelay, and Sleep defaulted to time.Sleep.
func NewRetryPolicy(maxAttempts int, baseDelay time.Duration) *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts: maxAttempts,
		BaseDelay:   baseDelay,
		Sleep:       time.Sleep,
	}
}

// ShouldRetry reports whether a job that just failed on attempt should
// be retried.
//
// TODO(TDD): implement test-first (README.md seam 2).
//
// Expected shape: a job gets p.MaxAttempts total tries, so this is just
// `attempt < p.MaxAttempts`.
func (p *RetryPolicy) ShouldRetry(attempt int) bool {
	panic(errors.New("not implemented"))
}

// NextDelay returns the exponential backoff delay before the next retry
// of attempt.
//
// TODO(TDD): implement test-first (README.md seam 2).
//
// Expected shape: classic exponential backoff — the delay roughly
// doubles with each attempt, e.g. `p.BaseDelay * 2^(attempt-1)`.
// Consider adding jitter (a small random fudge factor) so many
// simultaneously-failing jobs don't all retry in lockstep — not
// required for the first green test, but worth researching.
func (p *RetryPolicy) NextDelay(attempt int) time.Duration {
	panic(errors.New("not implemented"))
}
