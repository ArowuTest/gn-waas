-- ============================================================
-- GN-WAAS Migration 006: GWL Case Management Tables
-- Description: Tables to support the GWL Case Management Portal.
--   GWL supervisors use these to manage flagged anomalies end-to-end:
--   assign field officers, approve reclassifications, issue credits,
--   dispute flags, and track resolution outcomes.
-- ============================================================

-- ── GWL Case Actions ─────────────────────────────────────────────────────────
-- Every action taken on an anomaly_flag is recorded here as an immutable log.
-- This gives a full audit trail: who did what, when, and why.
CREATE TABLE IF NOT EXISTS gwl_case_actions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    anomaly_flag_id     UUID NOT NULL REFERENCES anomaly_flags(id) ON DELETE CASCADE,
    account_id          UUID REFERENCES water_accounts(id),
    district_id         UUID REFERENCES districts(id),

    -- Who took the action
    performed_by_id     UUID,                          -- Keycloak user UUID
    performed_by_name   TEXT NOT NULL,
    performed_by_role   TEXT NOT NULL,                 -- GWL_SUPERVISOR, GWL_BILLING_OFFICER, etc.

    -- What action was taken
    action_type         TEXT NOT NULL,                 -- ASSIGNED, APPROVED_RECLASSIFICATION,
                                                       -- ISSUED_CREDIT, DISPUTED, ESCALATED,
                                                       -- FIELD_EVIDENCE_REVIEWED, CLOSED, REOPENED
    action_notes        TEXT,
    action_metadata     JSONB DEFAULT '{}',            -- flexible payload per action type

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gwl_case_actions_flag    ON gwl_case_actions(anomaly_flag_id);
CREATE INDEX IF NOT EXISTS idx_gwl_case_actions_account ON gwl_case_actions(account_id);
CREATE INDEX IF NOT EXISTS idx_gwl_case_actions_created ON gwl_case_actions(created_at DESC);

