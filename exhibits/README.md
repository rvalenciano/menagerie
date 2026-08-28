# exhibits

The shared backend fleet of the `menagerie` monorepo: a single small
Go HTTP server, deployed three times with different health profiles —
each instance its own "exhibit" of backend temperament — used by every
project here that needs "N backend instances to talk to":
`load-balancer`, `circuit-breaker`, and anything added later.

Health is a pure function of wall-clock time (`unhealthy for
UNHEALTHY_SECONDS out of every CYCLE_PERIOD_SECONDS seconds`), not
request count — so it gives every caller a consistent answer
regardless of how often or how many of them are asking, with no shared
mutable state and no locking.

| Compose service   | `UNHEALTHY_SECONDS` | `CYCLE_PERIOD_SECONDS` | Behavior          |
|-------------------|---------------------|------------------------|-------------------|
| `backend-healthy` | `0`                 | (irrelevant)           | Never unhealthy   |
| `backend-flaky`   | `6`                 | `10`                   | Down 6s, up 4s, repeating |
| `backend-down`    | `10`                | `10`                   | Always unhealthy  |

Both `/` (real traffic) and `/health` (active health-check probes)
report the same underlying state, so the load balancer's health
checker and its proxy always agree with each other.

This is shared infrastructure, not a TDD exercise on its own — there's
no algorithmic decision to test here (see the root `README.md` for
where the interesting TDD seams live).
