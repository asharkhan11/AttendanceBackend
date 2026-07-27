-- Migration: 000007_school_calendar.up.sql
-- Creates tables for weekly school day schedules and specific date holidays.

CREATE TABLE school_weekly_schedules (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id      UUID        NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    day_of_week    SMALLINT    NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    is_working_day BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (school_id, day_of_week)
);
CREATE INDEX idx_weekly_schedules_school ON school_weekly_schedules(school_id);

CREATE TABLE school_date_holidays (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id    UUID         NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    holiday_date DATE         NOT NULL,
    title        VARCHAR(255) NOT NULL,
    description  TEXT,
    is_holiday   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (school_id, holiday_date)
);
CREATE INDEX idx_date_holidays_school_date ON school_date_holidays(school_id, holiday_date);

-- Row Level Security
ALTER TABLE school_weekly_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE school_weekly_schedules FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON school_weekly_schedules
    USING (school_id::text = current_setting('app.current_school_id', true));

ALTER TABLE school_date_holidays ENABLE ROW LEVEL SECURITY;
ALTER TABLE school_date_holidays FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON school_date_holidays
    USING (school_id::text = current_setting('app.current_school_id', true));

-- App Permissions
GRANT SELECT, INSERT, UPDATE, DELETE ON school_weekly_schedules TO attendance_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON school_date_holidays TO attendance_app;
