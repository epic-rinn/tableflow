SELECT request_hash, status_code, response FROM idempotency_requests
WHERE scope = $1 AND operation = $2 AND idem_key = $3
