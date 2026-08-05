-- Migration: 000010_whatsapp_webhook_security_definer.down.sql
-- Drops get_whatsapp_log_school_id function.

DROP FUNCTION IF EXISTS get_whatsapp_log_school_id(VARCHAR);
