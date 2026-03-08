-- Migration 032: Revenue Recovery Pipeline Integrity
-- ============================================================
-- Fixes structural issues found during end-to-end pipeline audit:
--
--   A. revenue_recovery_events.audit_event_id was NOT NULL — but recovery
--      events are created at anomaly confirmation time, before an audit_event
--      necessarily exists. Make it nullable.
--
--   B. revenue_recovery_events.account_id was NOT NULL — but district-level
--      flags (DISTRICT_IMBALANCE, NIGHT_FLOW_ANOMALY) have no account_id.
--      Make it nullable so district-level leakage can be tracked.
--
--   C. Add 'GRA_SIGNED' and 'FIELD_VERIFIED' as valid status values for
--      revenue_recovery_events (status is VARCHAR so no enum change needed,
--      but we document the full state machine here).
--
--   D. Fix v_leakage_pipeline view:
--      - Stage 2 (Field Verified) GHS: use COALESCE(confirmed_leakage_ghs,
--        monthly_leakage_ghs, estimated_loss_ghs) so it's never zero
--      - Stage 2 count: include flags that have field_outcome but are not
--        yet confirmed (confirmed_fraud IS NULL OR confirmed_fraud = FALSE)
--      - Stage 4 (GRA Signed): use status = 'GRA_SIGNED' (not CONFIRMED)
--
--   E. Add index on revenue_recovery_events(status) for pipeline queries.
--
--   F. Add fn_update_recovery_on_gra_sign() trigger: when audit_events
--      gra_status is updated to 'SIGNED', automatically update the linked
--      revenue_recovery_event to 'GRA_SIGNED'.
--
-- ============================================================

-- ── A. Make audit_event_id nullable ──────────────────────────────────────────
-- Recovery events are created at anomaly confirmation, before audit_event exists.
ALTER TABLE revenue_recovery_events
    ALTER COLUMN audit_event_id DROP NOT NULL;

-- ── B. Make account_id nullable ───────────────────────────────────────────────
-- District-level leakage flags (DISTRICT_IMBALANCE) have no account_id.
ALTER TABLE revenue_recovery_events
    ALTER COLUMN account_id DROP NOT NULL;

-- ── C. Document the full status state machine ─────────────────────────────────
-- revenue_recovery_events.status valid values (VARCHAR, not enum):
--   PENDING        → created, awaiting field verification
--   FIELD_VERIFIED → field officer confirmed leakage on-site
--   CONFIRMED      → audit manager confirmed, recovery amount set
--   GRA_SIGNED     → GRA VSDC QR code received, legally binding
--   COLLECTED      → money actually recovered from customer
--   DISPUTED       → customer disputes the finding
COMMENT ON COLUMN revenue_recovery_events.status IS
    'State machine: PENDING → FIELD_VERIFIED → CONFIRMED → GRA_SIGNED → COLLECTED | DISPUTED';

