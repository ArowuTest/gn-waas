-- Migration 047: Safe column fixes using individual DO blocks
-- ===========================================================
-- Previous migrations (024-046) failed silently because they ran as a single
-- transaction. One failing statement (e.g., CREATE POLICY, ALTER DEFAULT
-- PRIVILEGES) rolled back ALL column additions.
--
-- This migration wraps each critical operation in its own DO block with
-- EXCEPTION WHEN OTHERS THEN NULL, so individual failures are isolated.
-- The column additions will succeed even if RLS/grant statements fail.

-- ── Step 1: Reset failed migration entries so they can re-run ─────────────────
-- (belt-and-suspenders: also fix columns directly in this migration)
DELETE FROM schema_migrations
WHERE filename IN (
    '024_add_district_id_to_illegal_connections.sql',
    '043_grant_permissions_fix.sql',
    '044_seed_water_balance_records.sql',
    '045_fix_illegal_connections_live_db.sql',
    '046_emergency_column_fixes.sql'
);

-- ── Step 2: illegal_connections columns ──────────────────────────────────────
DO $$ BEGIN
    ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}';
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'photo_hashes: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS district_id UUID REFERENCES districts(id) ON DELETE SET NULL;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'district_id: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS account_number VARCHAR(50);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'account_number: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS gps_accuracy NUMERIC(8,2) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'gps_accuracy: %', SQLERRM; END $$;

DO $$ BEGIN
    CREATE INDEX IF NOT EXISTS idx_illegal_connections_district ON illegal_connections (district_id) WHERE district_id IS NOT NULL;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'idx_illegal_connections_district: %', SQLERRM; END $$;

-- ── Step 3: audit_events columns ─────────────────────────────────────────────
DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS confirmed_loss_ghs NUMERIC(15,2) DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'confirmed_loss_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS recovered_ghs NUMERIC(15,2) DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'recovered_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS success_fee_ghs NUMERIC(15,2) DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'success_fee_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS gra_status VARCHAR(50) NOT NULL DEFAULT 'PENDING';
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'gra_status: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}';
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae_photo_hashes: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS variance_pct NUMERIC(8,4);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'variance_pct: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS variance_ghs NUMERIC(15,2);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'variance_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS ocr_confidence NUMERIC(5,4);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ocr_confidence: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS ocr_status VARCHAR(20) DEFAULT 'PENDING';
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ocr_status: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS meter_reading_m3 NUMERIC(12,4);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'meter_reading_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS shadow_bill_ghs NUMERIC(15,2);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'shadow_bill_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actual_bill_ghs NUMERIC(15,2);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'actual_bill_ghs: %', SQLERRM; END $$;

-- ── Step 4: water_balance_records columns ────────────────────────────────────
DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS nrw_percent NUMERIC(8,4);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'nrw_percent: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS ili_score NUMERIC(8,4);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ili_score: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS iwa_grade VARCHAR(2);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'iwa_grade: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS estimated_revenue_recovery_ghs NUMERIC(15,2) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'estimated_revenue_recovery_ghs: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS data_confidence_score INTEGER;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'data_confidence_score: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS computed_at TIMESTAMPTZ;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'computed_at: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS system_input_volume_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'system_input_volume_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS billed_metered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'billed_metered_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS billed_unmetered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'billed_unmetered_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unbilled_metered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'unbilled_metered_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unbilled_unmetered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'unbilled_unmetered_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_authorised_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'total_authorised_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unauthorised_consumption_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'unauthorised_consumption_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS metering_inaccuracies_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'metering_inaccuracies_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS data_handling_errors_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'data_handling_errors_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_apparent_losses_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'total_apparent_losses_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS main_leakage_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'main_leakage_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS storage_overflow_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'storage_overflow_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS service_conn_leakage_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'service_conn_leakage_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_real_losses_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'total_real_losses_m3: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_nrw_m3 NUMERIC(15,4) NOT NULL DEFAULT 0;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'total_nrw_m3: %', SQLERRM; END $$;

-- ── Step 5: districts columns ─────────────────────────────────────────────────
DO $$ BEGIN
    ALTER TABLE districts ADD COLUMN IF NOT EXISTS loss_ratio_pct NUMERIC(5,2);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'loss_ratio_pct: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE districts ADD COLUMN IF NOT EXISTS gps_latitude NUMERIC(10,7);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'gps_latitude: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE districts ADD COLUMN IF NOT EXISTS gps_longitude NUMERIC(10,7);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'gps_longitude: %', SQLERRM; END $$;

-- ── Step 6: field_jobs columns ────────────────────────────────────────────────
DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome VARCHAR(50);
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'outcome: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome_notes TEXT;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'outcome_notes: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS meter_found BOOLEAN;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'meter_found: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS address_confirmed BOOLEAN;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'address_confirmed: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS recommended_action TEXT;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'recommended_action: %', SQLERRM; END $$;

DO $$ BEGIN
    ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome_recorded_at TIMESTAMPTZ;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'outcome_recorded_at: %', SQLERRM; END $$;

-- ── Step 7: Re-grant permissions ──────────────────────────────────────────────
DO $$ BEGIN
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app;
    GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'grant: %', SQLERRM; END $$;

-- ── Step 8: RLS on illegal_connections ───────────────────────────────────────
DO $$ BEGIN
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
EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'rls: %', SQLERRM; END $$;

