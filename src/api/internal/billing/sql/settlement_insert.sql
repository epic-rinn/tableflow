INSERT INTO settlements (branch_id, visit_id, snapshot_id, receipt_reference, amount_satang, method,
                         verification_note, external_reference, confirmed_by, member_id, eligible_satang,
                         points_earned, loyalty_policy_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id
