-- $2 new state; $3 bumps the bill version (begin/reopen invalidate prior bills).
UPDATE visits
SET state = $2,
    bill_version = bill_version + CASE WHEN $3::boolean THEN 1 ELSE 0 END,
    paid_at = CASE WHEN $2 = 'paid' THEN now() ELSE paid_at END,
    version = version + 1, updated_at = now()
WHERE id = $1
RETURNING bill_version, version
