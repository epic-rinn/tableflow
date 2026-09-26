-- Unlocked peek to find the visit to lock first (lock order visit → settlement).
SELECT visit_id FROM settlements WHERE id = $1
