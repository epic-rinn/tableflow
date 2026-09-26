#!/bin/sh
# First-boot roles for the staging database (runs once as the superuser).
# Owner runs migrations; the API connects as the DML-only app role.
set -eu
psql -v ON_ERROR_STOP=1 -U postgres -v owner="$TABLEFLOW_OWNER_PASSWORD" -v app="$TABLEFLOW_APP_PASSWORD" <<'SQL'
CREATE ROLE tableflow_owner LOGIN PASSWORD :'owner';
CREATE ROLE tableflow_app LOGIN PASSWORD :'app';
CREATE DATABASE tableflow OWNER tableflow_owner;
REVOKE CONNECT ON DATABASE tableflow FROM PUBLIC;
GRANT CONNECT ON DATABASE tableflow TO tableflow_app;
\connect tableflow
ALTER SCHEMA public OWNER TO tableflow_owner;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO tableflow_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO tableflow_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO tableflow_app;
SQL
