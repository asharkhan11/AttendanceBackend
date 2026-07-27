-- Migration: 000003_remove_assignments.up.sql
-- Drops assignment tables completely per requirement to focus purely on attendance.

DROP TABLE IF EXISTS teacher_subject_assignments CASCADE;
DROP TABLE IF EXISTS class_teacher_assignments CASCADE;
