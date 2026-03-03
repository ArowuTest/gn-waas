-- ============================================================
-- GN-WAAS Migration: 014_meter_readings_and_gwl_bills
-- Description: Create meter_readings and gwl_bills tables that
--              are referenced by seed scripts but were missing
--              from the original migration set.
--
-- meter_readings — physical meter read events (OCR + manual)
-- gwl_bills      — GWL billing records mirrored from CDC
-- ============================================================

BEGIN;

-- ── meter_readings ────────────────────────────────────────────────────────
-- One row per meter read event. Populated by:
--   • Field officer OCR capture (mobile app)
--   • Manual entry by GWL billing staff
--   • CDC ingestor (bulk import from GWL ERP)
CREATE TABLE IF NOT EXISTS meter_readings (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID NOT NULL REFERENCES water_accounts(id) ON DELETE CASCADE,
    reading_date        DATE NOT NULL,
    reading_m3          NUMERIC(12,3) NOT NULL,          -- Cumulative meter reading
    consumption_m3      NUMERIC(12,3),                   -- Delta from previous reading
    flow_rate_m3h       NUMERIC(10,4),                   -- Instantaneous flow at read time
    pressure_bar        NUMERIC(6,3),                    -- Line pressure at read time
    read_method         VARCHAR(30) NOT NULL DEFAULT 'MANUAL',  -- OCR | MANUAL | AMR | CDC
    reader_id           UUID REFERENCES users(id),       -- NULL = automated / CDC
    ocr_confidence      NUMERIC(5,2),                    -- 0-100 OCR confidence score
    photo_hash          VARCHAR(64),                     -- SHA-256 of meter photo
    photo_url           TEXT,                            -- Object-store URL
    gps_latitude        NUMERIC(10,8),
    gps_longitude       NUMERIC(11,8),
    gps_precision_m     NUMERIC(6,2),
    device_id           VARCHAR(100),
    notes               TEXT,
    is_estimated        BOOLEAN NOT NULL DEFAULT FALSE,  -- TRUE = interpolated / estimated
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT meter_readings_account_date_unique UNIQUE (account_id, reading_date)
);

CREATE INDEX IF NOT EXISTS idx_meter_readings_account   ON meter_readings(account_id);
CREATE INDEX IF NOT EXISTS idx_meter_readings_date      ON meter_readings(reading_date DESC);
CREATE INDEX IF NOT EXISTS idx_meter_readings_reader    ON meter_readings(reader_id) WHERE reader_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_meter_readings_method    ON meter_readings(read_method);

COMMENT ON TABLE meter_readings IS
    'Physical meter read events. Source of truth for consumption calculations.';

-- ── gwl_bills ─────────────────────────────────────────────────────────────
-- GWL billing records mirrored from the GWL ERP via CDC ingestor.
-- Used by the sentinel to compute shadow bills and detect billing variance.
CREATE TABLE IF NOT EXISTS gwl_bills (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id              UUID NOT NULL REFERENCES water_accounts(id) ON DELETE CASCADE,
    gwl_bill_id             VARCHAR(100) NOT NULL UNIQUE,  -- GWL's own bill reference
    billing_period_start    DATE NOT NULL,
    billing_period_end      DATE NOT NULL,

    -- Meter readings at billing time
    previous_reading_m3     NUMERIC(12,3) NOT NULL DEFAULT 0,
    current_reading_m3      NUMERIC(12,3) NOT NULL DEFAULT 0,
    consumption_m3          NUMERIC(12,3) NOT NULL DEFAULT 0,

    -- GWL billing amounts (what GWL actually charged)
    gwl_category            account_category NOT NULL,
    gwl_amount_ghs          NUMERIC(12,2) NOT NULL DEFAULT 0,  -- Pre-VAT
    gwl_vat_ghs             NUMERIC(12,2) NOT NULL DEFAULT 0,
    gwl_total_ghs           NUMERIC(12,2) NOT NULL DEFAULT 0,  -- Total billed

    -- Shadow bill (PURC tariff recalculation by sentinel)
    shadow_amount_ghs       NUMERIC(12,2),
    shadow_vat_ghs          NUMERIC(12,2),
    shadow_total_ghs        NUMERIC(12,2),
    variance_pct            NUMERIC(8,4),                      -- (shadow-gwl)/gwl * 100
    variance_flag           BOOLEAN NOT NULL DEFAULT FALSE,    -- TRUE if |variance| > threshold

    -- Read metadata
    gwl_reader_id           UUID REFERENCES users(id),
    gwl_read_date           DATE,
    gwl_read_method         VARCHAR(30),

    -- Payment status
    payment_status          VARCHAR(20) NOT NULL DEFAULT 'UNPAID',  -- UNPAID|PAID|PARTIAL|WAIVED
    payment_date            DATE,
    payment_amount_ghs      NUMERIC(12,2) NOT NULL DEFAULT 0,

    -- Audit trail
    cdc_synced_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gwl_bills_account        ON gwl_bills(account_id);
CREATE INDEX IF NOT EXISTS idx_gwl_bills_period         ON gwl_bills(billing_period_start DESC);
CREATE INDEX IF NOT EXISTS idx_gwl_bills_variance_flag  ON gwl_bills(variance_flag) WHERE variance_flag = TRUE;
CREATE INDEX IF NOT EXISTS idx_gwl_bills_payment_status ON gwl_bills(payment_status);

COMMENT ON TABLE gwl_bills IS
    'GWL billing records mirrored from ERP via CDC. Used for shadow-bill variance detection.';

-- ── illegal_connections: add photo_hashes column (missing from migration 013) ─
-- The audit_event_repo.go CreateIllegalConnection INSERT includes photo_hashes
-- for chain-of-custody verification but the column was omitted from migration 013.
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN illegal_connections.photo_hashes IS
    'SHA-256 hashes of each photo, computed on device at capture time. '
    'Used for server-side chain-of-custody verification.';

COMMIT;
