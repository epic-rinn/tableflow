-- Orders, line cancellations and settlement (MVP-12) all lock the visit row,
-- which serialises them (ORD-A4).
SELECT branch_id, state FROM visits WHERE id = $1 FOR UPDATE
