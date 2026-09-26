-- Tickets by join date ($4 timezone) with their current outcome.
SELECT (created_at AT TIME ZONE $4)::date::text, count(*),
       count(*) FILTER (WHERE state = 'seated'), count(*) FILTER (WHERE state = 'no_show'),
       count(*) FILTER (WHERE state = 'cancelled')
FROM queue_tickets
WHERE branch_id = $1 AND created_at >= $2 AND created_at < $3
GROUP BY 1
