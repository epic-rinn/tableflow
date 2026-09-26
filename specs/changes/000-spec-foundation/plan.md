# Change: 000-spec-foundation

Status: done. Date: 2026-09-26. Scope: repository knowledge base, source boundaries, and agent workflows.

## Outcome

Turn the initial research draft into a navigable, requirement-based MVP specification for Next.js PWA, Go, and PostgreSQL without an ORM. Add repeatable post-implementation code and DB/API review procedures, plus structural validation.

## Decisions

- Preserve `specs/idea.md` as research history; route new work through the [knowledge map](../../README.md).
- Separate product/features, architecture, API inventory, quality gates, ADRs, and per-change evidence.
- Choose pgx with handwritten SQL and Goose SQL migrations; defer runtime initialization to M0.
- Use repository-local delivery/code-review/DB-API skills and root/scoped AGENTS.md instructions.
- User refinement: separate runtime projects in `src/admin`, `src/pwa`, and `src/api`; move all development docs and scoped guidance outside source into `docs/development/`, routed by root AGENTS.md. Keep knowledge validation in `tooling/specs/`.
- Keep loyalty discounts tier-based for MVP; defer point redemption and finite-stock inventory. These refinements are explicit in [MVP scope](../../product/mvp.md).

## Validation scope

Check local links/required files, skill frontmatter/UI metadata, workflow YAML syntax, and cross-document consistency. Review concurrency/financial requirements statically. No actual queries/endpoints exist, so query plans, load tests, app tests, and browser tests are not applicable to this setup; they remain mandatory for affected implementation milestones.
