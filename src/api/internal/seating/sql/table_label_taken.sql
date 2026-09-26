SELECT EXISTS (SELECT 1 FROM dining_tables WHERE branch_id = $1 AND label = $2 AND id <> $3)
