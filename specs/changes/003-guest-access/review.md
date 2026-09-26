# Review: 003-guest-access

Date: 2026-09-26 (M0 milestone gate, [ADR-0004](../../decisions/0004-milestone-verification.md)). Reviewer: Claude — **self-review**.
Scope: commit `888e01f` plus gate fixes (uncommitted before the gate commit): migration `20260926140000_guest_access.sql`, `internal/access`, `internal/platform/{idempotency,throttle,token}`, httpx Origin guard, config (`PWA_ORIGINS`, `DATA_ENCRYPTION_KEY`), PWA `/q`, `/t`, OpenAPI. Result: **pass**.

Code review (tableflow-code-review) and DB/API review (tableflow-db-api-review, [performance.md](performance.md)) ran over the combined M0 changes after the first full gate run.

## Findings

| Severity | Location | Trigger / evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/access/http.go` `RequireAnonymous` | Code review: an invalid anonymous cookie went through the shared error path that clears the **guest** cookie | A diner with a stale anonymous cookie would lose their table session when joining the queue (MVP-05) | Fixed: clears only the anonymous cookie; `TestStaleAnonymousCookieKeepsGuestSession` |
| P3 | `src/pwa/components/QrEntry.tsx` | Gate run: `entry.spec.ts` failed — opening a second QR link in a tab already on `/q` (fragment-only navigation) showed "scan again" | Guest could not switch tickets/tables without a reload | Fixed: uses `useFragmentToken` (re-reads on `hashchange`) and exchanges each new token once |
| P3 (accepted) | Idempotency retention | Records expire after 24 h; the data model also requires retention until the ticket/visit terminates | Only matters once business mutations use the store | Follow-up owned by MVP-05/06 (pass a resource-aware TTL) |
| P3 (accepted) | `capabilities.resource_id` | No FK until tickets/visits exist | Orphan capabilities possible only via direct SQL | MVP-05/06 add the foreign keys |

Checked: capability and session secrets are stored only as SHA-256 hashes; tokens are exchanged from the fragment through a POST body and never appear in request URLs or `Referer` (browser test inspects every request); rotation invalidates derived sessions, including exchanges racing the rotation; guest mutations need a PWA Origin; the staff, guest, anonymous and member cookies are mutually isolated; idempotency replays are sealed with AES-GCM bound to scope/operation/key; rolled-back work leaves no record.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T12:07Z; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome 153) | **passed**: specs-check (61 files), Go format/vet/race, PostgreSQL tests, admin/PWA builds, browser tests admin 6/6 and PWA 9/9, artifact check (2687 files / 64 sentinels) |
| Go tests with `TABLEFLOW_REQUIRE_DB=1` | 72 passed, 0 failed/skipped (whole API) |
| MVP-03 tests | TestCapabilityExchangeMultipleDiners, TestTokensStoredHashedOnly, TestRotationRevokesDerivedSessions, TestRotationRacingExchangeNeverLeaksOldGeneration, TestRevokedExpiredAndWrongKindRejected, TestGuestRoutesRequirePwaOrigin, TestCookieKindsAreIsolated, TestAnonymousSessionReuse, TestCapabilityExchangeRateLimited, TestGuestRevalidateBlocksOnRotation, TestStaleAnonymousCookieKeepsGuestSession, TestGuestContractConformance, TestIdempotencyConcurrentSameKey (10 parallel), TestIdempotencyConflictReplayRollback, TestIdempotencyRolledBackOwnerLetsWaiterProceed, TestIdempotencyResponseEncrypted, TestParseKey, TestUpgradeFromPreviousRelease |
| PWA `entry.spec.ts` | passed: token absent from URLs/Referer/storage, HttpOnly Lax cookie, multi-diner, invalid/missing/wrong-kind messages |
| First gate run (12:01Z) | Go all passed; 1 browser failure (P3 above), fixed and re-run |

Mutation experiments were not repeated for MVP-03 (the lock-order ones were done in MVP-02). This is a coverage limit, not evidence.

## Delivery decision

No open P0/P1. Accepted P3 follow-ups owned by MVP-05/06. Status: **done** (M0 gate, 2026-09-26).
