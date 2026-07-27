-- Migration: 000001_init_schema.up.sql
-- Creates the complete multi-tenant attendance system schema.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─── Enums ────────────────────────────────────────────────────────────────────
CREATE TYPE user_role         AS ENUM ('admin', 'teacher');
CREATE TYPE attendance_status AS ENUM ('present', 'absent', 'late', 'excused');
CREATE TYPE session_status    AS ENUM ('open', 'closed');

-- ─── Tenant Root ──────────────────────────────────────────────────────────────
CREATE TABLE schools (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    code       VARCHAR(50)  NOT NULL UNIQUE,
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE academic_years (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id    UUID        NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    name         VARCHAR(50) NOT NULL,
    start_date   DATE        NOT NULL,
    end_date     DATE        NOT NULL,
    is_current   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, name)
);
CREATE INDEX idx_academic_years_school ON academic_years(school_id);

-- ─── Users ────────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id     UUID        NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    role          user_role   NOT NULL,
    full_name     VARCHAR(255) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash TEXT        NOT NULL,
    phone         VARCHAR(20),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, email)
);
CREATE INDEX idx_users_school ON users(school_id);

CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

-- ─── Academic Structure ───────────────────────────────────────────────────────
CREATE TABLE classes (
    id               UUID     PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id        UUID     NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    academic_year_id UUID     NOT NULL REFERENCES academic_years(id) ON DELETE CASCADE,
    grade_level      SMALLINT NOT NULL,
    section          VARCHAR(10) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, academic_year_id, grade_level, section)
);
CREATE INDEX idx_classes_school ON classes(school_id);

CREATE TABLE subjects (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  UUID         NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    code       VARCHAR(50)  NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (school_id, code)
);
CREATE INDEX idx_subjects_school ON subjects(school_id);

CREATE TABLE class_subjects (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  UUID NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    class_id   UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    UNIQUE (class_id, subject_id)
);
CREATE INDEX idx_class_subjects_class ON class_subjects(class_id);

-- ─── Assignments ──────────────────────────────────────────────────────────────
CREATE TABLE teacher_subject_assignments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  UUID NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    teacher_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    class_id   UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id UUID NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (teacher_id, class_id, subject_id)
);
CREATE INDEX idx_tsa_school        ON teacher_subject_assignments(school_id);
CREATE INDEX idx_tsa_teacher       ON teacher_subject_assignments(teacher_id);
CREATE INDEX idx_tsa_class_subject ON teacher_subject_assignments(class_id, subject_id);

CREATE TABLE class_teacher_assignments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  UUID NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    teacher_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    class_id   UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (class_id)
);
CREATE INDEX idx_cta_teacher ON class_teacher_assignments(teacher_id);

-- ─── Students ─────────────────────────────────────────────────────────────────
CREATE TABLE students (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id        UUID         NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    class_id         UUID         NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    admission_number VARCHAR(50)  NOT NULL,
    roll_number      VARCHAR(20)  NOT NULL,
    full_name        VARCHAR(255) NOT NULL,
    gender           VARCHAR(20),
    dob              DATE,
    guardian_name    VARCHAR(255),
    guardian_phone   VARCHAR(20),
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (school_id, admission_number),
    UNIQUE (class_id, roll_number)
);
CREATE INDEX idx_students_school ON students(school_id);
CREATE INDEX idx_students_class  ON students(class_id);

-- ─── Attendance ───────────────────────────────────────────────────────────────
CREATE TABLE attendance_sessions (
    id            UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id     UUID           NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    class_id      UUID           NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    subject_id    UUID           NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    teacher_id    UUID           NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    session_date  DATE           NOT NULL,
    period_number SMALLINT       NOT NULL DEFAULT 1,
    status        session_status NOT NULL DEFAULT 'open',
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ    NOT NULL DEFAULT now(),
    UNIQUE (class_id, subject_id, session_date, period_number)
);
CREATE INDEX idx_sessions_school_date   ON attendance_sessions(school_id, session_date);
CREATE INDEX idx_sessions_class_subject ON attendance_sessions(class_id, subject_id);

