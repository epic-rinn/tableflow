SELECT branch_id, amount_satang, member_id, eligible_satang, points_earned, loyalty_policy_version FROM settlements WHERE id = $1 FOR UPDATE
