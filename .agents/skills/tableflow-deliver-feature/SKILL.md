---
name: tableflow-deliver-feature
description: Implement a TableFlow feature or substantial fix from its specs through tests and mandatory post-implementation review. Use for repository feature delivery, not general product brainstorming.
---

# Deliver a TableFlow feature

Use the repository root containing `specs/README.md`. Read the [knowledge map](../../../specs/README.md), relevant feature IDs, scoped AGENTS.md files, and [workflow](../../../specs/delivery/workflow.md). Load architecture/security/performance documents only as needed.

## Prepare the change

Inspect existing code and user edits before planning. Create or update `specs/changes/<id>/plan.md` and `tasks.md` from the repository templates. State assumptions, contract/migration impact, transaction invariants, and named acceptance tests. Follow the user's already-authorized scope; do not request a ritual approval for reversible planning or implementation.

Use Next.js for presentation, Go for business authority, pgx with handwritten SQL, and Goose migrations. Do not introduce an ORM or duplicate the domain logic in Next.js. Break broad work into reviewable vertical slices while completing the user's requested scope.

## Implement and verify

Keep requirement IDs traceable to meaningful tests. Update contracts and canonical specs when behavior changes. Run only real documented commands and introduce required checks alongside new application scaffolds. Record exact results and environment; a mocked repository test cannot establish PostgreSQL concurrency safety.

For SQL/endpoint/data-fetching changes, prepare representative query and endpoint measurements required by [performance](../../../specs/quality/performance.md). Missing runtime infrastructure must be reported as missing evidence, not a performance pass.

## Mandatory review before delivery

After implementation and tests, run [tableflow-code-review](../tableflow-code-review/SKILL.md) against the final changes. Run [tableflow-db-api-review](../tableflow-db-api-review/SKILL.md) whenever its data-path triggers apply. These reviews occur after implementing the feature and before telling the user it is delivered.

Resolve blocking findings within scope, rerun affected checks, and re-review fixes. Use an independent reviewer only when delegation is explicitly authorized; otherwise perform a distinct self-review pass and label it honestly. Do not recursively invoke delivery from review skills.

Complete the change's review report and task status. Final handoff states behavior, checks actually run, material findings, and remaining limitations. Follow [definition of done](../../../specs/quality/testing.md); do not mark incomplete verification as delivered. This skill grants no permission to deploy, merge, publish, or access production.
