-- Migration: 000009_add_missing_indexes.up.sql
-- Optimizes table scans on whatsapp message webhook status updates, session filters, and audit logs.

CREATE UNIQUE INDEX IF NOT EXISTS idx_whatsapp_logs_meta_message_id 
    ON whatsapp_logs(meta_message_id);

CREATE INDEX IF NOT EXISTS idx_whatsapp_logs_session_student_status 
    ON whatsapp_logs(session_id, student_id, status);

CREATE INDEX IF NOT EXISTS idx_attendance_sessions_class_id 
    ON attendance_sessions(class_id);

CREATE INDEX IF NOT EXISTS idx_attendance_audit_log_school_changed 
    ON attendance_audit_log(school_id, changed_at);
