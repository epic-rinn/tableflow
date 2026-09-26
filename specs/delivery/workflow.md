# Spec-driven delivery workflow

## Small context, explicit evidence

1. **Frame:** Read the knowledge map and relevant requirement IDs. Check current implementation, user changes, and dependencies. A research note is not an implemented feature.
2. **Plan:** Create `specs/changes/<id>/plan.md` and `tasks.md`. Record behavior delta, affected contracts, transaction/query plan, risks, and acceptance tests. Routine fixes may use a short packet. Clarify only decisions that block correct work; label reversible assumptions.
3. **Implement:** Work in small vertical slices. Write SQL migrations/queries, HTTP contract, Go behavior, and UI as appropriate. Keep tasks current and canonical specs aligned with accepted decisions.
4. **Verify:** Run relevant checks and capture outputs/environment. Diagnose failures; do not mark blocked checks as passed.
5. **Review:** Invoke `$tableflow-code-review` after implementation and before the final delivery message. For data-path changes invoke `$tableflow-db-api-review` too. A separate review pass must inspect the final code; implementation notes alone are not review evidence. Use an independent reviewer when explicitly requested/available, otherwise identify the review as self-review.
6. **Fix and recheck:** Resolve blocking findings, rerun affected tests, and review changed lines again. Persist residual risks and evidence.
7. **Close:** Complete the change's `review.md`, update roadmap/status, and summarize behavior, checks, findings, and limitations. No commit, push, merge, or deployment is implied.

## Status vocabulary

`planned → in-progress → implemented-unverified → reviewed → done`. Use `blocked` with a concrete dependency when needed. `done` requires [testing gates](../quality/testing.md). A spec can be complete while its implementation remains planned.

This process follows patterns documented in [research](../research/ai-development.md). It is intentionally file-based. Root AGENTS.md provides the recurring review instruction; skills provide procedures. Neither guarantees that a future agent complied. The review report and CI evidence make compliance assessable.
