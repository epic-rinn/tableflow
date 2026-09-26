-- $1 token_hash, $2 idle timeout.
SELECT s.id, a.id, a.email, a.locale, a.email_verified_at IS NOT NULL,
       s.last_seen_at < now() - interval '1 minute' AS stale
FROM member_sessions s
JOIN member_accounts a ON a.id = s.member_id
WHERE s.token_hash = $1 AND s.revoked_at IS NULL
  AND s.expires_at > now() AND s.last_seen_at > now() - $2::interval
