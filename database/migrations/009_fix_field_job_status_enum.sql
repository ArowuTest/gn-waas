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
--
-- V19-DB-05 fix: PostgreSQL does NOT allow ALTER TYPE … ADD VALUE
-- inside a transaction block (BEGIN/COMMIT). The original migration
-- wrapped these statements in a transaction, which causes:
--   ERROR: ALTER TYPE ... ADD VALUE cannot run inside a transaction block
-- The fix is to run them outside any transaction. IF NOT EXISTS makes
-- this migration safe to re-run (idempotent).
-- ============================================================

-- Must run outside a transaction block (no BEGIN/COMMIT wrapper).
ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'QUEUED'  BEFORE 'ASSIGNED';
ALTER TYPE field_job_status ADD VALUE IF NOT EXISTS 'SOS'     AFTER  'ESCALATED';
