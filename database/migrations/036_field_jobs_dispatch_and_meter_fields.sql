-- ============================================================
-- GN-WAAS Migration: 036_field_jobs_dispatch_and_meter_fields
-- Description: Add missing columns required by the offline sync
--              handler (offline_sync_handler.go) that were
--              referenced in SQL but never defined in any migration.
--
-- Root cause: offline_sync_handler.go was written against a
-- planned schema that was never fully migrated. This caused:
--   1. /offline/pull  → HTTP 500 (field officers cannot download jobs)
--   2. /offline/push  → HTTP 500 (meter readings cannot be submitted)
--
-- Bugs fixed:
--   BUG-SYNC-01: field_jobs missing job_type, scheduled_date, instructions
--   BUG-SYNC-03: water_accounts missing calibration_factor
--   BUG-SYNC-03: meter_readings missing adjusted_reading_m3,
--                calibration_factor_applied, ocr_reading_m3
--
-- All additions use IF NOT EXISTS for idempotency (safe to re-run).
-- ============================================================

-- ── A. field_jobs: dispatch planning fields ───────────────────────────────────
-- job_type: categorises the field visit so the mobile app can render the
--   correct form (METER_READING, ILLEGAL_CONNECTION, AUDIT_VERIFICATION, etc.)
-- scheduled_date: the calendar date the job is planned for; used for
--   ordering the officer's work queue and for SLA tracking.
-- instructions: free-text guidance from the supervisor to the field officer
--   (e.g. "Bring ladder – meter is on roof", "Customer speaks Twi only").
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS job_type       VARCHAR(50),
    ADD COLUMN IF NOT EXISTS scheduled_date DATE,
    ADD COLUMN IF NOT EXISTS instructions   TEXT;

-- Backfill job_type from the linked audit_event anomaly type where possible.
-- Jobs without an audit_event (direct dispatch) default to 'METER_READING'.
UPDATE field_jobs fj
SET job_type = COALESCE(
    (SELECT af.anomaly_type::text
     FROM audit_events ae
     JOIN anomaly_flags af ON af.id = ae.anomaly_flag_id
     WHERE ae.id = fj.audit_event_id
     LIMIT 1),
    'METER_READING'
)
WHERE fj.job_type IS NULL;

-- Backfill scheduled_date from created_at for existing jobs.
UPDATE field_jobs
SET scheduled_date = created_at::date
WHERE scheduled_date IS NULL;

-- Index for the mobile pull query: officer's pending jobs ordered by schedule.
CREATE INDEX IF NOT EXISTS idx_field_jobs_officer_schedule
    ON field_jobs(assigned_officer_id, scheduled_date)
    WHERE status NOT IN ('COMPLETED', 'CANCELLED', 'FAILED');

COMMENT ON COLUMN field_jobs.job_type IS
    'Type of field visit: METER_READING, ILLEGAL_CONNECTION, AUDIT_VERIFICATION, etc. '
    'Drives the mobile app form selection.';
COMMENT ON COLUMN field_jobs.scheduled_date IS
    'Calendar date the job is planned for. Used for officer work-queue ordering and SLA tracking.';
COMMENT ON COLUMN field_jobs.instructions IS
    'Free-text guidance from supervisor to field officer. Delivered offline to the mobile app.';


-- ── B. water_accounts: meter calibration factor ───────────────────────────────
-- calibration_factor: multiplier applied to raw meter readings to correct for
--   known meter drift or systematic under/over-registration. Default 1.0 = no
--   correction. Set by the Audit Manager after a certified meter test.
ALTER TABLE water_accounts
    ADD COLUMN IF NOT EXISTS calibration_factor NUMERIC(6,4) NOT NULL DEFAULT 1.0000;

COMMENT ON COLUMN water_accounts.calibration_factor IS
    'Meter calibration multiplier (default 1.0 = no correction). '
    'Applied by the mobile app and offline sync handler when recording field readings. '
    'Updated by Audit Manager after certified meter test.';


-- ── C. meter_readings: field officer submission columns ───────────────────────
-- adjusted_reading_m3: the calibration-corrected reading value stored alongside
--   the raw reading for full audit traceability.
-- calibration_factor_applied: the factor that was in effect at read time,
--   captured immutably so future calibration changes do not alter history.
-- ocr_reading_m3: the value the OCR engine extracted from the meter photo.
--   Stored separately from reading_m3 so discrepancies can be flagged.
ALTER TABLE meter_readings
    ADD COLUMN IF NOT EXISTS adjusted_reading_m3        NUMERIC(12,3),
    ADD COLUMN IF NOT EXISTS calibration_factor_applied NUMERIC(6,4),
    ADD COLUMN IF NOT EXISTS ocr_reading_m3             NUMERIC(12,3);

-- Backfill: for existing readings, adjusted = raw (calibration factor was 1.0)
UPDATE meter_readings
SET adjusted_reading_m3        = reading_m3,
    calibration_factor_applied = 1.0
WHERE adjusted_reading_m3 IS NULL;

COMMENT ON COLUMN meter_readings.adjusted_reading_m3 IS
    'Calibration-corrected reading (reading_m3 × calibration_factor_applied). '
    'Used by the sentinel for shadow-bill calculations.';
COMMENT ON COLUMN meter_readings.calibration_factor_applied IS
    'Calibration factor in effect at read time. Immutable after insert.';
COMMENT ON COLUMN meter_readings.ocr_reading_m3 IS
    'Value extracted by the OCR engine from the meter photo. '
    'NULL for manual / AMR / CDC readings. Compared to reading_m3 to detect discrepancies.';
