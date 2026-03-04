-- ============================================================
-- GN-WAAS Migration: 020_raw_gwl_data_columns
-- Description: Add raw_gwl_data JSONB column to meter_readings
--              and gwl_bills tables.
--
-- NEW-DB-01 fix: The cdc-ingestor service stores the raw GWL source
-- row as JSONB in both meter_readings and gwl_bills for full audit
-- traceability. However, neither table had this column defined in
-- the original schema, causing a fatal runtime error:
--   ERROR: column "raw_gwl_data" of relation "meter_readings" does not exist
--   ERROR: column "raw_gwl_data" of relation "gwl_bills" does not exist
--
-- Using ADD COLUMN IF NOT EXISTS makes this migration safe to re-run.
-- The column is nullable so existing rows are unaffected.
-- ============================================================

-- Add raw_gwl_data to meter_readings
-- Stores the verbatim GWL source row as JSONB for audit traceability.
ALTER TABLE meter_readings
    ADD COLUMN IF NOT EXISTS raw_gwl_data JSONB;

COMMENT ON COLUMN meter_readings.raw_gwl_data IS
    'Verbatim GWL source row captured at CDC sync time. '
    'Enables full audit traceability back to the GWL ERP record.';

-- Add raw_gwl_data to gwl_bills
-- Stores the verbatim GWL billing row as JSONB for audit traceability.
ALTER TABLE gwl_bills
    ADD COLUMN IF NOT EXISTS raw_gwl_data JSONB;

COMMENT ON COLUMN gwl_bills.raw_gwl_data IS
    'Verbatim GWL billing row captured at CDC sync time. '
    'Enables full audit traceability back to the GWL ERP billing record.';
