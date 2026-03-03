-- ============================================================
-- GN-WAAS Migration: 019_meter_readings_iot_columns
-- Description: Add IoT-specific columns to meter_readings that
--              the meter-ingestor service already writes to.
--
-- V19-DB-03 fix: meter-ingestor/internal/repository/meter_reading_repo.go
-- references battery_voltage and tamper_detected in its INSERT and SELECT
-- statements, but these columns were never defined in any migration.
-- Without this migration the meter-ingestor fails with:
--   ERROR: column "battery_voltage" of relation "meter_readings" does not exist
-- ============================================================

-- battery_voltage: IoT device battery level in Volts (0 if not applicable / non-smart meter)
ALTER TABLE meter_readings
    ADD COLUMN IF NOT EXISTS battery_voltage  NUMERIC(5,2) NOT NULL DEFAULT 0;

-- tamper_detected: flag set by smart meter firmware when physical tampering is detected
ALTER TABLE meter_readings
    ADD COLUMN IF NOT EXISTS tamper_detected  BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for tamper investigation queries (find all tampered readings in a district)
CREATE INDEX IF NOT EXISTS idx_meter_readings_tamper
    ON meter_readings(tamper_detected)
    WHERE tamper_detected = TRUE;

COMMENT ON COLUMN meter_readings.battery_voltage IS
    'IoT device battery voltage in Volts. 0 for non-smart / manual readings.';
COMMENT ON COLUMN meter_readings.tamper_detected IS
    'TRUE when the smart meter firmware detected physical tampering at read time.';
