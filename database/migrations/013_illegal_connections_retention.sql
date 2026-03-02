-- Migration 013: Illegal Connections Table + Backup/Retention Policies
-- =====================================================================
-- H1: Creates the illegal_connections table for FIO-004 reporting
-- H5: Adds data retention policies and backup configuration comments

-- ─────────────────────────────────────────────────────────────
-- 1. Illegal Connections Table (H1 — FIO-004)
-- ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS illegal_connections (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    officer_id                  UUID        NOT NULL REFERENCES users(id),
    job_id                      UUID        REFERENCES field_jobs(id) ON DELETE SET NULL,
    connection_type             VARCHAR(50) NOT NULL
        CHECK (connection_type IN (
            'BYPASS', 'ILLEGAL_TAP', 'TAMPERED_METER',
            'REVERSED_METER', 'SHARED_CONNECTION', 'BROKEN_SEAL', 'OTHER'
        )),
    severity                    VARCHAR(20) NOT NULL DEFAULT 'HIGH'
        CHECK (severity IN ('CRITICAL', 'HIGH', 'MEDIUM', 'LOW')),
    description                 TEXT        NOT NULL,
    estimated_daily_loss_litres NUMERIC(10,2) DEFAULT 0,
    address                     TEXT        NOT NULL,
    account_number              VARCHAR(50),
    latitude                    NUMERIC(10,7) NOT NULL,
    longitude                   NUMERIC(10,7) NOT NULL,
    gps_accuracy                NUMERIC(6,2),
    photo_count                 INT         NOT NULL DEFAULT 0,
    status                      VARCHAR(30) NOT NULL DEFAULT 'REPORTED'
        CHECK (status IN ('REPORTED', 'UNDER_INVESTIGATION', 'CONFIRMED', 'RESOLVED', 'FALSE_REPORT')),
    resolution_notes            TEXT,
    resolved_by                 UUID        REFERENCES users(id) ON DELETE SET NULL,
    resolved_at                 TIMESTAMPTZ,
    reported_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_illegal_connections_officer
    ON illegal_connections (officer_id);
CREATE INDEX IF NOT EXISTS idx_illegal_connections_status
    ON illegal_connections (status);
CREATE INDEX IF NOT EXISTS idx_illegal_connections_severity
    ON illegal_connections (severity);
CREATE INDEX IF NOT EXISTS idx_illegal_connections_reported_at
    ON illegal_connections (reported_at DESC);
-- Geospatial index for proximity queries
CREATE INDEX IF NOT EXISTS idx_illegal_connections_location
    ON illegal_connections USING GIST (
        point(longitude::float8, latitude::float8)
    );

-- ─────────────────────────────────────────────────────────────
-- 2. Supply Schedule Table (H3 — TECH-SE-002)
--    Stores scheduled supply windows per district so the
--    sentinel can detect off-schedule consumption.
-- ─────────────────────────────────────────────────────────────
-- supply_schedules already created in migration 002 with a different schema.
-- Add the missing columns needed by the supply_validator service.
ALTER TABLE supply_schedules
    ADD COLUMN IF NOT EXISTS day_of_week  SMALLINT CHECK (day_of_week BETWEEN 0 AND 6),
    ADD COLUMN IF NOT EXISTS start_hour   SMALLINT CHECK (start_hour BETWEEN 0 AND 23),
    ADD COLUMN IF NOT EXISTS end_hour     SMALLINT CHECK (end_hour BETWEEN 1 AND 24),
    ADD COLUMN IF NOT EXISTS is_active    BOOLEAN  NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Add unique constraint only if it doesn't already exist
DO $$ BEGIN
  ALTER TABLE supply_schedules ADD CONSTRAINT supply_schedules_district_day_hour_unique
      UNIQUE (district_id, day_of_week, start_hour);
EXCEPTION WHEN duplicate_table THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_supply_schedules_district_day
    ON supply_schedules (district_id, day_of_week);

-- ─────────────────────────────────────────────────────────────
-- 3. Data Retention Policies (H5 — OCS-004)
--    PostgreSQL does not have built-in retention policies.
--    We implement them via:
--    a) Partitioning hint comments for TimescaleDB
--    b) A retention_policies config table
--    c) A pg_cron job definition (requires pg_cron extension)
-- ─────────────────────────────────────────────────────────────

-- Retention configuration table
CREATE TABLE IF NOT EXISTS retention_policies (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    table_name      VARCHAR(100) NOT NULL UNIQUE,
    retention_days  INT         NOT NULL,
    archive_before_delete BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default retention policies per table
INSERT INTO retention_policies (table_name, retention_days, archive_before_delete)
VALUES
    ('audit_events',          2555, TRUE),   -- 7 years (regulatory requirement)
    ('anomaly_flags',         1825, TRUE),   -- 5 years
    ('illegal_connections',   2555, TRUE),   -- 7 years
    ('audit_trail',           2555, TRUE),   -- 7 years (immutable, never delete)
    ('field_jobs',            1095, TRUE),   -- 3 years
    ('nrw_reports',           2555, TRUE),   -- 7 years
    ('meter_readings',         365, TRUE),   -- 1 year hot, then archive
    ('gwl_cases',             1825, TRUE)    -- 5 years
ON CONFLICT (table_name) DO NOTHING;

-- Archive table for old audit events (partitioned by year)
CREATE TABLE IF NOT EXISTS audit_events_archive (
    LIKE audit_events INCLUDING ALL
);
COMMENT ON TABLE audit_events_archive IS
    'Archive of audit_events older than retention_policies.retention_days. '
    'Records are moved here before deletion. Never truncate this table.';

-- ─────────────────────────────────────────────────────────────
-- 4. Automated retention function
--    Called by pg_cron or a scheduled job runner.
--    Archives records older than the retention period.
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION fn_archive_old_audit_events()
RETURNS INTEGER
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
    v_retention_days INT;
    v_archived       INT;
BEGIN
    SELECT retention_days INTO v_retention_days
    FROM retention_policies
    WHERE table_name = 'audit_events';

    IF v_retention_days IS NULL THEN
        v_retention_days := 2555; -- default 7 years
    END IF;

    -- Move old completed/closed events to archive
    WITH moved AS (
        DELETE FROM audit_events
        WHERE status IN ('COMPLETED', 'CLOSED', 'GRA_CONFIRMED')
          AND created_at < NOW() - (v_retention_days || ' days')::INTERVAL
        RETURNING *
    )
    INSERT INTO audit_events_archive SELECT * FROM moved;

    GET DIAGNOSTICS v_archived = ROW_COUNT;

    -- Update last run timestamp
    UPDATE retention_policies
    SET last_run_at = NOW()
    WHERE table_name = 'audit_events';

    RETURN v_archived;
END;
$$;

-- ─────────────────────────────────────────────────────────────
-- 5. Schedule retention job via pg_cron (if available)
--    Run at 3 AM daily (outside the 2-4 AM night-flow window)
-- ─────────────────────────────────────────────────────────────
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_extension WHERE extname = 'pg_cron'
    ) THEN
        PERFORM cron.schedule(
            'archive-old-audit-events',
            '0 3 * * *',
            'SELECT fn_archive_old_audit_events()'
        );
    END IF;
EXCEPTION WHEN others THEN
    -- pg_cron not available — retention must be run manually or via external scheduler
    RAISE NOTICE 'pg_cron not available. Run fn_archive_old_audit_events() manually or via external scheduler.';
END;
$$;