-- ── Reclassification Requests ─────────────────────────────────────────────────
-- When GWL agrees an account is in the wrong category, they raise a formal
-- reclassification request. This creates a paper trail and tracks whether
-- the correction was actually applied in GWL's billing system.
CREATE TABLE IF NOT EXISTS reclassification_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    anomaly_flag_id         UUID NOT NULL REFERENCES anomaly_flags(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    district_id             UUID NOT NULL REFERENCES districts(id),

    -- The change being requested
    current_category        TEXT NOT NULL,             -- e.g. RESIDENTIAL
    recommended_category    TEXT NOT NULL,             -- e.g. COMMERCIAL
    justification           TEXT NOT NULL,             -- plain-English reason
    supporting_evidence     JSONB DEFAULT '{}',        -- consumption stats, photos, etc.

    -- Estimated financial impact
    monthly_revenue_impact_ghs  NUMERIC(12,2),         -- positive = underbilling recovery
    annual_revenue_impact_ghs   NUMERIC(12,2),

    -- Workflow status
    status                  TEXT NOT NULL DEFAULT 'PENDING',
                                                       -- PENDING, APPROVED, REJECTED,
                                                       -- APPLIED_IN_GWL, VERIFIED
    requested_by_id         UUID,
    requested_by_name       TEXT NOT NULL,
    approved_by_id          UUID,
    approved_by_name        TEXT,
    approved_at             TIMESTAMPTZ,
    applied_in_gwl_at       TIMESTAMPTZ,               -- when GWL billing system was updated
    gwl_reference           TEXT,                      -- GWL's internal change reference

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_reclass_account  ON reclassification_requests(account_id);
CREATE INDEX IF NOT EXISTS idx_reclass_status   ON reclassification_requests(status);
CREATE INDEX IF NOT EXISTS idx_reclass_district ON reclassification_requests(district_id);

-- ── Credit Requests ───────────────────────────────────────────────────────────
-- When GN-WAAS detects overbilling, GWL can issue a credit to the customer.
-- This table tracks the approval and application of that credit.
CREATE TABLE IF NOT EXISTS credit_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    anomaly_flag_id         UUID NOT NULL REFERENCES anomaly_flags(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    district_id             UUID NOT NULL REFERENCES districts(id),

    -- The overbilling details
    gwl_bill_id             UUID REFERENCES gwl_billing_records(id),
    billing_period_start    DATE NOT NULL,
    billing_period_end      DATE NOT NULL,
    gwl_amount_ghs          NUMERIC(12,2) NOT NULL,    -- what GWL charged
    shadow_amount_ghs       NUMERIC(12,2) NOT NULL,    -- what GN-WAAS calculated
    overcharge_amount_ghs   NUMERIC(12,2) NOT NULL,    -- the difference
    credit_amount_ghs       NUMERIC(12,2) NOT NULL,    -- amount to credit (may differ)

    reason                  TEXT NOT NULL,
    notes                   TEXT,

    -- Workflow status
    status                  TEXT NOT NULL DEFAULT 'PENDING',
                                                       -- PENDING, APPROVED, REJECTED,
                                                       -- APPLIED_IN_GWL, VERIFIED
    requested_by_id         UUID,
    requested_by_name       TEXT NOT NULL,
    approved_by_id          UUID,
    approved_by_name        TEXT,
    approved_at             TIMESTAMPTZ,
    applied_in_gwl_at       TIMESTAMPTZ,
    gwl_credit_reference    TEXT,                      -- GWL's credit note reference

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_account  ON credit_requests(account_id);
CREATE INDEX IF NOT EXISTS idx_credit_status   ON credit_requests(status);
CREATE INDEX IF NOT EXISTS idx_credit_district ON credit_requests(district_id);

-- ── GWL Monthly Reports ───────────────────────────────────────────────────────
-- Auto-generated monthly summary reports for GWL management and Ministry of Finance.
CREATE TABLE IF NOT EXISTS gwl_monthly_reports (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_period               DATE NOT NULL,         -- first day of the month
    district_id                 UUID REFERENCES districts(id),  -- NULL = national summary

    -- Case statistics
    total_cases_flagged         INT DEFAULT 0,
    critical_cases              INT DEFAULT 0,
    cases_resolved              INT DEFAULT 0,
    cases_pending               INT DEFAULT 0,
    cases_disputed              INT DEFAULT 0,

    -- Financial impact
    total_underbilling_ghs      NUMERIC(14,2) DEFAULT 0,
    total_overbilling_ghs       NUMERIC(14,2) DEFAULT 0,
    revenue_recovered_ghs       NUMERIC(14,2) DEFAULT 0,
    credits_issued_ghs          NUMERIC(14,2) DEFAULT 0,

    -- Reclassifications
    reclassifications_requested INT DEFAULT 0,
    reclassifications_applied   INT DEFAULT 0,

    -- Field operations
    field_jobs_assigned         INT DEFAULT 0,
    field_jobs_completed        INT DEFAULT 0,

    -- Report metadata
    generated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    generated_by                TEXT DEFAULT 'SYSTEM',
    report_data                 JSONB DEFAULT '{}',    -- full report payload for PDF generation

    UNIQUE(report_period, district_id)
);

CREATE INDEX IF NOT EXISTS idx_gwl_reports_period   ON gwl_monthly_reports(report_period DESC);
CREATE INDEX IF NOT EXISTS idx_gwl_reports_district ON gwl_monthly_reports(district_id);

-- ── Add GWL-specific columns to anomaly_flags ─────────────────────────────────
-- Extend the existing anomaly_flags table with GWL workflow columns
ALTER TABLE anomaly_flags
    ADD COLUMN IF NOT EXISTS gwl_status         TEXT DEFAULT 'PENDING_REVIEW',
                                                -- PENDING_REVIEW, UNDER_INVESTIGATION,
                                                -- FIELD_ASSIGNED, EVIDENCE_SUBMITTED,
                                                -- APPROVED_FOR_CORRECTION, DISPUTED,
                                                -- CORRECTED, CLOSED
    ADD COLUMN IF NOT EXISTS gwl_assigned_to_id UUID,
    ADD COLUMN IF NOT EXISTS gwl_assigned_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gwl_resolved_at    TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS gwl_resolution     TEXT,
    ADD COLUMN IF NOT EXISTS gwl_notes          TEXT;

CREATE INDEX IF NOT EXISTS idx_anomaly_gwl_status ON anomaly_flags(gwl_status);
CREATE INDEX IF NOT EXISTS idx_anomaly_gwl_assigned ON anomaly_flags(gwl_assigned_to_id);
