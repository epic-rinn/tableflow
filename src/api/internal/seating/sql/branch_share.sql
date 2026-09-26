-- Branch settings; FOR SHARE serialises joins with seating-group replacement.
SELECT timezone, call_hold_minutes FROM branches WHERE id = $1 FOR SHARE
