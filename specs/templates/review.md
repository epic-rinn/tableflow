# Review: <change-id>

Date: <date>. Reviewer: <identity; independent or self-review>.
Scope: <base/head or file list; include untracked files>. Result: <pass / changes-required / unverified>.

## Findings

| Severity | File:line and requirement | Trigger / evidence | User or operational impact | Resolution and recheck |
| --- | --- | --- | --- | --- |
| <P0–P3> | <location> | <repro> | <impact> | <status> |

If no findings, state that explicitly and list remaining coverage limits. Never fabricate a finding to fill the table.

## Verification

| Command / test | Environment / fixture | Result | Evidence |
| --- | --- | --- | --- |
| <actual command> | <versions> | <passed/failed/not run> | <output path or concise result> |

## Database and endpoints

List changed routes and queries, plans/latencies/cardinalities, query counts, locks, pool waits, pagination/body bounds, migration compatibility, auth/cache checks. Link performance evidence or state not applicable with reason. Distinguish static reasoning from measured results.

## Delivery decision

Open findings, missing checks, accepted follow-ups with owner, canonical spec updates, and final definition-of-done status. Re-review must cover fixes introduced after the original review.
