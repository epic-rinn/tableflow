# ADR-0004: Verify and review per milestone

Date: 2026-09-26. Status: accepted (owner decision). Amends the per-task gate in the [task backlog](../delivery/tasks.md) and [workflow](../delivery/workflow.md).

## Context

MVP-01 and MVP-02 were each verified and reviewed before the next task started. The owner decided to implement all tasks of a milestone first and run tests and reviews once per milestone, committing locally during implementation.

## Decision

- Within a milestone, a task may start when its dependencies are `implemented-unverified` or better. Each task still gets its change packet, requirement-mapped tests (written with the code) and a commit, and its status becomes `implemented-unverified`.
- During implementation only compilation checks run (`go build`, `go vet`, TypeScript typecheck), so commits are not broken. Tests are written but not run.
- When every task of the milestone is implemented, the milestone gate runs: `make verify`, required performance measurements, `tableflow-code-review` and `tableflow-db-api-review` over the milestone's combined changes. Findings are fixed and re-checked, each task's `review.md` records the shared evidence, and tasks move to `done` together.
- A milestone gate must pass before the next milestone starts. `done` still requires the [definition of done](../quality/testing.md).

## Alternatives

Per-task gates (the previous rule) catch defects earlier and keep fixes local, at the cost of more frequent full runs. The owner preferred fewer, larger gates.

## Consequences

Defects in an earlier task of a milestone may surface only at the gate, after later tasks build on them, and fixes can span several tasks. Commits between gates are unverified and must not be described as tested. Review reports identify the milestone gate that produced their evidence.
