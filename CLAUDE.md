# Claude implementation entrypoint

Read [AGENTS.md](AGENTS.md) first; it is the shared engineering policy. Then read [specs/README.md](specs/README.md) and the assigned task in the [MVP backlog](specs/delivery/tasks.md). Do not load unrelated specifications or edit the Word report.

## Responsibilities

- Codex primarily maintains task definitions, requirements coordination, and the original Word report.
- Claude implements assigned application tasks, adds tests, performs post-implementation reviews, and records implementation evidence/status. The user approves product decisions and pilot readiness.
- Keep docs, specs, and AI instructions outside `src/`. Do not create duplicate Word documents or modify the report as part of implementation.

## How to start

If no task is named, select the earliest `planned` task whose dependencies are `done`. Start with **MVP-01**, using [001-foundation](specs/changes/001-foundation/plan.md). Work on one task per handoff; do not implement the entire backlog implicitly.

Read and follow [.agents/skills/tableflow-deliver-feature/SKILL.md](.agents/skills/tableflow-deliver-feature/SKILL.md) as a local procedure. If this client cannot invoke a skill by name, read its instructions and carry them out directly. Do the same for both applicable review procedures; a slash-command integration is not required.

Before coding, inspect current files/user edits, mark the task `in-progress`, and create or update its change packet. Preserve scope and canonical requirements. Ask only about decisions that block correct implementation; record reversible assumptions. Do not invent credentials, operator approvals, measured performance, or completed tests.

After implementation, run the relevant checks, perform a distinct code-review pass and a DB/API review for data paths, fix blocking findings, and re-review. Record commands, results, performance evidence, and unresolved limits in the change packet. Use `implemented-unverified` when required checks cannot run; only use `done` when the shared definition of done is met. Keep feature implementation status and the roadmap accurate without labelling a partially implemented feature complete.

Stop after the assigned task and report changed files, requirement coverage, actual checks, review results, blockers, and the next eligible task. Do not commit, push, deploy, or start another agent unless explicitly authorized.

## Initial handoff prompt

> Read CLAUDE.md and implement MVP-01 from specs/delivery/tasks.md using specs/changes/001-foundation/. Follow the linked delivery and review procedures. Keep source under src/admin, src/pwa, and src/api; use Go, PostgreSQL, handwritten SQL through pgx, and Goose. Do not implement identity or restaurant features yet. Run the checks you introduce, record evidence and review findings, update task status, and stop after MVP-01. Do not edit the Word report.
