-- Target must belong to the actor's branch; otherwise it is "not found".
SELECT status, version FROM staff_accounts
WHERE id = $1 AND branch_id = $2
FOR UPDATE