-- ── D. Rebuild v_leakage_pipeline with corrected logic ───────────────────────
CREATE OR REPLACE VIEW v_leakage_pipeline AS
SELECT
    d.id                                                        AS district_id,
    d.district_name,

    -- ── Stage 1: Detected ────────────────────────────────────────────────────
    -- All open REVENUE_LEAKAGE flags (not yet field-verified or confirmed)
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.status = 'OPEN'
          AND af.field_outcome IS NULL
    )                                                           AS detected_count,
    COALESCE(SUM(af.monthly_leakage_ghs) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.status = 'OPEN'
          AND af.field_outcome IS NULL
    ), 0)                                                       AS detected_monthly_ghs,

    -- ── Stage 2: Field Verified ───────────────────────────────────────────────
    -- Field officer visited and recorded outcome; not yet confirmed by manager
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.field_outcome IS NOT NULL
          AND (af.confirmed_fraud IS NULL OR af.confirmed_fraud = FALSE)
          AND af.status NOT IN ('FALSE_POSITIVE', 'RESOLVED')
    )                                                           AS field_verified_count,
    COALESCE(SUM(
        COALESCE(af.confirmed_leakage_ghs, af.monthly_leakage_ghs, af.estimated_loss_ghs, 0)
    ) FILTER (
        WHERE af.leakage_category = 'REVENUE_LEAKAGE'
          AND af.field_outcome IS NOT NULL
          AND (af.confirmed_fraud IS NULL OR af.confirmed_fraud = FALSE)
          AND af.status NOT IN ('FALSE_POSITIVE', 'RESOLVED')
    ), 0)                                                       AS field_verified_ghs,

    -- ── Stage 3: Confirmed ────────────────────────────────────────────────────
    -- Audit manager confirmed fraud; recovery event created
    COUNT(af.id) FILTER (
        WHERE af.confirmed_fraud = TRUE
          AND af.leakage_category = 'REVENUE_LEAKAGE'
    )                                                           AS confirmed_count,
    COALESCE(SUM(af.confirmed_leakage_ghs) FILTER (
        WHERE af.confirmed_fraud = TRUE
          AND af.leakage_category = 'REVENUE_LEAKAGE'
    ), 0)                                                       AS confirmed_ghs,

    -- ── Stage 4: GRA Signed ───────────────────────────────────────────────────
    -- GRA VSDC QR code received; legally binding audit report
    COUNT(rre.id) FILTER (
        WHERE rre.status IN ('GRA_SIGNED', 'COLLECTED')
    )                                                           AS gra_signed_count,
    COALESCE(SUM(rre.recovered_ghs) FILTER (
        WHERE rre.status IN ('GRA_SIGNED', 'COLLECTED')
    ), 0)                                                       AS gra_signed_ghs,

    -- ── Stage 5: Collected ────────────────────────────────────────────────────
    -- Money actually recovered from the customer
    COUNT(rre.id) FILTER (WHERE rre.status = 'COLLECTED')      AS collected_count,
    COALESCE(SUM(rre.recovered_ghs) FILTER (
        WHERE rre.status = 'COLLECTED'
    ), 0)                                                       AS collected_ghs,

    -- ── Compliance flags (separate — PURC violations, not revenue leakage) ───
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'COMPLIANCE'
          AND af.status = 'OPEN'
    )                                                           AS compliance_flags_open,

    -- ── Data quality flags (need field verification) ──────────────────────────
    COUNT(af.id) FILTER (
        WHERE af.leakage_category = 'DATA_QUALITY'
          AND af.status = 'OPEN'
    )                                                           AS data_quality_flags_open

FROM districts d
LEFT JOIN anomaly_flags af ON af.district_id = d.id
LEFT JOIN revenue_recovery_events rre
    ON  rre.district_id    = d.id
    AND rre.anomaly_flag_id = af.id
WHERE d.is_active = TRUE
GROUP BY d.id, d.district_name;

COMMENT ON VIEW v_leakage_pipeline IS
    'Revenue leakage pipeline by district. '
    'Stage 1 (Detected): open flags with no field visit. '
    'Stage 2 (Field Verified): field outcome recorded, not yet confirmed. '
    'Stage 3 (Confirmed): fraud confirmed by audit manager. '
    'Stage 4 (GRA Signed): GRA VSDC QR code received. '
    'Stage 5 (Collected): money recovered. '
    'Compliance and data quality flags shown separately.';

-- ── E. Index for pipeline status queries ─────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_rre_status ON revenue_recovery_events(status);
CREATE INDEX IF NOT EXISTS idx_af_field_outcome ON anomaly_flags(field_outcome)
    WHERE field_outcome IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_af_confirmed_fraud ON anomaly_flags(confirmed_fraud)
    WHERE confirmed_fraud = TRUE;
CREATE INDEX IF NOT EXISTS idx_af_leakage_cat_status
    ON anomaly_flags(leakage_category, status);

