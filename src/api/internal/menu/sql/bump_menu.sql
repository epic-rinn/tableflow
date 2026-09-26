UPDATE menus SET revision = $2, updated_at = now() WHERE branch_id = $1
