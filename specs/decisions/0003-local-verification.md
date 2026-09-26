# ADR-0003: Local verification gate instead of hosted CI

Date: 2026-09-26. Status: accepted (owner decision).

## Context

MVP-01 added a GitHub Actions workflow for runtime checks. Its first run hung in the browser smoke step and could not be inspected or cancelled from the development environment. The owner decided not to use GitHub CI for this project and to treat local verification as sufficient.

## Decision

Remove the GitHub Actions workflows (`runtime.yml`, `specs.yml`). `make verify` (`tooling/runtime/verify.sh`) is the required verification gate. It runs specs-check, Go format/vet/race tests, the PostgreSQL-backed tests on the compose database, both frontend checks, the Playwright browser smoke tests against a started API, and the artifact boundary check. A task's review records the actual `make verify` run (date, versions, result) as its evidence. Targeted `make` targets remain available during development.

## Alternatives

- Keep GitHub Actions with job timeouts: rejected by the owner.
- Another hosted CI: same objection, and adds external setup.

## Consequences

Verification depends on the developer machine: Docker, Go (the toolchain is pinned in `go.mod`), Node.js 24 and Corepack. `verify.sh` refuses other Node majors. There is no automatic check on push and no independent build environment, so each review must state the environment it ran in. Results from a different OS or browser are not implied. Revisit this decision if more contributors join or a release pipeline is introduced (MVP-21).
