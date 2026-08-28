"""Demo: enqueue a mix of jobs onto a WorkerPool and print outcomes.

Simulates three kinds of jobs:
  - always-succeed: finishes on the first attempt.
  - fail-then-succeed: fails a few times, then succeeds.
  - always-fail: never succeeds, and should end up dead-lettered.

Jobs are persisted to Postgres via JobQueue, using the connection string
in the required DATABASE_URL environment variable (see docker-compose.yml,
service `job-queue-demo`).

Until the seams in job_queue/queue.py, job_queue/retry.py, and
job_queue/worker.py are implemented via TDD, this will surface a
NotImplementedError — first from JobQueue.enqueue below — that's
expected.
"""

import os

from job_queue.queue import JobQueue
from job_queue.retry import RetryPolicy
from job_queue.worker import WorkerPool

NUM_WORKERS = 4
MAX_ATTEMPTS = 5


class Job:
    """A fake unit of work used to demonstrate the worker pool."""

    def __init__(self, name: str, fail_count: int = 0) -> None:
        self.name = name
        self.fail_count = fail_count
        self.attempts = 0

    def run(self) -> None:
        self.attempts += 1
        if self.attempts <= self.fail_count:
            raise RuntimeError(
                f"{self.name}: simulated failure (attempt {self.attempts})"
            )
        print(f"{self.name}: succeeded on attempt {self.attempts}")


def build_demo_jobs() -> list[Job]:
    jobs = [Job(f"always-succeed-{i}", fail_count=0) for i in range(3)]
    jobs += [Job(f"fail-then-succeed-{i}", fail_count=2) for i in range(3)]
    jobs += [Job(f"always-fail-{i}", fail_count=MAX_ATTEMPTS + 1) for i in range(2)]
    return jobs


def main() -> None:
    database_url = os.environ["DATABASE_URL"]
    job_queue = JobQueue(database_url=database_url)
    retry_policy = RetryPolicy(max_attempts=MAX_ATTEMPTS, base_delay=0.1)
    pool = WorkerPool(NUM_WORKERS, job_queue, retry_policy)

    for job in build_demo_jobs():
        job_queue.enqueue(job)

    pool.start()
    pool.stop()

    print()
    print(f"Dead-lettered jobs ({len(pool.dead_letters)}):")
    for job in pool.dead_letters:
        print(f"  - {job.name} (exhausted after {job.attempts} attempts)")


if __name__ == "__main__":
    main()
