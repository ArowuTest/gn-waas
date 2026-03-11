-- Migration 043: Permissions fix + water balance seed data
-- ==========================================================
-- Purpose:
--   1. Re-grant permissions to gnwaas_app on all tables created after
--      migration 012 (the original GRANT only covered tables existing at
--      that point in time).
--   2. Seed realistic water balance records for all 25 districts across
--      Jan–Mar 2026 so the NRW Analysis page has data to display.
--
-- Design:
--   • Every statement that could fail on an already-initialised DB is
--     wrapped in its own DO block with EXCEPTION WHEN OTHERS THEN RAISE NOTICE.
--   • The water balance INSERT uses ON CONFLICT DO NOTHING so re-running
--     this migration is safe.
-- ==========================================================

-- ─── Part 1: Re-grant permissions ────────────────────────────────────────────
-- Migration 012 ran GRANT on all tables that existed at that time.
-- Tables created in migrations 013–042 (illegal_connections, meter_readings,
-- gwl_bills, water_balance_records, etc.) were not covered.
-- This blanket re-grant fixes all of them at once.

DO $$ BEGIN
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES    IN SCHEMA public TO gnwaas_app;
    GRANT USAGE, SELECT                  ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app;
    GRANT EXECUTE                        ON ALL FUNCTIONS IN SCHEMA public TO gnwaas_app;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'Part 1 grant: %', SQLERRM;
END $$;

-- Ensure future tables are also covered (ALTER DEFAULT PRIVILEGES requires
-- the role that will CREATE the tables — gnwaas_user — to run this).
DO $$ BEGIN
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
        GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES    TO gnwaas_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
        GRANT USAGE, SELECT                  ON SEQUENCES TO gnwaas_app;
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'Part 1 default privileges: %', SQLERRM;
END $$;

-- ─── Part 2: Water balance seed data ─────────────────────────────────────────
-- Populate water_balance_records with realistic data for all 25 districts
-- across Jan–Mar 2026. Values are based on Ghana Water Limited's published
-- NRW figures (national average ~51.6 %).
--
-- IWA/AWWA M36 Water Balance components (all in m³):
--   System Input Volume = Authorised Consumption + Water Losses
--   Water Losses        = Real Losses + Apparent Losses
--   NRW                 = Unbilled Authorised + Water Losses

INSERT INTO water_balance_records (
    district_id,
    period_start,
    period_end,
    system_input_volume_m3,
    billed_metered_m3,
    billed_unmetered_m3,
    unbilled_metered_m3,
    unbilled_unmetered_m3,
    total_authorised_m3,
    unauthorised_consumption_m3,
    metering_inaccuracies_m3,
    data_handling_errors_m3,
    total_apparent_losses_m3,
    main_leakage_m3,
    storage_overflow_m3,
    service_conn_leakage_m3,
    total_real_losses_m3,
    total_nrw_m3,
    nrw_percent,
    ili_score,
    iwa_grade,
    estimated_revenue_recovery_ghs,
    data_confidence_score,
    computed_at
)
SELECT
    d.id                                                        AS district_id,
    gs.period_start,
    (gs.period_start + INTERVAL '1 month' - INTERVAL '1 day')  AS period_end,

    -- System Input Volume: 8,000–25,000 m³/month depending on district size
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000))::numeric, 2)
        AS system_input_volume_m3,

    -- Billed metered: ~45–50 % of input (national billing rate)
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.47, 2)
        AS billed_metered_m3,

    -- Billed unmetered: ~2 % (flat-rate customers)
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.02, 2)
        AS billed_unmetered_m3,

    -- Unbilled metered: ~1 % (fire hydrants, flushing)
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.01, 2)
        AS unbilled_metered_m3,

    -- Unbilled unmetered: ~0.5 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.005, 2)
        AS unbilled_unmetered_m3,

    -- Total authorised = billed metered + billed unmetered + unbilled metered + unbilled unmetered
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.505, 2)
        AS total_authorised_m3,

    -- Apparent losses — unauthorised consumption (ghost accounts, bypasses): ~8 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.08, 2)
        AS unauthorised_consumption_m3,

    -- Apparent losses — metering inaccuracies: ~3 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.03, 2)
        AS metering_inaccuracies_m3,

    -- Apparent losses — data handling errors: ~1 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.01, 2)
        AS data_handling_errors_m3,

    -- Total apparent losses: ~12 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.12, 2)
        AS total_apparent_losses_m3,

    -- Real losses — main leakage: ~20 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.20, 2)
        AS main_leakage_m3,

    -- Real losses — storage overflow: ~5 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.05, 2)
        AS storage_overflow_m3,

    -- Real losses — service connection leakage: ~14.5 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.145, 2)
        AS service_conn_leakage_m3,

    -- Total real losses: ~39.5 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.395, 2)
        AS total_real_losses_m3,

    -- Total NRW = apparent + real + unbilled authorised ≈ 51.5 %
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.515, 2)
        AS total_nrw_m3,

    -- NRW % (varies 45–58 % by district)
    ROUND((45.0 + ABS(hashtext(d.id::text || gs.period_start::text) % 13))::numeric, 2)
        AS nrw_percent,

    -- ILI score (Infrastructure Leakage Index): 3.5–8.5 range for Ghana
    ROUND((3.5 + (ABS(hashtext(d.id::text)) % 50) / 10.0)::numeric, 2)
        AS ili_score,

    -- IWA grade: C or D (typical for developing-country utilities)
    CASE WHEN ABS(hashtext(d.id::text)) % 2 = 0 THEN 'C' ELSE 'D' END
        AS iwa_grade,

    -- Estimated revenue recovery (GHS): apparent losses × avg tariff GHS 10.83/m³
    ROUND((8000 + (hashtext(d.id::text || gs.period_start::text) % 17000)) * 0.12 * 10.83, 2)
        AS estimated_revenue_recovery_ghs,

    -- Data confidence score: 60–85 (moderate confidence for estimated data)
    60 + ABS(hashtext(d.id::text || gs.period_start::text) % 26)
        AS data_confidence_score,

    NOW() AS computed_at

FROM districts d
CROSS JOIN (
    VALUES
        ('2026-01-01'::DATE),
        ('2026-02-01'::DATE),
        ('2026-03-01'::DATE)
) AS gs(period_start)
ON CONFLICT DO NOTHING;

-- Update districts.loss_ratio_pct to reflect the seeded NRW data
DO $$ BEGIN
    UPDATE districts d
    SET loss_ratio_pct = (
        SELECT ROUND(AVG(wb.nrw_percent)::numeric, 2)
        FROM water_balance_records wb
        WHERE wb.district_id = d.id
          AND wb.period_start >= '2026-01-01'
    )
    WHERE EXISTS (
        SELECT 1 FROM water_balance_records wb WHERE wb.district_id = d.id
    );
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'Update loss_ratio_pct: %', SQLERRM;
END $$;
