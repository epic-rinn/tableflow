-- A member's profiles with each branch's current loyalty policy (NULL = pilot defaults).
SELECT p.branch_id, b.name, p.points_balance, p.qualifying_spend_satang, p.tier,
       lp.version, lp.satang_per_point, lp.silver_threshold, lp.silver_discount_bp, lp.gold_threshold, lp.gold_discount_bp
FROM member_profiles p
JOIN branches b ON b.id = p.branch_id
LEFT JOIN LATERAL (
    SELECT * FROM loyalty_policies l WHERE l.branch_id = p.branch_id ORDER BY l.version DESC LIMIT 1
) lp ON true
WHERE p.member_id = $1
ORDER BY b.name, p.branch_id
