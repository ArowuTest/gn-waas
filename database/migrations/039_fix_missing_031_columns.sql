-- Migration 039: Ensure migration 031 columns exist (idempotent re-apply)
-- This migration safely re-applies the leakage_category columns from 031
-- in case 031 failed silently on the production database.

-- Create leakage_category ENUM if it doesn't exist
DO $$ BEGIN
    CREATE TYPE leakage_category AS ENUM (
        'REVENUE_LEAKAGE',
        'COMPLIANCE',
        'DATA_QUALITY'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- Create field_job_outcome ENUM if it doesn't exist
DO $$ BEGIN
    CREATE TYPE field_job_outcome AS ENUM (
        'CONFIRMED_FRAUD',
        'CONFIRMED_LEAK',
        'FALSE_POSITIVE',
        'NEEDS_ESCALATION',
        'METER_REPLACED',
        'ACCOUNT_TERMINATED',
        'PENDING_REVIEW'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- Add columns to anomaly_flags if they don't exist
ALTER TABLE anomaly_flags
    ADD COLUMN IF NOT EXISTS leakage_category       leakage_category,
    ADD COLUMN IF NOT EXISTS monthly_leakage_ghs    NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS annualised_leakage_ghs NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS field_job_id           UUID REFERENCES field_jobs(id),
    ADD COLUMN IF NOT EXISTS field_outcome          field_job_outcome,
    ADD COLUMN IF NOT EXISTS confirmed_leakage_ghs  NUMERIC(14,2);

-- Add OUTCOME_RECORDED to field_job_status if not exists
DO $$ BEGIN
    ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'OUTCOME_RECORDED';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- Add ADDRESS_UNVERIFIED, UNMETERED_CONSUMPTION, FRAUDULENT_ACCOUNT to anomaly_type
DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'ADDRESS_UNVERIFIED';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'UNMETERED_CONSUMPTION';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

DO $$ BEGIN
    ALTER TYPE anomaly_type ADD VALUE IF NOT EXISTS 'FRAUDULENT_ACCOUNT';
EXCEPTION WHEN duplicate_object THEN NULL; END$$;

-- Backfill leakage_category for existing anomaly_flags
UPDATE anomaly_flags
SET monthly_leakage_ghs    = COALESCE(monthly_leakage_ghs, estimated_loss_ghs),
    annualised_leakage_ghs = COALESCE(annualised_leakage_ghs, estimated_loss_ghs * 12),
    leakage_category = COALESCE(leakage_category, CASE
        WHEN anomaly_type::text IN (
            'SHADOW_BILL_VARIANCE', 'CATEGORY_MISMATCH', 'PHANTOM_METER',
            'DISTRICT_IMBALANCE', 'UNMETERED_CONSUMPTION', 'GHOST_ACCOUNT',
            'ADDRESS_UNVERIFIED', 'FRAUDULENT_ACCOUNT'
        ) THEN 'REVENUE_LEAKAGE'::leakage_category
        WHEN anomaly_type::text IN ('OUTAGE_CONSUMPTION') THEN 'COMPLIANCE'::leakage_category
        ELSE 'DATA_QUALITY'::leakage_category
    END)
WHERE leakage_category IS NULL;

-- Add columns to revenue_recovery_events if they don't exist
ALTER TABLE revenue_recovery_events
    ADD COLUMN IF NOT EXISTS leakage_category   leakage_category,
    ADD COLUMN IF NOT EXISTS monthly_leakage_ghs NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS detection_date     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS field_verified_at  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS field_verified_by  UUID REFERENCES users(id);

-- Add outcome columns to field_jobs if they don't exist
ALTER TABLE field_jobs
    ADD COLUMN IF NOT EXISTS outcome            field_job_outcome,
    ADD COLUMN IF NOT EXISTS outcome_notes      TEXT,
    ADD COLUMN IF NOT EXISTS outcome_recorded_at TIMESTAMPTZ;

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_anomaly_leakage_cat ON anomaly_flags(leakage_category);
CREATE INDEX IF NOT EXISTS idx_anomaly_monthly_ghs ON anomaly_flags(monthly_leakage_ghs DESC NULLS LAST);
