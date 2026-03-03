-- GN-WAAS Migration: 017_schema_corrections
-- Date: 2026-03-03
-- Description: Fix two schema gaps identified in the v17 code audit.
--
--   DB-H03: water_accounts is missing the is_active column.
--           cdc_sync_service.go inserts into is_active on every CDC sync,
--           causing every billing record upsert to fail with
--           "column is_active does not exist".
--
--   DB-H08: illegal_connections is missing the photo_hashes column.
--           audit_event_repo.go CreateIllegalConnection inserts photo_hashes
--           on every illegal-connection report, causing the INSERT to fail
--           with "column photo_hashes does not exist".

-- ── DB-H03: Add is_active to water_accounts ───────────────────────────────────
-- The CDC ingestor mirrors GWL's account active/inactive flag into this column.
-- Default TRUE so existing rows are treated as active after the migration.
ALTER TABLE water_accounts
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

COMMENT ON COLUMN water_accounts.is_active IS
    'Mirrors the GWL account active/inactive flag. Synced by cdc-ingestor on every CDC event.';

-- ── DB-H08: Add photo_hashes to illegal_connections ──────────────────────────
-- Stores the SHA-256 hashes of all photos attached to an illegal-connection
-- report. Used for chain-of-custody verification and tamper detection.
ALTER TABLE illegal_connections
    ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] DEFAULT '{}';

COMMENT ON COLUMN illegal_connections.photo_hashes IS
    'SHA-256 hashes of attached evidence photos. Computed on-device by the Flutter app.';
