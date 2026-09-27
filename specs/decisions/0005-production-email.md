# ADR-0005: Resend for transactional email

Status: accepted; amended 2026-09-27 (owner): Resend for local development too, Mailpit removed. Live configuration and delivery verification pending.

## Context

Member verification/password reset and staff activation need transactional email. Local SMTP/Mailpit already provides development delivery; production provider selection was left open. Keep that transport boundary rather than introducing vendor-specific behavior into identity handlers.

## Decision

Use Resend through the SMTP adapter for MVP delivery, locally and in production; Mailpit is removed. Automated tests use a test-only file outbox (`MAIL_ADAPTER=file`) because they cannot read a real inbox; it never delivers mail and logs a warning at startup. Configure `smtp.resend.com` on port `587` with required STARTTLS and certificate validation, username `resend`, and the Resend API key supplied as the SMTP password through deployment secrets. No plaintext fallback or secrets in the repository. Claude must verify that the existing adapter supports these requirements, not assume local SMTP behavior is production-ready.

The user must supply a sending domain they control, complete domain verification and provision credentials before live delivery. Keep open/click tracking disabled for authentication messages. Missing configuration must fail clearly: without credentials the API logs an error at startup and every send fails with "delivery not configured"; there is no plaintext mode to fall back to. No account creation, purchase or DNS changes are authorized by this document.

## Alternatives and consequences

Resend HTTP API would require a separate adapter; another SMTP provider remains replaceable through configuration. SMTP reuses the current boundary, but authentication, TLS, timeouts, failure handling and controlled end-to-end delivery still need verification. Provider selection does not complete ACC-002's live-email gate or MVP-21. No price or deliverability guarantee is implied.

## Claude handoff

Under MVP-21, document production variables without values, validate TLS/authentication and timeout/error paths, and test verification/reset/activation delivery using authorized test recipients once the domain/key are available. Preserve generic public authentication responses and never log tokens or credentials. Continue other MVP work without waiting for the domain/key.

## Implementation status (2026-09-27)

`internal/platform/mail`: `SMTP_TLS` is `starttls` (default, port 587) or `implicit` (465); certificates are verified (TLS 1.2+); STARTTLS is required when selected, and PLAIN auth runs only after TLS. Tests: `TestStartTLSNeverFallsBackToPlaintext`, `TestUnconfiguredOrPlaintextSMTPRefuses`, `TestFileOutboxWritesPrivateJSON`, `TestMailConfigRequiresTLS`. Remaining under MVP-21: live delivery to authorised test recipients once the domain and key exist, and confirming open/click tracking is off for these messages in the Resend dashboard.

## Sources

- [Resend SMTP configuration](https://resend.com/docs/send-with-smtp)
- [Resend domain verification and tracking](https://resend.com/docs/dashboard/domains/introduction)

## Live verification (2026-09-27)

The user has no sending domain, so delivery was verified in Resend **test mode** (sender `onboarding@resend.dev`, key in the git-ignored staging env).
- Mail to a non-owner address was refused by Resend after TLS and authentication (`550 … only send testing emails to your own email address`). The API logged a warning without tokens.
- Verification and password-reset emails to the account owner's address were delivered (confirmed by the user).
- Production still requires a verified domain before launch (MVP-22 checklist).

