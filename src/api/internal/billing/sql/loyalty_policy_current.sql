SELECT version, satang_per_point, silver_threshold, silver_discount_bp, gold_threshold, gold_discount_bp
FROM loyalty_policies WHERE branch_id = $1
ORDER BY version DESC LIMIT 1
