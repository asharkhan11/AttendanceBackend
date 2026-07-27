-- Migration: 000008_whatsapp_integration.down.sql

DROP TABLE IF EXISTS whatsapp_logs CASCADE;

ALTER TABLE attendance_sessions
    DROP COLUMN IF EXISTS finalized_at,
    DROP COLUMN IF EXISTS is_finalized;

ALTER TABLE schools
    DROP COLUMN IF EXISTS whatsapp_monthly_limit,
    DROP COLUMN IF EXISTS is_whatsapp_enabled;
