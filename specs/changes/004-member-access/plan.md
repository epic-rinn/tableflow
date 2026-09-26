# Change: 004-member-access — Member identity

Status: implemented-unverified. Date: 2026-09-26. Scope owner: Claude. Task: MVP-04. Verification: M0 milestone gate ([ADR-0004](../../decisions/0004-milestone-verification.md)).

## Problem and behavior

Maps ACC-002 (member part) and ACC-004 in [access](../../features/05-access-pwa.md). Customers can sign up with an email and password, verify their email through a link, sign in and out, and reset a forgotten password in the PWA. A reset revokes every existing session. Responses never reveal whether an email is registered. Emails go through a mail adapter; local development and tests use Mailpit or a recording fake. No points, tiers or visit claims yet (MVP-14/15).

## Decisions (reversible unless noted)

- **Separate tables** `member_accounts`, `member_sessions`, `member_tokens`, as decided in MVP-02: a member credential never carries staff authority.
- **Unverified members can sign in.** `email_verified` is exposed, and features that need a trusted email (visit claims in MVP-14) must require it. A password reset proves mailbox ownership and marks the email verified. This stops someone who registered another person's email from keeping access: the real owner resets the password, which revokes the squatter's sessions.
- **Neutral discovery:** signup and reset requests always return `202 {"status":"check_email"}`. Signing up with an existing email sends that address a "you already have an account" notice instead of a second registration. Wrong password and unknown email return the same 401 after an equal-cost hash.
- **Asynchronous mail:** emails are queued to a bounded in-process queue (capacity 100, 2 workers, 10 s SMTP timeout) after the transaction commits, so response time does not depend on whether an email was sent. If the queue is full, the message is dropped and logged. Undelivered mail is lost on restart and users re-request it. The production provider is **unresolved** (deployment decision, MVP-21); only SMTP to Mailpit is implemented.
- **Tokens:** 256-bit, SHA-256 stored, single use, one open token per member and purpose (issuing a new one revokes the old). Verification is valid for 24 h, reset for 1 h. Links point to `PWA_PUBLIC_URL/account/verify#token` and `/account/reset/confirm#token`.
- **Sessions:** cookie `__Host-tf_member` (HttpOnly, Secure, SameSite=Lax, Path=/, host-only on the PWA origin), 30 days absolute and 7 days idle, stored as hash only; `last_seen_at` is written at most once per minute. The member cookie coexists with guest and anonymous cookies: signing in does not clear them.
- **Throttles (per 10 min):** login 10 per email and 100 per IP; signup 20 per IP; reset request 5 per email and 30 per IP; verification resend 5 per email; token submissions 30 per IP.
- **Origin:** every member mutation requires a `PWA_ORIGINS` origin.
- **Routes:** a member-specific `DELETE /sessions/member` and `GET /members/me` instead of overloading the staff-only `/sessions/current`. The HTTP contract is updated accordingly.

## Routes

| Route | Result |
| --- | --- |
| POST `/members` `{email,password,locale}` | 202 neutral |
| POST `/members/verification` `{email}` | 202 neutral (resend) |
| POST `/members/verify` `{token}` | 200 `{status:"verified"}`; 422 `TOKEN_INVALID` |
| POST `/sessions/member` `{email,password}` | 201 member identity + cookie; 401; 429 |
| GET `/members/me` | 200 own identity; 401 |
| DELETE `/sessions/member` | 204 |
| POST `/members/password-reset/request` `{email}` | 202 neutral |
| POST `/members/password-reset/confirm` `{token,new_password}` | 200; revokes all sessions; 422 `TOKEN_INVALID` |

## Query and endpoint impact

These follow the staff patterns: authentication is one unique-hash lookup; confirmation locks the account, then the token; session revocation uses a partial index. Measured at the M0 gate.

## Verification map

| Requirement / scenario | Named test | Status |
| --- | --- | --- |
| ACC-002 signup/verify/login/logout | TestMemberSignupVerifyLogin | written, not run |
| Neutral discovery | TestMemberAccountDiscoveryIsNeutral | written, not run |
| Single-use, expired and replayed tokens | TestMemberTokensSingleUseAndExpiry | written, not run |
| Reset revokes sessions and verifies email | TestPasswordResetRevokesSessions | written, not run |
| Concurrent reset confirmation, one winner | TestConcurrentResetConfirmOneWinner | written, not run |
| Rate limits | TestMemberRateLimits | written, not run |
| Coexistence with guest cookies; isolation from staff | TestMemberCookieCoexistsAndIsolated | written, not run |
| Cross-account denial | TestMemberSeesOnlyOwnAccount | written, not run |
| No credential caching (headers) | TestMemberResponsesPrivate | written, not run |
| Contract | TestMemberContractConformance | written, not run |
| Browser journey through Mailpit | PWA `account.spec.ts` | written, not run |

## Final decisions

Filled in at the M0 gate. Unresolved for the user: production email provider and sender domain (MVP-21).
