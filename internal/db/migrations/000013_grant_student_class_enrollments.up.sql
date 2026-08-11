-- Migration: 000013_grant_student_class_enrollments.up.sql
-- Grant table privileges on student_class_enrollments to attendance_app role

GRANT SELECT, INSERT, UPDATE, DELETE ON student_class_enrollments TO attendance_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO attendance_app;
