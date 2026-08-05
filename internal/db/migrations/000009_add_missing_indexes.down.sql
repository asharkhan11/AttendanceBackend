-- Migration: 000009_add_missing_indexes.down.sql
-- Reverts missing indexes.

DROP INDEX IF EXISTS idx_whatsapp_logs_meta_message_id;
DROP INDEX IF EXISTS idx_whatsapp_logs_session_student_status;
DROP INDEX IF EXISTS idx_attendance_sessions_class_id;
DROP INDEX IF EXISTS idx_attendance_audit_log_school_changed;
