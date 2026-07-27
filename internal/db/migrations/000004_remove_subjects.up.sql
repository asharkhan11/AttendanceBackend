-- Migration: 000004_remove_subjects.up.sql
-- Removes subjects and class_subjects tables, and drops subject_id from attendance_sessions.

ALTER TABLE attendance_sessions DROP CONSTRAINT IF EXISTS attendance_sessions_school_id_class_id_subject_id_sess_key;
ALTER TABLE attendance_sessions DROP CONSTRAINT IF EXISTS uq_session_tuple;

ALTER TABLE attendance_sessions DROP COLUMN IF EXISTS subject_id CASCADE;

ALTER TABLE attendance_sessions ADD CONSTRAINT uq_session_tuple UNIQUE (school_id, class_id, session_date, period_number);

DROP TABLE IF EXISTS class_subjects CASCADE;
DROP TABLE IF EXISTS subjects CASCADE;
