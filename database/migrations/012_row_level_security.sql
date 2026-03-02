-- Migration 012: Row-Level Security (RLS) for District Multi-Tenancy
-- ===================================================================
-- C4: Enforce data isolation at the database level so that a query
--     bug or compromised application role can never expose one
--     district's data to another.
--
-- Strategy:
--   - Enable RLS on all district-scoped tables
--   - Create policies that check the current_setting('app.district_id')
--     session variable, which the API gateway sets at the start of
--     every request via SET LOCAL app.district_id = '<uuid>'
--   - SYSTEM_ADMIN and SUPER_ADMIN bypass RLS (BYPASSRLS privilege)
--   - A separate gnwaas_app role is used for application queries
--
-- The API gateway must call:
--   SET LOCAL app.district_id = '<district_uuid>';
--   SET LOCAL app.user_role   = '<role>';
-- at the start of every transaction.

-- ─────────────────────────────────────────────────────────────
-- 1. Create application roles if they don't exist
-- ─────────────────────────────────────────────────────────────
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
        CREATE ROLE gnwaas_app LOGIN PASSWORD 'CHANGE_IN_PRODUCTION';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_admin') THEN
        CREATE ROLE gnwaas_admin LOGIN PASSWORD 'CHANGE_IN_PRODUCTION' BYPASSRLS;
    END IF;
END;
$$;

-- Grant table access to app role
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO gnwaas_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;

-- ─────────────────────────────────────────────────────────────
-- 2. Helper function: get current district from session variable
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION current_district_id() RETURNS UUID
LANGUAGE plpgsql STABLE SECURITY DEFINER AS $$
DECLARE
    v_district_id TEXT;
BEGIN
    v_district_id := current_setting('app.district_id', true);
    IF v_district_id IS NULL OR v_district_id = '' THEN
        RETURN NULL;
    END IF;
    RETURN v_district_id::UUID;
EXCEPTION WHEN others THEN
    RETURN NULL;
END;
$$;

-- Helper: is the current user a system-level admin?
CREATE OR REPLACE FUNCTION current_user_is_admin() RETURNS BOOLEAN
LANGUAGE plpgsql STABLE SECURITY DEFINER AS $$
DECLARE
    v_role TEXT;
BEGIN
    v_role := current_setting('app.user_role', true);
    RETURN v_role IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'NATIONAL_REGULATOR');
EXCEPTION WHEN others THEN
    RETURN FALSE;
END;
$$;

-- ─────────────────────────────────────────────────────────────
-- 3. Enable RLS on district-scoped tables
-- ─────────────────────────────────────────────────────────────

-- water_accounts
ALTER TABLE water_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE water_accounts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_water_accounts_district ON water_accounts;
CREATE POLICY rls_water_accounts_district ON water_accounts
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- audit_events
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_audit_events_district ON audit_events;
CREATE POLICY rls_audit_events_district ON audit_events
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- anomaly_flags
ALTER TABLE anomaly_flags ENABLE ROW LEVEL SECURITY;
ALTER TABLE anomaly_flags FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_anomaly_flags_district ON anomaly_flags;
CREATE POLICY rls_anomaly_flags_district ON anomaly_flags
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- field_jobs
ALTER TABLE field_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE field_jobs FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_field_jobs_district ON field_jobs;
CREATE POLICY rls_field_jobs_district ON field_jobs
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- nrw_reports
ALTER TABLE nrw_reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE nrw_reports FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_nrw_reports_district ON nrw_reports;
CREATE POLICY rls_nrw_reports_district ON nrw_reports
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- gwl_cases
ALTER TABLE gwl_cases ENABLE ROW LEVEL SECURITY;
ALTER TABLE gwl_cases FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_gwl_cases_district ON gwl_cases;
CREATE POLICY rls_gwl_cases_district ON gwl_cases
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- users: officers can only see users in their own district
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_users_district ON users;
CREATE POLICY rls_users_district ON users
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
        OR id::TEXT = current_setting('app.user_id', true)  -- users can always see themselves
    );

-- ─────────────────────────────────────────────────────────────
-- 4. Districts table: all authenticated users can read districts
--    but only admins can write
-- ─────────────────────────────────────────────────────────────
ALTER TABLE districts ENABLE ROW LEVEL SECURITY;
ALTER TABLE districts FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_districts_read ON districts;
CREATE POLICY rls_districts_read ON districts
    FOR SELECT TO gnwaas_app
    USING (true);  -- all authenticated users can list districts

DROP POLICY IF EXISTS rls_districts_write ON districts;
CREATE POLICY rls_districts_write ON districts
    FOR INSERT TO gnwaas_app
    WITH CHECK (current_user_is_admin());

DROP POLICY IF EXISTS rls_districts_update ON districts;
CREATE POLICY rls_districts_update ON districts
    FOR UPDATE TO gnwaas_app
    USING (current_user_is_admin());

-- ─────────────────────────────────────────────────────────────
-- 5. audit_trail: append-only for app role, full read for admins
-- ─────────────────────────────────────────────────────────────
ALTER TABLE audit_trail ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_trail FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_audit_trail_insert ON audit_trail;
CREATE POLICY rls_audit_trail_insert ON audit_trail
    FOR INSERT TO gnwaas_app
    WITH CHECK (true);  -- anyone can append

DROP POLICY IF EXISTS rls_audit_trail_select ON audit_trail;
CREATE POLICY rls_audit_trail_select ON audit_trail
    FOR SELECT TO gnwaas_app
    USING (current_user_is_admin());  -- only admins can read audit trail

-- ─────────────────────────────────────────────────────────────
-- 6. Comment documenting the RLS contract for developers
-- ─────────────────────────────────────────────────────────────
COMMENT ON FUNCTION current_district_id() IS
    'Returns the district UUID from the session variable app.district_id. '
    'The API gateway must SET LOCAL app.district_id = <uuid> at the start '
    'of every authenticated request transaction.';

COMMENT ON FUNCTION current_user_is_admin() IS
    'Returns true if app.user_role is SYSTEM_ADMIN, SUPER_ADMIN, or NATIONAL_REGULATOR. '
    'These roles bypass district-level RLS and can see all districts.';
