"""The orchestration seam: N worker threads pulling from a JobQueue and
retrying failed jobs according to a RetryPolicy, dead-lettering jobs that
exhaust their retries.
"""

import threading

from job_queue.queue import JobQueue
from job_queue.retry import RetryPolicy


class WorkerPool:
    """Owns N worker threads, a JobQueue, a RetryPolicy, and a
    dead-letter list.

    Each worker pulls a job, runs it, and on failure consults the
    RetryPolicy to decide whether to re-enqueue (after backoff) or move
    the job to the dead-letter list.
    """

    def __init__(
        self,
        num_workers: int,
        job_queue: JobQueue,
        retry_policy: RetryPolicy,
    ) -> None:
        self.num_workers = num_workers
        self.job_queue = job_queue
        self.retry_policy = retry_policy
        self.dead_letters: list = []
        self._threads: list[threading.Thread] = []
        self._stop_event = threading.Event()

    def start(self) -> None:
        """Spin up `num_workers` worker threads that pull jobs from
        `job_queue` and dispatch them (with retry-on-failure) until the
        pool is stopped.

        TODO(TDD): implement test-first (README.md seam 3).
        """
        raise NotImplementedError("implement via TDD")

    def _dispatch(self, job, attempt: int = 1) -> None:
        """Run a single job attempt, and on failure consult
        `retry_policy` to either re-enqueue the job (after backoff) or
        append it to `dead_letters` once retries are exhausted.

        TODO(TDD): implement test-first (README.md seam 3).
        """
        raise NotImplementedError("implement via TDD")

    def stop(self) -> None:
        """Signal worker threads to stop pulling new jobs and join them."""
        self._stop_event.set()
        for thread in self._threads:
            thread.join()
