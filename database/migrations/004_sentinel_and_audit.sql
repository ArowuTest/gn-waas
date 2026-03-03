-- ============================================================
-- GN-WAAS Migration: 004_sentinel_and_audit
-- Description: Sentinel anomaly detection, audit events,
--              field jobs, OCR records, and GRA compliance
-- ============================================================

-- ============================================================
-- SENTINEL ANOMALY FLAGS
-- ============================================================
CREATE TABLE anomaly_flags (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id          UUID REFERENCES water_accounts(id),
    district_id         UUID NOT NULL REFERENCES districts(id),
    anomaly_type        anomaly_type NOT NULL,
    alert_level         alert_level NOT NULL,
    fraud_type          fraud_type,
    title               VARCHAR(200) NOT NULL,
    description         TEXT NOT NULL,

    -- Financial impact
    estimated_loss_ghs  NUMERIC(12,2),
    billing_period_start DATE,
    billing_period_end   DATE,

    -- Evidence references
    shadow_bill_id      UUID REFERENCES shadow_bills(id),
    gwl_bill_id         UUID REFERENCES gwl_billing_records(id),
    evidence_data       JSONB NOT NULL DEFAULT '{}',      -- Flexible evidence store

    -- Lifecycle
    status              VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    assigned_to         UUID REFERENCES users(id),
    resolved_at         TIMESTAMPTZ,
    resolution_notes    TEXT,
    false_positive      BOOLEAN,
    confirmed_fraud     BOOLEAN,
    recovered_amount_ghs NUMERIC(12,2),

    -- Immutability
    sentinel_version    VARCHAR(20) NOT NULL DEFAULT '1.0',
    detection_hash      VARCHAR(64),                      -- SHA-256 of detection inputs
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_anomaly_account ON anomaly_flags(account_id);
CREATE INDEX idx_anomaly_district ON anomaly_flags(district_id);
CREATE INDEX idx_anomaly_type ON anomaly_flags(anomaly_type);
CREATE INDEX idx_anomaly_level ON anomaly_flags(alert_level);
CREATE INDEX idx_anomaly_status ON anomaly_flags(status);
CREATE INDEX idx_anomaly_created ON anomaly_flags(created_at DESC);

COMMENT ON TABLE anomaly_flags IS 'Sentinel-detected anomalies. Immutable once created (append-only updates via audit_log).';

-- ============================================================
-- AUDIT EVENTS (Full audit lifecycle)
-- ============================================================
CREATE TABLE audit_events (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    audit_reference         VARCHAR(50) NOT NULL UNIQUE,  -- Human-readable: AUD-2026-001234
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    district_id             UUID NOT NULL REFERENCES districts(id),
    anomaly_flag_id         UUID REFERENCES anomaly_flags(id),
    status                  audit_status NOT NULL DEFAULT 'PENDING',

    -- Assignment
    assigned_officer_id     UUID REFERENCES users(id),
    assigned_supervisor_id  UUID REFERENCES users(id),
    assigned_at             TIMESTAMPTZ,
    due_date                TIMESTAMPTZ,

    -- Field evidence
    field_job_id            UUID,                         -- FK set after field_jobs created
    meter_photo_url         TEXT,
    surroundings_photo_url  TEXT,
    ocr_reading_value       NUMERIC(12,4),
    manual_reading_value    NUMERIC(12,4),
    ocr_status              ocr_status,
    gps_latitude            NUMERIC(10,8),
    gps_longitude           NUMERIC(11,8),
    gps_precision_m         NUMERIC(6,2),
    officer_signature_url   TEXT,
    customer_signature_url  TEXT,
    tamper_evidence_detected BOOLEAN NOT NULL DEFAULT FALSE,
    tamper_evidence_url     TEXT,

    -- GRA Compliance
    gra_status              gra_compliance_status NOT NULL DEFAULT 'PENDING',
    gra_sdc_id              VARCHAR(100),
    gra_qr_code_url         TEXT,
    gra_signed_at           TIMESTAMPTZ,
    gra_invoice_number      VARCHAR(100),

    -- Financial outcome
    gwl_billed_ghs          NUMERIC(12,2),
    shadow_bill_ghs         NUMERIC(12,2),
    variance_pct            NUMERIC(8,4),
    confirmed_loss_ghs      NUMERIC(12,2),
    recovery_invoice_ghs    NUMERIC(12,2),               -- 3% success fee base
    success_fee_ghs         NUMERIC(12,2),               -- 3% of recovered

    -- Immutability lock
    is_locked               BOOLEAN NOT NULL DEFAULT FALSE,
    locked_at               TIMESTAMPTZ,
    lock_reason             VARCHAR(100),                 -- 'GRA_SIGNED', 'CLOSED'

    -- Metadata
    notes                   TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_account ON audit_events(account_id);
CREATE INDEX idx_audit_events_district ON audit_events(district_id);
CREATE INDEX idx_audit_events_status ON audit_events(status);
CREATE INDEX idx_audit_events_officer ON audit_events(assigned_officer_id);
CREATE INDEX idx_audit_events_gra ON audit_events(gra_status);
CREATE INDEX idx_audit_events_created ON audit_events(created_at DESC);
CREATE INDEX idx_audit_events_reference ON audit_events(audit_reference);

COMMENT ON TABLE audit_events IS 'Full audit event lifecycle. Locked after GRA signing - immutable.';

-- ============================================================
-- FIELD JOBS (Mobile app dispatch)
-- ============================================================
CREATE TABLE field_jobs (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_reference           VARCHAR(50) NOT NULL UNIQUE,  -- JOB-2026-001234
    audit_event_id          UUID REFERENCES audit_events(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    district_id             UUID NOT NULL REFERENCES districts(id),
    assigned_officer_id     UUID NOT NULL REFERENCES users(id),
    status                  field_job_status NOT NULL DEFAULT 'ASSIGNED',

    -- Blind audit (officer sees GPS only, not account details)
    is_blind_audit          BOOLEAN NOT NULL DEFAULT TRUE,
    target_gps_lat          NUMERIC(10,8) NOT NULL,
    target_gps_lng          NUMERIC(11,8) NOT NULL,
    gps_fence_radius_m      NUMERIC(8,2) NOT NULL DEFAULT 5.0,

    -- Dispatch
    dispatched_at           TIMESTAMPTZ,
    arrived_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    officer_gps_lat         NUMERIC(10,8),
    officer_gps_lng         NUMERIC(11,8),
    gps_verified            BOOLEAN,

    -- Biometric
    biometric_verified      BOOLEAN NOT NULL DEFAULT FALSE,
    biometric_verified_at   TIMESTAMPTZ,

    -- Priority
    priority                INTEGER NOT NULL DEFAULT 5,   -- 1=Critical, 10=Low
    requires_security_escort BOOLEAN NOT NULL DEFAULT FALSE,
    security_notes          TEXT,

    -- SOS
    sos_triggered           BOOLEAN NOT NULL DEFAULT FALSE,
    sos_triggered_at        TIMESTAMPTZ,
    sos_resolved_at         TIMESTAMPTZ,

    notes                   TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add FK back to audit_events
ALTER TABLE audit_events ADD CONSTRAINT fk_audit_field_job
    FOREIGN KEY (field_job_id) REFERENCES field_jobs(id);

CREATE INDEX idx_field_jobs_officer ON field_jobs(assigned_officer_id);
CREATE INDEX idx_field_jobs_district ON field_jobs(district_id);
CREATE INDEX idx_field_jobs_status ON field_jobs(status);
CREATE INDEX idx_field_jobs_created ON field_jobs(created_at DESC);

COMMENT ON TABLE field_jobs IS 'Field officer dispatch jobs. Blind audit by default - officer sees GPS only.';

-- ============================================================
-- GRA VSDC COMPLIANCE LOG (Immutable)
-- ============================================================
CREATE TABLE gra_compliance_log (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    audit_event_id      UUID NOT NULL REFERENCES audit_events(id),
    account_id          UUID NOT NULL REFERENCES water_accounts(id),
    attempt_number      INTEGER NOT NULL DEFAULT 1,
    status              gra_compliance_status NOT NULL,

    -- Request payload (stored for audit trail)
    business_tin        VARCHAR(20) NOT NULL,
    invoice_number      VARCHAR(100) NOT NULL,
    total_amount_ghs    NUMERIC(12,2) NOT NULL,
    vat_amount_ghs      NUMERIC(12,2) NOT NULL,
    request_payload     JSONB NOT NULL,

    -- Response
    sdc_id              VARCHAR(100),
    qr_code_url         TEXT,
    qr_code_string      TEXT,
    response_payload    JSONB,
    error_message       TEXT,

    -- Timing
    requested_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at        TIMESTAMPTZ,
    response_time_ms    INTEGER,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gra_log_audit ON gra_compliance_log(audit_event_id);
CREATE INDEX idx_gra_log_status ON gra_compliance_log(status);
CREATE INDEX idx_gra_log_tin ON gra_compliance_log(business_tin);

COMMENT ON TABLE gra_compliance_log IS 'Immutable GRA VSDC API interaction log. Every attempt recorded.';

-- ============================================================
-- AUDIT TRAIL (Immutable append-only log for all changes)
-- ============================================================
CREATE TABLE audit_trail (
    id              BIGSERIAL PRIMARY KEY,
    entity_type     VARCHAR(50) NOT NULL,
    entity_id       UUID NOT NULL,
    action          VARCHAR(20) NOT NULL,                 -- CREATE, UPDATE, DELETE, VIEW
    changed_by      UUID REFERENCES users(id),
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    old_values      JSONB,
    new_values      JSONB,
    ip_address      INET,
    user_agent      TEXT,
    request_id      VARCHAR(100)
);

-- DB-H01 fix: wrap in DO block so migration succeeds even without TimescaleDB
DO $hyper$ BEGIN
    PERFORM create_hypertable('audit_trail', 'changed_at',
        chunk_time_interval => INTERVAL '1 month',
        if_not_exists => TRUE
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'TimescaleDB not available — audit_trail will be a plain table: %', SQLERRM;
END $hyper$;

CREATE INDEX idx_audit_trail_entity ON audit_trail(entity_type, entity_id);
CREATE INDEX idx_audit_trail_user ON audit_trail(changed_by);
CREATE INDEX idx_audit_trail_time ON audit_trail(changed_at DESC);

COMMENT ON TABLE audit_trail IS 'Immutable append-only audit trail. No records can be deleted. TimescaleDB hypertable.';
