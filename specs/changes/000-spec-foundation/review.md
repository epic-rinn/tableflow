# Review: 000-spec-foundation

Date: 2026-09-26. Reviewer: Codex, self-review.
Scope: explicit inventory of repository setup/specification files; no Git baseline exists. Result: pass for specification/tooling setup, not application implementation.

## Resolved findings

| Severity | Location / requirement | Trigger and impact | Resolution |
| --- | --- | --- | --- |
| P2 | Queue spec / QUE-001 | Daily display sequence resets while an earlier party remains waiting; using it as global priority could reorder guests | Defined separate monotonic join order and a midnight acceptance scenario |
| P2 | Loyalty spec / LOY-004 | Tax-inclusive prices make an undefined pre-tax earning base ambiguous | Explicitly subtract rounded included food tax from eligible food spend |
| P2 | HTTP contract / LOY-001 | Claiming a member changes discount eligibility while another operator sees an old bill | Claim requires expected version and increments bill version |
| P2 | Repository boundaries | Initial scaffold mixed development README/AGENTS files into intended source locations and had one frontend boundary | Applied user correction: three runtime projects, external guides/instructions, separate admin/PWA origins, Docker exclusions, and source-boundary validation |

No remaining blocking findings in this setup. The final specification defines independent frontend builds and host-only sessions; runtime implementations must prove those properties.

## Verification

- `make specs-check`: local links, required files/skills, and runtime source boundaries passed.
- Python validator smoke checks: valid/missing/escaping links, external/fenced/anchor links, skill metadata, and allowed/forbidden source paths checked without modifying application files.
- Ruby YAML parsing: all three skill frontmatter blocks, their UI metadata, and the GitHub workflow parsed successfully. The bundled Python skill validator was unavailable because PyYAML was not installed; repository validation plus Ruby parsing was used instead.
- Checked obsolete scaffold paths and reviewed relocated documentation links, root discovery rules, and the admin/PWA/API allocation.

## Database and API review scope

Static review covered claim exclusivity, lock order, idempotency, order/settlement serialization, point reversal, request bounds, proposed indexes, and measurement requirements. No executable SQL, endpoints, database, or app exists. Query plans, runtime latency, browser behavior, image contents, and integration/load tests are unmeasured; this setup does not claim they pass.

## Delivery decision

Knowledge base and review workflow setup is complete. M0–M6 remain planned. Root agent instructions require post-implementation review before feature delivery; CI currently checks repository structure only. No hosted review bot, branch protection, runtime deployment, or automatic model execution was configured.
