-- ============================================================
-- Migration 015: Add GWL_SUPERVISOR to user_role enum
-- ============================================================
-- GWL_SUPERVISOR is a billing/case supervisor role used by the
-- GWL Case Management Portal. It sits between GWL_MANAGER
-- (district manager) and GWL_ANALYST (read-only analyst),
-- allowing supervisors to review, action, and escalate cases
-- without full manager privileges.
-- ============================================================

ALTER TYPE user_role ADD VALUE IF NOT EXISTS 'GWL_SUPERVISOR' AFTER 'GWL_MANAGER';
