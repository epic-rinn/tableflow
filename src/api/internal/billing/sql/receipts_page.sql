-- Newest first; keyset on (paid_at, id). $4 optional exact receipt reference.
SELECT s.id, s.receipt_reference, s.amount_satang, s.method, s.paid_at, t.label, r.id IS NOT NULL
FROM settlements s
JOIN visits v ON v.id = s.visit_id
JOIN dining_tables t ON t.id = v.table_id
LEFT JOIN refunds r ON r.settlement_id = s.id
WHERE s.branch_id = $1
  AND (s.paid_at, s.id) < ($2, $3)
  AND ($4::text IS NULL OR s.receipt_reference = $4)
ORDER BY s.paid_at DESC, s.id DESC
LIMIT $5
