-- Migration: 000005_class_teacher_mapping.up.sql
-- Adds 1-to-1 relationship between classes and class_teacher (users with role 'teacher')

ALTER TABLE classes
    ADD COLUMN class_teacher_id UUID UNIQUE REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_classes_class_teacher ON classes(class_teacher_id);
