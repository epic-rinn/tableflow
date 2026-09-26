UPDATE visits SET state = $2, close_reason = $3, ended_at = now(), version = version + 1, updated_at = now()
WHERE id = $1
