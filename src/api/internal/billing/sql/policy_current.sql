-- Latest charge policy version for a branch (none = unconfigured).
SELECT version, tax_mode, tax_bp, service_bp, created_at
FROM charge_policies WHERE branch_id = $1
ORDER BY version DESC LIMIT 1
