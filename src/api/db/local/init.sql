-- Local development and CI only. Runs once as the container superuser when the
-- data volume is first created. Passwords are dummy values for disposable use.
CREATE ROLE tableflow_owner LOGIN PASSWORD 'owner_dev_only';
CREATE ROLE tableflow_app LOGIN PASSWORD 'app_dev_only';

CREATE DATABASE tableflow OWNER tableflow_owner;
REVOKE CONNECT ON DATABASE tableflow FROM PUBLIC;
GRANT CONNECT ON DATABASE tableflow TO tableflow_app;

\connect tableflow
ALTER SCHEMA public OWNER TO tableflow_owner;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO tableflow_app;

-- Objects created by migrations (run as tableflow_owner) grant DML only.
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO tableflow_app;
ALTER DEFAULT PRIVILEGES FOR ROLE tableflow_owner IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO tableflow_app;
