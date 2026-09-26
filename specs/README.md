# Knowledge map

Status: desired MVP baseline; runtime foundation (MVP-01) done, domain features not started. Last reviewed: 2026-09-26.

## Read by task

| Task | Read |
| --- | --- |
| Understand scope | [MVP](product/mvp.md), [glossary](product/glossary.md) |
| Queue or seating | [Queue and seating](features/01-queue-seating.md), [data model](architecture/data-model.md) |
| Menu, ordering, kitchen | [Ordering](features/02-ordering.md) |
| Bills or loyalty | [Settlement](features/03-settlement.md), [loyalty](features/04-loyalty.md) |
| Login, access, PWA | [Access and PWA](features/05-access-pwa.md), [security](architecture/security.md) |
| Admin panel or source layout | [Admin](features/06-admin.md), [repository boundaries](architecture/repository.md) |
| Service or schema design | [System](architecture/system.md), [data model](architecture/data-model.md), [ADRs](decisions/README.md) |
| Implement HTTP behavior | [HTTP contract](api/http.md) and relevant feature |
| Finish a feature | [Workflow](delivery/workflow.md), [testing](quality/testing.md), [performance](quality/performance.md) |
| Choose next work | [MVP task backlog](delivery/tasks.md), [roadmap](delivery/roadmap.md), [active changes](changes/README.md) |
| Claude implementation handoff | [Claude entrypoint](../CLAUDE.md), [first task packet](changes/001-foundation/plan.md) |
| Understand rationale | [Original idea](idea.md), [industry sources](research/ai-development.md) |

## Authority and maintenance

1. The user's current instructions govern the work. Reflect changed decisions in the affected documents.
2. Product/features describe desired behavior; ADRs describe accepted technical decisions; API and data documents define cross-component contracts. Conflicts between them are defects to resolve before affected implementation.
3. Executable migrations become the authority for the implemented database schema. Feature specs remain the authority for intended behavior; implementation drift is not an implicit requirement change.
4. Change folders hold plans, tasks, proposed deltas, and review evidence. A completed change merges its accepted behavior into canonical specs and remains as dated history, not a competing live specification.
5. Research, examples, and templates are non-normative. Performance budgets and default business settings are engineering proposals, explicitly identified as such.

Use stable requirement IDs (`QUE-001`, `ORD-001`, etc.) in tests, tasks, and review reports. Never recycle removed IDs. A spec status is separate from implementation status. Update only relevant documents; do not load the entire knowledge base into every AI prompt.

## Change packet

Copy [change template](templates/change.md) into `specs/changes/<id>/plan.md`, create `tasks.md` from the [task template](templates/tasks.md), and create `review.md` from the [review template](templates/review.md). Store query/load artifacts in that change folder when needed. Use relative Markdown links to existing local files; describe future paths in backticks.
