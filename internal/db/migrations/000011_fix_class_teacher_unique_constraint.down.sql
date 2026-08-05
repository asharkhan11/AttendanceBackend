-- Migration: 000011_fix_class_teacher_unique_constraint.down.sql

ALTER TABLE classes DROP CONSTRAINT IF EXISTS classes_academic_year_class_teacher_key;

ALTER TABLE classes ADD CONSTRAINT classes_class_teacher_id_key UNIQUE (class_teacher_id);
