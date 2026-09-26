# Security boundaries

Requirements: [access](../features/05-access-pwa.md). This document owns implementation threat considerations, not a legal compliance claim.

| Boundary | Required control | Evidence |
| --- | --- | --- |
| Public IDs versus authorization | UUID/display number alone never grants access; verify branch and resource ownership in Go | Negative cross-branch and cross-visit tests |
| Shared dining capability | Random token, hashed lookup, revocable generation, no profile or cashier rights | Rotation invalidates derived sessions; other diners remain supported |
| QR transport | Fragment token → POST exchange, history cleanup, no third-party scripts on entry, `Referrer-Policy: no-referrer` | Browser/network/log inspection |
| Cookie sessions | HttpOnly/Secure/SameSite, CSRF and Origin checks, bounded lifetime/revocation | Forged-origin, logout, expiry, role-change tests |
| Passwords and recovery | Maintained password hashing; single-use expiring verification/reset tokens; neutral account-existence responses | Auth/reset abuse and replay tests |
| SQL | Parameterized values; allowlisted identifiers/orderings; least-privilege role | Injection cases, repository review |
| Payment/refund | Role check, explicit verification, idempotency, immutable audit, no automatic slip trust | Concurrent confirmation and unauthorized refund tests |
| Caching | No session data in browser/service-worker/shared caches | Offline and user-switch browser tests |
| Abuse | Bounded body/list sizes, rate limits by actor and source; prevent queue joins from unbounded retries | Load and abuse tests; trusted-proxy configuration |
| Logs and reports | Redact tokens/passwords/contact details, stable route labels, sanitized error messages | Log review and metric-cardinality checks |

Record production data retention and deletion/anonymization procedures before launch; preserve necessary financial integrity while minimizing personal data. Backups must be restorable and access-controlled. Development fixtures are synthetic. No raw production dump is needed for performance review.
