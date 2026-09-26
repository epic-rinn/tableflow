-- Sales by paid date and method; member settlements counted separately.
SELECT (paid_at AT TIME ZONE $4)::date::text, method, count(*), sum(amount_satang)::bigint,
       count(*) FILTER (WHERE member_id IS NOT NULL)
FROM settlements
WHERE branch_id = $1 AND paid_at >= $2 AND paid_at < $3
GROUP BY 1, 2
