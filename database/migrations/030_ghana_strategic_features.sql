-- Migration 030: Ghana-specific strategic features
-- Addresses real-world African/Ghana operational realities:
--
--  A. Mobile Money (MoMo) payment reconciliation — MTN MoMo, Vodafone Cash, AirtelTigo
--  B. SMS notification tracking — Hubtel/Arkesel gateway records
--  C. Compound house / shared meter support — common in Ghanaian urban areas
--  D. Meter calibration & age tracking — old meters under-read (major NRW source)
--  E. Whistleblower / anonymous tip system — personnel collusion detection
--  F. Donor / development partner KPI tracking — World Bank, AfDB reporting
--  G. Seasonal NRW threshold configuration — Ghana rainy seasons (Apr-Jul, Sep-Nov)
--  H. Ghana Card (NIA) number on accounts — ghost account detection
--  I. Offline sync queue — field officers in low-connectivity areas
--  J. Audit evidence package tracking — legal proceedings support
--  K. Load shedding / ECG outage correlation — distinguish power-cut from theft
--  L. Exchange rate tracking — GHS/USD for donor reporting

-- ═══════════════════════════════════════════════════════════════════════════════
-- A. MOBILE MONEY PAYMENT RECONCILIATION
-- ═══════════════════════════════════════════════════════════════════════════════

