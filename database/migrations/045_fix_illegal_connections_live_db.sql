-- Migration 045: Fix illegal_connections table for live DB
-- =========================================================
-- The live database is missing columns that were added in migrations 013-024
-- but may not have been applied correctly. This migration idempotently ensures
-- all required columns exist.
--
-- Root cause: migration 024 added district_id and photo_hashes to
-- illegal_connections, but the live DB shows these columns are absent,
-- causing "column does not exist" errors on INSERT.
--
-- This migration is safe to run multiple times (all ADD COLUMN IF NOT EXISTS).

-- ── Ensure photo_hashes column exists (added in migration 014) ────────────────
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}';

-- ── Ensure district_id column exists (added in migration 024) ─────────────────
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS district_id UUID REFERENCES districts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_illegal_connections_district
    ON illegal_connections (district_id)
    WHERE district_id IS NOT NULL;

-- ── Ensure account_number column exists ───────────────────────────────────────
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS account_number VARCHAR(50);

-- ── Ensure gps_accuracy column exists ────────────────────────────────────────
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS gps_accuracy NUMERIC(8,2) NOT NULL DEFAULT 0;

-- ── Re-enable RLS (idempotent) ────────────────────────────────────────────────
ALTER TABLE illegal_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE illegal_connections FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_illegal_connections_district ON illegal_connections;
CREATE POLICY rls_illegal_connections_district ON illegal_connections
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
        OR officer_id::TEXT = current_setting('app.user_id', true)
    );

-- ── Re-grant permissions (belt-and-suspenders after migration 043) ────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON illegal_connections TO gnwaas_app;

-- ── Ensure water_balance_records has all required columns ─────────────────────
-- (in case migration 023 columns are also missing on live DB)
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS nrw_percent                    NUMERIC(8,4);
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS ili_score                      NUMERIC(8,4);
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS iwa_grade                      VARCHAR(2);
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS estimated_revenue_recovery_ghs NUMERIC(15,2) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS data_confidence_score          INTEGER;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS computed_at                    TIMESTAMPTZ;

-- ── Re-grant on water_balance_records ────────────────────────────────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON water_balance_records TO gnwaas_app;

COMMENT ON TABLE illegal_connections IS
    'Field reports of illegal water connections. Migration 045 ensures all '
    'columns (district_id, photo_hashes, account_number, gps_accuracy) exist '
    'on the live DB regardless of prior migration state.';
