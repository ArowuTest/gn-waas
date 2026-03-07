-- Migration 029: GRA retry queue, geocoding source, production-record ingestion tracking
-- Addresses:
--   Q1: GRA fallback states (PROVISIONAL, RETRY_QUEUED)
--   Q2: Fix gra_compliance_status enum
--   Q6: gps_source column for address-based geocoding fallback
--   Q8: production_record_source for software-only night-flow

-- ── Q1/Q2: Extend gra_compliance_status enum ─────────────────────────────────
-- Add PROVISIONAL (signed internally, pending GRA confirmation)
-- Add RETRY_QUEUED (failed, queued for automatic retry)
-- Add EXEMPT_MANUAL (System Admin override with documented reason)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumlabel = 'PROVISIONAL'
          AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'gra_compliance_status')
    ) THEN
        ALTER TYPE gra_compliance_status ADD VALUE 'PROVISIONAL';
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumlabel = 'RETRY_QUEUED'
          AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'gra_compliance_status')
    ) THEN
        ALTER TYPE gra_compliance_status ADD VALUE 'RETRY_QUEUED';
    END IF;
END$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum
        WHERE enumlabel = 'EXEMPT_MANUAL'
          AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'gra_compliance_status')
    ) THEN
        ALTER TYPE gra_compliance_status ADD VALUE 'EXEMPT_MANUAL';
    END IF;
END$$;

-- ── Q1: GRA retry tracking columns on audit_events ───────────────────────────
ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS gra_retry_count       SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS gra_last_attempt_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gra_retry_after       TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gra_failure_reason    TEXT,
    ADD COLUMN IF NOT EXISTS gra_exempt_reason     TEXT,
    ADD COLUMN IF NOT EXISTS gra_exempt_by         UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS gra_provisional_at    TIMESTAMPTZ;

COMMENT ON COLUMN audit_events.gra_retry_count     IS 'Number of GRA signing attempts made';
COMMENT ON COLUMN audit_events.gra_last_attempt_at IS 'Timestamp of last GRA signing attempt';
COMMENT ON COLUMN audit_events.gra_retry_after     IS 'Do not retry before this timestamp (exponential backoff)';
COMMENT ON COLUMN audit_events.gra_failure_reason  IS 'Last GRA API error message';
COMMENT ON COLUMN audit_events.gra_exempt_reason   IS 'Reason for EXEMPT_MANUAL override (System Admin documented)';
COMMENT ON COLUMN audit_events.gra_exempt_by       IS 'User who granted EXEMPT_MANUAL status';
COMMENT ON COLUMN audit_events.gra_provisional_at  IS 'When audit was marked PROVISIONAL (GRA down, internal QR generated)';

CREATE INDEX IF NOT EXISTS idx_audit_events_gra_retry
    ON audit_events(gra_status, gra_retry_after)
    WHERE gra_status IN ('FAILED', 'RETRY_QUEUED');

-- ── Q6: GPS source tracking on water_accounts ────────────────────────────────
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gps_source_type') THEN
        CREATE TYPE gps_source_type AS ENUM (
            'GWL_PROVIDED',      -- Coordinates came directly from GWL GIS export
            'GEOCODED_OSM',      -- Derived from address via OpenStreetMap Nominatim
            'GEOCODED_GOOGLE',   -- Derived from address via Google Maps API
            'FIELD_CONFIRMED',   -- Field officer confirmed/corrected on first visit
            'MANUAL_ADMIN',      -- Manually entered by System Admin
            'UNKNOWN'            -- Source not recorded (legacy data)
        );
    END IF;
END$$;

ALTER TABLE water_accounts
    ADD COLUMN IF NOT EXISTS gps_source          gps_source_type NOT NULL DEFAULT 'UNKNOWN',
    ADD COLUMN IF NOT EXISTS gps_geocode_quality NUMERIC(4,2),   -- 0-100 confidence score
    ADD COLUMN IF NOT EXISTS gps_geocoded_at     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gps_confirmed_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gps_confirmed_by    UUID REFERENCES users(id);

COMMENT ON COLUMN water_accounts.gps_source          IS 'How GPS coordinates were obtained';
COMMENT ON COLUMN water_accounts.gps_geocode_quality IS 'Geocoding confidence 0-100 (NULL if GWL-provided or field-confirmed)';
COMMENT ON COLUMN water_accounts.gps_geocoded_at     IS 'When geocoding was last attempted';
COMMENT ON COLUMN water_accounts.gps_confirmed_at    IS 'When field officer confirmed GPS on-site';

-- ── Q8: Production record ingestion source tracking ──────────────────────────
-- district_production_records tracks where production data came from
-- (GWL bulk meter API, file upload, or statistical estimate)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'production_data_source') THEN
        CREATE TYPE production_data_source AS ENUM (
            'GWL_BULK_METER_API',   -- Real-time from GWL existing bulk meters via API
            'GWL_FILE_UPLOAD',      -- Daily/monthly CSV file uploaded by GWL
            'CDC_REPLICA',          -- Derived from GWL billing DB replica
            'STATISTICAL_ESTIMATE', -- Estimated from billing records (Phase 1 fallback)
            'MANUAL_ENTRY'          -- Manually entered by Audit Manager
        );
    END IF;
END$$;

-- Add source column to district_production_records if it exists
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables
               WHERE table_name = 'district_production_records') THEN
        ALTER TABLE district_production_records
            ADD COLUMN IF NOT EXISTS data_source    production_data_source NOT NULL DEFAULT 'STATISTICAL_ESTIMATE',
            ADD COLUMN IF NOT EXISTS source_file_id UUID,
            ADD COLUMN IF NOT EXISTS data_confidence NUMERIC(4,2) NOT NULL DEFAULT 60.0;
        COMMENT ON COLUMN district_production_records.data_source
            IS 'How production volume data was obtained';
        COMMENT ON COLUMN district_production_records.data_confidence
            IS 'Data confidence score 0-100 for this production record';
    END IF;
END$$;

-- ── File upload tracking table (Q4 + Q5 + Q8) ────────────────────────────────
CREATE TABLE IF NOT EXISTS gwl_file_imports (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    import_type     VARCHAR(30) NOT NULL,
    -- 'ACCOUNTS', 'BILLING', 'METER_READINGS', 'PRODUCTION_RECORDS'
    filename        VARCHAR(255) NOT NULL,
    file_size_bytes BIGINT,
    uploaded_by     UUID REFERENCES users(id),
    period_month    DATE,           -- billing period this file covers (first day of month)
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    -- PENDING → PROCESSING → COMPLETED → FAILED → PARTIAL
    records_total   INTEGER NOT NULL DEFAULT 0,
    records_success INTEGER NOT NULL DEFAULT 0,
    records_failed  INTEGER NOT NULL DEFAULT 0,
    error_summary   JSONB,          -- array of {row, error} for failed rows
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gwl_file_imports_type   ON gwl_file_imports(import_type);
CREATE INDEX IF NOT EXISTS idx_gwl_file_imports_status ON gwl_file_imports(status);
CREATE INDEX IF NOT EXISTS idx_gwl_file_imports_period ON gwl_file_imports(period_month);

COMMENT ON TABLE gwl_file_imports IS
    'Tracks every GWL data file upload (accounts, billing, meter readings, production records). '
    'Used when GWL cannot provide a live database replica or API.';

