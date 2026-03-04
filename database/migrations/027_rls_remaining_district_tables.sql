-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 027: RLS for remaining district-scoped tables
-- ─────────────────────────────────────────────────────────────────────────────
-- Adds Row-Level Security to the five district-scoped tables that were missed
-- in earlier migrations:
--   • audit_thresholds      — per-district sentinel thresholds (NULL = global)
--   • data_confidence_scores — AWWA grading per district/period
--   • notifications          — district-scoped broadcast messages
--   • recovery_records       — revenue recovery per district
--   • supply_schedules       — supply rationing schedules per district
--
-- Pattern mirrors migrations 012, 024, 025, 026:
--   FORCE ROW LEVEL SECURITY  — applies even to table owner
--   FOR ALL TO gnwaas_app     — policy applies only to the app role
--   current_user_is_admin()   — SYSTEM_ADMIN / SUPER_ADMIN / MOF_AUDITOR bypass
--   current_district_id()     — district_id from SET LOCAL app.district_id
--
-- audit_thresholds has district_id NULLABLE (NULL = global default).
-- The policy allows access when district_id IS NULL (global row visible to all)
-- OR district_id matches the session district, OR the user is an admin.
-- ─────────────────────────────────────────────────────────────────────────────

-- ── 1. audit_thresholds ───────────────────────────────────────────────────────
ALTER TABLE audit_thresholds ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_thresholds FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_thresholds_district_isolation ON audit_thresholds;
CREATE POLICY audit_thresholds_district_isolation ON audit_thresholds
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id IS NULL                          -- global defaults visible to all
        OR district_id = current_district_id()
    );

-- ── 2. data_confidence_scores ─────────────────────────────────────────────────
ALTER TABLE data_confidence_scores ENABLE ROW LEVEL SECURITY;
ALTER TABLE data_confidence_scores FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS data_confidence_scores_district_isolation ON data_confidence_scores;
CREATE POLICY data_confidence_scores_district_isolation ON data_confidence_scores
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- ── 3. notifications ──────────────────────────────────────────────────────────
-- Notifications can be:
--   a) user-specific  (user_id IS NOT NULL, district_id may be NULL)
--   b) district-scoped broadcast (district_id IS NOT NULL, user_id may be NULL)
--   c) system-wide    (both NULL — visible to all)
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS notifications_district_isolation ON notifications;
CREATE POLICY notifications_district_isolation ON notifications
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id IS NULL                          -- system-wide or user-specific
        OR district_id = current_district_id()
    );

-- ── 4. recovery_records ───────────────────────────────────────────────────────
ALTER TABLE recovery_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE recovery_records FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS recovery_records_district_isolation ON recovery_records;
CREATE POLICY recovery_records_district_isolation ON recovery_records
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- ── 5. supply_schedules ───────────────────────────────────────────────────────
ALTER TABLE supply_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE supply_schedules FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS supply_schedules_district_isolation ON supply_schedules;
CREATE POLICY supply_schedules_district_isolation ON supply_schedules
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );
