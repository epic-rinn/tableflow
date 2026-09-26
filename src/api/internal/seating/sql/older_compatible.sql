-- Is an older waiting party compatible with this table? ($2 = join_order of
-- the party being served; walk-ins pass the maximum bigint.)
SELECT EXISTS (
    SELECT 1 FROM queue_tickets
    WHERE branch_id = $1 AND state = 'waiting' AND join_order < $2
      AND party_size <= $3 AND needs <@ $4::text[]
)
