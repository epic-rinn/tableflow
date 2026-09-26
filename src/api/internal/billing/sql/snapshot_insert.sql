INSERT INTO bill_snapshots (branch_id, visit_id, bill_version, policy_version, tax_mode, tax_bp, service_bp,
                            discount_bp, gross_satang, discount_satang, service_satang, tax_satang, total_satang,
                            lines, created_by, member_id, member_tier, loyalty_policy_version,
                            satang_per_point)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING id
