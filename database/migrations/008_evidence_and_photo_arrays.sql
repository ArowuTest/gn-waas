-- ============================================================
-- Migration 008: Evidence Photo Arrays & MinIO Object Keys
-- ============================================================
-- Upgrades audit_events to store arrays of photo URLs and
-- MinIO object keys, replacing the single meter_photo_url TEXT
-- column. Also adds evidence_object_keys for tamper-proof
-- MinIO storage references.
-- ============================================================

BEGIN;

-- ── 1. Add photo_urls array column to audit_events ──────────────────────────
-- Migrate existing single meter_photo_url into the new array
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS photo_urls            TEXT[]   NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS evidence_object_keys  TEXT[]   NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS photo_hashes          TEXT[]   NOT NULL DEFAULT '{}';

-- Migrate existing data: if meter_photo_url is set, copy into array
UPDATE audit_events
SET photo_urls = ARRAY[meter_photo_url]
WHERE meter_photo_url IS NOT NULL
  AND meter_photo_url <> ''
  AND array_length(photo_urls, 1) IS NULL;

-- ── 2. Add photo_urls to field_jobs evidence JSONB index ────────────────────
-- Ensure evidence_data JSONB is indexed for photo lookups
CREATE INDEX IF NOT EXISTS idx_audit_events_photo_hashes
  ON audit_events USING GIN (photo_hashes);

CREATE INDEX IF NOT EXISTS idx_audit_events_evidence_keys
  ON audit_events USING GIN (evidence_object_keys);

-- ── 3. Add evidence_object_key to field_jobs for presigned URL tracking ─────
ALTER TABLE field_jobs
  ADD COLUMN IF NOT EXISTS evidence_submitted_at  TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS evidence_photo_count   INTEGER NOT NULL DEFAULT 0;

-- ── 4. Create evidence_uploads tracking table ────────────────────────────────
-- Tracks each MinIO upload: presigned URL issued, upload confirmed, hash verified
CREATE TABLE IF NOT EXISTS evidence_uploads (
  id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  field_job_id      UUID        NOT NULL REFERENCES field_jobs(id) ON DELETE CASCADE,
  audit_event_id    UUID        REFERENCES audit_events(id) ON DELETE SET NULL,
  object_key        TEXT        NOT NULL UNIQUE,
  photo_hash_sha256 TEXT        NOT NULL,
  hash_verified     BOOLEAN     NOT NULL DEFAULT FALSE,
  file_size_bytes   BIGINT,
  content_type      TEXT        NOT NULL DEFAULT 'image/jpeg',
  uploaded_by       UUID        NOT NULL REFERENCES users(id),
  presigned_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  uploaded_at       TIMESTAMPTZ,
  verified_at       TIMESTAMPTZ,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evidence_uploads_job_id
  ON evidence_uploads(field_job_id);

CREATE INDEX IF NOT EXISTS idx_evidence_uploads_object_key
  ON evidence_uploads(object_key);

CREATE INDEX IF NOT EXISTS idx_evidence_uploads_hash
  ON evidence_uploads(photo_hash_sha256);

-- ── 5. Add immutable audit log trigger for evidence_uploads ─────────────────
-- Prevent deletion or update of verified evidence (tamper-proof)
CREATE OR REPLACE FUNCTION prevent_verified_evidence_mutation()
RETURNS TRIGGER AS $$
BEGIN
  IF OLD.hash_verified = TRUE THEN
    RAISE EXCEPTION 'Cannot modify verified evidence upload (object_key: %)', OLD.object_key;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_verified_evidence_mutation ON evidence_uploads;
CREATE TRIGGER trg_prevent_verified_evidence_mutation
  BEFORE UPDATE OR DELETE ON evidence_uploads
  FOR EACH ROW
  EXECUTE FUNCTION prevent_verified_evidence_mutation();

-- ── 6. Add GRA compliance lock column to audit_events ───────────────────────
-- Spec requires: audit saved only after QR code receipt from GRA VSDC
ALTER TABLE audit_events
  ADD COLUMN IF NOT EXISTS gra_qr_code          TEXT,
  ADD COLUMN IF NOT EXISTS gra_receipt_number   TEXT,
  ADD COLUMN IF NOT EXISTS gra_locked_at        TIMESTAMPTZ;

-- ── 7. Add composite index for sentinel queries ──────────────────────────────
CREATE INDEX IF NOT EXISTS idx_audit_events_district_created
  ON audit_events(district_id, created_at DESC)
  WHERE district_id IS NOT NULL;

COMMIT;
