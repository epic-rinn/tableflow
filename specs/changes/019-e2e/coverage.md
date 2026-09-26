# Requirement-to-test coverage (MVP-19)

Generated on 2026-09-27 by the script in the [plan](plan.md). The script matches each requirement and acceptance ID in `specs/features/*.md` against the test files that cite it (Go tests, admin/PWA Playwright specs and the cross-app journey). 81 IDs; 0 without a citing test.

| ID | Feature | Tests (first four) |
| --- | --- | --- |
| QUE-001 | 01-queue-seating | queue.spec.ts › queue |
| QUE-002 | 01-queue-seating | TestGroupPositionIndependentOfOtherGroups, TestTrackingShowsOnlyOwnTicket |
| QUE-003 | 01-queue-seating | TestBypassRequiresManagerReason, visit.spec.ts |
| QUE-004 | 01-queue-seating | TestOverdueHoldPersists, TestCancelCalledReleasesHold, TestGuestAndHostCancel |
| SEA-001 | 01-queue-seating | TestBypassRequiresManagerReason |
| SEA-002 | 01-queue-seating | TestConcurrentCallsOneClaim, TestConcurrentSeatsOneVisit |
| SEA-003 | 01-queue-seating | TestMoveKeepsAccessAndCleansOldTable, TestMoveVersusDepartOneWinner |
| SEA-004 | 01-queue-seating | TestPaidVisitKeepsTable, TestCloseEmptyAndReady, TestBeginRequiresCharges, TestConfirmEffects |
| QUE-A1 | 01-queue-seating | TestQueueJoinIdempotent |
| QUE-A2 | 01-queue-seating | TestGroupPositionIndependentOfOtherGroups |
| QUE-A3 | 01-queue-seating | TestNoShowVersusSeatOneWinner |
| QUE-A4 | 01-queue-seating | TestJoinOrderSurvivesDailyRenumbering |
| SEA-A1 | 01-queue-seating | TestConcurrentCallsOneClaim, TestConcurrentSeatsOneVisit |
| SEA-A2 | 01-queue-seating | TestMoveKeepsAccessAndCleansOldTable |
| SEA-A3 | 01-queue-seating | TestPaidVisitKeepsTable |
| MEN-001 | 02-ordering | TestMenuReplaceValidation |
| ORD-001 | 02-ordering | TestOrderValidation, dining.spec.ts › dining |
| ORD-002 | 02-ordering | TestStaleMenuRejectsWholeOrder |
| ORD-003 | 02-ordering | TestOrderRetryAndConflict |
| ORD-004 | 02-ordering | TestLineTransitionMatrix, TestLateCancellationNeedsManager |
| ORD-005 | 02-ordering | TestTwoPhonesOrderOnce, TestAssistedOrderRecordsStaff |
| ORD-006 | 02-ordering | TestAssistanceCoalescesAndAcknowledges |
| ORD-007 | 02-ordering | TestPaidVisitRejectsFinancialChanges, TestOrdersOnlyOnOpenVisits |
| ORD-A1 | 02-ordering | TestTwoPhonesOrderOnce |
| ORD-A2 | 02-ordering | TestOrderRetryAndConflict |
| ORD-A3 | 02-ordering | TestStaleMenuRejectsWholeOrder |
| ORD-A4 | 02-ordering | TestSettlementVersusOrderSubmission, TestCancellationVersusBegin |
| ORD-A5 | 02-ordering | TestRejectedLineExcludedFromTotal |
| ORD-A6 | 02-ordering | TestAssistanceCoalescesAndAcknowledges |
| BIL-001 | 03-settlement | cashier.spec.ts › cashier |
| BIL-002 | 03-settlement | TestBeginRequiresResolvedLines |
| BIL-003 | 03-settlement | TestCalculateFixtures |
| BIL-004 | 03-settlement | TestCalculateFixtures |
| BIL-005 | 03-settlement | TestConcurrentConfirmOneSettlement, TestConfirmEffects |
| BIL-006 | 03-settlement | TestConcurrentConfirmOneSettlement, TestConfirmEffects |
| BIL-007 | 03-settlement | TestConcurrentRefundOneRecord, TestReopenInvalidatesConfirmation |
| BIL-008 | 03-settlement | TestRotateAccessOnlyBeforePayment |
| BIL-A1 | 03-settlement | TestBeginRejectsStaleBill |
| BIL-A2 | 03-settlement | TestCalculateFixtures, TestBillUsesSnapshotsAndChargeableLines |
| BIL-A3 | 03-settlement | TestConcurrentConfirmOneSettlement |
| BIL-A4 | 03-settlement | TestGuestCannotSettle |
| BIL-A5 | 03-settlement | TestReopenInvalidatesConfirmation |
| BIL-A6 | 03-settlement | TestConcurrentRefundOneRecord, TestMemberRefundReversesOnce |
| LOY-001 | 04-loyalty | loyalty.spec.ts › member points at the table |
| LOY-002 | 04-loyalty | TestClaimConflictsAndDetach |
| LOY-003 | 04-loyalty | TestTierSnapshotAtBegin |
| LOY-004 | 04-loyalty | TestEligibleSpendFixtures |
| LOY-005 | 04-loyalty | TestConcurrentMemberSettlements |
| LOY-006 | 04-loyalty | TestMemberRefundReversesOnce |
| LOY-007 | 04-loyalty | TestMemberLoyaltyHistory |
| LOY-A1 | 04-loyalty | TestClaimNeedsBothSessions |
| LOY-A2 | 04-loyalty | TestConcurrentMemberSettlements |
| LOY-A3 | 04-loyalty | TestTierSnapshotAtBegin |
| LOY-A4 | 04-loyalty | TestMemberRefundReversesOnce |
| ACC-001 | 05-access-pwa | TestCapabilityExchangeMultipleDiners, TestTokensStoredHashedOnly, TestRevokedExpiredAndWrongKindRejected, staff.spec.ts › staff access |
| ACC-002 | 05-access-pwa | TestNoPublicStaffRegistration, TestActivationTokenSingleUse, TestMemberSignupVerifyLogin, account.spec.ts › member account |
| ACC-003 | 05-access-pwa | TestStaffRoleMatrix |
| ACC-004 | 05-access-pwa | TestMutationsRequireAllowedOrigin, TestSessionCookieAttributes, TestLogoutRevokes, TestLoginRateLimited |
| OPS-001 | 05-access-pwa | TestAuditEventsViewer, TestStaffAdministrationAudited, TestTableConfiguration |
| OPS-002 | 05-access-pwa | TestDailyReportReconciles |
| PWA-001 | 05-access-pwa | pwa.spec.ts |
| PWA-002 | 05-access-pwa | pwa.spec.ts |
| PWA-003 | 05-access-pwa | journey.spec.ts › guest journey hardening, journey.spec.ts |
| PWA-004 | 05-access-pwa | journey.spec.ts |
| PWA-005 | 05-access-pwa | queue.spec.ts › queue |
| ACC-A1 | 05-access-pwa | TestCrossBranchStaffAccessIsHidden |
| ACC-A2 | 05-access-pwa | TestRotationRevokesDerivedSessions, TestRotateAccessInvalidatesOldQR |
| ACC-A3 | 05-access-pwa | TestMutationsRequireAllowedOrigin, TestRoleChangeRevokesSessions, TestInFlightMutationAfterRevocationFails, TestRevocationWaitsForInFlightMutation |
| PWA-A1 | 05-access-pwa | pwa.spec.ts |
| PWA-A2 | 05-access-pwa | journey.spec.ts › guest journey hardening, journey.spec.ts |
| PWA-A3 | 05-access-pwa | pwa.spec.ts |
| ADM-001 | 06-admin | TestStaffRoleMatrix, staff.spec.ts › staff access |
| ADM-002 | 06-admin | host.spec.ts, kitchen.spec.ts › kitchen |
| ADM-003 | 06-admin | kitchen.spec.ts › kitchen |
| ADM-004 | 06-admin | cashier.spec.ts |
| ADM-005 | 06-admin | cashier.spec.ts |
| ADM-006 | 06-admin | host.spec.ts, cashier.spec.ts |
| ADM-A1 | 06-admin | staff.spec.ts › staff access, staff.spec.ts |
| ADM-A2 | 06-admin | smoke.spec.ts › home page renders without console errors |
| ADM-A3 | 06-admin | visit.spec.ts |
| ADM-A4 | 06-admin | TestRoleChangeRevokesSessions, staff.spec.ts › staff access, staff.spec.ts |

