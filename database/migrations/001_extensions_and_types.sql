-- ============================================================
-- GN-WAAS: Ghana National Water Audit & Assurance System
-- Migration: 001_extensions_and_types
-- Description: Enable required PostgreSQL extensions and create
--              all custom ENUM types aligned with IWA/AWWA standard
-- ============================================================

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
-- PostGIS for district boundary geometry (GEOMETRY column in districts table).
-- Wrapped in DO block so migration succeeds on plain PostgreSQL without PostGIS.
-- The boundary_geom column will be TEXT type if PostGIS is unavailable (see migration 002).
DO $$ BEGIN
  CREATE EXTENSION IF NOT EXISTS "postgis";
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'postgis extension not available, skipping (district boundary GIS features disabled)';
END $$;
DO $$ BEGIN
  CREATE EXTENSION IF NOT EXISTS "timescaledb";
EXCEPTION WHEN OTHERS THEN
  RAISE NOTICE 'timescaledb extension not available, skipping (standard PostgreSQL mode)';
END $$;
CREATE EXTENSION IF NOT EXISTS "pg_trgm";        -- Fuzzy text search for accounts
CREATE EXTENSION IF NOT EXISTS "pgcrypto";       -- Cryptographic functions for audit hashing

-- ============================================================
-- ENUM TYPES
-- ============================================================

-- Account classification (PURC 2026 tariff categories)
CREATE TYPE account_category AS ENUM (
    'RESIDENTIAL',
    'PUBLIC_GOVT',
    'COMMERCIAL',
    'INDUSTRIAL',
    'BOTTLED_WATER',
    'UNKNOWN'
);

-- Account lifecycle status
CREATE TYPE account_status AS ENUM (
    'ACTIVE',
    'INACTIVE',
    'SUSPENDED',
    'FLAGGED',
    'UNDER_AUDIT',
    'GHOST'
);

-- Audit event lifecycle
CREATE TYPE audit_status AS ENUM (
    'PENDING',
    'IN_PROGRESS',
    'AWAITING_GRA',
    'GRA_CONFIRMED',
    'GRA_FAILED',
    'COMPLETED',
    'DISPUTED',
    'ESCALATED',
    'CLOSED',
    'PENDING_COMPLIANCE'
);

-- IWA/AWWA anomaly classification
CREATE TYPE anomaly_type AS ENUM (
    'UNAUTHORISED_CONSUMPTION',
    'METERING_INACCURACY',
    'DATA_HANDLING_ERROR',
    'PHANTOM_METER',
    'GHOST_ACCOUNT',
    'CATEGORY_MISMATCH',
    'OUTAGE_CONSUMPTION',
    'DISTRICT_IMBALANCE',
    'RATIONING_ANOMALY',
    'SHADOW_BILL_VARIANCE',
    'VAT_DISCREPANCY',
    'PHYSICAL_LEAK',
    'NIGHT_FLOW_ANOMALY'
);

-- Alert severity levels
CREATE TYPE alert_level AS ENUM (
    'INFO',
    'LOW',
    'MEDIUM',
    'HIGH',
    'CRITICAL'
);

-- Fraud classification
CREATE TYPE fraud_type AS ENUM (
    'GHOST_ACCOUNT',
    'PHANTOM_METER',
    'ILLEGAL_CONNECTION',
    'METER_TAMPERING',
    'READING_COLLUSION',
    'CATEGORY_DOWNGRADE',
    'VAT_EVASION',
    'OUTSIDE_NETWORK_BILLING',
    'OUTAGE_CONSUMPTION'
);

-- RBAC user roles
CREATE TYPE user_role AS ENUM (
    'SUPER_ADMIN',
    'SYSTEM_ADMIN',
    'MINISTER_VIEW',
    'GRA_OFFICER',
    'MOF_AUDITOR',
    'GWL_EXECUTIVE',
    'GWL_MANAGER',
    'GWL_ANALYST',
    'FIELD_SUPERVISOR',
    'FIELD_OFFICER',
    'MDA_USER'
);

-- User account status
CREATE TYPE user_status AS ENUM (
    'ACTIVE',
    'INACTIVE',
    'SUSPENDED',
    'PENDING'
);

-- District zone classification (heatmap)
CREATE TYPE district_zone_type AS ENUM (
    'RED',
    'YELLOW',
    'GREEN',
    'GREY'
);

-- Water supply status
CREATE TYPE supply_status AS ENUM (
    'NORMAL',
    'REDUCED',
    'OUTAGE',
    'MAINTENANCE',
    'UNKNOWN'
);

-- GRA VSDC compliance status
CREATE TYPE gra_compliance_status AS ENUM (
    'PENDING',
    'SIGNED',
    'FAILED',
    'RETRYING',
    'EXEMPT'
);

-- Field job lifecycle
CREATE TYPE field_job_status AS ENUM (
    'ASSIGNED',
    'DISPATCHED',
    'EN_ROUTE',
    'ON_SITE',
    'COMPLETED',
    'FAILED',
    'CANCELLED',
    'ESCALATED'
);

-- OCR reading result
CREATE TYPE ocr_status AS ENUM (
    'SUCCESS',
    'FAILED',
    'BLURRY',
    'TAMPERED',
    'CONFLICT',
    'PENDING'
);

-- IWA/AWWA Water Balance components
CREATE TYPE water_balance_component AS ENUM (
    'BILLED_METERED',
    'BILLED_UNMETERED',
    'UNBILLED_METERED',
    'UNBILLED_UNMETERED',
    'UNAUTHORISED_CONSUMPTION',
    'METERING_INACCURACIES',
    'DATA_HANDLING_ERRORS',
    'MAIN_LEAKAGE',
    'STORAGE_TANK_OVERFLOW',
    'SERVICE_CONNECTION_LEAKAGE'
);

-- AWWA Data Confidence Grade (1-10)
CREATE DOMAIN data_confidence_grade AS SMALLINT
    CHECK (VALUE >= 0 AND VALUE <= 10);

DO $$ BEGIN
  COMMENT ON EXTENSION timescaledb IS 'TimescaleDB for time-series water flow data';
EXCEPTION WHEN OTHERS THEN
  NULL; -- extension not installed, skip comment
END $$;
COMMENT ON TYPE account_category IS 'PURC 2026 tariff billing categories';
COMMENT ON TYPE anomaly_type IS 'IWA/AWWA aligned anomaly classification';
COMMENT ON TYPE water_balance_component IS 'IWA/AWWA Water Balance components';
