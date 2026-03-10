-- Migration 037: Fix RLS policies to use direct current_setting() calls
-- 
-- BUG-RLS-02: The original RLS policies used STABLE SECURITY DEFINER wrapper
-- functions (current_user_is_admin(), current_district_id()). PostgreSQL's query
-- planner evaluates STABLE functions at plan time, not execution time, causing
-- "One-Time Filter: false" and returning 0 rows for all non-superuser queries.
--
-- Fix: Replace wrapper function calls with direct current_setting() expressions,
-- which are always evaluated at execution time.
--
-- Also: Remove SUPERUSER from the app user (gnwaas) so RLS is actually enforced.
-- Superusers bypass RLS even with FORCE ROW LEVEL SECURITY.

-- Remove superuser from app user
ALTER ROLE gnwaas NOSUPERUSER;

-- Re-grant all necessary privileges
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO gnwaas;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO gnwaas;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO gnwaas;
GRANT USAGE ON SCHEMA public TO gnwaas;

-- Fix anomaly_flags RLS policy
DROP POLICY IF EXISTS rls_anomaly_flags_district ON anomaly_flags;
CREATE POLICY rls_anomaly_flags_district ON anomaly_flags
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR district_id::text = current_setting('app.district_id', true)
  );

-- Fix audit_events RLS policy
DROP POLICY IF EXISTS rls_audit_events_district ON audit_events;
CREATE POLICY rls_audit_events_district ON audit_events
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR district_id::text = current_setting('app.district_id', true)
  );

-- Fix field_jobs RLS policy
DROP POLICY IF EXISTS rls_field_jobs_district ON field_jobs;
CREATE POLICY rls_field_jobs_district ON field_jobs
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR district_id::text = current_setting('app.district_id', true)
  );

-- Fix water_accounts RLS policy
DROP POLICY IF EXISTS rls_water_accounts_district ON water_accounts;
CREATE POLICY rls_water_accounts_district ON water_accounts
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR district_id::text = current_setting('app.district_id', true)
  );

-- Fix revenue_recovery_events RLS policy
DROP POLICY IF EXISTS rls_revenue_recovery_events_district ON revenue_recovery_events;
CREATE POLICY rls_revenue_recovery_events_district ON revenue_recovery_events
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR district_id::text = current_setting('app.district_id', true)
  );

-- Fix meter_readings RLS policy
DROP POLICY IF EXISTS rls_meter_readings_district ON meter_readings;
CREATE POLICY rls_meter_readings_district ON meter_readings
  AS PERMISSIVE FOR ALL
  USING (
    current_setting('app.user_role', true) IN ('SYSTEM_ADMIN', 'SUPER_ADMIN', 'MOF_AUDITOR')
    OR account_id IN (
      SELECT id FROM water_accounts
      WHERE district_id::text = current_setting('app.district_id', true)
    )
  );

-- CRITICAL: Grant gnwaas_app role to gnwaas user
-- The RLS policies apply to gnwaas_app role, but the app connects as gnwaas.
-- Without this grant, gnwaas has no applicable policies -> default deny on all tables.
GRANT gnwaas_app TO gnwaas;
