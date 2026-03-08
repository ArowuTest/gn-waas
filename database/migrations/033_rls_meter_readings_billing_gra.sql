-- Migration 033: RLS for meter_readings, gwl_billing_records, and gra_compliance_log
-- ============================================================
-- RLS-M01 fix: These three district-linked tables were missing Row-Level Security
-- policies, relying solely on application-layer filtering. A compromised token
-- could read cross-district billing and meter data.
--
-- All three tables link to water_accounts (which has district_id) via account_id.
-- We use a sub-select to resolve the district rather than adding a redundant
-- district_id column, keeping the schema normalised.
--
-- Pattern follows migration 012 (DROP POLICY IF EXISTS for idempotency).
-- Helper functions current_user_is_admin() and current_district_id() are
-- defined in migration 012 and available here.
-- ============================================================

-- ─── meter_readings ──────────────────────────────────────────────────────────
-- Links to water_accounts via account_id. District is resolved via sub-select.
-- Field officers (FIELD_OFFICER, FIELD_SUPERVISOR) can read readings for
-- accounts in their assigned district. Admins see all.
ALTER TABLE meter_readings ENABLE ROW LEVEL SECURITY;
ALTER TABLE meter_readings FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_meter_readings_district ON meter_readings;
CREATE POLICY rls_meter_readings_district ON meter_readings
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR EXISTS (
            SELECT 1 FROM water_accounts wa
            WHERE wa.id = meter_readings.account_id
              AND wa.district_id = current_district_id()
        )
    );

COMMENT ON POLICY rls_meter_readings_district ON meter_readings
    IS 'RLS-M01 fix: Restricts meter readings to the authenticated user''s district '
       'via water_accounts.district_id. Admins (SUPER_ADMIN, SYSTEM_ADMIN, MOF_AUDITOR) bypass.';

-- ─── gwl_billing_records ─────────────────────────────────────────────────────
-- Links to water_accounts via account_id. Contains GWL billing amounts —
-- cross-district exposure would reveal competitor pricing intelligence.
ALTER TABLE gwl_billing_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE gwl_billing_records FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_gwl_billing_records_district ON gwl_billing_records;
CREATE POLICY rls_gwl_billing_records_district ON gwl_billing_records
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR EXISTS (
            SELECT 1 FROM water_accounts wa
            WHERE wa.id = gwl_billing_records.account_id
              AND wa.district_id = current_district_id()
        )
    );

COMMENT ON POLICY rls_gwl_billing_records_district ON gwl_billing_records
    IS 'RLS-M01 fix: Restricts GWL billing records to the authenticated user''s district.';

-- ─── gra_compliance_log ──────────────────────────────────────────────────────
-- Links to water_accounts via account_id. Contains GRA invoice data —
-- cross-district exposure would reveal tax compliance details.
ALTER TABLE gra_compliance_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE gra_compliance_log FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_gra_compliance_log_district ON gra_compliance_log;
CREATE POLICY rls_gra_compliance_log_district ON gra_compliance_log
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR EXISTS (
            SELECT 1 FROM water_accounts wa
            WHERE wa.id = gra_compliance_log.account_id
              AND wa.district_id = current_district_id()
        )
    );

COMMENT ON POLICY rls_gra_compliance_log_district ON gra_compliance_log
    IS 'RLS-M01 fix: Restricts GRA compliance log to the authenticated user''s district.';

-- ─── Indexes to support the RLS sub-selects efficiently ──────────────────────
-- These partial indexes ensure the EXISTS sub-selects in the RLS policies
-- use index scans rather than sequential scans on water_accounts.
CREATE INDEX IF NOT EXISTS idx_water_accounts_id_district
    ON water_accounts(id, district_id);

COMMENT ON INDEX idx_water_accounts_id_district
    IS 'Composite index supporting RLS sub-selects in meter_readings, '
       'gwl_billing_records, and gra_compliance_log policies.';
