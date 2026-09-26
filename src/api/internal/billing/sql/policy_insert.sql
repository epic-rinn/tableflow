INSERT INTO charge_policies (branch_id, version, tax_mode, tax_bp, service_bp, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING created_at
