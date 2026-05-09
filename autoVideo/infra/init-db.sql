-- init-db.sql: Create all service databases and grant privileges
-- Executed by PostgreSQL on first startup via docker-entrypoint-initdb.d

\set ON_ERROR_STOP on

CREATE DATABASE auth_db;
CREATE DATABASE project_db;
CREATE DATABASE script_db;
CREATE DATABASE character_db;
CREATE DATABASE image_db;
CREATE DATABASE video_db;
CREATE DATABASE task_db;
CREATE DATABASE model_db;
CREATE DATABASE storage_db;
CREATE DATABASE notify_db;

GRANT ALL PRIVILEGES ON DATABASE auth_db      TO postgres;
GRANT ALL PRIVILEGES ON DATABASE project_db   TO postgres;
GRANT ALL PRIVILEGES ON DATABASE script_db    TO postgres;
GRANT ALL PRIVILEGES ON DATABASE character_db TO postgres;
GRANT ALL PRIVILEGES ON DATABASE image_db     TO postgres;
GRANT ALL PRIVILEGES ON DATABASE video_db     TO postgres;
GRANT ALL PRIVILEGES ON DATABASE task_db      TO postgres;
GRANT ALL PRIVILEGES ON DATABASE model_db     TO postgres;
GRANT ALL PRIVILEGES ON DATABASE storage_db   TO postgres;
GRANT ALL PRIVILEGES ON DATABASE notify_db    TO postgres;
