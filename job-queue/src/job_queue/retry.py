"""Retry/backoff decision logic.

Pure decision logic, no I/O: given an attempt count, decide whether a
failed job should be retried and how long to wait before the next
attempt. `clock` and `sleep` are injected so tests never have to wait for
real backoff durations.
"""

import time


class RetryPolicy:
    """Decides whether to retry a failed job, and how long to wait.

    Args:
        max_attempts: total number of attempts allowed (including the
            first, non-retry attempt) before a job is considered
            exhausted.
        base_delay: base delay, in seconds, used by the backoff
            calculation in `next_delay`.
        clock: injectable time source (defaults to `time.monotonic`),
            useful for deterministic tests.
        sleep: injectable sleep function (defaults to `time.sleep`), so
            tests can avoid real delays.
    """

    def __init__(
        self,
        max_attempts: int,
        base_delay: float = 1.0,
        clock=time.monotonic,
        sleep=time.sleep,
    ) -> None:
        self.max_attempts = max_attempts
        self.base_delay = base_delay
        self.clock = clock
        self.sleep = sleep

    def should_retry(self, attempt: int) -> bool:
        """Return True if a job that just failed on `attempt` should be
        retried.

        TODO(TDD): implement test-first (README.md seam 2).
        """
        raise NotImplementedError("implement via TDD")

    def next_delay(self, attempt: int) -> float:
        """Return the exponential backoff delay, in seconds, before the
        next retry of `attempt`.

        TODO(TDD): implement test-first (README.md seam 2).
        """
        raise NotImplementedError("implement via TDD")
