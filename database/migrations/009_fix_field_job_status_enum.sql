-- ============================================================
-- GN-WAAS Migration: 009_fix_field_job_status_enum
-- Description: Add QUEUED and SOS to field_job_status enum.
--
--   QUEUED  — job created but not yet assigned to an officer
--   SOS     — officer triggered emergency alert while on-site
--
-- These values are used throughout the backend (field_job_repo.go,
-- audit_handler.go) and the Flutter mobile app. Without them the
-- DB rejects inserts/updates with an invalid-enum-value error.
-- ============================================================

BEGIN;

-- PostgreSQL requires ALTER TYPE … ADD VALUE outside a transaction
-- for older versions, but from PG 12+ it is allowed inside one.
-- We use IF NOT EXISTS to make this migration idempotent.

ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'QUEUED'  BEFORE 'ASSIGNED';
ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'SOS'     AFTER  'ESCALATED';

COMMIT;
