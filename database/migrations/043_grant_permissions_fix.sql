-- Migration 043: Fix missing permissions for gnwaas_app role
-- ============================================================
-- Problem: GRANT in migration 012 only covered tables existing at that time.
-- Tables created in migrations 013+ (illegal_connections, etc.) were not
-- covered by the blanket GRANT. This caused INSERT/UPDATE to fail with
-- "permission denied for table illegal_connections" when running as gnwaas_app.
--
-- Fix: Re-grant all necessary permissions on ALL current tables.
-- Also set ALTER DEFAULT PRIVILEGES so future tables are covered automatically.

-- ─── Re-grant on all existing tables ─────────────────────────────────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO gnwaas_app;

-- ─── Default privileges for future tables ────────────────────────────────────
-- Any table created by the gnwaas superuser will automatically grant
-- SELECT, INSERT, UPDATE, DELETE to gnwaas_app.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gnwaas_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO gnwaas_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO gnwaas_app;

-- ─── Explicit grant for illegal_connections (belt-and-suspenders) ─────────────
GRANT SELECT, INSERT, UPDATE, DELETE ON illegal_connections TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON water_balance_records TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON production_records TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON meter_readings TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON gwl_billing_records TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON whistleblower_tips TO gnwaas_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON offline_sync_queue TO gnwaas_app;

COMMENT ON SCHEMA public IS 'GN-WAAS main schema. gnwaas_app has full DML permissions via migration 043.';
