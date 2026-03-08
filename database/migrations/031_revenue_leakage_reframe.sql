-- Migration 031: Revenue Leakage Reframe
-- ============================================================
-- Corrects the conceptual model of GN-WAAS:
--   GN-WAAS is a REVENUE LEAKAGE detection and recovery system.
--   Every anomaly must have a GHS value. The pipeline is:
--   DETECTED → FIELD_VERIFIED → CONFIRMED → GRA_SIGNED → COLLECTED
--
-- Key changes:
--   A. Replace GHOST_ACCOUNT anomaly type with ADDRESS_UNVERIFIED (screening)
--      and UNMETERED_CONSUMPTION (confirmed revenue leakage)
--   B. Add FRAUDULENT_ACCOUNT for confirmed fake accounts (GWL internal fraud)
--   C. Add field_job_outcome enum for structured field officer findings
--   D. Add outcome columns to field_jobs table
--   E. Add leakage_category to anomaly_flags (REVENUE_LEAKAGE vs COMPLIANCE)
--   F. Add monthly_leakage_ghs to anomaly_flags (always populated)
--   G. Add annualised_leakage_ghs to anomaly_flags (monthly × 12)
--   H. Add auto-recovery trigger: confirmed anomaly → revenue_recovery_event
--   I. Add leakage pipeline view for dashboard
-- ============================================================

-- ── A. Extend anomaly_type enum ───────────────────────────────────────────────
-- Replace GHOST_ACCOUNT with three precise types:
--   ADDRESS_UNVERIFIED    = GPS outside network, needs field check (LOW severity)
--   UNMETERED_CONSUMPTION = Real address, no meter, water flowing, no billing
--   FRAUDULENT_ACCOUNT    = Field confirmed: address doesn't exist (GWL staff fraud)

DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'ADDRESS_UNVERIFIED';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'UNMETERED_CONSUMPTION';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'FRAUDULENT_ACCOUNT';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- ── B. Extend fraud_type enum ─────────────────────────────────────────────────
DO $$ BEGIN
    ALTER TYPE fraud_type ADD VALUE IF NOT EXISTS 'UNMETERED_ADDRESS';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE fraud_type ADD VALUE IF NOT EXISTS 'FRAUDULENT_ACCOUNT';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE fraud_type ADD VALUE IF NOT EXISTS 'UNREGISTERED_CONNECTION';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- ── C. Field job outcome enum ─────────────────────────────────────────────────
