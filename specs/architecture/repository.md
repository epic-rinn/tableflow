# Repository and build boundaries

Status: accepted; runtime foundation implemented in MVP-01. Decision: [ADR-0002](../decisions/0002-runtime-boundaries.md).

```text
tableflow/
├── src/
│   ├── admin/                 Next.js staff and manager application
│   ├── pwa/                   Next.js customer progressive web app
│   └── api/                   Go service and module-local SQL
│       └── db/migrations/     Goose SQL migrations
├── specs/                     Requirements, contracts, ADRs, review evidence
├── docs/development/          Setup and engineering guides
├── .agents/skills/            AI delivery/review procedures
├── tooling/specs/             Documentation/skill validation
├── tooling/runtime/           Built-artifact boundary check, verify and restore drill
├── deploy/                    Container images and the staging stack (MVP-21); not runtime source
├── compose.yaml               Local development services only
├── .github/                   PR template (no hosted CI; ADR-0003)
├── AGENTS.md                  Repository-level AI instructions
└── README.md                  Human entrypoint
```

`src/` is the application source boundary. It contains runtime code, configuration, dependencies/lockfiles, migrations, assets, and executable application tests as they are implemented. It does not contain Markdown docs, agent instructions/skills, specification copies, or review reports. Empty `.gitkeep` files preserve the initial directories and are removed when real source is added.

Docs/specs/AI material remains versioned alongside the applications for traceability but is not part of their source trees or shipped artifacts. Root `.dockerignore` excludes these directories from a root-context image build. Next.js apps build `output: "standalone"` traced only within their own project, and `make artifact-check` (`tooling/runtime/check_artifacts.py`) scans the Go binary and both standalone/static outputs for repository docs/specs/AI content; `.dockerignore` does not itself configure non-Docker deployment or application packaging.

Generate any frontend transport types from the canonical contract at development/build time into the appropriate runtime application. Neither app nor the API imports Markdown, review files, or AI tooling at runtime. Avoid a shared runtime package until concrete duplicated code justifies it; any future shared code belongs under `src/` and requires a recorded structure change.

Application guidance is routed from root AGENTS.md to [admin](../../docs/development/admin.md), [PWA](../../docs/development/pwa.md), [API](../../docs/development/api.md), and [database](../../docs/development/database.md) guides. Keep AGENTS.md out of source directories.
