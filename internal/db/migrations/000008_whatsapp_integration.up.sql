-- Migration: 000008_whatsapp_integration.up.sql
-- Adds WhatsApp integration settings for schools, session finalization state, and WhatsApp logs table.

ALTER TABLE schools
    ADD COLUMN IF NOT EXISTS is_whatsapp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS whatsapp_monthly_limit INT NOT NULL DEFAULT 1000;

ALTER TABLE attendance_sessions
    ADD COLUMN IF NOT EXISTS is_finalized BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS finalized_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS whatsapp_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    school_id       UUID        NOT NULL REFERENCES schools(id) ON DELETE CASCADE,
    session_id      UUID        REFERENCES attendance_sessions(id) ON DELETE SET NULL,
    student_id      UUID        REFERENCES students(id) ON DELETE SET NULL,
    recipient_phone VARCHAR(50) NOT NULL,
    status          VARCHAR(30) NOT NULL,
    meta_message_id VARCHAR(255),
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_whatsapp_logs_school_created ON whatsapp_logs(school_id, created_at);

ALTER TABLE whatsapp_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_logs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS whatsapp_logs_tenant_isolation ON whatsapp_logs;
CREATE POLICY whatsapp_logs_tenant_isolation ON whatsapp_logs
    FOR ALL
    USING (school_id = NULLIF(current_setting('app.current_school_id', true), '')::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON whatsapp_logs TO attendance_app;

