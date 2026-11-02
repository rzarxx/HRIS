-- Menyiapkan database pengembangan hris_dev dan tiga role sesuai MASTER.md §5.10.
-- Dijalankan oleh dev-db-setup.ps1 sebagai superuser. Aman dijalankan berulang.
\set ON_ERROR_STOP on
\getenv migrate_pw HRIS_MIGRATE_PW
\getenv app_pw HRIS_APP_PW
\getenv ro_pw HRIS_RO_PW

SELECT format('CREATE ROLE hris_migrate LOGIN PASSWORD %L', :'migrate_pw')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hris_migrate') \gexec
SELECT format('CREATE ROLE hris_app LOGIN PASSWORD %L', :'app_pw')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hris_app') \gexec
SELECT format('CREATE ROLE hris_ro LOGIN PASSWORD %L', :'ro_pw')
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'hris_ro') \gexec

SELECT format('ALTER ROLE hris_migrate PASSWORD %L', :'migrate_pw') \gexec
SELECT format('ALTER ROLE hris_app PASSWORD %L', :'app_pw') \gexec
SELECT format('ALTER ROLE hris_ro PASSWORD %L', :'ro_pw') \gexec

SELECT 'CREATE DATABASE hris_dev OWNER hris_migrate ENCODING ''UTF8'' TEMPLATE template0'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'hris_dev') \gexec

\connect hris_dev

GRANT CONNECT ON DATABASE hris_dev TO hris_app, hris_ro;
GRANT ALL ON SCHEMA public TO hris_migrate;
GRANT USAGE ON SCHEMA public TO hris_app, hris_ro;

ALTER DEFAULT PRIVILEGES FOR ROLE hris_migrate IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO hris_app;
ALTER DEFAULT PRIVILEGES FOR ROLE hris_migrate IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO hris_app;
ALTER DEFAULT PRIVILEGES FOR ROLE hris_migrate IN SCHEMA public
  GRANT SELECT ON TABLES TO hris_ro;

\echo Database hris_dev siap.
