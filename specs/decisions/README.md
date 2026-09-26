# Architecture decisions

| ID | Decision | Status |
| --- | --- | --- |
| [0001](0001-stack.md) | Next.js PWA + Go modular monolith + PostgreSQL, pgx and Goose | Accepted |
| [0002](0002-runtime-boundaries.md) | Separate admin/PWA/API projects and keep knowledge/AI files outside runtime source | Accepted |
| [0003](0003-local-verification.md) | Local `make verify` gate instead of hosted GitHub CI | Accepted |
| [0004](0004-milestone-verification.md) | Run tests and reviews once per milestone | Accepted |
| [0005](0005-production-email.md) | Resend SMTP (TLS required) locally and in production; Mailpit removed | Accepted; live configuration pending |
| [0006](0006-ui-stack.md) | Tailwind CSS v4 + shadcn/ui in both frontends; modern admin panel, Grab-style mobile PWA | Accepted |

New ADRs record context, decision, alternatives, consequences, date, and superseded decisions. Use them for durable tradeoffs, not routine implementation details. See the [ADR template](../templates/adr.md).
