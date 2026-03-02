-- ============================================================
-- GN-WAAS Migration 010: Schema Corrections
-- Description:
--   1. Create audit_ref_seq sequence (used by audit_event_repo.go)
--   2. Create job_ref_seq sequence (for future use)
--   3. Make field_jobs.assigned_officer_id nullable
--      (QUEUED jobs have no officer yet; officer is set on ASSIGN)
--   4. Change field_jobs.status default from 'ASSIGNED' to 'QUEUED'
--      (matches the code's initial state for newly created jobs)
-- ============================================================

-- ── 1. Sequences for human-readable references ───────────────────────────────
-- audit_event_repo.go uses: nextval('audit_ref_seq')
CREATE SEQUENCE IF NOT EXISTS audit_ref_seq
    START WITH 1000
    INCREMENT BY 1
    NO MAXVALUE
    CACHE 10;

-- field_job_repo.go uses fmt.Sprintf for now, but sequence is ready for future use
CREATE SEQUENCE IF NOT EXISTS job_ref_seq
    START WITH 1000
    INCREMENT BY 1
    NO MAXVALUE
    CACHE 10;

-- ── 2. Make field_jobs.assigned_officer_id nullable ──────────────────────────
-- A QUEUED job has not yet been assigned to an officer.
-- The NOT NULL constraint was incorrect for the QUEUED state.
-- Officer is populated when the job transitions to ASSIGNED.
ALTER TABLE field_jobs
    ALTER COLUMN assigned_officer_id DROP NOT NULL;

-- ── 3. Change field_jobs.status default to QUEUED ────────────────────────────
-- The code sets status = 'QUEUED' on creation.
-- The SQL default was 'ASSIGNED' which was inconsistent.
ALTER TABLE field_jobs
    ALTER COLUMN status SET DEFAULT 'QUEUED';

-- ── 4. Add index for QUEUED jobs (common query pattern) ──────────────────────
CREATE INDEX IF NOT EXISTS idx_field_jobs_queued
    ON field_jobs(district_id, created_at DESC)
    WHERE status = 'QUEUED';
