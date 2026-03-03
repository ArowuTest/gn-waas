-- ============================================================
-- GN-WAAS Migration: 003_billing_and_shadow_ledger
-- Description: GWL billing records, shadow bills, and the
--              IWA/AWWA water balance reconciliation tables
-- ============================================================

-- ============================================================
-- GWL BILLING RECORDS (Mirrored from CDC)
-- ============================================================
CREATE TABLE gwl_billing_records (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    gwl_bill_id         VARCHAR(100) NOT NULL UNIQUE,     -- GWL's own bill reference
    account_id          UUID NOT NULL REFERENCES water_accounts(id),
    billing_period_start DATE NOT NULL,
    billing_period_end   DATE NOT NULL,
    previous_reading    NUMERIC(12,4),
    current_reading     NUMERIC(12,4),
    consumption_m3      NUMERIC(12,4) NOT NULL,
    gwl_category        account_category NOT NULL,        -- Category GWL used
    gwl_amount_ghs      NUMERIC(12,2) NOT NULL,           -- What GWL billed
    gwl_vat_ghs         NUMERIC(12,2) NOT NULL DEFAULT 0,
    gwl_total_ghs       NUMERIC(12,2) NOT NULL,
    gwl_reader_id       VARCHAR(50),
    gwl_read_date       DATE,
    gwl_read_method     VARCHAR(20),                      -- MANUAL, AMR, ESTIMATED
    payment_status      VARCHAR(20) NOT NULL DEFAULT 'UNPAID',
    payment_date        DATE,
    payment_amount_ghs  NUMERIC(12,2),
    gwl_sync_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cdc_event_id        VARCHAR(100),                     -- CDC event reference
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gwl_billing_account ON gwl_billing_records(account_id);
CREATE INDEX idx_gwl_billing_period ON gwl_billing_records(billing_period_start, billing_period_end);
CREATE INDEX idx_gwl_billing_sync ON gwl_billing_records(gwl_sync_at);

COMMENT ON TABLE gwl_billing_records IS 'GWL actual billing records mirrored via CDC. Immutable after sync.';

-- ============================================================
-- SHADOW BILLS (GN-WAAS calculated correct bills)
-- ============================================================
CREATE TABLE shadow_bills (
    id                      UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    gwl_bill_id             UUID NOT NULL REFERENCES gwl_billing_records(id),
    account_id              UUID NOT NULL REFERENCES water_accounts(id),
    billing_period_start    DATE NOT NULL,
    billing_period_end      DATE NOT NULL,
    consumption_m3          NUMERIC(12,4) NOT NULL,
    correct_category        account_category NOT NULL,    -- Category we determined
    tariff_rate_id          UUID REFERENCES tariff_rates(id),
    vat_config_id           UUID REFERENCES vat_config(id),

    -- Tier breakdown
    tier1_volume_m3         NUMERIC(12,4) NOT NULL DEFAULT 0,
    tier1_rate              NUMERIC(10,4) NOT NULL DEFAULT 0,
    tier1_amount_ghs        NUMERIC(12,2) NOT NULL DEFAULT 0,
    tier2_volume_m3         NUMERIC(12,4) NOT NULL DEFAULT 0,
    tier2_rate              NUMERIC(10,4) NOT NULL DEFAULT 0,
    tier2_amount_ghs        NUMERIC(12,2) NOT NULL DEFAULT 0,
    service_charge_ghs      NUMERIC(12,2) NOT NULL DEFAULT 0,

    -- Totals
    subtotal_ghs            NUMERIC(12,2) NOT NULL,
    vat_amount_ghs          NUMERIC(12,2) NOT NULL,
    total_shadow_bill_ghs   NUMERIC(12,2) NOT NULL,

    -- Variance analysis
    gwl_total_ghs           NUMERIC(12,2) NOT NULL,       -- Copy from gwl_billing_records
    variance_ghs            NUMERIC(12,2) NOT NULL,       -- shadow - gwl
    variance_pct            NUMERIC(8,4) NOT NULL,        -- (variance / gwl_total) * 100
    is_flagged              BOOLEAN NOT NULL DEFAULT FALSE,
    flag_reason             TEXT,

    -- Calculation metadata
    calculated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    calculation_version     VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shadow_bills_account ON shadow_bills(account_id);
CREATE INDEX idx_shadow_bills_period ON shadow_bills(billing_period_start);
CREATE INDEX idx_shadow_bills_flagged ON shadow_bills(is_flagged) WHERE is_flagged = TRUE;
CREATE INDEX idx_shadow_bills_variance ON shadow_bills(variance_pct);

COMMENT ON TABLE shadow_bills IS 'GN-WAAS calculated shadow bills using PURC 2026 tariffs. Compared against GWL actuals.';

-- ============================================================
-- IWA/AWWA WATER BALANCE (District-level monthly)
-- ============================================================
CREATE TABLE water_balance_records (
    id                          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_id                 UUID NOT NULL REFERENCES districts(id),
    period_start                DATE NOT NULL,
    period_end                  DATE NOT NULL,

    -- System Input Volume
    system_input_volume_m3      NUMERIC(15,4) NOT NULL DEFAULT 0,
    input_data_source           VARCHAR(50) NOT NULL DEFAULT 'GWL_PRODUCTION',

    -- Authorised Consumption
    billed_metered_m3           NUMERIC(15,4) NOT NULL DEFAULT 0,
    billed_unmetered_m3         NUMERIC(15,4) NOT NULL DEFAULT 0,
    unbilled_metered_m3         NUMERIC(15,4) NOT NULL DEFAULT 0,
    unbilled_unmetered_m3       NUMERIC(15,4) NOT NULL DEFAULT 0,
    total_authorised_m3         NUMERIC(15,4) GENERATED ALWAYS AS (
        billed_metered_m3 + billed_unmetered_m3 +
        unbilled_metered_m3 + unbilled_unmetered_m3
    ) STORED,

    -- Apparent Losses (NRW - software detectable)
    unauthorised_consumption_m3 NUMERIC(15,4) NOT NULL DEFAULT 0,
    metering_inaccuracies_m3    NUMERIC(15,4) NOT NULL DEFAULT 0,
    data_handling_errors_m3     NUMERIC(15,4) NOT NULL DEFAULT 0,
    total_apparent_losses_m3    NUMERIC(15,4) GENERATED ALWAYS AS (
        unauthorised_consumption_m3 + metering_inaccuracies_m3 + data_handling_errors_m3
    ) STORED,

    -- Real Losses (NRW - physical, statistical estimate)
    main_leakage_m3             NUMERIC(15,4) NOT NULL DEFAULT 0,
    storage_overflow_m3         NUMERIC(15,4) NOT NULL DEFAULT 0,
    service_conn_leakage_m3     NUMERIC(15,4) NOT NULL DEFAULT 0,
    total_real_losses_m3        NUMERIC(15,4) GENERATED ALWAYS AS (
        main_leakage_m3 + storage_overflow_m3 + service_conn_leakage_m3
    ) STORED,

    -- NRW Summary
    total_nrw_m3                NUMERIC(15,4) GENERATED ALWAYS AS (
        system_input_volume_m3 - (
            billed_metered_m3 + billed_unmetered_m3 +
            unbilled_metered_m3 + unbilled_unmetered_m3
        )
    ) STORED,
    nrw_pct                     NUMERIC(8,4),             -- Calculated separately (avoid div/0)

    -- Financial impact
    apparent_loss_value_ghs     NUMERIC(15,2) NOT NULL DEFAULT 0,
    real_loss_value_ghs         NUMERIC(15,2) NOT NULL DEFAULT 0,
    total_nrw_value_ghs         NUMERIC(15,2) NOT NULL DEFAULT 0,

    -- AWWA KPIs
    ili_value                   NUMERIC(8,4),             -- Infrastructure Leakage Index
    apparent_loss_per_conn_lpd  NUMERIC(10,4),            -- Litres/connection/day
    real_loss_pct_of_input      NUMERIC(8,4),

    -- Data quality (AWWA Grading Matrix)
    data_confidence_grade       data_confidence_grade DEFAULT 0,
    confidence_notes            TEXT,

    calculated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_water_balance_district ON water_balance_records(district_id);
CREATE INDEX idx_water_balance_period ON water_balance_records(period_start, period_end);
CREATE UNIQUE INDEX idx_water_balance_district_period
    ON water_balance_records(district_id, period_start, period_end);

COMMENT ON TABLE water_balance_records IS 'IWA/AWWA Water Balance records per district per period. Core reconciliation output.';

-- ============================================================
-- PRODUCTION RECORDS (GWL water pumped/treated)
-- ============================================================
CREATE TABLE production_records (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    district_id         UUID NOT NULL REFERENCES districts(id),
    recorded_at         TIMESTAMPTZ NOT NULL,
    volume_m3           NUMERIC(15,4) NOT NULL,
    source_type         VARCHAR(50) NOT NULL DEFAULT 'TREATMENT_PLANT',
    source_reference    VARCHAR(100),
    data_source         VARCHAR(50) NOT NULL DEFAULT 'GWL_MANUAL',
    recorded_by         UUID REFERENCES users(id),
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- DB-H01 fix: wrap in DO block so migration succeeds even without TimescaleDB
DO $hyper$ BEGIN
    PERFORM create_hypertable('production_records', 'recorded_at',
        chunk_time_interval => INTERVAL '1 month',
        if_not_exists => TRUE
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'TimescaleDB not available — production_records will be a plain table: %', SQLERRM;
END $hyper$;

CREATE INDEX idx_production_district_time ON production_records(district_id, recorded_at DESC);

COMMENT ON TABLE production_records IS 'GWL water production records. Used for district-level balance checks.';
