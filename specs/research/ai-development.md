# Evidence behind this workflow

Reviewed 2026-09-26. These are documented approaches from maintainers, not a survey proving universal industry practice.

| Primary source | Practice adopted here | Deliberate adaptation |
| --- | --- | --- |
| [GitHub Spec Kit](https://github.com/github/spec-kit), [agentic workflow](https://github.github.com/spec-kit/reference/agentic-sdd.html) | Separate requirements, planning, tasks, implementation, and consistency checks. | Use plain repository files and stable requirement IDs; no tool-specific slash commands or CLI installation required. |
| [OpenSpec overview](https://github.com/Fission-AI/OpenSpec/blob/main/docs/overview.md) | Keep canonical specs distinct from scoped change proposals and their history. | Use `specs/changes/` in the existing folder rather than creating a second competing `openspec/` tree. |
| [OpenAI AGENTS.md guidance](https://learn.chatgpt.com/docs/agent-configuration/agents-md) | Short shared instructions plus more specific local guidance and explicit code-review rules. | Root instructions route agents to relevant documents instead of loading the whole knowledge base. |
| [OpenAI skills guidance](https://learn.chatgpt.com/docs/build-skills) | Repository skills under `.agents/skills/`, with concise discovery descriptions and on-demand instructions. | Separate delivery, code review, and database/API review. Instructions require review; they are not a CI execution engine. |

This is a lightweight local workflow inspired by these projects, not an installed Spec Kit/OpenSpec distribution or a claim of compatibility with their commands. A versioned knowledge base is sufficient at this size; embeddings, vector storage, and an AI-specific backend are unnecessary.

## Technical references

- [Next.js PWA guide](https://nextjs.org/docs/app/guides/progressive-web-apps): manifest/service-worker foundations. Our cache exclusions are product security requirements.
- [Goose](https://github.com/pressly/goose): SQL migrations, transaction controls, and migration commands.
- [pgxpool](https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool): Go PostgreSQL connection pooling.
- [PostgreSQL EXPLAIN](https://www.postgresql.org/docs/18/using-explain.html): query plans and actual execution analysis. A plan cost is not endpoint latency; `ANALYZE` executes the statement.

Refresh relevant official documentation when implementing version-sensitive behavior. Preserve evidence of what was actually measured separately from vendor claims and design targets.
