-- scripts/init_db.sql
-- Runs as postgres superuser during docker-compose first boot.
-- Creates the attendance_app role that the API connects as (non-superuser, RLS applies).

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app') THEN
    CREATE ROLE attendance_app LOGIN PASSWORD 'AttendanceApp2026';
  END IF;
  ALTER ROLE attendance_owner WITH PASSWORD 'AttendanceOwner2026';
  ALTER ROLE attendance_owner BYPASSRLS;
END
$$;

GRANT CONNECT ON DATABASE attendance TO attendance_app;
GRANT USAGE ON SCHEMA public TO attendance_app;

-- Permissions are granted AFTER schema creation (01_schema.sql), so we
-- use a function that runs post-schema. This file only creates the role.
-- The schema migration (01_schema.sql) includes GRANT statements for the role.
