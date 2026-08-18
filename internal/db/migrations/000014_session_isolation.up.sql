-- Migration: 000014_session_isolation.up.sql
-- Adds academic_year_id to students table to isolate student records per session,
-- updates uniqueness constraints to allow identical admission numbers across different academic sessions,
-- and ensures student data changes in a new session never alter historical session records.

-- 1. Add academic_year_id to students table
ALTER TABLE students 
ADD COLUMN IF NOT EXISTS academic_year_id UUID REFERENCES academic_years(id) ON DELETE CASCADE;

-- 2. Backfill academic_year_id from classes
UPDATE students s
SET academic_year_id = c.academic_year_id
FROM classes c
WHERE s.class_id = c.id AND s.academic_year_id IS NULL;

-- 3. Update students academic_year_id to NOT NULL for consistency where possible
-- (If there are orphaned students without classes, assign them to current or latest academic year)
UPDATE students s
SET academic_year_id = (
    SELECT id FROM academic_years ay 
    WHERE ay.school_id = s.school_id 
    ORDER BY is_current DESC, created_at DESC 
    LIMIT 1
)
WHERE s.academic_year_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_students_academic_year ON students(academic_year_id);

-- 4. Replace global (school_id, admission_number) uniqueness constraint with (academic_year_id, admission_number)
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'students_school_id_admission_number_key'
    ) THEN
        ALTER TABLE students DROP CONSTRAINT students_school_id_admission_number_key;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'students_academic_year_admission_number_key'
    ) THEN
        ALTER TABLE students ADD CONSTRAINT students_academic_year_admission_number_key UNIQUE (academic_year_id, admission_number);
    END IF;
END $$;
