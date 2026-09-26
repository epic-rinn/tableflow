SELECT s.id, s.branch_id, s.visit_id, s.receipt_reference, s.amount_satang, s.method, s.verification_note,
       s.external_reference, s.paid_at, cs.display_name, t.label, v.state,
       b.bill_version, b.policy_version, b.tax_mode, b.tax_bp, b.service_bp, b.discount_bp,
       b.gross_satang, b.discount_satang, b.service_satang, b.tax_satang, b.total_satang, b.lines,
       r.id, r.amount_satang, r.reason, r.external_reference, rs.display_name, r.created_at,
       s.member_id IS NOT NULL, s.points_earned, s.eligible_satang, b.member_tier
FROM settlements s
JOIN bill_snapshots b ON b.id = s.snapshot_id
JOIN visits v ON v.id = s.visit_id
JOIN dining_tables t ON t.id = v.table_id
JOIN staff_accounts cs ON cs.id = s.confirmed_by
LEFT JOIN refunds r ON r.settlement_id = s.id
LEFT JOIN staff_accounts rs ON rs.id = r.recorded_by
WHERE s.id = $1
