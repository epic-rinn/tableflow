INSERT INTO menus (branch_id) VALUES ($1) ON CONFLICT (branch_id) DO NOTHING
