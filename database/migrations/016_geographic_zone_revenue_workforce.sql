-- ============================================================
-- Migration 016: Geographic Zone, Revenue Recovery & Workforce Tracking
-- ============================================================
-- Adds three capabilities missing from the initial schema:
--
-- 1. geographic_zone on districts — the URBAN/PERI_URBAN/RURAL/INDUSTRIAL
--    classification for tariff defaults, field routing, and security escort
--    decisions. Distinct from zone_type (NRW heatmap risk colour).
--
-- 2. revenue_recovery_events — tracks every GHS amount recovered as a
--    direct result of a GN-WAAS audit flag, enabling the 3% success-fee
--    calculation required by the managed-service monetisation model.
--
-- 3. officer_gps_tracks — stores GPS breadcrumbs for field officers so
--    the Admin Portal "Workforce Oversight" view can verify officers are
--    physically visiting flagged properties (anti-desk-audit control).
-- ============================================================

-- ── 1. Geographic zone enum ───────────────────────────────────────────────
CREATE TYPE geographic_zone_type AS ENUM (
    'URBAN',
    'PERI_URBAN',
    'RURAL',
    'INDUSTRIAL'
);

-- ── 2. Add geographic_zone to districts ──────────────────────────────────
ALTER TABLE districts
    ADD COLUMN IF NOT EXISTS geographic_zone geographic_zone_type NOT NULL DEFAULT 'URBAN';

COMMENT ON COLUMN districts.geographic_zone IS
    'Geographic/demographic classification of the DMA. Used for tariff-category '
    'defaults, field-officer routing, and security-escort decisions. '
    'Distinct from zone_type which is the NRW heatmap risk colour (RED/YELLOW/GREEN/GREY).';

-- ── 3. Revenue recovery events ───────────────────────────────────────────
CREATE TABLE IF NOT EXISTS revenue_recovery_events (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    audit_event_id          UUID NOT NULL REFERENCES audit_events(id),
    anomaly_flag_id         UUID REFERENCES anomaly_flags(id),
    district_id             UUID NOT NULL REFERENCES districts(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    -- Financial amounts
    variance_ghs            NUMERIC(15,2) NOT NULL,   -- shadow_bill - gwl_billed
    recovered_ghs           NUMERIC(15,2) NOT NULL,   -- amount actually collected
    success_fee_ghs         NUMERIC(15,2) GENERATED ALWAYS AS (recovered_ghs * 0.03) STORED,
    -- Classification
    recovery_type           VARCHAR(50) NOT NULL DEFAULT 'UNDERBILLING',
    -- e.g. UNDERBILLING, GHOST_ACCOUNT, PHANTOM_METER, OUTAGE_BILLING
    -- Status
    status                  VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    -- PENDING → CONFIRMED → COLLECTED → DISPUTED
    confirmed_at            TIMESTAMPTZ,
    collected_at            TIMESTAMPTZ,
    confirmed_by            UUID REFERENCES users(id),
    notes                   TEXT,
    -- Immutability
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_revenue_recovery_district  ON revenue_recovery_events(district_id);
CREATE INDEX idx_revenue_recovery_account   ON revenue_recovery_events(account_id);
CREATE INDEX idx_revenue_recovery_status    ON revenue_recovery_events(status);
CREATE INDEX idx_revenue_recovery_created   ON revenue_recovery_events(created_at DESC);

COMMENT ON TABLE revenue_recovery_events IS
    'Each row represents one revenue recovery event triggered by a GN-WAAS audit. '
    'success_fee_ghs is auto-calculated as 3% of recovered_ghs (managed-service model).';

-- ── 4. Officer GPS tracks (workforce oversight) ──────────────────────────
CREATE TABLE IF NOT EXISTS officer_gps_tracks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    officer_id      UUID NOT NULL REFERENCES users(id),
    field_job_id    UUID REFERENCES field_jobs(id),   -- NULL = idle/patrol
    latitude        NUMERIC(10,8) NOT NULL,
    longitude       NUMERIC(11,8) NOT NULL,
    accuracy_m      NUMERIC(6,2),                     -- GPS accuracy in metres
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    device_id       VARCHAR(100)                      -- device fingerprint
);

-- DB-H01 fix: wrap in DO block so migration succeeds even without TimescaleDB
DO $hyper$ BEGIN
    PERFORM create_hypertable('officer_gps_tracks', 'recorded_at', if_not_exists => TRUE);
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'TimescaleDB not available — officer_gps_tracks will be a plain table: %', SQLERRM;
END $hyper$;

CREATE INDEX idx_gps_tracks_officer  ON officer_gps_tracks(officer_id, recorded_at DESC);
CREATE INDEX idx_gps_tracks_job      ON officer_gps_tracks(field_job_id) WHERE field_job_id IS NOT NULL;

COMMENT ON TABLE officer_gps_tracks IS
    'GPS breadcrumb trail for field officers. Enables the Admin Portal Workforce '
    'Oversight view to verify officers physically visit flagged properties. '
    'Stored as a TimescaleDB hypertable for efficient time-range queries.';

-- ── 5. Seed geographic_zone on existing districts ────────────────────────
-- Accra/Tema are urban; northern/rural districts are rural; industrial zones
UPDATE districts SET geographic_zone = 'URBAN'      WHERE district_name ILIKE '%accra%' OR district_name ILIKE '%tema%' OR district_name ILIKE '%kumasi%';
UPDATE districts SET geographic_zone = 'INDUSTRIAL' WHERE district_name ILIKE '%industrial%' OR district_name ILIKE '%harbour%';
UPDATE districts SET geographic_zone = 'RURAL'      WHERE district_name ILIKE '%north%' OR district_name ILIKE '%savannah%' OR district_name ILIKE '%volta%';
UPDATE districts SET geographic_zone = 'PERI_URBAN' WHERE geographic_zone = 'URBAN' AND district_name NOT ILIKE '%accra%' AND district_name NOT ILIKE '%tema%' AND district_name NOT ILIKE '%kumasi%';
