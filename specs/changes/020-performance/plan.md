# Change: 020-performance — Representative performance qualification

Status: implemented-unverified (M6 gate pending; qualification run complete). Date: 2026-09-27. Scope owner: Claude. Task: MVP-20. Verification: M6 gate (ADR-0004).

## Problem and behavior

Performance gates for all implemented requirements, under the canonical profile in [performance](../../quality/performance.md): the Go API at 2 vCPU/1 GiB and PostgreSQL 18 at 2 vCPU/2 GiB, with the full synthetic fixture, the mixed polling and mutation workload, a 2-minute warm-up and a 10-minute measured run.

## Decisions

- **Harness:** `src/api/tests/performance/qualify.sh` builds resource-limited containers locally, reusing the `postgres:18.6` image so no new images are pulled. It then:
  - builds the fixture from the milestone seed scripts in order;
  - mints load identities (only SHA-256 digests are stored);
  - runs the static Linux API binary;
  - runs `qualify_load.py`;
  - samples `docker stats`, `pg_stat_activity` lock waits and `pg_stat_database`, and captures the API pool statistics;
  - always removes its containers.
- **Pool statistics:** `DB_POOL_STATS_INTERVAL` (API config, off by default) logs `pgxpool` saturation: acquires, waited acquires and average acquire time. It records the pool waits this task requires and serves operations monitoring (MVP-21).
- **Load runner:** Python standard library only. k6 was suggested in `docs/development/performance.md`, but it would add an unpinned external tool and the Python runner already records everything required.

## Verification

See [performance.md](performance.md). All routes are within budget, with 0 unexpected errors, 0 lock waits and 0 deadlocks, and no pool saturation.
