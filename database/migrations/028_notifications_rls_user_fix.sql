-- ─────────────────────────────────────────────────────────────────────────────
-- Migration 028: Fix notifications RLS policy (RLS-L01)
-- ─────────────────────────────────────────────────────────────────────────────
-- v27 audit advisory RLS-L01:
--   The notifications policy in migration 027 hides a user-specific notification
--   from its intended recipient when the notification has BOTH user_id SET and
--   district_id SET, and the user's session district differs from the stored
--   district_id.
--
--   Example: a notification created for user X (user_id = X) in district A is
--   invisible to user X when they are operating in district B, because the
--   policy only passes rows where district_id IS NULL OR district_id = session
--   district.
--
-- Fix: extend the USING clause with a third OR branch:
--   OR (user_id IS NOT NULL
--       AND user_id::TEXT = current_setting('app.user_id', TRUE))
--
--   This ensures that any notification explicitly addressed to the current user
--   (by user_id) is always visible to them, regardless of district_id.
--
-- The fix is a DROP + CREATE of the policy only — no schema changes.
-- ─────────────────────────────────────────────────────────────────────────────

DROP POLICY IF EXISTS notifications_district_isolation ON notifications;

CREATE POLICY notifications_district_isolation ON notifications
    FOR ALL TO gnwaas_app
    USING (
        current_user_is_admin()
        OR district_id IS NULL                          -- system-wide broadcasts
        OR district_id = current_district_id()          -- district-scoped broadcasts
        OR (                                            -- RLS-L01 fix: user-specific
            user_id IS NOT NULL
            AND user_id::TEXT = current_setting('app.user_id', TRUE)
        )
    );