-- MoMo provider enum
DO $$ BEGIN
    CREATE TYPE momo_provider AS ENUM (
        'MTN_MOMO',
        'VODAFONE_CASH',
        'AIRTELTIGO_MONEY',
        'ZEEPAY',
        'G_MONEY',
        'UNKNOWN'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- MoMo payment reconciliation status
DO $$ BEGIN
    CREATE TYPE momo_reconciliation_status AS ENUM (
        'MATCHED',          -- payment matched to a GWL bill
        'UNMATCHED',        -- payment received but no matching bill found
        'OVERPAYMENT',      -- payment > bill amount (possible fraud signal)
        'UNDERPAYMENT',     -- payment < bill amount
        'DUPLICATE',        -- same transaction ID seen twice
        'GHOST_ACCOUNT',    -- payment for account that doesn't exist in GWL
        'PENDING'           -- not yet reconciled
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS mobile_money_payments (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Raw transaction data from MoMo provider export
    transaction_id          VARCHAR(100) NOT NULL,          -- MoMo transaction reference
    provider                momo_provider NOT NULL,
    sender_phone            VARCHAR(20),                    -- customer's MoMo number
    sender_name             VARCHAR(200),
    amount_ghs              NUMERIC(12,2) NOT NULL,
    transaction_date        TIMESTAMPTZ NOT NULL,
    narration               TEXT,                           -- free-text from MoMo export
    -- GWL account matching
    gwl_account_number      VARCHAR(50),                    -- extracted from narration or reference
    account_id              UUID REFERENCES water_accounts(id),
    gwl_bill_id             UUID,                           -- matched bill (FK added below)
    -- Reconciliation result
    reconciliation_status   momo_reconciliation_status NOT NULL DEFAULT 'PENDING',
    variance_ghs            NUMERIC(12,2),                  -- payment - bill amount
    reconciled_at           TIMESTAMPTZ,
    reconciled_by           UUID REFERENCES users(id),
    -- Fraud flags
    is_fraud_flag           BOOLEAN NOT NULL DEFAULT FALSE,
    fraud_reason            TEXT,
    -- Import tracking
    import_batch_id         UUID,                           -- links to gwl_file_imports
    raw_row                 JSONB,                          -- original CSV row for audit
    -- Timestamps
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, transaction_id)
);

CREATE INDEX IF NOT EXISTS idx_momo_payments_account ON mobile_money_payments(account_id);
CREATE INDEX IF NOT EXISTS idx_momo_payments_status ON mobile_money_payments(reconciliation_status);
CREATE INDEX IF NOT EXISTS idx_momo_payments_date ON mobile_money_payments(transaction_date);
CREATE INDEX IF NOT EXISTS idx_momo_payments_phone ON mobile_money_payments(sender_phone);
CREATE INDEX IF NOT EXISTS idx_momo_payments_fraud ON mobile_money_payments(is_fraud_flag) WHERE is_fraud_flag = TRUE;

-- MoMo reconciliation summary per billing period
CREATE TABLE IF NOT EXISTS momo_reconciliation_summary (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    district_id             UUID NOT NULL REFERENCES districts(id),
    period_start            DATE NOT NULL,
    period_end              DATE NOT NULL,
    total_payments          INTEGER NOT NULL DEFAULT 0,
    total_amount_ghs        NUMERIC(14,2) NOT NULL DEFAULT 0,
    matched_count           INTEGER NOT NULL DEFAULT 0,
    matched_amount_ghs      NUMERIC(14,2) NOT NULL DEFAULT 0,
    unmatched_count         INTEGER NOT NULL DEFAULT 0,
    unmatched_amount_ghs    NUMERIC(14,2) NOT NULL DEFAULT 0,
    ghost_account_count     INTEGER NOT NULL DEFAULT 0,
    ghost_account_amount_ghs NUMERIC(14,2) NOT NULL DEFAULT 0,
    overpayment_count       INTEGER NOT NULL DEFAULT 0,
    fraud_flag_count        INTEGER NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(district_id, period_start)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- B. SMS NOTIFICATION TRACKING
-- ═══════════════════════════════════════════════════════════════════════════════

DO $$ BEGIN
    CREATE TYPE sms_provider AS ENUM ('HUBTEL', 'ARKESEL', 'WIGAL', 'TWILIO');
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    CREATE TYPE sms_status AS ENUM ('QUEUED', 'SENT', 'DELIVERED', 'FAILED', 'BOUNCED');
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    CREATE TYPE sms_template AS ENUM (
        'JOB_ASSIGNED',         -- field officer: new job assigned
        'JOB_REMINDER',         -- field officer: job due today
        'AUDIT_COMPLETE',       -- customer: audit completed, reference number
        'PAYMENT_RECEIPT',      -- customer: payment received
        'ANOMALY_ALERT',        -- supervisor: anomaly flagged in district
        'GRA_SIGNED',           -- audit manager: GRA signing confirmed
        'PROVISIONAL_RECEIPT',  -- customer: provisional receipt (GRA pending)
        'CUSTOM'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS sms_notifications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_phone     VARCHAR(20) NOT NULL,
    recipient_name      VARCHAR(200),
    recipient_user_id   UUID REFERENCES users(id),
    template            sms_template NOT NULL DEFAULT 'CUSTOM',
    message_body        TEXT NOT NULL,
    -- Delivery tracking
    provider            sms_provider NOT NULL DEFAULT 'HUBTEL',
    provider_message_id VARCHAR(200),                       -- provider's message ID for status polling
    status              sms_status NOT NULL DEFAULT 'QUEUED',
    sent_at             TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    failed_at           TIMESTAMPTZ,
    failure_reason      TEXT,
    retry_count         SMALLINT NOT NULL DEFAULT 0,
    -- Context
    related_entity_type VARCHAR(50),                        -- 'audit_event', 'field_job', 'anomaly'
    related_entity_id   UUID,
    district_id         UUID REFERENCES districts(id),
    -- Cost tracking (Hubtel charges per SMS)
    cost_ghs            NUMERIC(8,4),
    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_status ON sms_notifications(status);
CREATE INDEX IF NOT EXISTS idx_sms_phone ON sms_notifications(recipient_phone);
CREATE INDEX IF NOT EXISTS idx_sms_entity ON sms_notifications(related_entity_type, related_entity_id);
CREATE INDEX IF NOT EXISTS idx_sms_created ON sms_notifications(created_at);

-- ═══════════════════════════════════════════════════════════════════════════════
-- C. COMPOUND HOUSE / SHARED METER SUPPORT
-- ═══════════════════════════════════════════════════════════════════════════════
-- In Ghana, compound houses share a single meter. Multiple households
-- (sub-accounts) are billed pro-rata based on household size or fixed split.

DO $$ BEGIN
    CREATE TYPE compound_split_method AS ENUM (
        'EQUAL',            -- divide equally among sub-accounts
        'HOUSEHOLD_SIZE',   -- pro-rata by number of people
        'FIXED_FRACTION',   -- each sub-account has a fixed % share
        'METERED'           -- each sub-account has its own sub-meter
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS compound_house_groups (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_name          VARCHAR(200) NOT NULL,
    master_account_id   UUID NOT NULL REFERENCES water_accounts(id),  -- the metered account
    district_id         UUID NOT NULL REFERENCES districts(id),
    split_method        compound_split_method NOT NULL DEFAULT 'EQUAL',
    total_households    SMALLINT NOT NULL DEFAULT 1,
    address_line1       VARCHAR(300),
    address_line2       VARCHAR(300),
    gps_latitude        NUMERIC(10,7),
    gps_longitude       NUMERIC(10,7),
    notes               TEXT,
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS compound_house_members (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id            UUID NOT NULL REFERENCES compound_house_groups(id) ON DELETE CASCADE,
    account_id          UUID NOT NULL REFERENCES water_accounts(id),
    household_size      SMALLINT NOT NULL DEFAULT 1,        -- number of people (for HOUSEHOLD_SIZE split)
    fixed_fraction      NUMERIC(5,4),                       -- 0.0000-1.0000 (for FIXED_FRACTION split)
    sub_meter_number    VARCHAR(50),                        -- if METERED split
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    joined_at           DATE NOT NULL DEFAULT CURRENT_DATE,
    left_at             DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_compound_master ON compound_house_groups(master_account_id);
CREATE INDEX IF NOT EXISTS idx_compound_members_group ON compound_house_members(group_id);
CREATE INDEX IF NOT EXISTS idx_compound_members_account ON compound_house_members(account_id);

-- Add compound house flag to water_accounts
ALTER TABLE water_accounts
    ADD COLUMN IF NOT EXISTS is_compound_master   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS compound_group_id    UUID REFERENCES compound_house_groups(id),
    ADD COLUMN IF NOT EXISTS ghana_card_number     VARCHAR(20),        -- NIA Ghana Card number
    ADD COLUMN IF NOT EXISTS phone_number          VARCHAR(20),        -- for SMS notifications
    ADD COLUMN IF NOT EXISTS momo_number           VARCHAR(20),        -- MoMo wallet number
    ADD COLUMN IF NOT EXISTS momo_provider         momo_provider;

CREATE INDEX IF NOT EXISTS idx_accounts_ghana_card ON water_accounts(ghana_card_number) WHERE ghana_card_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_phone ON water_accounts(phone_number) WHERE phone_number IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_momo ON water_accounts(momo_number) WHERE momo_number IS NOT NULL;

-- ═══════════════════════════════════════════════════════════════════════════════
-- D. METER CALIBRATION & AGE TRACKING
-- ═══════════════════════════════════════════════════════════════════════════════
-- Old meters under-read — a major source of apparent NRW in Ghana.
-- Calibration factor corrects raw readings before billing.

DO $$ BEGIN
    CREATE TYPE meter_condition AS ENUM (
        'GOOD',
        'FAIR',             -- minor wear, calibration factor applied
        'POOR',             -- significant under-reading, replacement recommended
        'FAULTY',           -- not reading correctly, flagged for immediate replacement
        'REPLACED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS meter_calibrations (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    meter_serial_number     VARCHAR(100),
    meter_model             VARCHAR(100),
    meter_install_date      DATE,
    meter_age_years         NUMERIC(4,1) GENERATED ALWAYS AS (
                                EXTRACT(YEAR FROM AGE(CURRENT_DATE, meter_install_date))
                            ) STORED,
    calibration_date        DATE NOT NULL DEFAULT CURRENT_DATE,
    calibration_factor      NUMERIC(6,4) NOT NULL DEFAULT 1.0000,
                            -- factor > 1.0 means meter under-reads (multiply reading by factor)
                            -- e.g. 1.05 = meter reads 5% low
    previous_factor         NUMERIC(6,4),
    condition               meter_condition NOT NULL DEFAULT 'GOOD',
    calibrated_by           UUID REFERENCES users(id),
    calibration_method      VARCHAR(100),                   -- 'FIELD_TEST', 'LAB_TEST', 'STATISTICAL'
    next_calibration_due    DATE,
    notes                   TEXT,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,  -- only one active calibration per account
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calibrations_account ON meter_calibrations(account_id);
CREATE INDEX IF NOT EXISTS idx_calibrations_due ON meter_calibrations(next_calibration_due) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_calibrations_condition ON meter_calibrations(condition);

-- Add meter info to water_accounts
ALTER TABLE water_accounts
    ADD COLUMN IF NOT EXISTS meter_serial_number    VARCHAR(100),
    ADD COLUMN IF NOT EXISTS meter_install_date     DATE,
    ADD COLUMN IF NOT EXISTS active_calibration_id  UUID REFERENCES meter_calibrations(id),
    ADD COLUMN IF NOT EXISTS calibration_factor     NUMERIC(6,4) NOT NULL DEFAULT 1.0000;

-- ═══════════════════════════════════════════════════════════════════════════════
-- E. WHISTLEBLOWER / ANONYMOUS TIP SYSTEM
-- ═══════════════════════════════════════════════════════════════════════════════

DO $$ BEGIN
    CREATE TYPE tip_category AS ENUM (
        'GHOST_ACCOUNT',        -- account exists in GWL but no physical meter/customer
        'PHANTOM_METER',        -- meter number doesn't correspond to real installation
        'BILLING_MANIPULATION', -- GWL staff manipulating bills
        'METER_TAMPERING',      -- physical meter bypass or tampering
        'COLLUSION',            -- GWL staff + customer collusion
        'ILLEGAL_CONNECTION',   -- unauthorised water connection
        'FIELD_OFFICER_FRAUD',  -- field officer accepting bribes
        'OTHER'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    CREATE TYPE tip_status AS ENUM (
        'NEW',
        'UNDER_REVIEW',
        'INVESTIGATION_OPENED',
        'SUBSTANTIATED',        -- tip led to confirmed fraud finding
        'UNSUBSTANTIATED',      -- investigated, no fraud found
        'DUPLICATE',
        'CLOSED'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS whistleblower_tips (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Anonymous submission — no user ID stored
    tip_reference           VARCHAR(20) NOT NULL UNIQUE,    -- e.g. TIP-2026-001234 (given to tipster)
    category                tip_category NOT NULL,
    district_id             UUID REFERENCES districts(id),  -- optional
    gwl_account_number      VARCHAR(50),                    -- optional
    description             TEXT NOT NULL,
    -- Evidence (optional)
    photo_urls              TEXT[],                         -- S3/MinIO URLs
    location_lat            NUMERIC(10,7),
    location_lng            NUMERIC(10,7),
    -- Contact (optional — tipster may choose to provide for reward)
    contact_phone           VARCHAR(20),                    -- encrypted at rest
    contact_method          VARCHAR(20),                    -- 'SMS', 'PHONE', 'NONE'
    -- Investigation
    status                  tip_status NOT NULL DEFAULT 'NEW',
    assigned_to             UUID REFERENCES users(id),
    investigation_notes     TEXT,
    linked_audit_event_id   UUID REFERENCES audit_events(id),
    -- Reward tracking (3% of recovered amount if tip leads to recovery)
    reward_eligible         BOOLEAN NOT NULL DEFAULT FALSE,
    reward_amount_ghs       NUMERIC(12,2),
    reward_paid_at          TIMESTAMPTZ,
    -- Metadata
    submission_ip_hash      VARCHAR(64),                    -- SHA-256 of IP (not IP itself)
    user_agent_hash         VARCHAR(64),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tips_status ON whistleblower_tips(status);
CREATE INDEX IF NOT EXISTS idx_tips_category ON whistleblower_tips(category);
CREATE INDEX IF NOT EXISTS idx_tips_district ON whistleblower_tips(district_id);
CREATE INDEX IF NOT EXISTS idx_tips_reference ON whistleblower_tips(tip_reference);

-- ═══════════════════════════════════════════════════════════════════════════════
-- F. DONOR / DEVELOPMENT PARTNER KPI TRACKING
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS donor_kpi_snapshots (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_date               DATE NOT NULL,
    period_label                VARCHAR(20) NOT NULL,       -- e.g. 'Q1-2026', 'FY2026'
    -- IWA/AWWA Water Balance KPIs
    system_input_volume_m3      NUMERIC(16,2),              -- total water produced
    authorized_consumption_m3   NUMERIC(16,2),              -- billed + unbilled authorised
    water_losses_m3             NUMERIC(16,2),              -- real + apparent losses
    real_losses_m3              NUMERIC(16,2),              -- physical leakage
    apparent_losses_m3          NUMERIC(16,2),              -- commercial losses (theft + errors)
    nrw_percentage              NUMERIC(5,2),               -- NRW %
    nrw_target_percentage       NUMERIC(5,2) DEFAULT 20.0,  -- target (typically 20%)
    -- Revenue KPIs
    revenue_gap_identified_ghs  NUMERIC(16,2),
    revenue_recovered_ghs       NUMERIC(16,2),
    recovery_rate_pct           NUMERIC(5,2),
    -- Audit KPIs
    audits_completed            INTEGER,
    audits_gra_signed           INTEGER,
    field_jobs_completed        INTEGER,
    -- Fraud KPIs
    ghost_accounts_identified   INTEGER,
    phantom_meters_identified   INTEGER,
    illegal_connections_found   INTEGER,
    -- USD equivalent (for donor reporting)
    usd_exchange_rate           NUMERIC(10,4),              -- GHS per USD at snapshot date
    revenue_gap_usd             NUMERIC(16,2),
    revenue_recovered_usd       NUMERIC(16,2),
    -- Disbursement-Linked Indicators (DLIs) — World Bank / AfDB
    dli_nrw_reduction_pct       NUMERIC(5,2),               -- NRW reduction vs baseline
    dli_audits_target           INTEGER,
    dli_audits_achieved         INTEGER,
    dli_recovery_target_ghs     NUMERIC(16,2),
    dli_recovery_achieved_ghs   NUMERIC(16,2),
    -- Metadata
    generated_by                UUID REFERENCES users(id),
    notes                       TEXT,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(snapshot_date, period_label)
);

CREATE INDEX IF NOT EXISTS idx_donor_kpi_date ON donor_kpi_snapshots(snapshot_date);

-- ═══════════════════════════════════════════════════════════════════════════════
-- G. SEASONAL NRW THRESHOLD CONFIGURATION
-- ═══════════════════════════════════════════════════════════════════════════════
-- Ghana has two rainy seasons: major (Apr-Jul) and minor (Sep-Nov).
-- During rainy season, groundwater infiltration increases apparent production,
-- and consumption patterns change. Thresholds should be seasonally adjusted.

DO $$ BEGIN
    CREATE TYPE ghana_season AS ENUM (
        'DRY_HARMATTAN',    -- Dec-Mar: dry, dusty, high consumption
        'MAJOR_RAINY',      -- Apr-Jul: heavy rain, infiltration risk
        'DRY_INTERLUDE',    -- Aug: brief dry spell
        'MINOR_RAINY',      -- Sep-Nov: lighter rain
        'CUSTOM'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS seasonal_threshold_config (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    season                      ghana_season NOT NULL,
    month_start                 SMALLINT NOT NULL,          -- 1-12
    month_end                   SMALLINT NOT NULL,          -- 1-12
    -- Variance threshold adjustments (additive to base threshold)
    variance_threshold_adj_pct  NUMERIC(5,2) NOT NULL DEFAULT 0,
                                -- e.g. +5 during rainy season (more tolerance)
    -- Night-flow adjustments
    night_flow_baseline_adj_m3  NUMERIC(8,2) NOT NULL DEFAULT 0,
                                -- additional baseline night flow during rainy season
    night_flow_threshold_adj_pct NUMERIC(5,2) NOT NULL DEFAULT 0,
    -- Production record adjustments
    infiltration_factor         NUMERIC(5,4) NOT NULL DEFAULT 0,
                                -- fraction of production to subtract as infiltration
    -- Consumption adjustments
    consumption_multiplier      NUMERIC(5,4) NOT NULL DEFAULT 1.0,
                                -- expected consumption relative to dry season
    -- Metadata
    description                 TEXT,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    effective_from              DATE NOT NULL DEFAULT CURRENT_DATE,
    created_by                  UUID REFERENCES users(id),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default Ghana seasonal config
INSERT INTO seasonal_threshold_config
    (season, month_start, month_end, variance_threshold_adj_pct,
     night_flow_baseline_adj_m3, night_flow_threshold_adj_pct,
     infiltration_factor, consumption_multiplier, description)
VALUES
    ('DRY_HARMATTAN', 12, 3,  0,    0,    0,    0.000, 1.10,
     'Dec-Mar: dry harmattan, higher consumption, tighter thresholds'),
    ('MAJOR_RAINY',   4,  7,  5,    2.5,  10,   0.030, 0.90,
     'Apr-Jul: major rainy season, infiltration risk, relaxed thresholds'),
    ('DRY_INTERLUDE', 8,  8,  0,    0,    0,    0.005, 1.00,
     'Aug: brief dry interlude'),
    ('MINOR_RAINY',   9,  11, 3,    1.5,  5,    0.015, 0.95,
     'Sep-Nov: minor rainy season, moderate adjustment')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════════
-- H. OFFLINE SYNC QUEUE
-- ═══════════════════════════════════════════════════════════════════════════════
-- Field officers work in areas with poor connectivity (rural Ghana).
-- Actions are queued locally and synced when connectivity is restored.

DO $$ BEGIN
    CREATE TYPE sync_action_type AS ENUM (
        'METER_READING',
        'FIELD_JOB_UPDATE',
        'GPS_CONFIRM',
        'PHOTO_UPLOAD',
        'AUDIT_SIGNATURE',
        'ANOMALY_REPORT',
        'TIP_SUBMISSION'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    CREATE TYPE sync_status AS ENUM (
        'PENDING',
        'PROCESSING',
        'APPLIED',
        'CONFLICT',         -- server state changed since offline action was queued
        'REJECTED',         -- action rejected (validation failed)
        'SUPERSEDED'        -- newer action for same entity already applied
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

CREATE TABLE IF NOT EXISTS offline_sync_queue (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id           VARCHAR(100) NOT NULL,              -- field officer's device ID
    user_id             UUID NOT NULL REFERENCES users(id),
    action_type         sync_action_type NOT NULL,
    entity_type         VARCHAR(50) NOT NULL,               -- 'field_job', 'water_account', etc.
    entity_id           UUID NOT NULL,
    payload             JSONB NOT NULL,                     -- the action data
    client_timestamp    TIMESTAMPTZ NOT NULL,               -- when action was taken offline
    client_sequence     BIGINT NOT NULL,                    -- monotonic sequence from device
    -- Server processing
    status              sync_status NOT NULL DEFAULT 'PENDING',
    processed_at        TIMESTAMPTZ,
    conflict_details    JSONB,                              -- what conflicted
    rejection_reason    TEXT,
    -- Network info
    sync_batch_id       UUID,                               -- groups actions from one sync session
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_queue_user ON offline_sync_queue(user_id, status);
CREATE INDEX IF NOT EXISTS idx_sync_queue_device ON offline_sync_queue(device_id);
CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON offline_sync_queue(status);
CREATE INDEX IF NOT EXISTS idx_sync_queue_entity ON offline_sync_queue(entity_type, entity_id);

-- ═══════════════════════════════════════════════════════════════════════════════
-- I. AUDIT EVIDENCE PACKAGE
-- ═══════════════════════════════════════════════════════════════════════════════
-- For legal proceedings, all evidence must be packaged with chain of custody.

CREATE TABLE IF NOT EXISTS audit_evidence_packages (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    audit_event_id          UUID NOT NULL REFERENCES audit_events(id),
    package_reference       VARCHAR(50) NOT NULL UNIQUE,    -- e.g. EVP-2026-001234
    -- Evidence items
    meter_photos            TEXT[],                         -- S3 URLs
    gps_coordinates         JSONB,                          -- {lat, lng, accuracy, timestamp}
    ocr_readings            JSONB,                          -- OCR results with confidence
    field_officer_signature TEXT,                           -- base64 signature
    customer_signature      TEXT,                           -- base64 signature (if obtained)
    witness_names           TEXT[],
    -- GRA evidence
    gra_qr_code_url         TEXT,
    gra_sdc_id              VARCHAR(100),
    gra_signed_at           TIMESTAMPTZ,
    -- Package integrity
    package_hash            VARCHAR(64),                    -- SHA-256 of all evidence
    hash_algorithm          VARCHAR(20) DEFAULT 'SHA-256',
    sealed_at               TIMESTAMPTZ,                    -- when package was finalised
    sealed_by               UUID REFERENCES users(id),
    -- Legal status
    submitted_to_gwl        BOOLEAN NOT NULL DEFAULT FALSE,
    submitted_to_gwl_at     TIMESTAMPTZ,
    submitted_to_purc       BOOLEAN NOT NULL DEFAULT FALSE,
    submitted_to_purc_at    TIMESTAMPTZ,
    submitted_to_court      BOOLEAN NOT NULL DEFAULT FALSE,
    submitted_to_court_at   TIMESTAMPTZ,
    court_case_number       VARCHAR(100),
    -- PDF export
    pdf_url                 TEXT,
    pdf_generated_at        TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evidence_audit ON audit_evidence_packages(audit_event_id);
CREATE INDEX IF NOT EXISTS idx_evidence_reference ON audit_evidence_packages(package_reference);

-- ═══════════════════════════════════════════════════════════════════════════════
-- J. ECG LOAD SHEDDING / OUTAGE CORRELATION
-- ═══════════════════════════════════════════════════════════════════════════════
-- When ECG cuts power, pumping stations stop → production drops.
-- This is NOT theft. The system must distinguish power-cut from fraud.

CREATE TABLE IF NOT EXISTS ecg_outage_records (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    district_id         UUID REFERENCES districts(id),      -- NULL = national outage
    outage_start        TIMESTAMPTZ NOT NULL,
    outage_end          TIMESTAMPTZ,
    duration_hours      NUMERIC(6,2) GENERATED ALWAYS AS (
                            EXTRACT(EPOCH FROM (outage_end - outage_start)) / 3600
                        ) STORED,
    outage_type         VARCHAR(50) NOT NULL DEFAULT 'LOAD_SHEDDING',
                        -- 'LOAD_SHEDDING', 'FAULT', 'MAINTENANCE', 'EMERGENCY'
    affected_pumping_stations TEXT[],
    estimated_production_loss_m3 NUMERIC(12,2),
    source              VARCHAR(50) NOT NULL DEFAULT 'MANUAL',
                        -- 'ECG_API', 'MANUAL', 'INFERRED_FROM_PRODUCTION'
    notes               TEXT,
    created_by          UUID REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ecg_outage_district ON ecg_outage_records(district_id);
CREATE INDEX IF NOT EXISTS idx_ecg_outage_start ON ecg_outage_records(outage_start);

-- ═══════════════════════════════════════════════════════════════════════════════
-- K. EXCHANGE RATE TRACKING (GHS/USD for donor reporting)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS exchange_rates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rate_date       DATE NOT NULL UNIQUE,
    ghs_per_usd     NUMERIC(10,4) NOT NULL,
    ghs_per_gbp     NUMERIC(10,4),
    ghs_per_eur     NUMERIC(10,4),
    source          VARCHAR(50) NOT NULL DEFAULT 'BOG',     -- Bank of Ghana
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_date ON exchange_rates(rate_date);

-- Seed approximate 2026 rate (Bank of Ghana)
INSERT INTO exchange_rates (rate_date, ghs_per_usd, ghs_per_gbp, ghs_per_eur, source)
VALUES ('2026-01-01', 15.50, 19.80, 16.90, 'BOG_SEED')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════════
-- L. MULTI-LANGUAGE SUPPORT METADATA
-- ═══════════════════════════════════════════════════════════════════════════════
-- Store translated strings for SMS templates and field app labels.

CREATE TABLE IF NOT EXISTS i18n_strings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         VARCHAR(200) NOT NULL,
    locale      VARCHAR(10) NOT NULL,   -- 'en', 'tw' (Twi), 'ga', 'ee' (Ewe), 'ha' (Hausa)
    value       TEXT NOT NULL,
    context     VARCHAR(100),           -- 'sms', 'field_app', 'report'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(key, locale)
);

-- Seed core SMS templates in Twi (Akan) — most widely spoken in Ghana
INSERT INTO i18n_strings (key, locale, value, context) VALUES
    ('sms.job_assigned',    'tw', 'Adwuma foforɔ wɔ wo din ase. Kɔ GN-WAAS app no mu nhwɛ. Ref: {ref}', 'sms'),
    ('sms.job_assigned',    'en', 'New field job assigned to you. Check GN-WAAS app. Ref: {ref}', 'sms'),
    ('sms.audit_complete',  'tw', 'Wo account {account} ppɛ nhwɛso ato mu. Ref: {ref}. Ɛho asɛm biara a, frɛ 0800-GNWAAS', 'sms'),
    ('sms.audit_complete',  'en', 'Your account {account} audit is complete. Ref: {ref}. Queries: 0800-GNWAAS', 'sms'),
    ('sms.provisional',     'en', 'Audit {ref} signed provisionally. GRA confirmation pending. Keep this reference.', 'sms'),
    ('sms.anomaly_alert',   'en', 'ALERT: Anomaly detected in {district}. {count} accounts flagged. Login to review.', 'sms')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES & PERFORMANCE
-- ═══════════════════════════════════════════════════════════════════════════════

-- Composite index for MoMo fraud detection
CREATE INDEX IF NOT EXISTS idx_momo_fraud_detection
    ON mobile_money_payments(sender_phone, transaction_date)
    WHERE reconciliation_status IN ('GHOST_ACCOUNT', 'UNMATCHED');

-- Composite index for seasonal threshold lookup
CREATE INDEX IF NOT EXISTS idx_seasonal_active
    ON seasonal_threshold_config(month_start, month_end)
    WHERE is_active = TRUE;
