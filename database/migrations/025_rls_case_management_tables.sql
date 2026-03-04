-- ============================================================
-- Migration 025: RLS for Case Management Tables
-- ============================================================
-- Adds Row-Level Security policies to three district-scoped
-- case management tables that were missing RLS coverage:
--
--   • gwl_case_actions       — audit trail of case workflow steps
--   • reclassification_requests — category change requests
--   • credit_requests        — overbilling credit requests
--
-- All three tables have a district_id column and are accessed
-- via gwl_case_repo.go which uses r.q(ctx) (RLS-activated
-- transaction). Adding explicit RLS policies provides
-- defence-in-depth: even a direct query on these tables will
-- be district-scoped.
--
-- Pattern mirrors migration 012 (existing RLS tables):
--   USING (current_user_is_admin() OR district_id = current_district_id())
--
-- current_user_is_admin() returns TRUE for SYSTEM_ADMIN / SUPER_ADMIN /
-- MOF_AUDITOR roles (set via SET LOCAL app.user_role).
-- current_district_id() returns the UUID from SET LOCAL app.district_id.
-- Both helper functions are defined in migration 012.
-- ============================================================

-- ─── gwl_case_actions ────────────────────────────────────────────────────────
-- Stores every workflow action taken on a GWL anomaly case.
-- district_id is nullable (NULL = system-generated action); the policy
-- allows NULL rows to be visible to admins only.
ALTER TABLE gwl_case_actions ENABLE ROW LEVEL SECURITY;

-- Admins see all rows; district users see only their district's rows.
-- Rows with NULL district_id are visible to admins only (system actions).
CREATE POLICY rls_gwl_case_actions_district ON gwl_case_actions
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- ─── reclassification_requests ───────────────────────────────────────────────
-- Category change requests raised by GWL billing officers.
-- district_id is NOT NULL (enforced by schema).
ALTER TABLE reclassification_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY rls_reclassification_requests_district ON reclassification_requests
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );

-- ─── credit_requests ─────────────────────────────────────────────────────────
-- Overbilling credit requests raised against a specific gwl_bill.
-- district_id is NOT NULL (enforced by schema).
ALTER TABLE credit_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY rls_credit_requests_district ON credit_requests
    USING (
        current_user_is_admin()
        OR district_id = current_district_id()
    );
