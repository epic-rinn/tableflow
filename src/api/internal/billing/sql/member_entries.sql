-- Newest first; keyset on (created_at, id). Only the member's own rows.
SELECT l.id, l.kind, l.points_delta, l.qualifying_delta_satang, l.created_at, s.receipt_reference, b.name
FROM loyalty_ledger l
JOIN settlements s ON s.id = l.settlement_id
JOIN branches b ON b.id = l.branch_id
WHERE l.member_id = $1 AND (l.created_at, l.id) < ($2, $3)
ORDER BY l.created_at DESC, l.id DESC
LIMIT $4
