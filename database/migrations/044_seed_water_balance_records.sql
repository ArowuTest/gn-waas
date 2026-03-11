-- Migration 044: Seed Water Balance Records for All Districts
-- ===========================================================
-- Purpose: Populate water_balance_records with realistic production-like data
-- for all 25 districts across Jan-Mar 2026. This enables:
--   1. NRW Analysis page to show real charts and trends
--   2. Sentinel pipeline testing (water balance computation)
--   3. DMA heatmap to show district-level NRW data
--   4. Reports to generate meaningful PDF/CSV exports
--
-- Data is based on Ghana Water Limited operational parameters:
--   - National average NRW: 51.6% (per GN-WAAS mandate)
--   - Target NRW: 20%
--   - Districts range from 35% (best) to 68% (worst)
--   - IWA/AWWA M36 Water Balance framework
--   - ILI (Infrastructure Leakage Index) range: 2.5 - 8.0
--
-- Sentinel writes these records in production via iwa_water_balance.go
-- This seed data replicates what sentinel would compute from real meter data.

DO $$
DECLARE
    -- Period definitions
    jan_start   TIMESTAMPTZ := '2026-01-01 00:00:00+00';
    jan_end     TIMESTAMPTZ := '2026-01-31 23:59:59+00';
    feb_start   TIMESTAMPTZ := '2026-02-01 00:00:00+00';
    feb_end     TIMESTAMPTZ := '2026-02-28 23:59:59+00';
    mar_start   TIMESTAMPTZ := '2026-03-01 00:00:00+00';
    mar_end     TIMESTAMPTZ := '2026-03-11 23:59:59+00';

    -- District record
    d           RECORD;

    -- Computed values
    sys_input   NUMERIC;
    nrw_pct     NUMERIC;
    real_loss   NUMERIC;
    apparent    NUMERIC;
    billed_m    NUMERIC;
    billed_u    NUMERIC;
    unbilled_m  NUMERIC;
    unbilled_u  NUMERIC;
    auth_cons   NUMERIC;
    unauth_cons NUMERIC;
    meter_inac  NUMERIC;
    data_err    NUMERIC;
    main_leak   NUMERIC;
    stor_over   NUMERIC;
    svc_leak    NUMERIC;
    total_nrw   NUMERIC;
    rev_recov   NUMERIC;
    ili         NUMERIC;
    dcs         INTEGER;
    seed_offset NUMERIC;

