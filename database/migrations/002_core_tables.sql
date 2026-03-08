-- ============================================================
-- GN-WAAS Migration: 002_core_tables
-- Description: Core reference tables - districts, accounts,
--              users, system configuration
-- ============================================================

-- ============================================================
-- SYSTEM CONFIGURATION (Admin-configurable values)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_config (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    config_key      VARCHAR(100) NOT NULL UNIQUE,
    config_value    TEXT NOT NULL,
    config_type     VARCHAR(20) NOT NULL DEFAULT 'STRING', -- STRING, NUMBER, BOOLEAN, JSON
    description     TEXT,
    is_sensitive    BOOLEAN NOT NULL DEFAULT FALSE,
    category        VARCHAR(50) NOT NULL DEFAULT 'GENERAL',
    updated_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE system_config IS 'Admin-configurable system parameters. No hardcoded values.';

-- ============================================================
-- TARIFF RATES (Admin-configurable, versioned)
-- ============================================================
CREATE TABLE IF NOT EXISTS tariff_rates (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category            account_category NOT NULL,
    tier_name           VARCHAR(50) NOT NULL,
    min_volume_m3       NUMERIC(10,4) NOT NULL DEFAULT 0,
    max_volume_m3       NUMERIC(10,4),                    -- NULL = unlimited
    rate_per_m3         NUMERIC(10,4) NOT NULL,
    service_charge_ghs  NUMERIC(12,2) NOT NULL DEFAULT 0,
    effective_from      DATE NOT NULL,
    effective_to        DATE,                             -- NULL = current
    approved_by         VARCHAR(100),                     -- PURC approval reference
    regulatory_ref      VARCHAR(100),                     -- e.g. "PURC-2026-01"
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT tariff_rates_category_tier_date_unique
        UNIQUE (category, tier_name, effective_from)
);

COMMENT ON TABLE tariff_rates IS 'PURC-approved tariff rates. Admin-configurable. Versioned by effective_from date.';

-- VAT configuration (separate for flexibility)
CREATE TABLE IF NOT EXISTS vat_config (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rate_percentage NUMERIC(5,2) NOT NULL,
    components      JSONB NOT NULL DEFAULT '{}', -- e.g. {"VAT": 12.5, "NHIL": 2.5, "GETFund": 2.5, "COVID": 1.0}
    effective_from  DATE NOT NULL,
    effective_to    DATE,
    regulatory_ref  VARCHAR(100),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE vat_config IS 'VAT rate configuration. Currently 20% effective rate (NHIL + GETFund + VAT).';

-- ============================================================
-- DISTRICTS / DMA (District Metered Areas)
-- ============================================================
CREATE TABLE IF NOT EXISTS districts (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_code           VARCHAR(20) NOT NULL UNIQUE,
    district_name           VARCHAR(100) NOT NULL,
    region                  VARCHAR(100) NOT NULL,
    parent_district_id      UUID REFERENCES districts(id),
    boundary_geom           GEOMETRY(MULTIPOLYGON, 4326),  -- PostGIS boundary
    total_area_km2          NUMERIC(10,2),
    population_estimate     INTEGER,
    total_connections       INTEGER NOT NULL DEFAULT 0,
    supply_status           supply_status NOT NULL DEFAULT 'NORMAL',
    zone_type               district_zone_type NOT NULL DEFAULT 'GREY',
    loss_ratio_pct          NUMERIC(5,2),                  -- Latest calculated NRW %
    data_confidence_grade   data_confidence_grade DEFAULT 0,
    arcgis_layer_id         VARCHAR(100),                  -- GWL ArcGIS reference
    -- DB-01 fix: GPS centroid for DMA map rendering and mobile geofence validation
    gps_latitude            NUMERIC(10,8),                 -- WGS-84 latitude  (-90  to  90)
    gps_longitude           NUMERIC(11,8),                 -- WGS-84 longitude (-180 to 180)
    geographic_zone         VARCHAR(50),                   -- Administrative zone label
    is_pilot_district       BOOLEAN NOT NULL DEFAULT FALSE,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_districts_region ON districts(region);
CREATE INDEX IF NOT EXISTS idx_districts_zone_type ON districts(zone_type);
CREATE INDEX IF NOT EXISTS idx_districts_pilot ON districts(is_pilot_district) WHERE is_pilot_district = TRUE;
CREATE INDEX IF NOT EXISTS idx_districts_boundary ON districts USING GIST(boundary_geom);

COMMENT ON TABLE districts IS 'GWL District Metered Areas (DMAs). Boundaries from ArcGIS.';

-- ============================================================
-- WATER ACCOUNTS (Mirrored from GWL CDC)
-- ============================================================
CREATE TABLE IF NOT EXISTS water_accounts (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    gwl_account_number      VARCHAR(50) NOT NULL UNIQUE,  -- GWL's own account ID
    account_holder_name     VARCHAR(200) NOT NULL,
    account_holder_tin      VARCHAR(20),                  -- GRA TIN
    category                account_category NOT NULL,
    status                  account_status NOT NULL DEFAULT 'ACTIVE',
    district_id             UUID NOT NULL REFERENCES districts(id),
    meter_number            VARCHAR(50),
    meter_serial            VARCHAR(100),
    meter_install_date      DATE,
    meter_last_replaced     DATE,
    address_line1           VARCHAR(200),
    address_line2           VARCHAR(200),
    gps_latitude            NUMERIC(10,8),
    gps_longitude           NUMERIC(11,8),
    gps_precision_m         NUMERIC(6,2),
    is_within_network       BOOLEAN,                      -- GIS boundary check result
    network_check_date      TIMESTAMPTZ,
    gwl_billing_cycle       VARCHAR(20),                  -- e.g. "MONTHLY", "BIMONTHLY"
    gwl_route_code          VARCHAR(20),                  -- GWL meter reading route
    gwl_reader_id           VARCHAR(50),                  -- Assigned GWL meter reader
    monthly_avg_consumption NUMERIC(10,4),                -- Rolling 12-month average
    is_phantom_flagged      BOOLEAN NOT NULL DEFAULT FALSE,
    phantom_flag_reason     TEXT,
    phantom_flag_date       TIMESTAMPTZ,
    gwl_sync_at             TIMESTAMPTZ,                  -- Last CDC sync timestamp
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_accounts_gwl_number ON water_accounts(gwl_account_number);
CREATE INDEX IF NOT EXISTS idx_accounts_district ON water_accounts(district_id);
CREATE INDEX IF NOT EXISTS idx_accounts_category ON water_accounts(category);
CREATE INDEX IF NOT EXISTS idx_accounts_status ON water_accounts(status);
CREATE INDEX IF NOT EXISTS idx_accounts_phantom ON water_accounts(is_phantom_flagged) WHERE is_phantom_flagged = TRUE;
CREATE INDEX IF NOT EXISTS idx_accounts_network ON water_accounts(is_within_network) WHERE is_within_network = FALSE;
CREATE INDEX IF NOT EXISTS idx_accounts_name_trgm ON water_accounts USING GIN(account_holder_name gin_trgm_ops);

COMMENT ON TABLE water_accounts IS 'Water accounts mirrored from GWL e-billing via CDC. Source of truth for audit.';

-- ============================================================
-- USERS (GN-WAAS system users)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    keycloak_id         VARCHAR(100) UNIQUE,              -- Keycloak subject ID
    email               VARCHAR(200) NOT NULL UNIQUE,
    full_name           VARCHAR(200) NOT NULL,
    phone_number        VARCHAR(20),
    role                user_role NOT NULL,
    status              user_status NOT NULL DEFAULT 'PENDING',
    district_id         UUID REFERENCES districts(id),    -- NULL = national access
    organisation        VARCHAR(200),                     -- GWL, GRA, MoF, etc.
    employee_id         VARCHAR(50),
    device_id           VARCHAR(100),                     -- For field officers (anti-sharing)
    last_login_at       TIMESTAMPTZ,
    last_location_lat   NUMERIC(10,8),
    last_location_lng   NUMERIC(11,8),
    failed_login_count  INTEGER NOT NULL DEFAULT 0,
    is_mfa_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_district ON users(district_id);
CREATE INDEX IF NOT EXISTS idx_users_keycloak ON users(keycloak_id);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

COMMENT ON TABLE users IS 'GN-WAAS system users. Keycloak is the IdP; this table stores profile and RBAC data.';

-- ============================================================
-- AUDIT THRESHOLDS (Admin-configurable per district)
-- ============================================================
CREATE TABLE IF NOT EXISTS audit_thresholds (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_id                 UUID REFERENCES districts(id), -- NULL = global default
    threshold_name              VARCHAR(100) NOT NULL,
    shadow_bill_variance_pct    NUMERIC(5,2) NOT NULL DEFAULT 15.0,
    night_flow_pct_of_daily     NUMERIC(5,2) NOT NULL DEFAULT 30.0,
    phantom_meter_months        INTEGER NOT NULL DEFAULT 6,     -- Months of identical readings
    district_imbalance_pct      NUMERIC(5,2) NOT NULL DEFAULT 20.0,
    rationing_drop_pct          NUMERIC(5,2) NOT NULL DEFAULT 40.0,
    gps_fence_radius_m          NUMERIC(8,2) NOT NULL DEFAULT 5.0,
    ocr_conflict_tolerance_pct  NUMERIC(5,2) NOT NULL DEFAULT 2.0,
    effective_from              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_to                TIMESTAMPTZ,
    set_by                      UUID REFERENCES users(id),
    reason                      TEXT,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE audit_thresholds IS 'Admin-configurable anomaly detection thresholds. Per-district overrides supported.';

-- ============================================================
-- SUPPLY SCHEDULES (For outage consumption detection)
-- ============================================================
CREATE TABLE IF NOT EXISTS supply_schedules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_id     UUID NOT NULL REFERENCES districts(id),
    supply_status   supply_status NOT NULL,
    reduction_pct   NUMERIC(5,2),                         -- % reduction during rationing
    start_time      TIMESTAMPTZ NOT NULL,
    end_time        TIMESTAMPTZ,
    reason          TEXT,
    recorded_by     UUID REFERENCES users(id),
    source          VARCHAR(50) NOT NULL DEFAULT 'MANUAL', -- MANUAL, GWL_API, SCADA
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_supply_schedules_district ON supply_schedules(district_id);
CREATE INDEX IF NOT EXISTS idx_supply_schedules_time ON supply_schedules(start_time, end_time);

COMMENT ON TABLE supply_schedules IS 'GWL supply schedule records. Used to detect consumption during outages.';
