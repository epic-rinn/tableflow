# Review: 014-member-claim

Date: 2026-09-27 (M4 milestone gate, ADR-0004). Reviewer: Claude — **self-review**. Scope: loyalty policy, member profiles, visit claim/detach, claim status and privacy, tier snapshot at begin, admin loyalty editor and cashier member panel, PWA member points card and sign-in return. Result: **pass**.

## Findings

| Severity | Location | Evidence | Impact | Resolution |
| --- | --- | --- | --- | --- |
| P2 | `internal/billing` (commit 41d22df) | `gofmt -l` listed three files after the API commit | `make verify` (api-check) would fail on that commit | Formatted in the gate commit; the gate ran on formatted code |
| P3 | Guest bill discount line | A guest sees the member discount percentage (e.g. 3%), from which a tier can be inferred | Minor disclosure to people at the same table; no identity, email or balance is exposed | Accepted: the total must include the discount; the label is generic "Member discount" |
| P3 | Claim eligibility | Members with an unconfirmed email can claim | An unverified account earns points | Accepted for MVP (the spec is silent); operator decision recorded for MVP-22 |
| P3 | `MemberPoints` card | Claim status is fetched on load and after a claim, not polled | Another diner's claim appears after a reload | Accepted; the claim endpoint returns the authoritative conflict |

**Checked:**
- **Both sessions required:** a guest-only claim gets 401; a member without the table session gets 401; a member at another table gets 404 (LOY-A1).
- **Privacy:** other diners and members see only `claimed`; cashiers see a masked email and the tier.
- **Claim rules (LOY-002):** re-claiming is idempotent; another member gets `ALREADY_CLAIMED`.
- **Detach:** cashier or manager only, with an audited reason; the replacement member claims themselves.
- **Immutability:** settling and paid claims cannot change, and payment revokes the table session, so retrospective claims fail.
- **Tier snapshot (LOY-A3):** the snapshot at begin is taken after the profile lock. The bill crossing a threshold uses the previous tier; the next visit gets the new tier; a tier change after begin does not alter the frozen bill.
- **Lock order:** member session (principal) → visit → guest session → member profile, consistent with the data-model order (visits → member profiles → children).
- **Safe redirect:** the sign-in `next` parameter accepts same-site paths only.

## Verification

| Command / test | Result |
| --- | --- |
| `make verify` (2026-09-27; macOS arm64, Go 1.27.1, Node 24.21.0, pnpm 12.6.0, postgres:18.6, Chrome) | **passed** on the first run: Go suite green (race, PostgreSQL, none skipped), admin 15/15, PWA 18/18 incl. `loyalty.spec.ts`, artifact check |
| MVP-14 tests | TestLoyaltyPolicyVersions, TestClaimNeedsBothSessions (LOY-A1), TestClaimConflictsAndDetach, TestTierSnapshotAtBegin (LOY-A3), TestLoyaltyContractConformance; PWA `loyalty.spec.ts` |
| Load | [performance](performance.md) |

## Delivery decision

No open P0/P1. Status: **done** (M4 gate).
