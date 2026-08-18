-- Migration: 000014_session_isolation.down.sql

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'students_academic_year_admission_number_key'
    ) THEN
        ALTER TABLE students DROP CONSTRAINT students_academic_year_admission_number_key;
    END IF;
END $$;

ALTER TABLE students DROP COLUMN IF EXISTS academic_year_id;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'students_school_id_admission_number_key'
    ) THEN
        ALTER TABLE students ADD CONSTRAINT students_school_id_admission_number_key UNIQUE (school_id, admission_number);
    END IF;
END $$;
