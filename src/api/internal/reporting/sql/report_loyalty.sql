SELECT (created_at AT TIME ZONE $4)::date::text,
       coalesce(sum(points_delta) FILTER (WHERE kind = 'earn'), 0)::bigint,
       coalesce(-sum(points_delta) FILTER (WHERE kind = 'reversal'), 0)::bigint
FROM loyalty_ledger
WHERE branch_id = $1 AND created_at >= $2 AND created_at < $3
GROUP BY 1
