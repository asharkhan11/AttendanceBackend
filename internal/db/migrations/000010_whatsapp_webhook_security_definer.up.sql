-- Migration: 000010_whatsapp_webhook_security_definer.up.sql
-- Creates the SECURITY DEFINER function to bypass Row Level Security 
-- when retrieving a school_id during unauthenticated WhatsApp webhooks.

CREATE OR REPLACE FUNCTION get_whatsapp_log_school_id(p_meta_message_id VARCHAR)
RETURNS UUID
LANGUAGE sql
STABLE
SECURITY DEFINER
AS $$
    SELECT school_id FROM whatsapp_logs WHERE meta_message_id = p_meta_message_id LIMIT 1;
$$;

REVOKE ALL ON FUNCTION get_whatsapp_log_school_id(VARCHAR) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION get_whatsapp_log_school_id(VARCHAR) TO attendance_app;
