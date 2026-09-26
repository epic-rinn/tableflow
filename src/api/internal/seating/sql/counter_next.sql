-- Next display number for the branch's current business date ($2 = time zone).
INSERT INTO queue_counters (branch_id, business_date, last_number)
VALUES ($1, (now() AT TIME ZONE $2)::date, 1)
ON CONFLICT (branch_id, business_date) DO UPDATE SET last_number = queue_counters.last_number + 1
RETURNING business_date, last_number
