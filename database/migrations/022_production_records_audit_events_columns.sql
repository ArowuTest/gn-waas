-- Migration 022: Add Missing Columns for Data-Flow Consistency
-- =============================================================
--
-- This migration adds columns that application services write to but that
-- were missing from the original schema, causing silent INSERT failures.
--
-- FLOW-01 fix: production_records — CDC-ingestor (demo_sync_service.go) writes
--   volume_treated_m3, pumping_hours, energy_kwh, data_quality_score but these
--   columns did not exist. The sentinel reads volume_m3 and recorded_at (correct
--   schema names), so only the extra columns need to be added here.
--
-- FLOW-07 fix: audit_events — sentinel iwa_water_balance.go reads
--   ocr_confidence and consumption_variance_m3 for metering inaccuracy
--   calculations. These columns did not exist, causing the query to fail
--   silently (non-fatal warning). Adding them enables accurate IWA M36
--   metering inaccuracy accounting.
-- =============================================================

-- ─── production_records: add richer operational columns ──────────────────────
ALTER TABLE production_records
    ADD COLUMN IF NOT EXISTS volume_treated_m3   NUMERIC(15,4),
    ADD COLUMN IF NOT EXISTS pumping_hours        NUMERIC(8,2),
    ADD COLUMN IF NOT EXISTS energy_kwh           NUMERIC(12,2),
    ADD COLUMN IF NOT EXISTS data_quality_score   NUMERIC(5,4);  -- 0.0–1.0 DCS

COMMENT ON COLUMN production_records.volume_treated_m3  IS 'Volume after treatment (typically 97-99% of produced volume).';
COMMENT ON COLUMN production_records.pumping_hours      IS 'Total pump operating hours for the period.';
COMMENT ON COLUMN production_records.energy_kwh         IS 'Energy consumed in kWh for the period.';
COMMENT ON COLUMN production_records.data_quality_score IS 'Data Confidence Score 0.0–1.0 assigned by CDC ingestor.';

-- ─── audit_events: add OCR confidence and consumption variance columns ────────
-- Used by sentinel iwa_water_balance.go to compute IWA M36 Metering Inaccuracies.
-- ocr_confidence: 0.0–1.0 score from OCR service (< 0.70 = low confidence).
-- consumption_variance_m3: difference between OCR reading and manual reading.
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS ocr_confidence          NUMERIC(5,4),   -- 0.0–1.0
    ADD COLUMN IF NOT EXISTS consumption_variance_m3 NUMERIC(12,3);  -- |ocr - manual| in m³

COMMENT ON COLUMN audit_events.ocr_confidence          IS 'OCR confidence score 0.0–1.0. Values < 0.70 indicate unreliable readings.';
COMMENT ON COLUMN audit_events.consumption_variance_m3 IS 'Absolute difference between OCR and manual reading in m³. Used for IWA M36 metering inaccuracy calculation.';

-- Index to speed up the sentinel metering inaccuracy query
CREATE INDEX IF NOT EXISTS idx_audit_events_ocr_confidence
    ON audit_events(ocr_confidence)
    WHERE ocr_confidence IS NOT NULL;
