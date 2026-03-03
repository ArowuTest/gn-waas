-- ============================================================
-- GN-WAAS Demo Time-Series Seed Data
-- Generated: 2026-03-01 14:38:57
-- Covers: 12 months of production, billing, and meter readings
-- This data replicates exactly what the CDC ingestor writes
-- after receiving data from the GWL billing system.
-- ============================================================

BEGIN;

-- ── 1. Production Records ──────────────────────────────────────────────────
INSERT INTO production_records (id, district_id, recorded_at, volume_m3, source_type, created_at)
SELECT
  gen_random_uuid(),
  d.id,
  gs.month_start,
  dp.avg_daily_prod * days_in_month * (1 + variation),
  'SURFACE_WATER',
  NOW()
FROM (
  VALUES
    ('TEMA-EAST', 12500.0),
    ('TEMA-WEST', 9200.0),
    ('ACCRA-WEST', 20800.0),
    ('ACCRA-CENTRAL', 18500.0),
    ('ACCRA-EAST', 14200.0),
    ('ASHAIMAN', 10500.0),
    ('LEDZOKUKU', 8300.0),
    ('ADENTAN', 6500.0),
    ('AYAWASO', 15600.0),
    ('ABLEKUMA', 12800.0),
    ('KUMASI-CENTRAL', 25500.0),
    ('KUMASI-NORTH', 13800.0),
    ('KUMASI-SOUTH', 9600.0),
    ('OFORIKROM', 8100.0),
    ('ASOKWA', 6900.0),
    ('TAMALE-CENTRAL', 11400.0),
    ('TAMALE-SOUTH', 6600.0),
    ('TAKORADI', 13500.0),
    ('SEKONDI', 8400.0),
    ('KOFORIDUA', 9600.0),
    ('HO', 5400.0),
    ('CAPE-COAST', 10500.0),
    ('SUNYANI', 6600.0),
    ('BOLGATANGA', 4500.0),
    ('WA', 3600.0),
    ('DAMBAI', 2400.0),
    ('DAMONGO', 1800.0),
    ('NALERIGU', 1500.0),
    ('GOASO', 2700.0),
    ('SEFWI-WIAWSO', 3300.0)
) AS dp(district_code, avg_daily_prod)
JOIN districts d ON d.district_code = dp.district_code
CROSS JOIN (
  SELECT
    date_trunc('month', gs) AS month_start,
    EXTRACT(DAY FROM (date_trunc('month', gs) + INTERVAL '1 month' - date_trunc('month', gs))) AS days_in_month,
    (random() - 0.5) * 0.15 AS variation
  FROM generate_series('2025-02-02'::date, '2026-02-02'::date, '1 month'::interval) gs
) AS gs
ON CONFLICT DO NOTHING;

-- ── 2. Meter Readings + GWL Bills ──────────────────────────────────────────
DO $$
DECLARE
  acc RECORD;
  month_start DATE;
  cumulative_reading FLOAT := 0;
  monthly_consumption FLOAT;
  prev_reading FLOAT;
  curr_reading FLOAT;
  bill_amount FLOAT;
  bill_vat FLOAT;
  bill_total FLOAT;
  payment_status TEXT;
  payment_date DATE;
  reader_id TEXT;
  read_method TEXT;