CREATE TABLE attendance_records (
    id         UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id  UUID              NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    session_id UUID              NOT NULL REFERENCES attendance_sessions(id) ON DELETE CASCADE,
    student_id UUID              NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    status     attendance_status NOT NULL,
    remarks    TEXT,
    marked_by  UUID              NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    marked_at  TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ       NOT NULL DEFAULT now(),
    UNIQUE (session_id, student_id)
);
CREATE INDEX idx_records_school             ON attendance_records(school_id);
CREATE INDEX idx_records_student            ON attendance_records(student_id);
CREATE INDEX idx_records_session            ON attendance_records(session_id);
CREATE INDEX idx_records_student_marked_at  ON attendance_records(student_id, marked_at);

CREATE TABLE attendance_audit_log (
    id                   UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id            UUID              NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    attendance_record_id UUID              NOT NULL REFERENCES attendance_records(id) ON DELETE CASCADE,
    old_status           attendance_status,
    new_status           attendance_status NOT NULL,
    changed_by           UUID              NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    changed_at           TIMESTAMPTZ       NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_record ON attendance_audit_log(attendance_record_id);

-- ─── Row Level Security ───────────────────────────────────────────────────────
-- The application role must NOT be a superuser/owner or policies are bypassed.
-- We use FORCE ROW LEVEL SECURITY so even the owner is restricted.

ALTER TABLE users                       ENABLE ROW LEVEL SECURITY;
ALTER TABLE academic_years              ENABLE ROW LEVEL SECURITY;
ALTER TABLE classes                     ENABLE ROW LEVEL SECURITY;
ALTER TABLE subjects                    ENABLE ROW LEVEL SECURITY;
ALTER TABLE class_subjects              ENABLE ROW LEVEL SECURITY;
ALTER TABLE teacher_subject_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE class_teacher_assignments   ENABLE ROW LEVEL SECURITY;
ALTER TABLE students                    ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_sessions         ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_records          ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_audit_log        ENABLE ROW LEVEL SECURITY;

-- Force RLS even for table owner (belt-and-suspenders against superuser connection bugs)
ALTER TABLE users                       FORCE ROW LEVEL SECURITY;
ALTER TABLE academic_years              FORCE ROW LEVEL SECURITY;
ALTER TABLE classes                     FORCE ROW LEVEL SECURITY;
ALTER TABLE subjects                    FORCE ROW LEVEL SECURITY;
ALTER TABLE class_subjects              FORCE ROW LEVEL SECURITY;
ALTER TABLE teacher_subject_assignments FORCE ROW LEVEL SECURITY;
ALTER TABLE class_teacher_assignments   FORCE ROW LEVEL SECURITY;
ALTER TABLE students                    FORCE ROW LEVEL SECURITY;
ALTER TABLE attendance_sessions         FORCE ROW LEVEL SECURITY;
ALTER TABLE attendance_records          FORCE ROW LEVEL SECURITY;
ALTER TABLE attendance_audit_log        FORCE ROW LEVEL SECURITY;

-- Policies: current_setting returns '' when not set (second arg = true = permissive),
-- so an unauthenticated query gets no rows.
CREATE POLICY tenant_isolation ON users
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON academic_years
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON classes
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON subjects
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON class_subjects
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON teacher_subject_assignments
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON class_teacher_assignments
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON students
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON attendance_sessions
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON attendance_records
    USING (school_id::text = current_setting('app.current_school_id', true));
CREATE POLICY tenant_isolation ON attendance_audit_log
    USING (school_id::text = current_setting('app.current_school_id', true));

-- ─── App user for RLS ─────────────────────────────────────────────────────────
-- The application connects as 'attendance_app' (not superuser).
-- Schools table has NO RLS — it is read by the login resolver before the school_id
-- is known (tenant resolution step). All other tables are protected by RLS.
--
-- ⚠ WARNING: The password below ('AttendanceApp2026') is a DEV DEFAULT ONLY.
-- In production, change it immediately after running this migration:
--   ALTER ROLE attendance_app PASSWORD 'your_strong_production_password';
-- The password used at runtime is configured via DATABASE_URL in .env, not here.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'attendance_app') THEN
        CREATE ROLE attendance_app LOGIN PASSWORD 'AttendanceApp2026';
    END IF;
END
$$;

GRANT CONNECT ON DATABASE attendance TO attendance_app;
GRANT USAGE ON SCHEMA public TO attendance_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO attendance_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO attendance_app;
