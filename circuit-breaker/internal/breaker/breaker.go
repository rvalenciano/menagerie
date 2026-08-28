// Package breaker implements a Circuit Breaker: a state machine that
// wraps calls to an unreliable dependency and fails fast once that
// dependency looks broken, instead of letting every caller wait out a
// timeout against it. It has no network I/O of its own, which is what
// makes it testable in isolation from whatever it ends up protecting.
package breaker

import (
	"errors"
	"time"
)

// ErrCircuitOpen is returned by Execute without calling the wrapped
// function when the breaker is open.
var ErrCircuitOpen = errors.New("breaker: circuit open")

// State is one of the breaker's three states.
type State int

const (
	// Closed is the normal state: calls pass through and failures are
	// counted.
	Closed State = iota
	// Open is the tripped state: calls fail fast with ErrCircuitOpen.
	Open
	// HalfOpen allows exactly one trial call through to test whether the
	// dependency has recovered.
	HalfOpen
)

// String implements fmt.Stringer.
func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures a Breaker.
type Config struct {
	// FailureThreshold is the number of consecutive failures that trips
	// the breaker from closed to open.
	FailureThreshold int
	// OpenTimeout is how long the breaker stays open before allowing a
	// single half-open trial call.
	OpenTimeout time.Duration
	// Clock returns the current time. Defaults to time.Now via
	// NewBreaker — tests inject a fake clock so they don't sleep.
	Clock func() time.Time
	// OnTransition, if non-nil, is an optional hook a caller can inject
	// to be notified whenever the breaker's state actually changes. It
	// exists so a separate concern (e.g. an audit log, see
	// internal/translog) can observe transitions without this package
	// needing to know anything about persistence. Left nil by
	// NewConfig; callers opt in explicitly.
	OnTransition func(from, to State)
}

// NewConfig returns a Config with the given threshold and timeout, and
// Clock defaulted to time.Now.
func NewConfig(failureThreshold int, openTimeout time.Duration) Config {
	return Config{
		FailureThreshold: failureThreshold,
		OpenTimeout:      openTimeout,
		Clock:            time.Now,
	}
}

// Breaker is a consecutive-failure-count circuit breaker. Closed→Open
// after Config.FailureThreshold consecutive failures; Open→HalfOpen only
// after Config.OpenTimeout has elapsed; HalfOpen→Closed on a successful
// trial call; HalfOpen→Open on a failed trial call.
type Breaker struct {
	cfg   Config
	state State
}

// NewBreaker builds a Breaker in the Closed state from cfg.
func NewBreaker(cfg Config) *Breaker {
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &Breaker{cfg: cfg, state: Closed}
}

// State returns the breaker's current state.
func (b *Breaker) State() State {
	return b.state
}

// Execute runs fn if the breaker is not open, and records the outcome
// against the breaker's state machine. If the breaker is open, fn is
// not called and ErrCircuitOpen is returned immediately. Once
// implemented, it should invoke cfg.OnTransition(oldState, newState)
// (if non-nil) exactly when the state actually changes.
//
// TODO(TDD): implement test-first against
// internal/breaker/breaker_test.go (seams 1 and 2 in README.md §3). This
// is the shell's first red test.
//
// Expected shape: if b.state is Open, check whether
// b.cfg.Clock().Sub(<time the breaker opened>) >= b.cfg.OpenTimeout —
// if not, return ErrCircuitOpen without calling fn; if so, move to
// HalfOpen and fall through to a trial call. Otherwise (Closed or
// HalfOpen) call fn(). On a nil error: reset any failure count, and if
// state was HalfOpen move to Closed. On a non-nil error: if state is
// HalfOpen, move straight back to Open (and record the new open time);
// if state is Closed, increment the consecutive-failure count and move
// to Open (recording the open time) once it reaches
// b.cfg.FailureThreshold. Whenever state actually changes, call
// b.cfg.OnTransition(old, new) if non-nil (see Config.OnTransition).
// You'll need a field to remember when the breaker opened, and a
// mutex — Execute must be safe for concurrent callers.
func (b *Breaker) Execute(fn func() error) error {
	return errors.New("not implemented")
}
