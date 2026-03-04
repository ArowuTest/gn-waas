-- Migration 021: Unify Billing Tables & Fix shadow_bills Constraints
-- ====================================================================
--
-- ARCH-01 fix: The system had two incompatible billing tables:
--   • gwl_billing_records (migration 003) — never populated; sentinel/tariff-engine read from it
--   • gwl_bills           (migration 014) — populated by cdc-ingestor; the correct source of truth
--
-- Strategy: Drop gwl_billing_records as a TABLE and recreate it as a VIEW over gwl_bills.
-- This means:
--   1. All existing sentinel and tariff-engine queries continue to work unchanged.
--   2. cdc-ingestor writes to gwl_bills (as it already does).
--   3. The VIEW exposes the same column names that sentinel/tariff-engine expect.
--
-- DB-01 fix: shadow_bills lacked a UNIQUE constraint on gwl_bill_id, causing
-- the ON CONFLICT (gwl_bill_id) clause in shadow_bill_repo.go to fail with
-- SQLSTATE 42P10 at runtime. This migration adds the constraint and updates
-- the FK to point to gwl_bills.id (the populated table).
--
-- No application code changes are required — the VIEW is transparent.
-- ====================================================================

-- ─── Step 1: Remove the FK from shadow_bills → gwl_billing_records ───────────
-- Must be done before dropping the table.
ALTER TABLE shadow_bills
    DROP CONSTRAINT IF EXISTS shadow_bills_gwl_bill_id_fkey;

-- ─── Step 1b: Remove FKs from anomaly_flags → gwl_billing_records ────────────
-- anomaly_flags.gwl_bill_id was defined in migration 004 as:
--   gwl_bill_id UUID REFERENCES gwl_billing_records(id)
-- PostgreSQL auto-names this FK as anomaly_flags_gwl_bill_id_fkey.
-- Must be dropped before the table can be dropped.
ALTER TABLE anomaly_flags
    DROP CONSTRAINT IF EXISTS anomaly_flags_gwl_bill_id_fkey;

-- ─── Step 1c: Remove FKs from credit_requests → gwl_billing_records ──────────
-- credit_requests.gwl_bill_id was defined in migration 006 as:
--   gwl_bill_id UUID REFERENCES gwl_billing_records(id)
-- PostgreSQL auto-names this FK as credit_requests_gwl_bill_id_fkey.
-- Must be dropped before the table can be dropped.
ALTER TABLE credit_requests
    DROP CONSTRAINT IF EXISTS credit_requests_gwl_bill_id_fkey;

-- ─── Step 2: Drop the empty gwl_billing_records table ────────────────────────
-- Safe: the table was never populated (cdc-ingestor always wrote to gwl_bills).
-- All FKs referencing it have been dropped above.
DROP TABLE IF EXISTS gwl_billing_records;

-- ─── Step 3: Recreate gwl_billing_records as a compatibility VIEW ─────────────
-- Exposes the same columns that sentinel and tariff-engine expect, sourced from
-- the live gwl_bills table. No code changes needed in those services.
--
-- Column mapping:
--   gwl_billing_records.id           → gwl_bills.id          (internal UUID PK)
--   gwl_billing_records.gwl_bill_id  → gwl_bills.id          (same UUID — used as FK target
--                                                              in shadow_bills.gwl_bill_id)
--   gwl_billing_records.account_id   → gwl_bills.account_id
--   gwl_billing_records.billing_*    → gwl_bills.billing_*
--   gwl_billing_records.consumption_m3 → gwl_bills.consumption_m3
--   gwl_billing_records.gwl_category → gwl_bills.gwl_category
--   gwl_billing_records.gwl_amount_ghs → gwl_bills.gwl_amount_ghs
--   gwl_billing_records.gwl_vat_ghs  → gwl_bills.gwl_vat_ghs
--   gwl_billing_records.gwl_total_ghs → gwl_bills.gwl_total_ghs
CREATE OR REPLACE VIEW gwl_billing_records AS
    SELECT
        id,
        id                   AS gwl_bill_id,       -- UUID alias for backward-compat FK joins
        account_id,
        billing_period_start,
        billing_period_end,
        consumption_m3,
        gwl_category,
        gwl_amount_ghs,
        gwl_vat_ghs,
        gwl_total_ghs
    FROM gwl_bills;

COMMENT ON VIEW gwl_billing_records IS
    'Compatibility view over gwl_bills. Allows sentinel and tariff-engine to query '
    'gwl_billing_records without code changes. cdc-ingestor writes to gwl_bills directly. '
    'ARCH-01 fix: migration 021.';

-- ─── Step 4: Add UNIQUE constraint on shadow_bills.gwl_bill_id ───────────────
-- Required for the ON CONFLICT (gwl_bill_id) DO UPDATE clause in
-- shadow_bill_repo.go (tariff-engine). Without this, the first duplicate
-- insert crashes with SQLSTATE 42P10.
ALTER TABLE shadow_bills
    ADD CONSTRAINT shadow_bills_gwl_bill_id_unique
    UNIQUE (gwl_bill_id);

-- ─── Step 5: Add FK from shadow_bills.gwl_bill_id → gwl_bills.id ─────────────
-- Restores referential integrity, now pointing to the populated table.
ALTER TABLE shadow_bills
    ADD CONSTRAINT shadow_bills_gwl_bill_id_fkey
    FOREIGN KEY (gwl_bill_id) REFERENCES gwl_bills(id) ON DELETE CASCADE;

-- ─── Step 6: Restore FKs from anomaly_flags and credit_requests → gwl_bills ──
-- Now that gwl_billing_records is a VIEW (not a table), FKs cannot reference it.
-- Instead, point directly to gwl_bills(id) which is the actual source of truth.
ALTER TABLE anomaly_flags
    ADD CONSTRAINT anomaly_flags_gwl_bill_id_fkey
    FOREIGN KEY (gwl_bill_id) REFERENCES gwl_bills(id) ON DELETE SET NULL;

ALTER TABLE credit_requests
    ADD CONSTRAINT credit_requests_gwl_bill_id_fkey
    FOREIGN KEY (gwl_bill_id) REFERENCES gwl_bills(id) ON DELETE SET NULL;

-- ─── Step 7: Grant DELETE on all tables to gnwaas_app ────────────────────────
-- Migration 012 only granted SELECT, INSERT, UPDATE. DELETE is needed for
-- operations like cancelling field jobs and removing stale records.
-- SEC-01 prerequisite: gnwaas_app must have full DML permissions.
GRANT DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app;

-- ─── Step 8: Grant EXECUTE on all functions to gnwaas_app ────────────────────
-- Needed for RLS helper functions (current_district_id, current_user_is_admin, etc.)
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO gnwaas_app;
