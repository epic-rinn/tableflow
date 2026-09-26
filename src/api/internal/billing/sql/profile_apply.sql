-- $2/$3 signed deltas; $4 the recalculated tier.
UPDATE member_profiles
SET points_balance = points_balance + $2,
    qualifying_spend_satang = qualifying_spend_satang + $3,
    tier = $4, version = version + 1, updated_at = now()
WHERE id = $1
RETURNING points_balance, qualifying_spend_satang
