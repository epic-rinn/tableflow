---
name: tableflow-code-review
description: Review completed TableFlow changes for spec compliance, correctness, authorization, concurrency, and regressions. Run after feature implementation and before delivery, or when a code review is requested.
---

# Review TableFlow changes

Read the relevant requirement IDs, scoped AGENTS.md rules, change plan/tasks, and [testing gates](../../../specs/quality/testing.md). Review first; do not silently turn an explicit review-only request into implementation or unrelated cleanup.

## Establish scope

Identify the baseline and changed files, including untracked files. If Git history is unavailable, use an explicit file inventory and disclose that no base diff exists. Inspect surrounding callers, middleware, SQL/migrations, and tests; do not infer behavior from filenames or the author's summary alone.

## Review for observable failures

- Trace the acceptance path and failure/retry paths. Compare implemented behavior with requirement IDs and the HTTP contract.
- Check Go authorization for branch/resource/role boundaries, secret handling, CSRF, session revocation, and guest/member separation.
- Check queue hold/visit/table ownership, state transitions, request idempotency, same-table and same-member contention, money rounding, payment verification, and refund/loyalty atomicity.
- Check SQL constraints, migration upgrades/backfills/recovery, request cancellation, resource cleanup, and errors. Run [DB/API review](../tableflow-db-api-review/SKILL.md) for any data-path change.
- Check Next.js private caching, PWA cache exclusions, offline/retry behavior, polling cleanup, accessibility, and whether UI success actually means server acknowledgement.
- Inspect whether tests prove the invariant under real concurrency and database behavior. Run targeted checks that materially establish a finding; never report unrun checks as passing.

Avoid speculative warnings, stylistic preferences already handled by tooling, and broad rewrites. A finding needs a concrete trigger, file/line, affected requirement, impact, and actionable correction. Performance suspicions without measurement are verification gaps, not invented latency regressions.

## Report and disposition

Use [review template](../../../specs/templates/review.md) in the current change's `review.md` for delivery work. For a standalone review, return findings in the requested format without creating a change packet unless asked.

Severity: P0 = immediate severe integrity/security failure; P1 = blocks correct/safe release; P2 = material but bounded issue with a workable path; P3 = minor improvement. Prioritize by impact and reproducibility, not by number of findings. State “no findings” if warranted and separately list unverified areas.

Label independent review versus self-review. Delivery work must resolve P0/P1 findings and rerun affected checks before closing; re-review the final fixes. Review-only work reports the evidence and stops. Do not approve a feature solely because its documentation or build passes.
