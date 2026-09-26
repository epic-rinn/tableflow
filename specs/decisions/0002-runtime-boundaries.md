# ADR-0002: Separate runtime projects from the knowledge base

Date: 2026-09-26. Status: accepted. Extends [ADR-0001](0001-stack.md).

## Context

The owner requires separate admin panel, customer PWA, and API applications and explicitly excludes specs, AI material, and docs from application source.

## Decision

Keep three runtime projects under `src/admin`, `src/pwa`, and `src/api`. Put SQL migrations inside the API project. Keep specs, docs, skills, review evidence, and their validation tooling outside `src/`. Replace source-local README/AGENTS files with guides under `docs/development/` routed by root AGENTS.md.

Deploy the admin and PWA to separate origins with same-origin API proxies to the shared Go service. This separates service-worker scope and session cookies while retaining one business backend.

## Alternatives and consequences

A single Next.js app would mix operational and customer UI, service-worker scope, and deployment lifecycle. Separate repositories would provide stronger repository isolation but make cross-contract changes harder; the user requested source separation, so one repository with explicit boundaries is sufficient.

Each frontend has its own build/tests and dependency manifest. Deployment artifacts must exclude the knowledge base and agent files. Root ignore rules support Docker; packaging checks are still required when actual deployment tooling is created.
