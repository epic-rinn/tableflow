INSERT INTO settlements (branch_id, visit_id, snapshot_id, receipt_reference, amount_satang, method,
                         verification_note, external_reference, confirmed_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id