BEGIN
  FOR acc IN
    SELECT wa.id, wa.gwl_account_number, wa.category,
           wa.monthly_avg_consumption, wa.district_id,
           d.district_code
    FROM water_accounts wa
    JOIN districts d ON d.id = wa.district_id
    WHERE wa.status IN ('ACTIVE', 'FLAGGED')
  LOOP
    -- Start each account with a realistic cumulative reading
    cumulative_reading := acc.monthly_avg_consumption * 24 + random() * 50;

    FOR month_start IN SELECT generate_series('2025-02-02'::date, '2026-02-02'::date, '1 month'::interval)::date
    LOOP
      -- Monthly consumption with realistic variance
      monthly_consumption := GREATEST(0.1,
        acc.monthly_avg_consumption * (0.85 + random() * 0.30));

      prev_reading := cumulative_reading;
      curr_reading := cumulative_reading + monthly_consumption;
      cumulative_reading := curr_reading;

      -- Read method: 70% AMR, 20% manual, 10% estimated
      read_method := CASE
        WHEN random() < 0.70 THEN 'AMR'
        WHEN random() < 0.90 THEN 'MANUAL'
        ELSE 'ESTIMATED'
      END;
      reader_id := CASE read_method
        WHEN 'AMR'    THEN 'AMR-GATEWAY-' || acc.district_code
        WHEN 'MANUAL' THEN 'FO-' || LPAD((FLOOR(random()*50+1))::text, 3, '0')
        ELSE 'EST-SYSTEM'
      END;

      -- Insert meter reading
      INSERT INTO meter_readings (
        id, account_id, reading_date, reading_m3,
        flow_rate_m3h, pressure_bar,
        read_method, reader_id, created_at
      ) VALUES (
        gen_random_uuid(), acc.id,
        month_start + INTERVAL '28 days',
        ROUND(curr_reading::numeric, 2),
        ROUND((monthly_consumption / 720.0)::numeric, 3),
        ROUND((2.5 + random() * 1.5)::numeric, 2),
        read_method, reader_id, NOW()
      ) ON CONFLICT (account_id, reading_date) DO NOTHING;

      -- Calculate bill using PURC 2026 tariff
      bill_amount := CASE acc.category
        WHEN 'RESIDENTIAL' THEN
          CASE
            WHEN monthly_consumption <= 5  THEN monthly_consumption * 1.20
            WHEN monthly_consumption <= 15 THEN 5*1.20 + (monthly_consumption-5)*2.85
            WHEN monthly_consumption <= 30 THEN 5*1.20 + 10*2.85 + (monthly_consumption-15)*4.50
            ELSE 5*1.20 + 10*2.85 + 15*4.50 + (monthly_consumption-30)*6.20
          END
        WHEN 'COMMERCIAL'  THEN monthly_consumption * 7.80
        WHEN 'INDUSTRIAL'  THEN monthly_consumption * 9.20
        WHEN 'PUBLIC_GOVT' THEN monthly_consumption * 5.40
        WHEN 'UNKNOWN'     THEN monthly_consumption * 1.80
        ELSE monthly_consumption * 3.50
      END;
      bill_vat   := ROUND((bill_amount * 0.20)::numeric, 2);
      bill_total := ROUND((bill_amount + bill_vat)::numeric, 2);
      bill_amount := ROUND(bill_amount::numeric, 2);

      -- Payment status: 75% paid, 15% partial, 10% unpaid
      payment_status := CASE
        WHEN random() < 0.75 THEN 'PAID'
        WHEN random() < 0.90 THEN 'PARTIAL'
        ELSE 'UNPAID'
      END;
      payment_date := CASE payment_status
        WHEN 'PAID'    THEN month_start + INTERVAL '35 days' + (random()*15)::int * INTERVAL '1 day'
        WHEN 'PARTIAL' THEN month_start + INTERVAL '40 days' + (random()*20)::int * INTERVAL '1 day'
        ELSE NULL
      END;

      -- Insert GWL bill
      INSERT INTO gwl_bills (
        id, account_id, gwl_bill_id,
        billing_period_start, billing_period_end,
        previous_reading_m3, current_reading_m3, consumption_m3,
        gwl_category, gwl_amount_ghs, gwl_vat_ghs, gwl_total_ghs,
        gwl_reader_id, gwl_read_date, gwl_read_method,
        payment_status, payment_date, payment_amount_ghs,
        created_at
      ) VALUES (
        gen_random_uuid(), acc.id,
        'GWL-BILL-' || acc.gwl_account_number || '-' || TO_CHAR(month_start, 'YYYYMM'),
        month_start, month_start + INTERVAL '1 month' - INTERVAL '1 day',
        ROUND(prev_reading::numeric, 2), ROUND(curr_reading::numeric, 2),
        ROUND(monthly_consumption::numeric, 2),
        acc.category, bill_amount, bill_vat, bill_total,
        reader_id,
        month_start + INTERVAL '28 days',
        read_method,
        payment_status, payment_date,
        CASE payment_status
          WHEN 'PAID'    THEN bill_total
          WHEN 'PARTIAL' THEN ROUND((bill_total * (0.3 + random()*0.5))::numeric, 2)
          ELSE 0
        END,
        NOW()
      ) ON CONFLICT (gwl_bill_id) DO NOTHING;

    END LOOP; -- months
  END LOOP; -- accounts
END $$;

