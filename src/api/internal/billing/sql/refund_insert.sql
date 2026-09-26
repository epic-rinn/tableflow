INSERT INTO refunds (branch_id, settlement_id, amount_satang, reason, external_reference, recorded_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id
