SELECT id, policy_version, tax_mode, tax_bp, service_bp, discount_bp,
       gross_satang, discount_satang, service_satang, tax_satang, total_satang, lines
FROM bill_snapshots WHERE visit_id = $1 AND bill_version = $2
