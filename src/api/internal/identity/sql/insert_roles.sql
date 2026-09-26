INSERT INTO staff_roles (staff_account_id, role)
SELECT $1, unnest($2::text[])
