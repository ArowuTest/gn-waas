-- ============================================================
-- Migration 038: Add password_hash to users table
-- Purpose: Enable native email+password login without Keycloak
--          for demo/staging environments.
--          In production, Keycloak remains the IdP.
-- ============================================================

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

COMMENT ON COLUMN users.password_hash IS
    'bcrypt hash of the user password. NULL = Keycloak-only auth. '
    'Set for demo/staging users to enable native login.';
