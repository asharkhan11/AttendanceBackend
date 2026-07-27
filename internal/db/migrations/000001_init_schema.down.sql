-- Migration: 000001_init_schema.down.sql
-- Tears down the complete schema. Use with caution on any non-dev environment.

DROP TABLE IF EXISTS attendance_audit_log      CASCADE;
DROP TABLE IF EXISTS attendance_records        CASCADE;
DROP TABLE IF EXISTS attendance_sessions       CASCADE;
DROP TABLE IF EXISTS students                  CASCADE;
DROP TABLE IF EXISTS class_teacher_assignments CASCADE;
DROP TABLE IF EXISTS teacher_subject_assignments CASCADE;
DROP TABLE IF EXISTS class_subjects            CASCADE;
DROP TABLE IF EXISTS subjects                  CASCADE;
DROP TABLE IF EXISTS classes                   CASCADE;
DROP TABLE IF EXISTS refresh_tokens            CASCADE;
DROP TABLE IF EXISTS users                     CASCADE;
DROP TABLE IF EXISTS academic_years            CASCADE;
DROP TABLE IF EXISTS schools                   CASCADE;

DROP TYPE IF EXISTS session_status    CASCADE;
DROP TYPE IF EXISTS attendance_status CASCADE;
DROP TYPE IF EXISTS user_role         CASCADE;
