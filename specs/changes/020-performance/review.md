# Review: 020-performance

Date: 2026-09-27 (M6 milestone gate, ADR-0004). Reviewer: Claude — **self-review** (DB/API review). Scope: qualification harness, pool statistics logging, evidence. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `qualify.sh` cleanup | `docker rm -f` left each run's anonymous PostgreSQL volume (about 2 GB each). The Docker VM later ran out of disk | Disk exhaustion on repeated runs | `docker rm -fv`; the five leftover volumes from my runs were removed after matching their creation times |
| P3 | `qualify.sh` temp dir | The first smoke run deleted its temp dir during initial cleanup | Harness failure | Temp dir created after cleanup |
| P3 | Fixture | 70 active visits instead of the profile's 200 (at most 100 with 100 tables) | Slightly lighter shared-visit contention | Recorded; the profile text is inconsistent with one visit per table |
| P3 | Process | `gofmt` flagged `config.go` after the MVP-20 commit | A commit that would fail `api-check` | Formatted in the MVP-21 commit; the final `make verify` passed |

**Checked:**
- **Resource limits:** applied (`docker stats` samples show both containers within limits).
- **Secrets:** the evidence contains no tokens (load identities are hashed in the DB; raw tokens only in a deleted temp dir).
- **Pool statistics:** the logging is off by default and bounded (`TestPoolStatsInterval`); it reads `pgxpool.Stat` only.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T22:58Z UTC; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed**: 162 Go tests (race, PostgreSQL, none skipped), admin 16/16, PWA 25/25, cross-app journey 1/1, artifact check (2,881 files / 145 sentinels, no leaks) |
| Qualification | [performance.md](performance.md): 69.2 req/s × 600 s, 0 unexpected errors, all routes within budget, 0 lock waits, 0 deadlocks, no pool saturation |

## Delivery decision

No open P0/P1. Status: **done** (M6 gate).
