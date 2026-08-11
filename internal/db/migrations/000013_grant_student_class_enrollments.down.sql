-- Migration: 000013_grant_student_class_enrollments.down.sql

REVOKE SELECT, INSERT, UPDATE, DELETE ON student_class_enrollments FROM attendance_app;
