-- Migration: 000012_student_class_enrollments.up.sql
-- Create student_class_enrollments table to track student class assignments per academic year.

CREATE TABLE IF NOT EXISTS student_class_enrollments (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id        UUID         NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    student_id       UUID         NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    class_id         UUID         NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    academic_year_id UUID         NOT NULL REFERENCES academic_years(id) ON DELETE CASCADE,
    roll_number      VARCHAR(20)  NOT NULL,
    status           VARCHAR(20)  NOT NULL DEFAULT 'active', -- 'active', 'promoted', 'graduated', 'transferred'
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (student_id, academic_year_id),
    UNIQUE (class_id, roll_number)
);

CREATE INDEX IF NOT EXISTS idx_enrollments_school ON student_class_enrollments(school_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_student ON student_class_enrollments(student_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_class ON student_class_enrollments(class_id);
CREATE INDEX IF NOT EXISTS idx_enrollments_year ON student_class_enrollments(academic_year_id);

ALTER TABLE student_class_enrollments ENABLE ROW LEVEL SECURITY;
ALTER TABLE student_class_enrollments FORCE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies WHERE tablename = 'student_class_enrollments' AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON student_class_enrollments
            USING (school_id = NULLIF(current_setting('app.current_school_id', true), '')::uuid);
    END IF;
END $$;

-- Backfill active enrollments for current students
INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
SELECT s.school_id, s.id, s.class_id, c.academic_year_id, s.roll_number, 'active'
FROM students s
JOIN classes c ON s.class_id = c.id
ON CONFLICT (student_id, academic_year_id) DO UPDATE SET class_id = EXCLUDED.class_id, roll_number = EXCLUDED.roll_number;

-- Backfill historic enrollments from past attendance_records -> attendance_sessions
INSERT INTO student_class_enrollments (school_id, student_id, class_id, academic_year_id, roll_number, status)
SELECT DISTINCT ar.school_id, ar.student_id, sess.class_id, c.academic_year_id, s.roll_number, 'promoted'
FROM attendance_records ar
JOIN attendance_sessions sess ON ar.session_id = sess.id
JOIN classes c ON sess.class_id = c.id
JOIN students s ON ar.student_id = s.id
ON CONFLICT (student_id, academic_year_id) DO NOTHING;

GRANT SELECT, INSERT, UPDATE, DELETE ON student_class_enrollments TO attendance_app;

