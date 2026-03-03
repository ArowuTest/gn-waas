-- Migration 011: Immutable Audit Trail + Database Hardening
-- ============================================================
-- C3: Make the audit_trail table truly immutable by adding
--     BEFORE UPDATE and BEFORE DELETE triggers that raise
--     exceptions, preventing any modification or deletion of
--     audit records.
--
-- Also hardens audit_events and anomaly_flags with similar
-- protections since they are also compliance-critical records.

-- ─────────────────────────────────────────────────────────────
-- 1. audit_trail table — defined in migration 004
-- ─────────────────────────────────────────────────────────────
-- DB-H01 fix: The audit_trail table is created with the correct schema
-- (UUID primary key, entity_id TEXT) in migration 004_sentinel_and_audit.sql.
-- The CREATE TABLE IF NOT EXISTS block that was here previously conflicted
-- with 004's definition (BIGSERIAL PK vs UUID PK, entity_id UUID vs TEXT),
-- causing schema inconsistency. Since 004 always runs first and creates the
-- table, this block is removed. The indexes below are kept with IF NOT EXISTS
-- guards so they are safe to run even if 004 already created them.

-- Indexes for fast lookups (IF NOT EXISTS guards prevent duplicate-index errors)
CREATE INDEX IF NOT EXISTS idx_audit_trail_entity
    ON audit_trail (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_trail_actor
    ON audit_trail (changed_by);
CREATE INDEX IF NOT EXISTS idx_audit_trail_created
    ON audit_trail (changed_at DESC);

-- ─────────────────────────────────────────────────────────────
-- 2. Immutability trigger function
--    Raises an exception on any UPDATE or DELETE attempt.
--    This is the database-level enforcement — no application
--    code can bypass it, even with direct DB access.
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION fn_prevent_audit_modification()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER  -- runs as the function owner, not the caller
AS $$
BEGIN
    RAISE EXCEPTION
        'IMMUTABLE_RECORD: % on table % is forbidden. Audit records cannot be modified or deleted. (id=%)',
        TG_OP, TG_TABLE_NAME, OLD.id
        USING ERRCODE = 'restrict_violation';
    RETURN NULL;
END;
$$;

-- ─────────────────────────────────────────────────────────────
-- 3. Apply immutability to audit_trail
-- ─────────────────────────────────────────────────────────────
DROP TRIGGER IF EXISTS trg_audit_trail_immutable ON audit_trail;
CREATE TRIGGER trg_audit_trail_immutable
    BEFORE UPDATE OR DELETE ON audit_trail
    FOR EACH ROW
    EXECUTE FUNCTION fn_prevent_audit_modification();

-- ─────────────────────────────────────────────────────────────
-- 4. Apply immutability to audit_events
--    Once an audit event is COMPLETED or GRA_CONFIRMED it must
--    not be altered. We allow updates only while status is
--    still in a mutable state (PENDING, IN_PROGRESS).
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION fn_prevent_audit_event_modification()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    -- Allow status transitions on non-terminal records
    IF OLD.status IN ('COMPLETED', 'GRA_CONFIRMED', 'CLOSED') THEN
        RAISE EXCEPTION
            'IMMUTABLE_RECORD: audit_event % is in terminal status % and cannot be modified.',
            OLD.id, OLD.status
            USING ERRCODE = 'restrict_violation';
    END IF;
    -- Never allow deletion of audit events
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION
            'IMMUTABLE_RECORD: audit_event % cannot be deleted. Use status=CLOSED instead.',
            OLD.id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_audit_events_immutable ON audit_events;
CREATE TRIGGER trg_audit_events_immutable
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION fn_prevent_audit_event_modification();

-- ─────────────────────────────────────────────────────────────
-- 5. Append-only anomaly_flags
--    Flags can be acknowledged/resolved but never deleted.
-- ─────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION fn_prevent_anomaly_flag_deletion()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    RAISE EXCEPTION
        'IMMUTABLE_RECORD: anomaly_flag % cannot be deleted. Set status=FALSE_POSITIVE instead.',
        OLD.id
        USING ERRCODE = 'restrict_violation';
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_anomaly_flags_no_delete ON anomaly_flags;
CREATE TRIGGER trg_anomaly_flags_no_delete
    BEFORE DELETE ON anomaly_flags
    FOR EACH ROW
    EXECUTE FUNCTION fn_prevent_anomaly_flag_deletion();

-- ─────────────────────────────────────────────────────────────
-- 6. Revoke DELETE privilege on compliance tables from the
--    application role (belt-and-suspenders with the triggers)
-- ─────────────────────────────────────────────────────────────
DO $$
BEGIN
    -- Only revoke if the role exists (won't fail in fresh installs)
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gnwaas_app') THEN
        REVOKE DELETE ON audit_trail   FROM gnwaas_app;
        REVOKE DELETE ON audit_events  FROM gnwaas_app;
        REVOKE DELETE ON anomaly_flags FROM gnwaas_app;
        REVOKE UPDATE ON audit_trail   FROM gnwaas_app;
    END IF;
END;
$$;
