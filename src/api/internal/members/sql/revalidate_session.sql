-- Re-check a member session inside a mutation transaction; FOR SHARE makes a
-- concurrent logout/reset wait for (or fail) the mutation.
SELECT s.member_id FROM member_sessions s
WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
FOR SHARE
