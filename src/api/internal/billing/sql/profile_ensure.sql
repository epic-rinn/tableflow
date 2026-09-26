-- First claim at a branch creates the member's profile (a child of the
-- visit lock in the lock order).
INSERT INTO member_profiles (branch_id, member_id) VALUES ($1, $2)
ON CONFLICT (branch_id, member_id) DO NOTHING
