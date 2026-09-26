# ADR-0005: Resend for production transactional email

Status: accepted provider choice; production configuration and delivery verification pending.

## Context

Member verification/password reset and staff activation need transactional email. Local SMTP/Mailpit already provides development delivery; production provider selection was left open. Keep that transport boundary rather than introducing vendor-specific behavior into identity handlers.

## Decision

Use Resend through the SMTP adapter for MVP production delivery; retain Mailpit locally. Configure `smtp.resend.com` on port `587` with required STARTTLS and certificate validation, username `resend`, and the Resend API key supplied as the SMTP password through deployment secrets. No plaintext fallback or secrets in the repository. Claude must verify that the existing adapter supports these requirements, not assume local SMTP behavior is production-ready.

The user must supply a sending domain they control, complete domain verification and provision credentials before live delivery. Keep open/click tracking disabled for authentication messages. Missing production configuration must fail clearly rather than silently deliver to Mailpit. No account creation, purchase or DNS changes are authorized by this document.

## Alternatives and consequences

Resend HTTP API would require a separate adapter; another SMTP provider remains replaceable through configuration. SMTP reuses the current boundary, but authentication, TLS, timeouts, failure handling and controlled end-to-end delivery still need verification. Provider selection does not complete ACC-002's live-email gate or MVP-21. No price or deliverability guarantee is implied.

## Claude handoff

Under MVP-21, document production variables without values, validate TLS/authentication and timeout/error paths, and test verification/reset/activation delivery using authorized test recipients once the domain/key are available. Preserve generic public authentication responses and never log tokens or credentials. Continue other MVP work without waiting for the domain/key.

## Sources

- [Resend SMTP configuration](https://resend.com/docs/send-with-smtp)
- [Resend domain verification and tracking](https://resend.com/docs/dashboard/domains/introduction)