-- Structured outcomes from field officer visits — drives auto-escalation logic
DO $$ BEGIN
    CREATE TYPE field_job_outcome AS ENUM (
        -- Meter-related outcomes
        'METER_FOUND_OK',               -- Meter present, reading taken, all good
        'METER_FOUND_TAMPERED',         -- Meter present but physically tampered
        'METER_FOUND_FAULTY',           -- Meter present but not recording correctly
        'METER_NOT_FOUND_INSTALL',      -- No meter, address valid → recommend installation
        -- Address-related outcomes
        'ADDRESS_VALID_UNREGISTERED',   -- Real address, consuming water, no GWL account
        'ADDRESS_INVALID',              -- Address does not exist → fraudulent account
        'ADDRESS_DEMOLISHED',           -- Property demolished, account should be closed
        'ACCESS_DENIED',                -- Could not access property, reschedule
        -- Category outcomes
        'CATEGORY_CONFIRMED_CORRECT',   -- Registered category matches actual use
        'CATEGORY_MISMATCH_CONFIRMED',  -- Confirmed commercial use, billed as residential
        -- Other
        'DUPLICATE_METER',              -- Two accounts sharing one physical meter
        'ILLEGAL_CONNECTION_FOUND'      -- Illegal tap/bypass found
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- ── D. Add outcome columns to field_jobs ──────────────────────────────────────
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome              field_job_outcome,
    ADD COLUMN IF NOT EXISTS outcome_notes        TEXT,
    ADD COLUMN IF NOT EXISTS outcome_recorded_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS meter_found          BOOLEAN,
    ADD COLUMN IF NOT EXISTS address_confirmed    BOOLEAN,
    ADD COLUMN IF NOT EXISTS recommended_action   VARCHAR(100);
    -- e.g. 'INSTALL_METER', 'RECLASSIFY_COMMERCIAL', 'CLOSE_ACCOUNT', 'ESCALATE_FRAUD'

CREATE INDEX IF NOT EXISTS idx_field_jobs_outcome ON field_jobs(outcome) WHERE outcome IS NOT NULL;

-- ── E. Add leakage classification to anomaly_flags ────────────────────────────
-- REVENUE_LEAKAGE = GWL is under-collecting money (our primary mission)
-- COMPLIANCE      = GWL is over-billing customers or violating PURC rules
-- DATA_QUALITY    = Possible data error, needs verification before classifying
DO $$ BEGIN
    CREATE TYPE leakage_category AS ENUM (
        'REVENUE_LEAKAGE',   -- GWL under-collecting: shadow bill > GWL bill
        'COMPLIANCE',        -- GWL over-billing or PURC rule violation
        'DATA_QUALITY'       -- Needs field verification before classification
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

ALTER TABLE anomaly_flags
    ADD COLUMN IF NOT EXISTS leakage_category       leakage_category,
    ADD COLUMN IF NOT EXISTS monthly_leakage_ghs    NUMERIC(14,2),  -- GHS/month being lost
    ADD COLUMN IF NOT EXISTS annualised_leakage_ghs NUMERIC(14,2),  -- monthly × 12
    ADD COLUMN IF NOT EXISTS field_job_id           UUID REFERENCES field_jobs(id),
    ADD COLUMN IF NOT EXISTS field_outcome          field_job_outcome,
    ADD COLUMN IF NOT EXISTS confirmed_leakage_ghs  NUMERIC(14,2);  -- set after field confirmation

-- Migrate existing estimated_loss_ghs → monthly_leakage_ghs for REVENUE_LEAKAGE flags
UPDATE anomaly_flags
SET monthly_leakage_ghs    = estimated_loss_ghs,
    annualised_leakage_ghs = estimated_loss_ghs * 12,
    leakage_category = CASE
        WHEN anomaly_type IN (
            'SHADOW_BILL_VARIANCE', 'CATEGORY_MISMATCH', 'PHANTOM_METER',
            'DISTRICT_IMBALANCE', 'UNMETERED_CONSUMPTION', 'GHOST_ACCOUNT'
        ) THEN 'REVENUE_LEAKAGE'::leakage_category
        WHEN anomaly_type IN ('OUTAGE_CONSUMPTION') THEN 'COMPLIANCE'::leakage_category
        ELSE 'DATA_QUALITY'::leakage_category
    END
WHERE estimated_loss_ghs IS NOT NULL AND monthly_leakage_ghs IS NULL;

-- Reclassify existing GHOST_ACCOUNT flags as DATA_QUALITY (they need field verification)
UPDATE anomaly_flags
SET leakage_category = 'DATA_QUALITY',
    monthly_leakage_ghs = 0,
    annualised_leakage_ghs = 0
WHERE anomaly_type = 'GHOST_ACCOUNT'
  AND leakage_category IS NULL;

CREATE INDEX IF NOT EXISTS idx_anomaly_leakage_cat ON anomaly_flags(leakage_category);
CREATE INDEX IF NOT EXISTS idx_anomaly_monthly_ghs ON anomaly_flags(monthly_leakage_ghs DESC NULLS LAST);

-- ── F. Extend revenue_recovery_events with new recovery types ─────────────────
-- Add UNMETERED_CONSUMPTION and FRAUDULENT_ACCOUNT as valid recovery types
-- (recovery_type is VARCHAR so no enum change needed)
-- Add leakage_source column to link back to the anomaly flag
ALTER TABLE revenue_recovery_events
    ADD COLUMN IF NOT EXISTS anomaly_flag_id    UUID REFERENCES anomaly_flags(id),
    ADD COLUMN IF NOT EXISTS leakage_category   leakage_category,
    ADD COLUMN IF NOT EXISTS monthly_leakage_ghs NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS detection_date     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS field_verified_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS field_verified_by  UUID REFERENCES users(id);

CREATE INDEX IF NOT EXISTS idx_rre_anomaly_flag ON revenue_recovery_events(anomaly_flag_id);
CREATE INDEX IF NOT EXISTS idx_rre_leakage_cat  ON revenue_recovery_events(leakage_category);

-- ── G. Auto-create revenue_recovery_event when anomaly confirmed ──────────────
-- Trigger: when anomaly_flags.confirmed_fraud = TRUE and leakage_category = REVENUE_LEAKAGE
-- → automatically insert a revenue_recovery_event in PENDING status
CREATE OR REPLACE FUNCTION fn_auto_create_recovery_event()
RETURNS TRIGGER AS $$
BEGIN
    -- Only fire when confirmed_fraud is set to TRUE for the first time
    -- and the flag is a revenue leakage type
    IF NEW.confirmed_fraud = TRUE
       AND (OLD.confirmed_fraud IS NULL OR OLD.confirmed_fraud = FALSE)
       AND NEW.leakage_category = 'REVENUE_LEAKAGE'
       AND NEW.account_id IS NOT NULL
    THEN
        -- Check no recovery event already exists for this flag
        IF NOT EXISTS (
            SELECT 1 FROM revenue_recovery_events
            WHERE anomaly_flag_id = NEW.id
        ) THEN
            INSERT INTO revenue_recovery_events (
                id,
                anomaly_flag_id,
                audit_event_id,
                district_id,
                account_id,
                variance_ghs,
                recovered_ghs,
                recovery_type,
                leakage_category,
                monthly_leakage_ghs,
                detection_date,
                status,
                notes,
                created_at,
                updated_at
            )
            SELECT
                uuid_generate_v4(),
                NEW.id,
                ae.id,                          -- link to audit event if exists
                NEW.district_id,
                NEW.account_id,
                COALESCE(NEW.monthly_leakage_ghs, NEW.estimated_loss_ghs, 0),
                0,                              -- recovered_ghs starts at 0
                CASE NEW.anomaly_type
                    WHEN 'SHADOW_BILL_VARIANCE'   THEN 'UNDERBILLING'
                    WHEN 'CATEGORY_MISMATCH'      THEN 'CATEGORY_FRAUD'
                    WHEN 'PHANTOM_METER'          THEN 'PHANTOM_METER'
                    WHEN 'DISTRICT_IMBALANCE'     THEN 'UNREGISTERED_CONSUMPTION'
                    WHEN 'UNMETERED_CONSUMPTION'  THEN 'UNMETERED_CONSUMPTION'
                    WHEN 'FRAUDULENT_ACCOUNT'     THEN 'FRAUDULENT_ACCOUNT'
                    ELSE 'UNDERBILLING'
                END,
                NEW.leakage_category,
                NEW.monthly_leakage_ghs,
                NEW.created_at,
                'PENDING',
                'Auto-created from confirmed anomaly flag ' || NEW.id::text,
                NOW(),
                NOW()
            FROM (SELECT id FROM audit_events WHERE anomaly_flag_id = NEW.id LIMIT 1) ae
            -- Use a cross join so the INSERT still fires even if no audit_event exists
            RIGHT JOIN (SELECT 1) dummy ON TRUE;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_auto_recovery_event ON anomaly_flags;
CREATE TRIGGER trg_auto_recovery_event
    AFTER UPDATE ON anomaly_flags
    FOR EACH ROW
    EXECUTE FUNCTION fn_auto_create_recovery_event();

-- ── H. Leakage pipeline view (for dashboard) ──────────────────────────────────
-- Shows the full revenue leakage pipeline in GHS at each stage
CREATE OR REPLACE VIEW v_leakage_pipeline AS
SELECT
    d.id                                                    AS district_id,
    d.district_name,
    -- Stage 1: Detected (all open revenue leakage flags)
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.status = 'OPEN'
    )                                                       AS detected_count,
    COALESCE(SUM(af.monthly_leakage_ghs) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.status = 'OPEN'
    ), 0)                                                   AS detected_monthly_ghs,

    -- Stage 2: Field verified (field job completed, outcome recorded)
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.field_outcome IS NOT NULL
          AND af.confirmed_fraud IS NULL
    )                                                       AS field_verified_count,
    COALESCE(SUM(af.confirmed_leakage_ghs) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.field_outcome IS NOT NULL
    ), 0)                                                   AS field_verified_ghs,

    -- Stage 3: Confirmed fraud (anomaly confirmed, recovery event created)
    COUNT(af.id) FILTER (
        WHERE af.confirmed_fraud = TRUE
          AND af.leakage_category = 'REVENUE_LEAKAGE'
    )                                                       AS confirmed_count,
    COALESCE(SUM(af.confirmed_leakage_ghs) FILTER (
        WHERE af.confirmed_fraud = TRUE
          AND af.leakage_category = 'REVENUE_LEAKAGE'
    ), 0)                                                   AS confirmed_ghs,

    -- Stage 4: GRA signed
    COUNT(rre.id) FILTER (
        WHERE rre.status IN ('GRA_SIGNED', 'COLLECTED')
    )                                                       AS gra_signed_count,
    COALESCE(SUM(rre.recovered_ghs) FILTER (
        WHERE rre.status IN ('GRA_SIGNED', 'COLLECTED')
    ), 0)                                                   AS gra_signed_ghs,

    -- Stage 5: Collected (money actually recovered)
    COUNT(rre.id) FILTER (WHERE rre.status = 'COLLECTED')  AS collected_count,
    COALESCE(SUM(rre.recovered_ghs) FILTER (
        WHERE rre.status = 'COLLECTED'
    ), 0)                                                   AS collected_ghs,

    -- Compliance flags (separate from revenue leakage)
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'COMPLIANCE'
          AND af.status = 'OPEN'
    )                                                       AS compliance_flags_open,

    -- Data quality flags (need field verification)
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'DATA_QUALITY'
          AND af.status = 'OPEN'
    )                                                       AS data_quality_flags_open

FROM districts d
LEFT JOIN anomaly_flags af ON af.district_id = d.id
LEFT JOIN revenue_recovery_events rre ON rre.district_id = d.id
    AND rre.anomaly_flag_id = af.id
WHERE d.is_active = TRUE
GROUP BY d.id, d.district_name;

COMMENT ON VIEW v_leakage_pipeline IS
    'Revenue leakage pipeline by district. Shows GHS at each stage: '
    'Detected → Field Verified → Confirmed → GRA Signed → Collected. '
    'Compliance flags (OUTAGE_CONSUMPTION) are shown separately.';

-- ── I. Update field_jobs status enum to include OUTCOME_RECORDED ──────────────
DO $$ BEGIN
    ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'OUTCOME_RECORDED';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;
