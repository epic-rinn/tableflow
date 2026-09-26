-- Refunds by refund date and the original settlement's method.
SELECT (r.created_at AT TIME ZONE $4)::date::text, s.method, count(*), sum(r.amount_satang)::bigint
FROM refunds r
JOIN settlements s ON s.id = r.settlement_id
WHERE r.branch_id = $1 AND r.created_at >= $2 AND r.created_at < $3
GROUP BY 1, 2
