-- Lock order: visits → member profiles → child rows (data model).
SELECT id, points_balance, qualifying_spend_satang, tier
FROM member_profiles WHERE branch_id = $1 AND member_id = $2
FOR UPDATE
