-- Migration 042: Fix field_job_outcome enum values
-- ─────────────────────────────────────────────────────────────────────────────
-- Problem: Migration 039 created field_job_outcome with incorrect values
-- (CONFIRMED_FRAUD, CONFIRMED_LEAK, FALSE_POSITIVE, etc.) when migration 031
-- failed silently on some deployments. The handler uses the correct values
-- from 031 (METER_FOUND_OK, METER_FOUND_TAMPERED, etc.) causing a cast error.
--
-- Fix: Add all missing enum values from migration 031 to field_job_outcome.
-- ALTER TYPE ... ADD VALUE is idempotent when wrapped in a DO block.
-- ─────────────────────────────────────────────────────────────────────────────

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'METER_FOUND_OK') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'METER_FOUND_OK';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'METER_FOUND_TAMPERED') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'METER_FOUND_TAMPERED';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'METER_FOUND_FAULTY') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'METER_FOUND_FAULTY';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'METER_NOT_FOUND_INSTALL') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'METER_NOT_FOUND_INSTALL';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'ADDRESS_VALID_UNREGISTERED') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'ADDRESS_VALID_UNREGISTERED';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'ADDRESS_INVALID') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'ADDRESS_INVALID';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'ADDRESS_DEMOLISHED') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'ADDRESS_DEMOLISHED';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'ACCESS_DENIED') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'ACCESS_DENIED';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'CATEGORY_CONFIRMED_CORRECT') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'CATEGORY_CONFIRMED_CORRECT';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'CATEGORY_MISMATCH_CONFIRMED') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'CATEGORY_MISMATCH_CONFIRMED';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'DUPLICATE_METER') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'DUPLICATE_METER';
    END IF;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_enum
        WHERE enumtypid = 'field_job_outcome'::regtype
          AND enumlabel = 'ILLEGAL_CONNECTION_FOUND') THEN
        ALTER TYPE field_job_outcome ADD VALUE 'ILLEGAL_CONNECTION_FOUND';
    END IF;
END $$;

-- Also ensure the outcome columns exist on field_jobs (031 may have failed)
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome              field_job_outcome,
    ADD COLUMN IF NOT EXISTS outcome_notes        TEXT,
    ADD COLUMN IF NOT EXISTS outcome_recorded_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS meter_found          BOOLEAN,
    ADD COLUMN IF NOT EXISTS address_confirmed    BOOLEAN,
    ADD COLUMN IF NOT EXISTS recommended_action   VARCHAR(100);

-- Ensure field_outcome column exists on anomaly_flags
ALTER TABLE anomaly_flags
    ADD COLUMN IF NOT EXISTS field_outcome field_job_outcome;

CREATE INDEX IF NOT EXISTS idx_field_jobs_outcome
    ON field_jobs(outcome) WHERE outcome IS NOT NULL;
