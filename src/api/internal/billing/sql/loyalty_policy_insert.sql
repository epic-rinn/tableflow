INSERT INTO loyalty_policies (branch_id, version, satang_per_point, silver_threshold, silver_discount_bp,
                              gold_threshold, gold_discount_bp, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
