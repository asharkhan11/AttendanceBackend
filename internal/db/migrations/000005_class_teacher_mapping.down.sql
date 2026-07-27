-- Migration: 000005_class_teacher_mapping.down.sql
DROP INDEX IF EXISTS idx_classes_class_teacher;
ALTER TABLE classes DROP COLUMN IF EXISTS class_teacher_id;
