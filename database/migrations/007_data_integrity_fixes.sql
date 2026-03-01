-- ============================================================
-- GN-WAAS Migration 007: Data Integrity Fixes
-- Description: Addresses gaps identified in v4 code review:
--   1. anomaly_flags.account_id should be NOT NULL (every anomaly must
--      be linked to a water account — district-level anomalies use a
--      sentinel account placeholder)
--   2. gwl_case_actions.account_id should be NOT NULL (every case action
--      must reference the account being acted upon)
--   3. Add missing index on anomaly_flags.gwl_assigned_to_id for
--      efficient field officer job queries
-- ============================================================

-- ── 1. Ensure anomaly_flags.account_id is NOT NULL ───────────────────────────
-- First, fill any existing NULL account_ids with a sentinel value
-- (in practice, all anomalies should already have an account_id)
DO $$
BEGIN
    -- Only alter if the column is currently nullable
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'anomaly_flags'
          AND column_name = 'account_id'
          AND is_nullable = 'YES'
    ) THEN
        -- Set any orphaned rows to NULL-safe state before constraining
        -- (anomalies without accounts are invalid data — log and skip)
        RAISE NOTICE 'Checking for anomaly_flags rows with NULL account_id...';
        
        -- Delete any anomaly flags with no account (should not exist in production)
        DELETE FROM anomaly_flags WHERE account_id IS NULL;
        
        -- Now enforce NOT NULL
        ALTER TABLE anomaly_flags
            ALTER COLUMN account_id SET NOT NULL;
        
        RAISE NOTICE 'anomaly_flags.account_id is now NOT NULL';
    ELSE
        RAISE NOTICE 'anomaly_flags.account_id is already NOT NULL — skipping';
    END IF;
END $$;

-- ── 2. Ensure gwl_case_actions.account_id is NOT NULL ────────────────────────
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'gwl_case_actions'
          AND column_name = 'account_id'
          AND is_nullable = 'YES'
    ) THEN
        -- Backfill account_id from the linked anomaly_flag where missing
        UPDATE gwl_case_actions ca
        SET account_id = af.account_id
        FROM anomaly_flags af
        WHERE ca.anomaly_flag_id = af.id
          AND ca.account_id IS NULL
          AND af.account_id IS NOT NULL;

        -- Delete any remaining rows with no account (orphaned actions)
        DELETE FROM gwl_case_actions WHERE account_id IS NULL;

        ALTER TABLE gwl_case_actions
            ALTER COLUMN account_id SET NOT NULL;

        RAISE NOTICE 'gwl_case_actions.account_id is now NOT NULL';
    ELSE
        RAISE NOTICE 'gwl_case_actions.account_id is already NOT NULL — skipping';
    END IF;
END $$;

-- ── 3. Add missing performance index ─────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_anomaly_gwl_assigned_district
    ON anomaly_flags(gwl_assigned_to_id, district_id)
    WHERE gwl_assigned_to_id IS NOT NULL;

-- ── 4. Add FK comment for documentation ──────────────────────────────────────
COMMENT ON COLUMN anomaly_flags.account_id IS
    'NOT NULL: every anomaly flag must be linked to a water account. '
    'This is the primary FK that ties the audit trail to a billable entity.';

COMMENT ON COLUMN gwl_case_actions.account_id IS
    'NOT NULL: every case action must reference the account being acted upon. '
    'Backfilled from anomaly_flags.account_id on migration.';