## Required concurrency groups ([testing](../../quality/testing.md))

Every group is covered by real PostgreSQL tests with independent connections and barriers:

| # | Group | Tests |
| --- | --- | --- |
| 1 | Two hosts competing for a table; no-show versus seating; moving versus departure | TestConcurrentCallsOneClaim, TestConcurrentSeatsOneVisit, TestNoShowVersusSeatOneWinner, TestMoveVersusDepartOneWinner |
| 2 | Same order key/body in parallel; same key with a different body; response-loss retry | TestOrderRetryAndConflict, TestIdempotencyConcurrentSameKey, TestIdempotencyConflictReplayRollback; PWA `journey.spec.ts` › lost response |
| 3 | Sold-out/menu change racing with order acceptance | TestMenuChangeVersusSubmission, TestStaleMenuRejectsWholeOrder |
| 4 | Order submission or line cancellation racing begin-settlement | TestSettlementVersusOrderSubmission, TestCancellationVersusBegin (added in MVP-19) |
| 5 | Two cashiers confirming; retry after response loss; refund replay | TestConcurrentConfirmOneSettlement, TestConcurrentRefundOneRecord |
| 6 | Two visits earning for one member; full refund after policy changes | TestConcurrentMemberSettlements, TestMemberRefundReversesOnce |
| 7 | Token rotation or role revocation while requests are in flight | TestRotationRacingExchangeNeverLeaksOldGeneration, TestGuestRevalidateBlocksOnRotation, TestInFlightMutationAfterRevocationFails, TestRevocationWaitsForInFlightMutation, TestConcurrentMutualDemotion |
