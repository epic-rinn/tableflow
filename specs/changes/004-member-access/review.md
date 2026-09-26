# Review: 004-member-access

Date: 2026-09-26 (M0 milestone gate, [ADR-0004](../../decisions/0004-milestone-verification.md)). Reviewer: Claude — **self-review**.
Scope: commit `351f3fa` plus gate fixes: migration `20260926150000_member_access.sql`, `internal/members`, `internal/platform/{mail,password}`, config (`SMTP_ADDR`, `MAIL_FROM`, `PWA_PUBLIC_URL`), PWA `/account/**`, OpenAPI. Result: **pass**.

## Findings

| Severity | Location | Trigger / evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/members/members.go` Signup | Code review: an existing email receives an "account exists" notice on every signup attempt; only a per-IP limit applied | Distributed requests could flood a person's inbox | Fixed: 5 signups per email per 10 min (the 429 applies to any email, so it reveals nothing); covered in TestMemberRateLimits |
| P3 | ResendVerification | No per-IP limit | Mail spraying across many unverified accounts | Fixed: 30 per IP per 10 min |
| P3 (accepted) | Reset request timing | A known email does a token insert and enqueue; an unknown one does not (small, sub-millisecond difference) | Theoretical enumeration by timing | Accepted; delivery is already asynchronous |
| P3 (accepted) | Mail queue | In-process queue; messages are lost on restart or when full (logged) | A user may need to request a link again | Accepted for MVP; revisit with the provider in MVP-21 |
| Decided (2026-09-26, owner) | Production email provider | Owner chose Resend via SMTP with TLS required; Mailpit stays local and the adapter stays replaceable | — | Implemented: `SMTP_TLS` implicit/starttls with no plaintext fallback, PLAIN auth only over TLS, config refuses plaintext to non-loopback relays (`TestStartTLSNeverFallsBackToPlaintext`, `TestSMTPRequiresTLSOutsideLoopback`). Sender domain and credentials pending from the owner; not blocking (MVP-21) |

Checked: neutral signup/reset responses (byte-identical bodies); identical login failure for unknown and wrong-password accounts; single-use, superseded and expired tokens rejected; a reset revokes every session, verifies the email and defeats an account squatter; parallel reset confirmations have one winner; member cookies coexist with guest cookies and never authenticate staff or guest routes; admin Origin rejected; responses `private, no-store`; no passwords or session values in browser storage (browser test).

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (final, 2026-09-26T12:07Z) | **passed** (see [003 review](../003-guest-access/review.md) for the environment) |
| MVP-04 Go tests | TestMemberSignupVerifyLogin, TestMemberAccountDiscoveryIsNeutral, TestMemberTokensSingleUseAndExpiry, TestPasswordResetRevokesSessions, TestConcurrentResetConfirmOneWinner (6 parallel), TestMemberRateLimits, TestMemberCookieCoexistsAndIsolated, TestMemberSeesOnlyOwnAccount, TestMemberResponsesPrivate, TestMemberContractConformance: all passed |
| PWA `account.spec.ts` (real SMTP to Mailpit) | passed: signup → emailed link → verify (fragment cleared) → sign in → sign out; reset signs out another browser; neutral acknowledgement for an unknown email |

## Delivery decision

No open P0/P1. Email provider unresolved (deployment decision). Status: **done** (M0 gate, 2026-09-26).
