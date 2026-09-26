SELECT id FROM assistance_requests WHERE visit_id = $1 AND topic = $2 AND state IN ('open', 'acknowledged')
