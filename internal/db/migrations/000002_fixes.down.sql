-- Migration: 000002_fixes.down.sql
-- Reverses 000002_fixes.up.sql

DROP INDEX IF EXISTS idx_refresh_tokens_hash;
DROP FUNCTION IF EXISTS get_user_school_id(UUID);
