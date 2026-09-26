UPDATE idempotency_requests SET status_code = $2, response = $3 WHERE id = $1 AND status_code = 0
