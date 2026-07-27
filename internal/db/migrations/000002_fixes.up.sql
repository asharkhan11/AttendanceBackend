-- Migration: 000002_fixes.up.sql
-- 1. Creates the missing get_user_school_id() SECURITY DEFINER function
--    (required by token refresh — see DECISIONS.md "Refresh token school_id lookup").
-- 2. Adds an index on refresh_tokens.token_hash for fast lookups.

-- ─── get_user_school_id ───────────────────────────────────────────────────────
-- A narrow SECURITY DEFINER function that bypasses RLS to resolve a user's
-- school_id during token refresh (before a tenant transaction exists).
-- Owned by the migration role (attendance_owner) — the only RLS bypass
-- in the system.
CREATE OR REPLACE FUNCTION get_user_school_id(p_user_id UUID)
RETURNS UUID
LANGUAGE sql
STABLE
SECURITY DEFINER
AS $$
    SELECT school_id FROM users WHERE id = p_user_id;
$$;

-- Restrict execution to the app role only
REVOKE ALL ON FUNCTION get_user_school_id(UUID) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION get_user_school_id(UUID) TO attendance_app;

-- ─── Index on refresh_tokens.token_hash ───────────────────────────────────────
-- Token lookups during refresh and logout query by token_hash. Without an
-- index, this becomes a sequential scan as the table grows.
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
