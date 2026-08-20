-- Migration: 000015_concurrency_indexes.down.sql

DROP INDEX IF EXISTS idx_attendance_sessions_school_date_class;
DROP INDEX IF EXISTS idx_students_school_class_active;
DROP INDEX IF EXISTS idx_attendance_records_session_status;
DROP INDEX IF EXISTS idx_users_school_role;
