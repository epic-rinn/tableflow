-- Claim ($2 = member) or detach ($2 = NULL); always a new bill version.
UPDATE visits
SET member_id = $2::uuid,
    member_claimed_at = CASE WHEN $2::uuid IS NULL THEN NULL ELSE now() END,
    bill_version = bill_version + 1, version = version + 1, updated_at = now()
WHERE id = $1
RETURNING bill_version
