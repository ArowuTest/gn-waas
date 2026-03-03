-- GN-WAAS Migration: 018_enum_values_and_rls_policies
-- Date: 2026-03-03
-- Description: Fixes two issues identified in the v18 code audit.
--
--   DB-H02: anomaly_type enum is missing BILLING_VARIANCE, NRW_SPIKE, and
--           OVERBILLING. These values are used in gwl_case_repo.go (GetCaseSummary,
--           GetMonthlyReport) and inserted by 006_demo_timeseries.sql seed data.
--           PostgreSQL raises "invalid input value for enum" at runtime without them.
--
--   BE-M01 (DB part): revenue_recovery_events and officer_gps_tracks have no RLS
--           policies. The RevenueRecoveryHandler and WorkforceHandler bypass RLS
--           by using the raw DB pool; adding policies here ensures that even if
--           a future handler forgets to use the RLS transaction, the database
--           itself enforces district-level isolation.
--           officer_gps_tracks also lacks a district_id column, which is required
--           for the RLS policy and for the workforce handler to scope queries.

-- ── DB-H02: Add missing anomaly_type enum values ──────────────────────────────
-- ALTER TYPE ... ADD VALUE cannot run inside a transaction block in PostgreSQL.
-- Each ADD VALUE is idempotent when wrapped in a DO block that checks pg_enum.

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumtypid = 'anomaly_type'::regtype
          AND enumlabel = 'BILLING_VARIANCE'
    ) THEN
        ALTER TYPE anomaly_type ADD VALUE 'BILLING_VARIANCE';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumtypid = 'anomaly_type'::regtype
          AND enumlabel = 'NRW_SPIKE'
    ) THEN
        ALTER TYPE anomaly_type ADD VALUE 'NRW_SPIKE';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumtypid = 'anomaly_type'::regtype
          AND enumlabel = 'OVERBILLING'
    ) THEN
        ALTER TYPE anomaly_type ADD VALUE 'OVERBILLING';
    END IF;
END $$;

-- ── BE-M01 (DB part): Add district_id to officer_gps_tracks ──────────────────
-- The WorkforceHandler needs district_id to scope GPS track queries per district.
-- Populated from the officer's district at INSERT time (set by the handler).
-- NULL allowed for backward compatibility with existing rows.
ALTER TABLE officer_gps_tracks
    ADD COLUMN IF NOT EXISTS district_id UUID REFERENCES districts(id);

CREATE INDEX IF NOT EXISTS idx_gps_tracks_district
    ON officer_gps_tracks(district_id, recorded_at DESC);

COMMENT ON COLUMN officer_gps_tracks.district_id IS
    'District the officer belongs to at the time of the GPS recording. '
    'Used for RLS district-level isolation. Populated by WorkforceHandler '
    'from the authenticated user''s rls_district_id.';

-- ── BE-M01 (DB part): RLS policies for revenue_recovery_events ───────────────
-- revenue_recovery_events already has district_id (migration 016).
ALTER TABLE revenue_recovery_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE revenue_recovery_events FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_revenue_recovery_district ON revenue_recovery_events;
CREATE POLICY rls_revenue_recovery_district ON revenue_recovery_events
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- ── BE-M01 (DB part): RLS policies for officer_gps_tracks ────────────────────
-- officer_gps_tracks now has district_id (added above).
ALTER TABLE officer_gps_tracks ENABLE ROW LEVEL SECURITY;
ALTER TABLE officer_gps_tracks FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS rls_officer_gps_tracks_district ON officer_gps_tracks;
CREATE POLICY rls_officer_gps_tracks_district ON officer_gps_tracks
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
        -- Field officers can always see their own tracks regardless of district
        OR officer_id = current_setting('app.user_id', true)::uuid
    );
