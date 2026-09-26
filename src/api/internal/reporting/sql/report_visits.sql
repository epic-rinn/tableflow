SELECT (opened_at AT TIME ZONE $4)::date::text, count(*)
FROM visits
WHERE branch_id = $1 AND opened_at >= $2 AND opened_at < $3
GROUP BY 1
