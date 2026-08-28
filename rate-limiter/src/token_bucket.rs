//! A token-bucket rate limiter. Pure logic, no I/O — time is injected via
//! a `Clock` so tests can control the passage of time without sleeping.

use std::sync::Mutex;
use std::time::Instant;

/// Something that can report the current instant. Production code uses
/// `SystemClock`; tests inject a fake clock they can advance manually.
pub trait Clock: Send + Sync {
    fn now(&self) -> Instant;
}

/// The default `Clock`, backed by `std::time::Instant::now()`.
pub struct SystemClock;

impl Clock for SystemClock {
    fn now(&self) -> Instant {
        Instant::now()
    }
}

struct State {
    tokens: f64,
    last_refill: Instant,
}

/// A token bucket: holds up to `capacity` tokens, refilling at
/// `refill_rate` tokens/second. `try_acquire` consumes one token if
/// available.
///
/// Internal state is behind a `Mutex` so `try_acquire` can take `&self`
/// (rather than `&mut self`), which is what lets it be shared across
/// threads/requests behind a plain `Arc<TokenBucket>`.
pub struct TokenBucket {
    capacity: u32,
    refill_rate: f64,
    clock: Box<dyn Clock>,
    state: Mutex<State>,
}

impl TokenBucket {
    /// Builds a bucket that starts full, using the real system clock.
    pub fn new(capacity: u32, refill_rate: f64) -> Self {
        Self::with_clock(capacity, refill_rate, Box::new(SystemClock))
    }

    /// Builds a bucket with an injected clock, for tests that need to
    /// control the passage of time.
    pub fn with_clock(capacity: u32, refill_rate: f64, clock: Box<dyn Clock>) -> Self {
        let now = clock.now();
        TokenBucket {
            capacity,
            refill_rate,
            clock,
            state: Mutex::new(State {
                tokens: capacity as f64,
                last_refill: now,
            }),
        }
    }

    /// Attempts to consume one token. Returns `true` if a token was
    /// available (and consumes it), `false` otherwise.
    ///
    /// TODO(TDD): implement via TDD (proposed seam 1 in README.md §3):
    /// refill tokens based on elapsed time since `last_refill` at
    /// `refill_rate` tokens/sec, capped at `capacity`, then decide
    /// whether to consume a token.
    pub fn try_acquire(&self) -> bool {
        todo!("implement via TDD")
    }
}
