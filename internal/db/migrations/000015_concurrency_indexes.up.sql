-- Migration: 000015_concurrency_indexes.up.sql
-- Optimizes query performance for 750+ concurrent multi-tenant users.

CREATE INDEX IF NOT EXISTS idx_attendance_sessions_school_date_class 
    ON attendance_sessions(school_id, session_date, class_id);

CREATE INDEX IF NOT EXISTS idx_students_school_class_active 
    ON students(school_id, class_id, is_active);

CREATE INDEX IF NOT EXISTS idx_attendance_records_session_status 
    ON attendance_records(session_id, status);

CREATE INDEX IF NOT EXISTS idx_users_school_role 
    ON users(school_id, role);
