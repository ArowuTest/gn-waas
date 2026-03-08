-- ============================================================
-- Migration 034: Add GPS columns to districts table
-- ============================================================
-- Bug DB-01 fix: The districts table was created in migration 002
-- without gps_latitude and gps_longitude columns. These are required by:
--   1. DistrictRepository.GetByID (field_job_repo.go) — SELECT query
--   2. Seed file 003_districts.sql — INSERT with GPS values
--   3. Mobile app geofence validation — centroid for proximity checks
--   4. DMA map rendering — district centroid markers
--
-- This migration is safe to run on existing databases.
-- Migration 002 has been updated for fresh installs; this migration
-- handles existing deployments that already ran migration 002.
-- ============================================================

-- Add GPS centroid columns to districts (idempotent)
ALTER TABLE districts
  ADD COLUMN IF NOT EXISTS gps_latitude  NUMERIC(10,8),  -- WGS-84 latitude  (-90  to  90)
  ADD COLUMN IF NOT EXISTS gps_longitude NUMERIC(11,8),  -- WGS-84 longitude (-180 to 180)
  ADD COLUMN IF NOT EXISTS geographic_zone VARCHAR(50);  -- Administrative zone label

-- Index for spatial queries (district lookup by approximate GPS)
CREATE INDEX IF NOT EXISTS idx_districts_gps
  ON districts (gps_latitude, gps_longitude)
  WHERE gps_latitude IS NOT NULL AND gps_longitude IS NOT NULL;

COMMENT ON COLUMN districts.gps_latitude  IS 'WGS-84 latitude of district centroid. Used for DMA map rendering and mobile geofence validation.';
COMMENT ON COLUMN districts.gps_longitude IS 'WGS-84 longitude of district centroid. Used for DMA map rendering and mobile geofence validation.';
COMMENT ON COLUMN districts.geographic_zone IS 'Administrative zone label (e.g. Greater Accra, Ashanti). Denormalised for fast filtering.';
