-- Migration: 000011_fix_class_teacher_unique_constraint.up.sql
-- Fixes unique constraint on classes.class_teacher_id so a teacher can be assigned
-- as class teacher for 1 class per academic year (rather than globally across all years).

ALTER TABLE classes DROP CONSTRAINT IF EXISTS classes_class_teacher_id_key;

ALTER TABLE classes ADD CONSTRAINT classes_academic_year_class_teacher_key UNIQUE (academic_year_id, class_teacher_id);