BEGIN
    -- Loop over all active districts
    FOR d IN SELECT id, district_code, district_name FROM districts WHERE is_active = TRUE ORDER BY district_code LOOP

        -- Each district gets a slightly different NRW profile
        -- Based on district_code hash to get deterministic but varied values
        seed_offset := (ascii(substring(d.district_code, 1, 1)) % 20)::NUMERIC;

        -- System input volume: 800,000 - 2,500,000 m3/month depending on district size
        sys_input := 1200000 + (seed_offset * 65000);

        -- NRW percentage: 35% - 68% (national average 51.6%)
        nrw_pct := 38.0 + (seed_offset * 1.5);

        -- ILI: 2.5 - 8.0 (higher = worse infrastructure)
        ili := 2.5 + (seed_offset * 0.28);

        -- Data Confidence Score: 55-85 (higher = more reliable data)
        dcs := 55 + (seed_offset::INTEGER % 30);

        -- Compute water balance components (IWA/AWWA M36)
        total_nrw   := sys_input * (nrw_pct / 100.0);
        auth_cons   := sys_input - total_nrw;

        -- Apparent losses: ~30% of NRW (meter inaccuracies + data errors + unauthorised)
        apparent    := total_nrw * 0.30;
        unauth_cons := apparent * 0.45;   -- 45% of apparent = unauthorised consumption
        meter_inac  := apparent * 0.35;   -- 35% = meter inaccuracies
        data_err    := apparent * 0.20;   -- 20% = data handling errors

        -- Real losses: ~70% of NRW (physical leakage)
        real_loss   := total_nrw * 0.70;
        main_leak   := real_loss * 0.55;  -- 55% = main pipe leakage
        stor_over   := real_loss * 0.10;  -- 10% = storage overflow
        svc_leak    := real_loss * 0.35;  -- 35% = service connection leakage

        -- Authorised consumption breakdown
        billed_m    := auth_cons * 0.82;  -- 82% billed metered
        billed_u    := auth_cons * 0.08;  -- 8% billed unmetered
        unbilled_m  := auth_cons * 0.06;  -- 6% unbilled metered (fire hydrants etc)
        unbilled_u  := auth_cons * 0.04;  -- 4% unbilled unmetered

        -- Revenue recovery potential: apparent losses × GHS 6.12/m3 (residential tariff)
        rev_recov   := apparent * 6.12;

        -- ── January 2026 ──────────────────────────────────────────────────────
        INSERT INTO water_balance_records (
            district_id, period_start, period_end,
            system_input_volume_m3,
            billed_metered_m3, billed_unmetered_m3,
            unbilled_metered_m3, unbilled_unmetered_m3,
            total_authorised_m3,
            unauthorised_consumption_m3, metering_inaccuracies_m3, data_handling_errors_m3,
            total_apparent_losses_m3,
            main_leakage_m3, storage_overflow_m3, service_conn_leakage_m3,
            total_real_losses_m3,
            total_nrw_m3,
            nrw_percent, ili_score, iwa_grade,
            estimated_revenue_recovery_ghs,
            data_confidence_score,
            computed_at
        ) VALUES (
            d.id, jan_start, jan_end,
            ROUND(sys_input, 2),
            ROUND(billed_m, 2), ROUND(billed_u, 2),
            ROUND(unbilled_m, 2), ROUND(unbilled_u, 2),
            ROUND(auth_cons, 2),
            ROUND(unauth_cons, 2), ROUND(meter_inac, 2), ROUND(data_err, 2),
            ROUND(apparent, 2),
            ROUND(main_leak, 2), ROUND(stor_over, 2), ROUND(svc_leak, 2),
            ROUND(real_loss, 2),
            ROUND(total_nrw, 2),
            ROUND(nrw_pct, 4),
            ROUND(ili, 4),
            CASE WHEN ili < 1.5 THEN 'A' WHEN ili < 2.5 THEN 'B' WHEN ili < 4.0 THEN 'C' ELSE 'D' END,
            ROUND(rev_recov, 2),
            dcs,
            jan_end
        )
        ON CONFLICT (district_id, period_start, period_end) DO UPDATE SET
            system_input_volume_m3 = EXCLUDED.system_input_volume_m3,
            nrw_percent = EXCLUDED.nrw_percent,
            total_nrw_m3 = EXCLUDED.total_nrw_m3,
            ili_score = EXCLUDED.ili_score,
            estimated_revenue_recovery_ghs = EXCLUDED.estimated_revenue_recovery_ghs,
            data_confidence_score = EXCLUDED.data_confidence_score,
            computed_at = NOW();

        -- ── February 2026 (slight improvement — sentinel interventions) ────────
        -- NRW improves by 1-3% as audit actions take effect
        nrw_pct     := nrw_pct - (1.0 + (seed_offset * 0.1));
        total_nrw   := sys_input * (nrw_pct / 100.0);
        auth_cons   := sys_input - total_nrw;
        apparent    := total_nrw * 0.30;
        unauth_cons := apparent * 0.45;
        meter_inac  := apparent * 0.35;
        data_err    := apparent * 0.20;
        real_loss   := total_nrw * 0.70;
        main_leak   := real_loss * 0.55;
        stor_over   := real_loss * 0.10;
        svc_leak    := real_loss * 0.35;
        billed_m    := auth_cons * 0.82;
        billed_u    := auth_cons * 0.08;
        unbilled_m  := auth_cons * 0.06;
        unbilled_u  := auth_cons * 0.04;
        rev_recov   := apparent * 6.12;
        ili         := ili - 0.15;

        INSERT INTO water_balance_records (
            district_id, period_start, period_end,
            system_input_volume_m3,
            billed_metered_m3, billed_unmetered_m3,
            unbilled_metered_m3, unbilled_unmetered_m3,
            total_authorised_m3,
            unauthorised_consumption_m3, metering_inaccuracies_m3, data_handling_errors_m3,
            total_apparent_losses_m3,
            main_leakage_m3, storage_overflow_m3, service_conn_leakage_m3,
            total_real_losses_m3,
            total_nrw_m3,
            nrw_percent, ili_score, iwa_grade,
            estimated_revenue_recovery_ghs,
            data_confidence_score,
            computed_at
        ) VALUES (
            d.id, feb_start, feb_end,
            ROUND(sys_input, 2),
            ROUND(billed_m, 2), ROUND(billed_u, 2),
            ROUND(unbilled_m, 2), ROUND(unbilled_u, 2),
            ROUND(auth_cons, 2),
            ROUND(unauth_cons, 2), ROUND(meter_inac, 2), ROUND(data_err, 2),
            ROUND(apparent, 2),
            ROUND(main_leak, 2), ROUND(stor_over, 2), ROUND(svc_leak, 2),
            ROUND(real_loss, 2),
            ROUND(total_nrw, 2),
            ROUND(nrw_pct, 4),
            ROUND(ili, 4),
            CASE WHEN ili < 1.5 THEN 'A' WHEN ili < 2.5 THEN 'B' WHEN ili < 4.0 THEN 'C' ELSE 'D' END,
            ROUND(rev_recov, 2),
            dcs,
            feb_end
        )
        ON CONFLICT (district_id, period_start, period_end) DO UPDATE SET
            system_input_volume_m3 = EXCLUDED.system_input_volume_m3,
            nrw_percent = EXCLUDED.nrw_percent,
            total_nrw_m3 = EXCLUDED.total_nrw_m3,
            ili_score = EXCLUDED.ili_score,
            estimated_revenue_recovery_ghs = EXCLUDED.estimated_revenue_recovery_ghs,
            data_confidence_score = EXCLUDED.data_confidence_score,
            computed_at = NOW();

        -- ── March 2026 (partial month — up to current date) ───────────────────
        -- Pro-rate to 11/31 days
        sys_input   := sys_input * (11.0 / 31.0);
        nrw_pct     := nrw_pct - 0.5;  -- Continued improvement
        total_nrw   := sys_input * (nrw_pct / 100.0);
        auth_cons   := sys_input - total_nrw;
        apparent    := total_nrw * 0.30;
        unauth_cons := apparent * 0.45;
        meter_inac  := apparent * 0.35;
        data_err    := apparent * 0.20;
        real_loss   := total_nrw * 0.70;
        main_leak   := real_loss * 0.55;
        stor_over   := real_loss * 0.10;
        svc_leak    := real_loss * 0.35;
        billed_m    := auth_cons * 0.82;
        billed_u    := auth_cons * 0.08;
        unbilled_m  := auth_cons * 0.06;
        unbilled_u  := auth_cons * 0.04;
        rev_recov   := apparent * 6.12;

        INSERT INTO water_balance_records (
            district_id, period_start, period_end,
            system_input_volume_m3,
            billed_metered_m3, billed_unmetered_m3,
            unbilled_metered_m3, unbilled_unmetered_m3,
            total_authorised_m3,
            unauthorised_consumption_m3, metering_inaccuracies_m3, data_handling_errors_m3,
            total_apparent_losses_m3,
            main_leakage_m3, storage_overflow_m3, service_conn_leakage_m3,
            total_real_losses_m3,
            total_nrw_m3,
            nrw_percent, ili_score, iwa_grade,
            estimated_revenue_recovery_ghs,
            data_confidence_score,
            computed_at
        ) VALUES (
            d.id, mar_start, mar_end,
            ROUND(sys_input, 2),
            ROUND(billed_m, 2), ROUND(billed_u, 2),
            ROUND(unbilled_m, 2), ROUND(unbilled_u, 2),
            ROUND(auth_cons, 2),
            ROUND(unauth_cons, 2), ROUND(meter_inac, 2), ROUND(data_err, 2),
            ROUND(apparent, 2),
            ROUND(main_leak, 2), ROUND(stor_over, 2), ROUND(svc_leak, 2),
            ROUND(real_loss, 2),
            ROUND(total_nrw, 2),
            ROUND(nrw_pct, 4),
            ROUND(ili, 4),
            CASE WHEN ili < 1.5 THEN 'A' WHEN ili < 2.5 THEN 'B' WHEN ili < 4.0 THEN 'C' ELSE 'D' END,
            ROUND(rev_recov, 2),
            dcs,
            mar_end
        )
        ON CONFLICT (district_id, period_start, period_end) DO UPDATE SET
            system_input_volume_m3 = EXCLUDED.system_input_volume_m3,
            nrw_percent = EXCLUDED.nrw_percent,
            total_nrw_m3 = EXCLUDED.total_nrw_m3,
            ili_score = EXCLUDED.ili_score,
            estimated_revenue_recovery_ghs = EXCLUDED.estimated_revenue_recovery_ghs,
            data_confidence_score = EXCLUDED.data_confidence_score,
            computed_at = NOW();

    END LOOP;

    RAISE NOTICE 'Migration 044: Water balance records seeded for all active districts (Jan-Mar 2026)';
END;
$$;

-- Update districts.loss_ratio_pct with the latest NRW values from water_balance_records
-- This ensures the NRW Summary page shows current values
UPDATE districts d
SET loss_ratio_pct = wb.nrw_percent
FROM (
    SELECT DISTINCT ON (district_id)
        district_id,
        nrw_percent
    FROM water_balance_records
    ORDER BY district_id, period_start DESC
) wb
WHERE d.id = wb.district_id;

COMMENT ON TABLE water_balance_records IS
    'IWA/AWWA Water Balance records per district per period. '
    'Seeded with production-like data in migration 044. '
    'In production, sentinel service writes these via iwa_water_balance.go.';
