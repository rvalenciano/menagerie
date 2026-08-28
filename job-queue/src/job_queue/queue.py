"""Postgres-backed queue that jobs move through on their way to a worker.

No retry/backoff knowledge lives here — see retry.py for that. This module
only needs to move jobs in and out of the `jobs` table (see
infra/postgres/init/01_job_queue.sql) safely under concurrent access from
multiple worker processes/threads.
"""

import uuid

import psycopg


class JobQueue:
    """A Postgres-backed queue of jobs, persisted in the `jobs` table.

    Unlike the original in-memory design, jobs survive a process
    restart: they live in Postgres, not in local memory, so `enqueue`
    and `dequeue` are backed by real SQL against the `jobs` table
    rather than a `queue.Queue`.

    Args:
        database_url: a libpq-style connection string, e.g.
            `postgres://user:pass@host:5432/dbname?sslmode=disable`.
            Passed straight to `psycopg.connect`. A single connection is
            opened eagerly at construction time; if you'd rather pool
            connections across multiple worker threads, swap this for a
            `psycopg_pool.ConnectionPool(database_url)` instead — that's
            a later refinement, not required for the first seam.
    """

    def __init__(self, database_url: str) -> None:
        self.database_url = database_url
        self._conn = psycopg.connect(database_url)

    def enqueue(self, payload: dict) -> uuid.UUID:
        """Insert a new job with the given payload, returning its id.

        TODO(TDD): implement test-first (README.md seam 1).

        Expected shape: JSON-encode `payload` and
        `INSERT INTO jobs (payload) VALUES (%s) RETURNING id`, then
        commit and return the returned `id` (a UUID). Everything else
        (status, attempt_count, next_retry_at) should be left to the
        table's defaults.
        """
        raise NotImplementedError("implement via TDD")

    def dequeue(self, timeout=None):
        """Atomically claim and return the next ready job, or None/raise
        on timeout if none is ready (exact behavior is the user's call).

        TODO(TDD): implement test-first (README.md seam 1).

        Expected shape, in one transaction: `SELECT id, payload, ...
        FROM jobs WHERE status = 'pending' AND next_retry_at <= now()
        ORDER BY next_retry_at FOR UPDATE SKIP LOCKED LIMIT 1`, then
        `UPDATE jobs SET status = 'in_progress', attempt_count =
        attempt_count + 1 WHERE id = %s`, then commit and return the
        claimed row. `FOR UPDATE SKIP LOCKED` is the key concept here —
        research it: it's what lets multiple worker processes/threads
        call `dequeue` concurrently against the same table without ever
        racing each other onto the same row.
        """
        raise NotImplementedError("implement via TDD")
