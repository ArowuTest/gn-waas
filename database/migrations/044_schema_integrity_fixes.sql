-- ============================================================
-- Migration 044: Schema Integrity Fixes
-- Fixes three schema gaps identified in code review:
--
-- 1. ORPHANED ENUM VALUES (field_job_outcome)
--    Migration 039 created field_job_outcome with a wrong value set
--    ('CONFIRMED_FRAUD', 'CONFIRMED_LEAK', etc.) when it should have
--    matched migration 031's canonical values. On a fresh DB, both
--    value sets end up in the type. Migration 042 added the correct
--    031 values; this migration documents the orphaned 039-era values
--    and renames them to NO_OP variants so existing rows are not
--    broken, then maps them to the canonical equivalents.
--    NOTE: PostgreSQL 14+ required for ALTER TYPE ... RENAME VALUE.
--    On PostgreSQL 13 this block is skipped safely via the DO guard.
--
-- 2. gwl_status DEFAULT 'PENDING_REVIEW'
--    anomaly_flags.gwl_status has no DEFAULT, causing COALESCE fallbacks
--    throughout the codebase. Add a proper default.
--
-- 3. anomaly_flags.field_job_id FK — ON DELETE SET NULL
--    The FK to field_jobs had no ON DELETE clause; dropping/cleaning
--    a field_job row would violate the constraint. This migration
--    re-creates the FK with ON DELETE SET NULL.
-- ============================================================

-- ── 1. Rename orphaned enum values (PostgreSQL 14+ only) ─────────────────────
DO $$
DECLARE
    pg_major INT := current_setting('server_version_num')::INT / 10000;
BEGIN
    IF pg_major >= 14 THEN
        -- Map 039-era values to canonical 031/042 equivalents.
        -- Only rename if the orphaned value still exists (idempotent).
        IF EXISTS (
            SELECT 1 FROM pg_enum e
            JOIN pg_type t ON t.oid = e.enumtypid
            WHERE t.typname = 'field_job_outcome'
              AND e.enumlabel = 'CONFIRMED_FRAUD'
        ) THEN
            ALTER TYPE field_job_outcome
                RENAME VALUE 'CONFIRMED_FRAUD'       TO 'FRAUDULENT_ACCOUNT_CONFIRMED';
        END IF;

        IF EXISTS (
            SELECT 1 FROM pg_enum e
            JOIN pg_type t ON t.oid = e.enumtypid
            WHERE t.typname = 'field_job_outcome'
              AND e.enumlabel = 'CONFIRMED_LEAK'
        ) THEN
            ALTER TYPE field_job_outcome
                RENAME VALUE 'CONFIRMED_LEAK'        TO 'CONFIRMED_LEAK_PHYSICAL';
        END IF;

        IF EXISTS (
            SELECT 1 FROM pg_enum e
            JOIN pg_type t ON t.oid = e.enumtypid
            WHERE t.typname = 'field_job_outcome'
              AND e.enumlabel = 'ACCOUNT_TERMINATED'
        ) THEN
            ALTER TYPE field_job_outcome
                RENAME VALUE 'ACCOUNT_TERMINATED'   TO 'ACCOUNT_TERMINATED_LEGACY';
        END IF;
    END IF;
END$$;

-- ── 2. Add DEFAULT to gwl_status ──────────────────────────────────────────────
-- anomaly_flags.gwl_status: add default so new rows always have a status.
-- The existing COALESCE('PENDING_REVIEW') fallbacks remain as a safety net
-- but will become no-ops once this default is active.
ALTER TABLE anomaly_flags
    ALTER COLUMN gwl_status SET DEFAULT 'PENDING_REVIEW';

-- Back-fill any existing NULL gwl_status rows.
UPDATE anomaly_flags
    SET gwl_status = 'PENDING_REVIEW'
WHERE gwl_status IS NULL;

COMMENT ON COLUMN anomaly_flags.gwl_status IS
    'GWL workflow status. Defaults to PENDING_REVIEW on insert. '
    'Valid values mirror the gwl_case_status enum used in gwl_cases.';

-- ── 3. Re-create field_job_id FK with ON DELETE SET NULL ──────────────────────
-- Drop the existing FK (created in migration 031/039 without ON DELETE).
ALTER TABLE anomaly_flags
    DROP CONSTRAINT IF EXISTS anomaly_flags_field_job_id_fkey;

-- Re-add with ON DELETE SET NULL so field_job deletion does not block.
ALTER TABLE anomaly_flags
    ADD CONSTRAINT anomaly_flags_field_job_id_fkey
        FOREIGN KEY (field_job_id)
        REFERENCES field_jobs(id)
        ON DELETE SET NULL;

COMMENT ON CONSTRAINT anomaly_flags_field_job_id_fkey ON anomaly_flags IS
    'FK to field_jobs. ON DELETE SET NULL ensures a deleted/cancelled job '
    'does not cascade-block the anomaly flag record.';