-- ── F. Trigger: GRA sign → update recovery event to GRA_SIGNED ───────────────
-- When an audit_event's gra_status is updated to 'SIGNED', automatically
-- update the linked revenue_recovery_event to 'GRA_SIGNED'.
-- This closes the loop between the GRA bridge and the revenue pipeline.
CREATE OR REPLACE FUNCTION fn_update_recovery_on_gra_sign()
RETURNS TRIGGER AS $$
BEGIN
    -- Only fire when gra_status transitions to SIGNED
    IF NEW.gra_status = 'SIGNED'
       AND (OLD.gra_status IS NULL OR OLD.gra_status <> 'SIGNED')
    THEN
        UPDATE revenue_recovery_events
        SET status     = 'GRA_SIGNED',
            updated_at = NOW()
        WHERE anomaly_flag_id = NEW.anomaly_flag_id
          AND status IN ('PENDING', 'FIELD_VERIFIED', 'CONFIRMED');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gra_sign_recovery ON audit_events;
CREATE TRIGGER trg_gra_sign_recovery
    AFTER UPDATE OF gra_status ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION fn_update_recovery_on_gra_sign();

-- ── G. Fix the migration 031 trigger to handle nullable audit_event_id ────────
-- Recreate fn_auto_create_recovery_event without the NOT NULL violation.
CREATE OR REPLACE FUNCTION fn_auto_create_recovery_event()
RETURNS TRIGGER AS $$
BEGIN
    -- Only fire when confirmed_fraud is set to TRUE for the first time
    -- and the flag is a revenue leakage type
    IF NEW.confirmed_fraud = TRUE
       AND (OLD.confirmed_fraud IS NULL OR OLD.confirmed_fraud = FALSE)
       AND NEW.leakage_category = 'REVENUE_LEAKAGE'
    THEN
        -- Check no recovery event already exists for this flag
        IF NOT EXISTS (
            SELECT 1 FROM revenue_recovery_events
            WHERE anomaly_flag_id = NEW.id
        ) THEN
            INSERT INTO revenue_recovery_events (
                id,
                anomaly_flag_id,
                audit_event_id,        -- nullable: linked if audit_event exists
                district_id,
                account_id,            -- nullable: NULL for district-level flags
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
            VALUES (
                uuid_generate_v4(),
                NEW.id,
                (SELECT id FROM audit_events WHERE anomaly_flag_id = NEW.id LIMIT 1),
                NEW.district_id,
                NEW.account_id,        -- may be NULL for district-level flags
                COALESCE(NEW.confirmed_leakage_ghs, NEW.monthly_leakage_ghs, NEW.estimated_loss_ghs, 0),
                0,                     -- recovered_ghs starts at 0
                CASE NEW.anomaly_type
                    WHEN 'SHADOW_BILL_VARIANCE'   THEN 'UNDERBILLING'
                    WHEN 'CATEGORY_MISMATCH'      THEN 'CATEGORY_FRAUD'
                    WHEN 'PHANTOM_METER'          THEN 'PHANTOM_METER'
                    WHEN 'DISTRICT_IMBALANCE'     THEN 'UNREGISTERED_CONSUMPTION'
                    WHEN 'UNMETERED_CONSUMPTION'  THEN 'UNMETERED_CONSUMPTION'
                    WHEN 'FRAUDULENT_ACCOUNT'     THEN 'FRAUDULENT_ACCOUNT'
                    WHEN 'UNAUTHORISED_CONSUMPTION' THEN 'UNREGISTERED_CONSUMPTION'
                    WHEN 'METERING_INACCURACY'    THEN 'UNDERBILLING'
                    WHEN 'VAT_DISCREPANCY'        THEN 'UNDERBILLING'
                    ELSE 'UNDERBILLING'
                END,
                NEW.leakage_category,
                NEW.monthly_leakage_ghs,
                NEW.created_at,
                'PENDING',
                'Auto-created from confirmed anomaly flag ' || NEW.id::text,
                NOW(),
                NOW()
            );
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Re-attach the trigger (DROP IF EXISTS + CREATE is idempotent)
DROP TRIGGER IF EXISTS trg_auto_recovery_event ON anomaly_flags;
CREATE TRIGGER trg_auto_recovery_event
    AFTER UPDATE ON anomaly_flags
    FOR EACH ROW
    EXECUTE FUNCTION fn_auto_create_recovery_event();
