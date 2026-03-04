-- Migration 023: water_balance_records — Add Sentinel-Written Columns
-- =====================================================================
--
-- The sentinel service (iwa_water_balance.go) INSERTs into water_balance_records
-- using column names that do not exist in the original schema (migration 003).
-- The API-gateway (data_handler.go) SELECTs the same columns.
-- This migration adds the missing columns so both services work correctly.
--
-- Missing columns (sentinel writes / API-gateway reads):
--   nrw_percent              — NRW as a percentage (schema had nrw_pct)
--   ili_score                — Infrastructure Leakage Index (schema had ili_value)
--   iwa_grade                — IWA letter grade A/B/C/D derived from ILI
--   estimated_revenue_recovery_ghs — GHS value of recoverable NRW
--                              (schema had apparent_loss_value_ghs / total_nrw_value_ghs)
--   data_confidence_score    — Integer 0–100 DCS (schema had data_confidence_grade enum)
--   computed_at              — Timestamp when sentinel computed this record
--                              (schema had calculated_at)
--
-- Additionally, the sentinel ON CONFLICT clause requires a UNIQUE constraint on
-- (district_id, period_start, period_end) which was missing from migration 003.
-- =====================================================================

-- ─── Add missing sentinel-written columns ────────────────────────────────────
ALTER TABLE water_balance_records
    ADD COLUMN IF NOT EXISTS nrw_percent                    NUMERIC(8,4),
    ADD COLUMN IF NOT EXISTS ili_score                      NUMERIC(8,4),
    ADD COLUMN IF NOT EXISTS iwa_grade                      VARCHAR(2),
    ADD COLUMN IF NOT EXISTS estimated_revenue_recovery_ghs NUMERIC(15,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS data_confidence_score          INTEGER,
    ADD COLUMN IF NOT EXISTS computed_at                    TIMESTAMPTZ;

COMMENT ON COLUMN water_balance_records.nrw_percent IS
    'Non-Revenue Water as a percentage of system input volume. '
    'Written by sentinel iwa_water_balance.go. '
    'Replaces the original nrw_pct column name used in migration 003.';

COMMENT ON COLUMN water_balance_records.ili_score IS
    'Infrastructure Leakage Index (ILI = Current Annual Real Losses / UARL). '
    'Written by sentinel iwa_water_balance.go. '
    'Replaces the original ili_value column name used in migration 003.';

COMMENT ON COLUMN water_balance_records.iwa_grade IS
    'IWA letter grade derived from ILI: A (<1.5), B (1.5–2.5), C (2.5–4), D (>4). '
    'Written by sentinel iwa_water_balance.go.';

COMMENT ON COLUMN water_balance_records.estimated_revenue_recovery_ghs IS
    'Estimated GHS value recoverable from apparent losses (underbilling, theft, etc.). '
    'Written by sentinel iwa_water_balance.go.';

COMMENT ON COLUMN water_balance_records.data_confidence_score IS
    'Integer 0–100 Data Confidence Score for this water balance record. '
    'Written by sentinel iwa_water_balance.go. '
    'Replaces the original data_confidence_grade enum column.';

COMMENT ON COLUMN water_balance_records.computed_at IS
    'Timestamp when sentinel last computed this water balance record. '
    'Written by sentinel iwa_water_balance.go. '
    'Replaces the original calculated_at column name used in migration 003.';

-- ─── Add UNIQUE constraint required for ON CONFLICT upsert ───────────────────
-- The sentinel INSERT uses ON CONFLICT (district_id, period_start, period_end)
-- DO UPDATE SET ... which requires a UNIQUE constraint on these three columns.
-- Without it, PostgreSQL raises SQLSTATE 42P10 at runtime.
--
-- DB-C01 fix: PostgreSQL does NOT support ADD CONSTRAINT IF NOT EXISTS syntax.
-- The idiomatic idempotent pattern is a DO block that checks pg_constraint first.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'water_balance_records'::regclass
          AND conname   = 'water_balance_records_district_period_unique'
    ) THEN
        ALTER TABLE water_balance_records
            ADD CONSTRAINT water_balance_records_district_period_unique
            UNIQUE (district_id, period_start, period_end);
    END IF;
END;
$$;

-- ─── Index for fast period-range queries ─────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_water_balance_district_period
    ON water_balance_records(district_id, period_start DESC);

CREATE INDEX IF NOT EXISTS idx_water_balance_computed_at
    ON water_balance_records(computed_at DESC)
    WHERE computed_at IS NOT NULL;
