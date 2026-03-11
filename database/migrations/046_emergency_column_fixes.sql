-- Migration 046: Emergency column fixes for live DB
-- ==================================================
-- Previous migrations (024, 045) may have been marked as applied in
-- schema_migrations even though they failed silently. This migration
-- uses a fresh filename to guarantee execution.
--
-- Fixes all missing columns that cause 500 errors in production.

-- ═══ illegal_connections: add all missing columns ═════════════════════════════
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS photo_hashes   TEXT[]        NOT NULL DEFAULT '{}';
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS district_id    UUID          REFERENCES districts(id) ON DELETE SET NULL;
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS account_number VARCHAR(50);
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS gps_accuracy   NUMERIC(8,2)  NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_illegal_connections_district
    ON illegal_connections (district_id)
    WHERE district_id IS NOT NULL;

-- ═══ water_balance_records: add all missing columns ══════════════════════════
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

-- ═══ audit_events: ensure all columns exist ══════════════════════════════════
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS photo_hashes   TEXT[]        NOT NULL DEFAULT '{}';
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS gra_status     VARCHAR(50)   NOT NULL DEFAULT 'PENDING';
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS confirmed_loss_ghs  NUMERIC(15,2);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS recovered_ghs       NUMERIC(15,2);

-- ═══ field_jobs: ensure outcome columns exist ════════════════════════════════
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome        VARCHAR(50);
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome_notes  TEXT;
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS meter_found    BOOLEAN;
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS address_confirmed BOOLEAN;
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS recommended_action TEXT;
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome_recorded_at TIMESTAMPTZ;

-- ═══ districts: ensure loss_ratio_pct exists ═════════════════════════════════
ALTER TABLE districts
    ADD COLUMN IF NOT EXISTS loss_ratio_pct NUMERIC(5,2);
ALTER TABLE districts
    ADD COLUMN IF NOT EXISTS gps_latitude   NUMERIC(10,7);
ALTER TABLE districts
    ADD COLUMN IF NOT EXISTS gps_longitude  NUMERIC(10,7);

-- ═══ Re-grant permissions on all tables ══════════════════════════════════════
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO gnwaas_app;

-- ═══ Re-enable RLS on illegal_connections ════════════════════════════════════
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


-- ═══ audit_events: ensure dashboard query columns exist ══════════════════════
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS confirmed_loss_ghs  NUMERIC(15,2) DEFAULT 0;
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS recovered_ghs       NUMERIC(15,2) DEFAULT 0;
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS success_fee_ghs     NUMERIC(15,2) DEFAULT 0;
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS gra_status          VARCHAR(50)   NOT NULL DEFAULT 'PENDING';
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS photo_hashes        TEXT[]        NOT NULL DEFAULT '{}';
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS variance_pct        NUMERIC(8,4);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS variance_ghs        NUMERIC(15,2);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS ocr_confidence      NUMERIC(5,4);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS ocr_status          VARCHAR(20)   DEFAULT 'PENDING';
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS meter_reading_m3    NUMERIC(12,4);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS shadow_bill_ghs     NUMERIC(15,2);
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS actual_bill_ghs     NUMERIC(15,2);

-- ═══ water_balance_records: ensure UNIQUE constraint exists ══════════════════
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'water_balance_records_district_id_period_start_period_end_key'
    ) THEN
        ALTER TABLE water_balance_records
            ADD CONSTRAINT water_balance_records_district_id_period_start_period_end_key
            UNIQUE (district_id, period_start, period_end);
    END IF;
END $$;

-- ═══ Ensure system_input_volume_m3 and other WB columns exist ════════════════
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS system_input_volume_m3          NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS billed_metered_m3               NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS billed_unmetered_m3             NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS unbilled_metered_m3             NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS unbilled_unmetered_m3           NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS total_authorised_m3             NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS unauthorised_consumption_m3     NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS metering_inaccuracies_m3        NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS data_handling_errors_m3         NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS total_apparent_losses_m3        NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS main_leakage_m3                 NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS storage_overflow_m3             NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS service_conn_leakage_m3         NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS total_real_losses_m3            NUMERIC(15,4) NOT NULL DEFAULT 0;
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS total_nrw_m3                    NUMERIC(15,4) NOT NULL DEFAULT 0;

COMMENT ON TABLE illegal_connections IS
    'Migration 046: Emergency column fixes applied. All missing columns added.';
