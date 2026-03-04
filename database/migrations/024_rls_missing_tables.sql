-- Migration 024: RLS Policies for District-Scoped Tables Missing from Migration 012
-- ==================================================================================
--
-- SEC-C02 (Critical): Several district-scoped tables lack Row-Level Security policies,
-- allowing a user with a valid token for one district to query another district's data
-- by simply changing the district_id query parameter. This is a major data-leak
-- vulnerability.
--
-- Tables missing RLS in migration 012:
--   • water_balance_records  — contains per-district IWA water balance calculations
--   • production_records     — contains per-district bulk production volumes
--   • illegal_connections    — contains sensitive field reports (SEC-H01 also applies)
--
-- SEC-H01 (High): The illegal_connections table has no district_id column, making it
-- impossible to apply a district-scoped RLS policy. Any user with table access can
-- see all reports from all districts.
--
-- Fix strategy:
--   1. Add district_id to illegal_connections (populated from the reporting officer's
--      district via the API handler).
--   2. Enable RLS + create district-scoped policies on all three tables.
--   3. Add supporting indexes for the new district_id column.
-- ==================================================================================

-- ─── Part 1: SEC-H01 — Add district_id to illegal_connections ────────────────
-- The column is nullable initially so existing rows are not rejected.
-- The API handler (ReportIllegalConnection) will populate it going forward.
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS district_id UUID REFERENCES districts(id) ON DELETE SET NULL;

-- Index for district-scoped queries and RLS policy evaluation
CREATE INDEX IF NOT EXISTS idx_illegal_connections_district
    ON illegal_connections (district_id)
    WHERE district_id IS NOT NULL;

COMMENT ON COLUMN illegal_connections.district_id IS
    'District where the illegal connection was found. Populated by the API gateway '
    'from the reporting officer''s JWT district claim. Required for RLS enforcement. '
    'SEC-H01 fix: migration 024.';

-- ─── Part 2: SEC-C02 — Enable RLS on water_balance_records ───────────────────
ALTER TABLE water_balance_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE water_balance_records FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_water_balance_records_district ON water_balance_records;
CREATE POLICY rls_water_balance_records_district ON water_balance_records
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

COMMENT ON TABLE water_balance_records IS
    'IWA/AWWA M36 water balance records per district per period. '
    'RLS enforced: district users see only their own district''s records. '
    'SEC-C02 fix: migration 024.';

-- ─── Part 3: SEC-C02 — Enable RLS on production_records ──────────────────────
ALTER TABLE production_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE production_records FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_production_records_district ON production_records;
CREATE POLICY rls_production_records_district ON production_records
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

COMMENT ON TABLE production_records IS
    'Bulk water production records per district. '
    'RLS enforced: district users see only their own district''s records. '
    'SEC-C02 fix: migration 024.';

-- ─── Part 4: SEC-C02 + SEC-H01 — Enable RLS on illegal_connections ───────────
-- Now that district_id exists, we can apply a district-scoped policy.
-- Officers can see all reports in their district; admins see all.
ALTER TABLE illegal_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE illegal_connections FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_illegal_connections_district ON illegal_connections;
CREATE POLICY rls_illegal_connections_district ON illegal_connections
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
        -- Allow the reporting officer to always see their own reports
        -- (handles the case where district_id is NULL for legacy rows)
        OR officer_id::TEXT = current_setting('app.user_id', true)
    );

COMMENT ON TABLE illegal_connections IS
    'Field reports of illegal water connections, bypasses, and meter tampering. '
    'RLS enforced: district users see only their own district''s reports. '
    'SEC-C02 + SEC-H01 fix: migration 024.';