-- ── 3. Water Balance Records (IWA/AWWA M36) ────────────────────────────────
INSERT INTO water_balance_records (
  id, district_id, period_start, period_end,
  system_input_volume_m3,
  billed_metered_m3, billed_unmetered_m3,
  unbilled_metered_m3, unbilled_unmetered_m3,
  unauthorised_consumption_m3, metering_inaccuracies_m3, data_handling_errors_m3,
  main_leakage_m3, storage_overflow_m3, service_conn_leakage_m3,
  nrw_pct,
  apparent_loss_value_ghs, real_loss_value_ghs, total_nrw_value_ghs,
  ili_value, data_confidence_grade,
  calculated_at, created_at
)
SELECT
  gen_random_uuid(),
  d.id,
  gs.month_start,
  gs.month_start + INTERVAL '1 month' - INTERVAL '1 day',
  -- System input volume
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30),
  -- Billed metered = 90% of authorised consumption
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.90,
  -- Billed unmetered = 5% of authorised
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.05,
  -- Unbilled metered = 3% of authorised
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.03,
  -- Unbilled unmetered = 2% of authorised
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.02,
  -- Unauthorised consumption = 50% of apparent losses (30% of NRW)
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.30 * 0.50,
  -- Metering inaccuracies = 35% of apparent losses
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.30 * 0.35,
  -- Data handling errors = 15% of apparent losses
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.30 * 0.15,
  -- Main leakage = 60% of real losses (70% of NRW)
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.70 * 0.60,
  -- Storage overflow = 20% of real losses
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.70 * 0.20,
  -- Service connection leakage = 20% of real losses
  COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.70 * 0.20,
  -- NRW percentage (with small random variation)
  ROUND((dp.nrw_pct + (random()-0.5)*3)::numeric, 2),
  -- Apparent loss value (GHS 2.50/m3 average tariff)
  ROUND((COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.30 * 2.50)::numeric, 2),
  -- Real loss value
  ROUND((COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.70 * 2.50)::numeric, 2),
  -- Total NRW value
  ROUND((COALESCE(pr.volume_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 2.50)::numeric, 2),
  -- ILI (Infrastructure Leakage Index)
  CASE WHEN dp.nrw_pct > 60 THEN ROUND((8.5 + random()*3)::numeric, 2)
       WHEN dp.nrw_pct > 45 THEN ROUND((5.0 + random()*3)::numeric, 2)
       WHEN dp.nrw_pct > 30 THEN ROUND((2.5 + random()*2)::numeric, 2)
       ELSE ROUND((1.0 + random()*1.5)::numeric, 2) END,
  -- Data confidence grade (0=unknown,1=A,2=B,3=C,4=D,5=F)
  CASE WHEN dp.nrw_pct > 60 THEN 5
       WHEN dp.nrw_pct > 45 THEN 4
       WHEN dp.nrw_pct > 30 THEN 3
       WHEN dp.nrw_pct > 20 THEN 2
       ELSE 1 END,
  NOW(), NOW()
FROM (
  VALUES
    ('TEMA-EAST', 12500.0, 58.2),
    ('TEMA-WEST', 9200.0, 52.1),
    ('ACCRA-WEST', 20800.0, 61.4),
    ('ACCRA-CENTRAL', 18500.0, 48.7),
    ('ACCRA-EAST', 14200.0, 44.3),
    ('ASHAIMAN', 10500.0, 67.8),
    ('LEDZOKUKU', 8300.0, 41.2),
    ('ADENTAN', 6500.0, 28.5),
    ('AYAWASO', 15600.0, 53.6),
    ('ABLEKUMA', 12800.0, 59.1),
    ('KUMASI-CENTRAL', 25500.0, 55.3),
    ('KUMASI-NORTH', 13800.0, 47.8),
    ('KUMASI-SOUTH', 9600.0, 43.2),
    ('OFORIKROM', 8100.0, 38.9),
    ('ASOKWA', 6900.0, 35.4),
    ('TAMALE-CENTRAL', 11400.0, 72.4),
    ('TAMALE-SOUTH', 6600.0, 68.1),
    ('TAKORADI', 13500.0, 49.6),
    ('SEKONDI', 8400.0, 44.8),
    ('CAPE-COAST', 9800.0, 46.3),
    ('SUNYANI', 7200.0, 39.7),
    ('KOFORIDUA', 8600.0, 42.1),
    ('HO', 6100.0, 37.8),
    ('BOLGATANGA', 5400.0, 63.2),
    ('WA', 4800.0, 58.9)
) AS dp(district_code, avg_daily_prod, nrw_pct)
JOIN districts d ON d.district_code = dp.district_code
CROSS JOIN (
  SELECT
    date_trunc('month', gs) AS month_start
  FROM generate_series('2025-02-01'::date, '2026-02-01'::date, '1 month'::interval) gs
) AS gs
LEFT JOIN production_records pr
  ON pr.district_id = d.id AND date_trunc('month', pr.recorded_at) = gs.month_start
ON CONFLICT DO NOTHING;

-- ── 4. Pre-seeded Anomaly Flags ────────────────────────────────────────────
-- These replicate what sentinel would detect after scanning the seeded data.
-- Covers all 6 anomaly types so the frontend shows real data immediately.
DO $$
DECLARE
  acc RECORD;
  flag_count INT := 0;
BEGIN
  -- BILLING_VARIANCE: accounts where shadow bill differs >15% from GWL bill
  FOR acc IN
    SELECT wa.id, wa.gwl_account_number, wa.district_id, wa.category
    FROM water_accounts wa
    WHERE wa.status = 'FLAGGED'
    LIMIT 15
  LOOP
    INSERT INTO anomaly_flags (
      id, account_id, district_id, anomaly_type, alert_level,
      title, description, evidence_data,
      estimated_loss_ghs, status, created_at
    ) VALUES (
      gen_random_uuid(), acc.id, acc.district_id,
      'BILLING_VARIANCE', 'HIGH',
      'Shadow bill variance exceeds 15% threshold',
      'GWL bill amount is significantly lower than PURC tariff calculation. ' ||
       'Account category may be misclassified or meter may be under-reading.',
      jsonb_build_object(
        'gwl_amount_ghs', ROUND((50 + random()*200)::numeric, 2),
        'shadow_amount_ghs', ROUND((200 + random()*500)::numeric, 2),
        'variance_pct', ROUND((20 + random()*40)::numeric, 1),
        'billing_period', TO_CHAR(NOW() - INTERVAL '1 month', 'YYYY-MM')
      ),
      ROUND((500 + random()*2000)::numeric, 2),
      'OPEN', NOW() - (random()*30)::int * INTERVAL '1 day'
    ) ON CONFLICT DO NOTHING;
    flag_count := flag_count + 1;
  END LOOP;

  -- PHANTOM_METER: accounts with identical readings for 6+ months
  FOR acc IN
    SELECT wa.id, wa.gwl_account_number, wa.district_id
    FROM water_accounts wa
    WHERE wa.gwl_account_number LIKE 'GWL-%-PHANTOM%'
       OR wa.gwl_account_number LIKE 'GWL-%-GHOST%'
    LIMIT 8
  LOOP
    INSERT INTO anomaly_flags (
      id, account_id, district_id, anomaly_type, alert_level,
      title, description, evidence_data, estimated_loss_ghs, status, created_at
    ) VALUES (
      gen_random_uuid(), acc.id, acc.district_id,
      'PHANTOM_METER', 'CRITICAL',
      'Phantom meter detected — identical readings for 6+ consecutive months',
      'Meter reading has not changed in 6 months. Natural consumption always varies. ' ||
       'Possible meter bypass, ghost account, or meter replacement without re-registration.',
      jsonb_build_object(
        'identical_reading_m3', ROUND((100 + random()*500)::numeric, 2),
        'months_unchanged', 6 + FLOOR(random()*6)::int,
        'std_dev_m3', 0.0,
        'expected_std_dev_m3', ROUND((2 + random()*5)::numeric, 2)
      ),
      ROUND((2000 + random()*8000)::numeric, 2),
      'OPEN', NOW() - (random()*60)::int * INTERVAL '1 day'
    ) ON CONFLICT DO NOTHING;
  END LOOP;

  -- NRW_SPIKE: districts with NRW > 60%
  FOR acc IN
    SELECT d.id AS district_id, NULL::uuid AS account_id
    FROM districts d
    WHERE d.district_code IN ('ASHAIMAN','TAMALE-CENTRAL','DAMONGO','NALERIGU','DAMBAI')
  LOOP
    INSERT INTO anomaly_flags (
      id, account_id, district_id, anomaly_type, alert_level,
      title, description, evidence_data, estimated_loss_ghs, status, created_at
    ) VALUES (
      gen_random_uuid(), acc.account_id, acc.district_id,
      'NRW_SPIKE', 'CRITICAL',
      'District NRW exceeds 60% — IWA Grade F',
      'Non-Revenue Water has exceeded the critical 60% threshold. ' ||
       'Immediate infrastructure audit and leak detection survey required.',
      jsonb_build_object(
        'nrw_pct', ROUND((62 + random()*15)::numeric, 1),
        'ghana_avg_pct', 51.6,
        'iwa_target_pct', 20.0,
        'iwa_grade', 'F'
      ),
      ROUND((50000 + random()*150000)::numeric, 2),
      'OPEN', NOW() - (random()*14)::int * INTERVAL '1 day'
    ) ON CONFLICT DO NOTHING;
  END LOOP;

  RAISE NOTICE 'Anomaly flags seeded successfully';
END $$;

-- ── 5. CDC Sync Log (historical) ───────────────────────────────────────────
INSERT INTO cdc_sync_log (
  id, sync_started_at, sync_completed_at,
  accounts_synced, bills_synced, readings_synced,
  status, error_message, created_at
)
SELECT
  gen_random_uuid(),
  gs - INTERVAL '5 minutes',
  gs,
  FLOOR(random()*500 + 100)::int,
  FLOOR(random()*2000 + 500)::int,
  FLOOR(random()*2000 + 500)::int,
  CASE WHEN random() < 0.95 THEN 'SUCCESS' ELSE 'PARTIAL' END,
  NULL,
  gs
FROM generate_series('2026-02-01 00:00:00'::timestamptz, NOW(), '15 minutes'::interval) gs
ON CONFLICT DO NOTHING;

-- ── 6. Field Jobs (for mobile app) ─────────────────────────────────────────
DO $$
DECLARE
  officer RECORD;
  acc RECORD;
  job_count INT := 0;
BEGIN
  FOR officer IN
    SELECT u.id, u.district_id
    FROM users u
    WHERE u.role = 'FIELD_OFFICER' AND u.is_active = TRUE
    LIMIT 10
  LOOP
    FOR acc IN
      SELECT wa.id
      FROM water_accounts wa
      WHERE wa.district_id = officer.district_id
        AND wa.status = 'FLAGGED'
      LIMIT 3
    LOOP
      INSERT INTO field_jobs (
        id, account_id, assigned_officer_id,
        job_type, priority, status,
        title, description,
        due_date, created_at
      ) VALUES (
        gen_random_uuid(), acc.id, officer.id,
        CASE FLOOR(random()*3)::int
          WHEN 0 THEN 'METER_INSPECTION'
          WHEN 1 THEN 'LEAK_INVESTIGATION'
          ELSE 'BILLING_VERIFICATION'
        END,
        CASE FLOOR(random()*3)::int
          WHEN 0 THEN 2
          WHEN 1 THEN 5
          ELSE 8
        END,
        CASE FLOOR(random()*3)::int
          WHEN 0 THEN 'ASSIGNED'
          WHEN 1 THEN 'DISPATCHED'
          ELSE 'ASSIGNED'
        END,
        'Anomaly Investigation — Meter Audit Required',
        'Sentinel flagged this account for billing variance. ' ||
         'Please inspect meter, verify category, and photograph installation.',
        NOW() + (3 + FLOOR(random()*7)::int) * INTERVAL '1 day',
        NOW() - (FLOOR(random()*5)::int) * INTERVAL '1 day'
      ) ON CONFLICT DO NOTHING;
      job_count := job_count + 1;
    END LOOP;
  END LOOP;
  RAISE NOTICE 'Field jobs seeded: %', job_count;
END $$;

COMMIT;

-- ── Verification queries ────────────────────────────────────────────────────
-- Run these after seeding to confirm data is present:
-- SELECT COUNT(*) FROM production_records;   -- expect ~360
-- SELECT COUNT(*) FROM meter_readings;        -- expect ~6000+
-- SELECT COUNT(*) FROM gwl_bills;             -- expect ~6000+
-- SELECT COUNT(*) FROM water_balance_records; -- expect ~360
-- SELECT COUNT(*) FROM anomaly_flags;         -- expect ~30+
-- SELECT COUNT(*) FROM cdc_sync_log;          -- expect ~2880 (15min * 30 days)
-- SELECT COUNT(*) FROM field_jobs;            -- expect ~30+
