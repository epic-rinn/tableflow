# Change: 003-guest-access — Guest capabilities and mutation infrastructure

Status: done (M0 gate). Date: 2026-09-26. Scope owner: Claude. Task: MVP-03. Verification: M0 milestone gate ([ADR-0004](../../decisions/0004-milestone-verification.md)).

## Problem and behavior

Maps ACC-001 and ACC-004 in [access](../../features/05-access-pwa.md) and the idempotency rules of the [HTTP contract](../../api/http.md) and [data model](../../architecture/data-model.md). It provides the primitives that QUE-001, SEA-002 and ORD-003 will use later; it does not complete those requirements.

After this change the API offers:
- an **anonymous session** (`POST /sessions/anonymous`), a short-lived browser identity that scopes the queue-join idempotency key later on;
- a **capability exchange** (`POST /sessions/capability`) that turns a QR token for a queue ticket or dining visit into a guest session cookie;
- **rotation and revocation** of capabilities, where rotation invalidates every guest session derived from the old token;
- a **persistent idempotency store**, so business mutations can safely replay committed results.

The PWA gains QR entry pages that take the token from the URL fragment, remove it from history and POST it. They show a neutral "connected" state and no queue or visit screens.

Out of scope: issuing capabilities from real tickets or visits (MVP-05/06), the rotate-access endpoint (MVP-07), and guest logout.

## Decisions (reversible unless noted)

- **Capabilities:** one row per `(kind, resource_id)` (kinds `queue`, `visit`); a 256-bit token stored only as SHA-256; `generation` increments on rotation; optional expiry; revocation for terminal states. `resource_id` has no foreign key until the ticket and visit tables exist (added with MVP-05/06).
- **Guest sessions:** cookie `__Host-tf_guest` (HttpOnly, Secure, SameSite=Lax so a QR-scanned navigation keeps working, Path=/, host-only on the PWA origin). Each session stores the capability generation it came from and is valid only while that generation is current and the capability is not revoked or expired. Absolute lifetime 12 h. The token is not consumed, so any number of diners can exchange the same active token.
- **Anonymous sessions:** cookie `__Host-tf_anon`, 24 h absolute. Posting with a valid cookie returns the existing session.
- **Origin:** guest and anonymous mutations require an `Origin` in `PWA_ORIGINS`. The shared Origin guard moves to `httpx`. Staff, guest and anonymous cookies have distinct names, and each route accepts only its own kind.
- **Throttling:** fixed windows in PostgreSQL, shared with identity through `internal/platform/throttle`: exchange 120 and anonymous bootstrap 60 per client IP per 10 minutes (diners share the restaurant NAT).
- **Idempotency:** `idempotency_requests` unique on `(scope, operation, key)`. The first request inserts a placeholder in the caller's transaction, runs its effects, then stores status and response. A concurrent same-key request blocks on the unique index until the first commits: it then replays, or proceeds if the first rolled back. A different request hash returns `409 IDEMPOTENCY_CONFLICT`. Rolled-back work leaves no record. Keys are UUIDs from the `Idempotency-Key` header, and the scope always includes the authenticated principal. Callers authorise before calling `Execute`, and therefore also before any replay.
- **Replay encryption:** every stored response is sealed with AES-256-GCM using `DATA_ENCRYPTION_KEY` (required, base64 32 bytes), with scope, operation and key as associated data. This covers the secret-bearing replays of queue join. Retention is 24 h; MVP-05/06 extend it to "until the resource terminates" when they use it.
- **Maintenance:** the hourly purge covers expired guest and anonymous sessions (30 days after expiry) and expired idempotency rows.

## Routes

| Route | Actor | Result |
| --- | --- | --- |
| POST `/sessions/anonymous` | Public, PWA Origin, throttled | 201 new (or 200 existing) anonymous session, cookie |
| POST `/sessions/capability` | Public, PWA Origin, throttled | `{token, kind}` → 201 guest cookie + `{kind, resource_id, branch_id, expires_at}`; 422 `TOKEN_INVALID` for any unusable token |
| GET `/sessions/guest` | Guest session | 200 `{kind, resource_id, branch_id, expires_at}`; 401 |

## Query and endpoint impact

Exchange: 1 throttle upsert, 1 capability lookup by unique hash, 1 session insert. Guest authentication: 1 join of session and capability by unique hash. Idempotency: 1 insert, 1 update, or for replays 1 insert (no-op) plus 1 select, inside the caller's transaction. Rotation: 1 capability update and 1 session revocation through the partial index on `capability_id`. Measured at the M0 gate.

## Verification map

| Requirement / scenario | Named test | Status |
| --- | --- | --- |
| ACC-001 hashed tokens, multi-diner exchange | TestCapabilityExchangeMultipleDiners, TestTokensStoredHashedOnly | passed (M0 gate) |
| ACC-A2 rotation invalidates derived sessions | TestRotationRevokesDerivedSessions | passed (M0 gate) |
| ACC-001 revocation, wrong kind, expiry | TestRevokedExpiredAndWrongKindRejected | passed (M0 gate) |
| ACC-004 Origin, cookie isolation | TestGuestRoutesRequirePwaOrigin, TestCookieKindsAreIsolated | passed (M0 gate) |
| ACC-004 throttling | TestCapabilityExchangeRateLimited | passed (M0 gate) |
| Anonymous bootstrap | TestAnonymousSessionReuse | passed (M0 gate) |
| Idempotency: same key concurrent → one result | TestIdempotencyConcurrentSameKey | passed (M0 gate) |
| Idempotency: different body conflict, replay, rollback | TestIdempotencyConflictReplayRollback | passed (M0 gate) |
| Replay encrypted at rest | TestIdempotencyResponseEncrypted | passed (M0 gate) |
| Fragment → POST, history cleanup (browser) | PWA `entry.spec.ts` | passed (M0 gate) |
| Contract | TestHealthOpenApiValidation + TestGuestContractConformance | passed (M0 gate) |

## Final decisions

Implemented as planned. Gate fixes are listed in [review](review.md).
