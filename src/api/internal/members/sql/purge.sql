WITH s AS (
    DELETE FROM member_sessions
    WHERE id IN (SELECT id FROM member_sessions WHERE expires_at < now() - interval '30 days' LIMIT 5000)
    RETURNING 1
), t AS (
    DELETE FROM member_tokens
    WHERE id IN (SELECT id FROM member_tokens WHERE expires_at < now() - interval '30 days' LIMIT 5000)
    RETURNING 1
)
SELECT (SELECT count(*) FROM s) + (SELECT count(*) FROM t)
