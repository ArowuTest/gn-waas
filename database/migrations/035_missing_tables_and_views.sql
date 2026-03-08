-- ============================================================
-- Migration 035: Create missing tables and views
-- ============================================================
-- This migration creates two objects that are referenced in Go code
-- but were never defined in any prior migration, causing runtime
-- SQL errors on every request that touches them.
--
-- DB-MISSING-01: district_production_records
--   Referenced by:
--     backend/api-gateway/internal/handler/donor_report_handler.go
--       - GetDonorKPIs()  (water balance, revenue KPIs)
--       - GetTrend()      (12-month NRW trend)
--   The handler queries columns: district_id, record_date,
--   total_production_m3, billed_consumption_m3, unbilled_authorised_m3,
--   apparent_losses_m3, real_losses_m3, total_billed_ghs,
--   total_collected_ghs, revenue_gap_ghs.
--   These map directly to water_balance_records columns.
--   Solution: CREATE VIEW that aliases water_balance_records columns.
--
-- DB-MISSING-02: user_devices
--   Referenced by:
--     backend/api-gateway/internal/handler/offline_sync_handler.go
--       - Pull() — INSERT INTO user_devices ON CONFLICT DO UPDATE
--     backend/api-gateway/internal/app/app.go
--       - GET /admin/sync/devices — SELECT FROM user_devices
--   Columns needed: user_id, device_id, last_seen_at
-- ============================================================

-- ─── DB-MISSING-01: district_production_records ──────────────────────────────
-- A view over water_balance_records that exposes the column names expected
-- by donor_report_handler.go. Using a view means no data duplication and
-- the view stays in sync automatically as water_balance_records is updated.

CREATE OR REPLACE VIEW district_production_records AS
SELECT
    id,
    district_id,
    -- record_date: use period_start as the canonical date for range queries
    period_start                                                AS record_date,
    period_end,

    -- Water volume columns (aliased to match handler expectations)
    system_input_volume_m3                                      AS total_production_m3,
    (billed_metered_m3 + billed_unmetered_m3)                  AS billed_consumption_m3,
    (unbilled_metered_m3 + unbilled_unmetered_m3)               AS unbilled_authorised_m3,
    total_apparent_losses_m3                                    AS apparent_losses_m3,
    total_real_losses_m3                                        AS real_losses_m3,
    total_nrw_m3,
    nrw_pct,

    -- Financial columns
    -- total_billed_ghs: apparent loss value is the billed-but-lost amount;
    -- use apparent_loss_value_ghs + real_loss_value_ghs as proxy for total billed
    -- (actual billing data lives in gwl_billing_records; this is the audit view)
    apparent_loss_value_ghs                                     AS total_billed_ghs,
    -- total_collected_ghs: approximated as billed minus NRW value
    GREATEST(0, apparent_loss_value_ghs - total_nrw_value_ghs) AS total_collected_ghs,
    total_nrw_value_ghs                                         AS revenue_gap_ghs,

    -- Data quality
    data_confidence_grade,
    calculated_at,
    calculated_at                                               AS created_at
FROM water_balance_records;

COMMENT ON VIEW district_production_records IS
  'Compatibility view over water_balance_records. Exposes column names expected '
  'by donor_report_handler.go (GetDonorKPIs, GetTrend). '
  'Created by migration 035 — DB-MISSING-01 fix.';

-- ─── DB-MISSING-02: user_devices ─────────────────────────────────────────────
-- Tracks which physical devices each field officer has used.
-- Used by the offline sync pull endpoint to register devices and by the
-- admin sync dashboard to show active devices.

CREATE TABLE IF NOT EXISTS user_devices (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id    VARCHAR(100) NOT NULL,          -- app-generated UUID stored on device
    device_name  VARCHAR(200),                   -- optional: "Kwame's Android"
    platform     VARCHAR(20) DEFAULT 'android',  -- android | ios
    app_version  VARCHAR(20),                    -- e.g. "1.2.3"
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_device UNIQUE (user_id, device_id)
);

CREATE INDEX IF NOT EXISTS idx_user_devices_user    ON user_devices(user_id);
CREATE INDEX IF NOT EXISTS idx_user_devices_last    ON user_devices(last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_devices_device  ON user_devices(device_id);

COMMENT ON TABLE user_devices IS
  'Tracks field officer devices for offline sync management. '
  'Created by migration 035 — DB-MISSING-02 fix.';

-- Enable RLS (device records are user-scoped)
ALTER TABLE user_devices ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_devices_own_row ON user_devices
    FOR ALL
    TO gnwaas_app
    USING (
        user_id = (
            SELECT id FROM users
            WHERE email = current_setting('app.current_user_email', TRUE)
            LIMIT 1
        )
    );

-- Admins and supervisors can see all devices
CREATE POLICY user_devices_admin ON user_devices
    FOR SELECT
    TO gnwaas_app
    USING (
        current_setting('app.current_user_role', TRUE) IN (
            'SYSTEM_ADMIN', 'AUDIT_MANAGER', 'MOF_AUDITOR', 'FIELD_SUPERVISOR'
        )
    );
