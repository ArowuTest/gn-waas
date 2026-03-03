-- ============================================================
-- GN-WAAS Migration: 005_data_confidence_and_reporting
-- Description: AWWA Data Confidence scoring, recovery tracking,
--              reporting tables, and notification system
-- ============================================================

-- ============================================================
-- DATA CONFIDENCE SCORES (AWWA Grading Matrix implementation)
-- ============================================================
CREATE TABLE data_confidence_scores (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_id             UUID NOT NULL REFERENCES districts(id),
    period_start            DATE NOT NULL,
    period_end              DATE NOT NULL,

    -- AWWA Grading Matrix dimensions (1-10 scale)
    billing_db_grade        data_confidence_grade DEFAULT 0,
    production_records_grade data_confidence_grade DEFAULT 0,
    gis_network_grade       data_confidence_grade DEFAULT 0,
    ocr_accuracy_grade      data_confidence_grade DEFAULT 0,
    gra_api_grade           data_confidence_grade DEFAULT 0,
    meter_coverage_grade    data_confidence_grade DEFAULT 0,
    supply_schedule_grade   data_confidence_grade DEFAULT 0,

    -- Composite score (weighted average)
    composite_grade         data_confidence_grade DEFAULT 0,
    composite_notes         TEXT,

    -- Supporting metrics
    billing_completeness_pct    NUMERIC(5,2),
    ocr_match_rate_pct          NUMERIC(5,2),
    gra_uptime_pct              NUMERIC(5,2),
    accounts_gis_matched_pct    NUMERIC(5,2),

    calculated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_confidence_district ON data_confidence_scores(district_id);
CREATE INDEX idx_confidence_period ON data_confidence_scores(period_start);

COMMENT ON TABLE data_confidence_scores IS 'AWWA Grading Matrix implementation. Scores data reliability 1-10 per dimension.';

-- ============================================================
-- RECOVERY TRACKING (3% success fee model)
-- ============================================================
CREATE TABLE recovery_records (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    audit_event_id          UUID NOT NULL REFERENCES audit_events(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    district_id             UUID NOT NULL REFERENCES districts(id),

    -- Recovery details
    original_gwl_bill_ghs   NUMERIC(12,2) NOT NULL,
    corrected_bill_ghs      NUMERIC(12,2) NOT NULL,
    recovered_amount_ghs    NUMERIC(12,2) NOT NULL,
    recovery_date           DATE NOT NULL,
    payment_reference       VARCHAR(100),

    -- Success fee calculation
    success_fee_rate_pct    NUMERIC(5,2) NOT NULL DEFAULT 3.0,
    success_fee_ghs         NUMERIC(12,2) NOT NULL,
    fee_invoice_number      VARCHAR(100),
    fee_paid                BOOLEAN NOT NULL DEFAULT FALSE,
    fee_paid_date           DATE,

    -- Attribution
    detected_by_anomaly     anomaly_type,
    confirmed_by            UUID REFERENCES users(id),
    notes                   TEXT,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_recovery_audit ON recovery_records(audit_event_id);
CREATE INDEX idx_recovery_district ON recovery_records(district_id);
CREATE INDEX idx_recovery_date ON recovery_records(recovery_date DESC);

COMMENT ON TABLE recovery_records IS 'Revenue recovery tracking. Basis for 3% success fee calculation.';

-- ============================================================
-- NOTIFICATIONS
-- ============================================================
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID REFERENCES users(id),
    role_target     user_role,                            -- Broadcast to role
    district_id     UUID REFERENCES districts(id),       -- District-scoped
    title           VARCHAR(200) NOT NULL,
    body            TEXT NOT NULL,
    notification_type VARCHAR(50) NOT NULL,              -- ANOMALY, AUDIT, SYSTEM, ALERT
    alert_level     alert_level,
    entity_type     VARCHAR(50),
    entity_id       UUID,
    is_read         BOOLEAN NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMPTZ,
    is_push_sent    BOOLEAN NOT NULL DEFAULT FALSE,
    push_sent_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user ON notifications(user_id, is_read);
CREATE INDEX idx_notifications_role ON notifications(role_target);
CREATE INDEX idx_notifications_created ON notifications(created_at DESC);

-- ============================================================
-- SYSTEM HEALTH METRICS (TimescaleDB)
-- ============================================================
CREATE TABLE system_health_metrics (
    time            TIMESTAMPTZ NOT NULL,
    service_name    VARCHAR(50) NOT NULL,
    metric_name     VARCHAR(100) NOT NULL,
    metric_value    NUMERIC(15,4) NOT NULL,
    unit            VARCHAR(20),
    tags            JSONB DEFAULT '{}'
);

-- DB-H01 fix: wrap in DO block so migration succeeds even without TimescaleDB
DO $hyper$ BEGIN
    PERFORM create_hypertable('system_health_metrics', 'time',
        chunk_time_interval => INTERVAL '1 day',
        if_not_exists => TRUE
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'TimescaleDB not available — system_health_metrics will be a plain table: %', SQLERRM;
END $hyper$;

CREATE INDEX idx_health_service_time ON system_health_metrics(service_name, time DESC);

-- ============================================================
-- CDC SYNC LOG (Track GWL database synchronisation)
-- ============================================================
CREATE TABLE cdc_sync_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    sync_type       VARCHAR(50) NOT NULL,                 -- BILLING, ACCOUNTS, PRODUCTION
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    records_synced  INTEGER NOT NULL DEFAULT 0,
    records_failed  INTEGER NOT NULL DEFAULT 0,
    last_event_id   VARCHAR(100),
    status          VARCHAR(20) NOT NULL DEFAULT 'RUNNING',
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE cdc_sync_log IS 'GWL CDC synchronisation log. Tracks data freshness.';

-- ============================================================
-- VIEWS for common queries
-- ============================================================

-- Active anomalies with account details
CREATE VIEW v_active_anomalies AS
SELECT
    af.id,
    af.anomaly_type,
    af.alert_level,
    af.fraud_type,
    af.title,
    af.estimated_loss_ghs,
    af.created_at,
    wa.gwl_account_number,
    wa.account_holder_name,
    wa.category,
    d.district_name,
    d.region,
    d.zone_type
FROM anomaly_flags af
JOIN water_accounts wa ON af.account_id = wa.id
JOIN districts d ON af.district_id = d.id
WHERE af.status = 'OPEN'
ORDER BY af.alert_level DESC, af.created_at DESC;

-- District NRW summary
CREATE VIEW v_district_nrw_summary AS
SELECT
    d.id AS district_id,
    d.district_code,
    d.district_name,
    d.region,
    d.zone_type,
    d.total_connections,
    wb.period_start,
    wb.period_end,
    wb.system_input_volume_m3,
    wb.total_nrw_m3,
    wb.nrw_pct,
    wb.total_apparent_losses_m3,
    wb.total_real_losses_m3,
    wb.total_nrw_value_ghs,
    wb.apparent_loss_value_ghs,
    wb.data_confidence_grade,
    dcs.composite_grade AS confidence_composite
FROM districts d
LEFT JOIN LATERAL (
    SELECT * FROM water_balance_records
    WHERE district_id = d.id
    ORDER BY period_start DESC
    LIMIT 1
) wb ON TRUE
LEFT JOIN LATERAL (
    SELECT * FROM data_confidence_scores
    WHERE district_id = d.id
    ORDER BY period_start DESC
    LIMIT 1
) dcs ON TRUE
WHERE d.is_active = TRUE;

COMMENT ON VIEW v_district_nrw_summary IS 'Latest NRW summary per district. Used for heatmap dashboard.';
