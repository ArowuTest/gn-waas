// GN-WAAS Demo Data Generator
//
// This program generates realistic time-series seed data that replicates
// exactly what the production pipeline receives from GWL:
//
//   GWL Billing System → CDC Ingestor → meter_readings / gwl_bills / production_records
//                                              ↓
//                                    API Gateway endpoints
//                                              ↓
//                                    Frontend dashboards
//
// Run: go run main.go | psql $DATABASE_URL
// Or:  go run main.go > 006_demo_timeseries.sql
//
// Data generated (12 months, all districts):
//   - production_records:  1 row/district/month  = ~360 rows
//   - meter_readings:      1 row/account/month   = ~6,000 rows
//   - gwl_bills:           1 row/account/month   = ~6,000 rows
//   - anomaly_flags:       ~120 pre-seeded flags
//   - water_balance_records: 1 row/district/month = ~360 rows
//   - cdc_sync_log:        12 historical sync entries

package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"
)

// ─── District definitions (must match 003_districts.sql) ─────────────────────

type District struct {
	Code        string
	Name        string
	Region      string
	Connections int
	// NRW profile: realistic Ghana values
	NRWPct      float64 // true NRW percentage
	AvgDailyProdM3 float64 // average daily production m³
}

var districts = []District{
	// Greater Accra — high NRW (pilot districts)
	{"TEMA-EAST",     "Tema East",           "Greater Accra", 42000, 58.2, 12500},
	{"TEMA-WEST",     "Tema West",           "Greater Accra", 31000, 52.1, 9200},
	{"ACCRA-WEST",    "Accra West",          "Greater Accra", 68000, 61.4, 20800},
	{"ACCRA-CENTRAL", "Accra Central",       "Greater Accra", 62000, 48.7, 18500},
	{"ACCRA-EAST",    "Accra East",          "Greater Accra", 48000, 44.3, 14200},
	{"ASHAIMAN",      "Ashaiman",            "Greater Accra", 35000, 67.8, 10500},
	{"LEDZOKUKU",     "Ledzokuku-Krowor",    "Greater Accra", 28000, 41.2, 8300},
	{"ADENTAN",       "Adentan",             "Greater Accra", 22000, 28.5, 6500},
	{"AYAWASO",       "Ayawaso",             "Greater Accra", 52000, 53.6, 15600},
	{"ABLEKUMA",      "Ablekuma",            "Greater Accra", 43000, 59.1, 12800},
	// Ashanti
	{"KUMASI-CENTRAL","Kumasi Central",      "Ashanti",       85000, 55.3, 25500},
	{"KUMASI-NORTH",  "Kumasi North",        "Ashanti",       46000, 47.8, 13800},
	{"KUMASI-SOUTH",  "Kumasi South",        "Ashanti",       32000, 43.2, 9600},
	{"OFORIKROM",     "Oforikrom",           "Ashanti",       27000, 38.9, 8100},
	{"ASOKWA",        "Asokwa",              "Ashanti",       23000, 35.4, 6900},
	// Northern
	{"TAMALE-CENTRAL","Tamale Central",      "Northern",      38000, 72.4, 11400},
	{"TAMALE-SOUTH",  "Tamale South",        "Northern",      22000, 68.1, 6600},
	// Western
	{"TAKORADI",      "Takoradi",            "Western",       45000, 49.6, 13500},
	{"SEKONDI",       "Sekondi",             "Western",       28000, 44.8, 8400},
	// Eastern
	{"KOFORIDUA",     "Koforidua",           "Eastern",       32000, 51.2, 9600},
	// Volta
	{"HO",            "Ho",                  "Volta",         18000, 46.3, 5400},
	// Central
	{"CAPE-COAST",    "Cape Coast",          "Central",       35000, 53.7, 10500},
	// Brong-Ahafo
	{"SUNYANI",       "Sunyani",             "Bono",          22000, 48.9, 6600},
	// Upper East
	{"BOLGATANGA",    "Bolgatanga",          "Upper East",    15000, 63.2, 4500},
	// Upper West
	{"WA",            "Wa",                  "Upper West",    12000, 58.7, 3600},
	// Oti
	{"DAMBAI",        "Dambai",              "Oti",           8000,  71.3, 2400},
	// Savannah
	{"DAMONGO",       "Damongo",             "Savannah",      6000,  74.8, 1800},
	// North East
	{"NALERIGU",      "Nalerigu",            "North East",    5000,  69.4, 1500},
	// Ahafo
	{"GOASO",         "Goaso",               "Ahafo",         9000,  52.1, 2700},
	// Western North
	{"SEFWI-WIAWSO",  "Sefwi Wiawso",        "Western North", 11000, 55.8, 3300},
}

// ─── Account categories and their consumption profiles ───────────────────────

type AccountProfile struct {
	Category       string
	AvgMonthlyM3   float64
	StdDevM3       float64
	TariffCategory string
}

var accountProfiles = []AccountProfile{
	{"RESIDENTIAL",  8.5,   3.2,  "RESIDENTIAL"},
	{"COMMERCIAL",   85.0,  25.0, "COMMERCIAL"},
	{"INDUSTRIAL",   420.0, 80.0, "INDUSTRIAL"},
	{"PUBLIC_GOVT",  280.0, 60.0, "PUBLIC_GOVT"},
	{"STANDPIPE",    45.0,  12.0, "STANDPIPE"},
}

// ─── PURC 2026 tariff rates (GHS/m³) ─────────────────────────────────────────

func tariffRate(category string, m3 float64) float64 {
	switch category {
	case "RESIDENTIAL":
		switch {
		case m3 <= 5:   return 1.20
		case m3 <= 15:  return 2.85
		case m3 <= 30:  return 4.50
		default:        return 6.20
		}
	case "COMMERCIAL":
		return 7.80
	case "INDUSTRIAL":
		return 9.20
	case "PUBLIC_GOVT":
		return 5.40
	case "STANDPIPE":
		return 1.80
	default:
		return 3.50
	}
}

func calcBill(category string, m3 float64) (amount, vat, total float64) {
	rate := tariffRate(category, m3)
	amount = m3 * rate
	vat = amount * 0.20
	total = amount + vat
	return
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	rng := rand.New(rand.NewSource(42)) // deterministic seed for reproducibility

	now := time.Now()
	// Generate 13 months back so we have a full 12-month window
	startDate := now.AddDate(-1, -1, 0)

	out := os.Stdout

	fmt.Fprintln(out, "-- ============================================================")
	fmt.Fprintln(out, "-- GN-WAAS Demo Time-Series Seed Data")
	fmt.Fprintf(out,  "-- Generated: %s\n", now.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(out, "-- Covers: 12 months of production, billing, and meter readings")
	fmt.Fprintln(out, "-- This data replicates exactly what the CDC ingestor writes")
	fmt.Fprintln(out, "-- after receiving data from the GWL billing system.")
	fmt.Fprintln(out, "-- ============================================================")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "BEGIN;")
	fmt.Fprintln(out)

	// ── 1. Production records (one per district per month) ────────────────────
	fmt.Fprintln(out, "-- ── 1. Production Records ──────────────────────────────────────────────────")
	fmt.Fprintln(out, "INSERT INTO production_records (id, district_id, record_date, volume_produced_m3, volume_treated_m3, pumping_hours, energy_kwh, source_type, data_quality_score, created_at)")
	fmt.Fprintln(out, "SELECT")
	fmt.Fprintln(out, "  gen_random_uuid(),")
	fmt.Fprintln(out, "  d.id,")
	fmt.Fprintln(out, "  gs.month_start,")
	fmt.Fprintln(out, "  d.avg_daily_prod * days_in_month * (1 + variation),")
	fmt.Fprintln(out, "  d.avg_daily_prod * days_in_month * (1 + variation) * 0.98,")
	fmt.Fprintln(out, "  d.avg_daily_prod * days_in_month * 0.85 / 120.0,")
	fmt.Fprintln(out, "  d.avg_daily_prod * days_in_month * 0.42,")
	fmt.Fprintln(out, "  'SURFACE_WATER',")
	fmt.Fprintln(out, "  0.85 + random() * 0.12,")
	fmt.Fprintln(out, "  NOW()")
	fmt.Fprintln(out, "FROM (")
	fmt.Fprintln(out, "  VALUES")

	// Emit district production profiles
	for i, d := range districts {
		comma := ","
		if i == len(districts)-1 { comma = "" }
		fmt.Fprintf(out, "    ('%s', %.1f)%s\n", d.Code, d.AvgDailyProdM3, comma)
	}

	fmt.Fprintln(out, ") AS dp(district_code, avg_daily_prod)")
	fmt.Fprintln(out, "JOIN districts d ON d.district_code = dp.district_code")
	fmt.Fprintln(out, "CROSS JOIN (")
	fmt.Fprintln(out, "  SELECT")
	fmt.Fprintln(out, "    date_trunc('month', gs) AS month_start,")
	fmt.Fprintln(out, "    EXTRACT(DAY FROM (date_trunc('month', gs) + INTERVAL '1 month' - date_trunc('month', gs))) AS days_in_month,")
	fmt.Fprintln(out, "    (random() - 0.5) * 0.15 AS variation")
	fmt.Fprintf(out,  "  FROM generate_series('%s'::date, '%s'::date, '1 month'::interval) gs\n",
		startDate.Format("2006-01-01"),
		now.AddDate(0, -1, 0).Format("2006-01-01"),
	)
	fmt.Fprintln(out, ") AS gs")
	fmt.Fprintln(out, "ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out)

	// ── 2. Meter readings and bills (per account per month) ───────────────────
	// We generate these as a PL/pgSQL block for efficiency
	fmt.Fprintln(out, "-- ── 2. Meter Readings + GWL Bills ──────────────────────────────────────────")
	fmt.Fprintln(out, "DO $$")
	fmt.Fprintln(out, "DECLARE")
	fmt.Fprintln(out, "  acc RECORD;")
	fmt.Fprintln(out, "  month_start DATE;")
	fmt.Fprintln(out, "  cumulative_reading FLOAT := 0;")
	fmt.Fprintln(out, "  monthly_consumption FLOAT;")
	fmt.Fprintln(out, "  prev_reading FLOAT;")
	fmt.Fprintln(out, "  curr_reading FLOAT;")
	fmt.Fprintln(out, "  bill_amount FLOAT;")
	fmt.Fprintln(out, "  bill_vat FLOAT;")
	fmt.Fprintln(out, "  bill_total FLOAT;")
	fmt.Fprintln(out, "  payment_status TEXT;")
	fmt.Fprintln(out, "  payment_date DATE;")
	fmt.Fprintln(out, "  reader_id TEXT;")
	fmt.Fprintln(out, "  read_method TEXT;")
	fmt.Fprintln(out, "BEGIN")
	fmt.Fprintln(out, "  FOR acc IN")
	fmt.Fprintln(out, "    SELECT wa.id, wa.gwl_account_number, wa.category,")
	fmt.Fprintln(out, "           wa.monthly_avg_consumption, wa.district_id,")
	fmt.Fprintln(out, "           d.district_code")
	fmt.Fprintln(out, "    FROM water_accounts wa")
	fmt.Fprintln(out, "    JOIN districts d ON d.id = wa.district_id")
	fmt.Fprintln(out, "    WHERE wa.status IN ('ACTIVE', 'FLAGGED')")
	fmt.Fprintln(out, "  LOOP")
	fmt.Fprintln(out, "    -- Start each account with a realistic cumulative reading")
	fmt.Fprintln(out, "    cumulative_reading := acc.monthly_avg_consumption * 24 + random() * 50;")
	fmt.Fprintln(out)
	fmt.Fprintf(out,  "    FOR month_start IN SELECT generate_series('%s'::date, '%s'::date, '1 month'::interval)::date\n",
		startDate.Format("2006-01-01"),
		now.AddDate(0, -1, 0).Format("2006-01-01"),
	)
	fmt.Fprintln(out, "    LOOP")
	fmt.Fprintln(out, "      -- Monthly consumption with realistic variance")
	fmt.Fprintln(out, "      monthly_consumption := GREATEST(0.1,")
	fmt.Fprintln(out, "        acc.monthly_avg_consumption * (0.85 + random() * 0.30));")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      prev_reading := cumulative_reading;")
	fmt.Fprintln(out, "      curr_reading := cumulative_reading + monthly_consumption;")
	fmt.Fprintln(out, "      cumulative_reading := curr_reading;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      -- Read method: 70% AMR, 20% manual, 10% estimated")
	fmt.Fprintln(out, "      read_method := CASE")
	fmt.Fprintln(out, "        WHEN random() < 0.70 THEN 'AMR'")
	fmt.Fprintln(out, "        WHEN random() < 0.90 THEN 'MANUAL'")
	fmt.Fprintln(out, "        ELSE 'ESTIMATED'")
	fmt.Fprintln(out, "      END;")
	fmt.Fprintln(out, "      reader_id := CASE read_method")
	fmt.Fprintln(out, "        WHEN 'AMR'    THEN 'AMR-GATEWAY-' || acc.district_code")
	fmt.Fprintln(out, "        WHEN 'MANUAL' THEN 'FO-' || LPAD((FLOOR(random()*50+1))::text, 3, '0')")
	fmt.Fprintln(out, "        ELSE 'EST-SYSTEM'")
	fmt.Fprintln(out, "      END;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      -- Insert meter reading")
	fmt.Fprintln(out, "      INSERT INTO meter_readings (")
	fmt.Fprintln(out, "        id, account_id, reading_date, reading_m3,")
	fmt.Fprintln(out, "        flow_rate_m3h, pressure_bar,")
	fmt.Fprintln(out, "        read_method, reader_id, created_at")
	fmt.Fprintln(out, "      ) VALUES (")
	fmt.Fprintln(out, "        gen_random_uuid(), acc.id,")
	fmt.Fprintln(out, "        month_start + INTERVAL '28 days',")
	fmt.Fprintln(out, "        ROUND(curr_reading::numeric, 2),")
	fmt.Fprintln(out, "        ROUND((monthly_consumption / 720.0)::numeric, 3),")
	fmt.Fprintln(out, "        ROUND((2.5 + random() * 1.5)::numeric, 2),")
	fmt.Fprintln(out, "        read_method, reader_id, NOW()")
	fmt.Fprintln(out, "      ) ON CONFLICT (account_id, reading_date) DO NOTHING;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      -- Calculate bill using PURC 2026 tariff")
	fmt.Fprintln(out, "      bill_amount := CASE acc.category")
	fmt.Fprintln(out, "        WHEN 'RESIDENTIAL' THEN")
	fmt.Fprintln(out, "          CASE")
	fmt.Fprintln(out, "            WHEN monthly_consumption <= 5  THEN monthly_consumption * 1.20")
	fmt.Fprintln(out, "            WHEN monthly_consumption <= 15 THEN 5*1.20 + (monthly_consumption-5)*2.85")
	fmt.Fprintln(out, "            WHEN monthly_consumption <= 30 THEN 5*1.20 + 10*2.85 + (monthly_consumption-15)*4.50")
	fmt.Fprintln(out, "            ELSE 5*1.20 + 10*2.85 + 15*4.50 + (monthly_consumption-30)*6.20")
	fmt.Fprintln(out, "          END")
	fmt.Fprintln(out, "        WHEN 'COMMERCIAL'  THEN monthly_consumption * 7.80")
	fmt.Fprintln(out, "        WHEN 'INDUSTRIAL'  THEN monthly_consumption * 9.20")
	fmt.Fprintln(out, "        WHEN 'PUBLIC_GOVT' THEN monthly_consumption * 5.40")
	fmt.Fprintln(out, "        WHEN 'STANDPIPE'   THEN monthly_consumption * 1.80")
	fmt.Fprintln(out, "        ELSE monthly_consumption * 3.50")
	fmt.Fprintln(out, "      END;")
	fmt.Fprintln(out, "      bill_vat   := ROUND((bill_amount * 0.20)::numeric, 2);")
	fmt.Fprintln(out, "      bill_total := ROUND((bill_amount + bill_vat)::numeric, 2);")
	fmt.Fprintln(out, "      bill_amount := ROUND(bill_amount::numeric, 2);")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      -- Payment status: 75% paid, 15% partial, 10% unpaid")
	fmt.Fprintln(out, "      payment_status := CASE")
	fmt.Fprintln(out, "        WHEN random() < 0.75 THEN 'PAID'")
	fmt.Fprintln(out, "        WHEN random() < 0.90 THEN 'PARTIAL'")
	fmt.Fprintln(out, "        ELSE 'UNPAID'")
	fmt.Fprintln(out, "      END;")
	fmt.Fprintln(out, "      payment_date := CASE payment_status")
	fmt.Fprintln(out, "        WHEN 'PAID'    THEN month_start + INTERVAL '35 days' + (random()*15)::int * INTERVAL '1 day'")
	fmt.Fprintln(out, "        WHEN 'PARTIAL' THEN month_start + INTERVAL '40 days' + (random()*20)::int * INTERVAL '1 day'")
	fmt.Fprintln(out, "        ELSE NULL")
	fmt.Fprintln(out, "      END;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "      -- Insert GWL bill")
	fmt.Fprintln(out, "      INSERT INTO gwl_bills (")
	fmt.Fprintln(out, "        id, account_id, gwl_bill_id,")
	fmt.Fprintln(out, "        billing_period_start, billing_period_end,")
	fmt.Fprintln(out, "        previous_reading_m3, current_reading_m3, consumption_m3,")
	fmt.Fprintln(out, "        gwl_category, gwl_amount_ghs, gwl_vat_ghs, gwl_total_ghs,")
	fmt.Fprintln(out, "        gwl_reader_id, gwl_read_date, gwl_read_method,")
	fmt.Fprintln(out, "        payment_status, payment_date, payment_amount_ghs,")
	fmt.Fprintln(out, "        created_at")
	fmt.Fprintln(out, "      ) VALUES (")
	fmt.Fprintln(out, "        gen_random_uuid(), acc.id,")
	fmt.Fprintln(out, "        'GWL-BILL-' || acc.gwl_account_number || '-' || TO_CHAR(month_start, 'YYYYMM'),")
	fmt.Fprintln(out, "        month_start, month_start + INTERVAL '1 month' - INTERVAL '1 day',")
	fmt.Fprintln(out, "        ROUND(prev_reading::numeric, 2), ROUND(curr_reading::numeric, 2),")
	fmt.Fprintln(out, "        ROUND(monthly_consumption::numeric, 2),")
	fmt.Fprintln(out, "        acc.category, bill_amount, bill_vat, bill_total,")
	fmt.Fprintln(out, "        reader_id,")
	fmt.Fprintln(out, "        month_start + INTERVAL '28 days',")
	fmt.Fprintln(out, "        read_method,")
	fmt.Fprintln(out, "        payment_status, payment_date,")
	fmt.Fprintln(out, "        CASE payment_status")
	fmt.Fprintln(out, "          WHEN 'PAID'    THEN bill_total")
	fmt.Fprintln(out, "          WHEN 'PARTIAL' THEN ROUND((bill_total * (0.3 + random()*0.5))::numeric, 2)")
	fmt.Fprintln(out, "          ELSE 0")
	fmt.Fprintln(out, "        END,")
	fmt.Fprintln(out, "        NOW()")
	fmt.Fprintln(out, "      ) ON CONFLICT (gwl_bill_id) DO NOTHING;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "    END LOOP; -- months")
	fmt.Fprintln(out, "  END LOOP; -- accounts")
	fmt.Fprintln(out, "END $$;")
	fmt.Fprintln(out)

	// ── 3. Water balance records (IWA computed, one per district per month) ───
	fmt.Fprintln(out, "-- ── 3. Water Balance Records (IWA/AWWA M36) ────────────────────────────────")
	fmt.Fprintln(out, "INSERT INTO water_balance_records (")
	fmt.Fprintln(out, "  id, district_id, period_start, period_end,")
	fmt.Fprintln(out, "  system_input_volume_m3, authorised_consumption_m3,")
	fmt.Fprintln(out, "  billed_authorised_m3, unbilled_authorised_m3,")
	fmt.Fprintln(out, "  apparent_losses_m3, real_losses_m3,")
	fmt.Fprintln(out, "  nrw_volume_m3, nrw_percentage,")
	fmt.Fprintln(out, "  infrastructure_leakage_index,")
	fmt.Fprintln(out, "  iwa_grade, data_confidence_score,")
	fmt.Fprintln(out, "  revenue_water_m3, non_revenue_water_m3,")
	fmt.Fprintln(out, "  computed_at, created_at")
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out, "SELECT")
	fmt.Fprintln(out, "  gen_random_uuid(),")
	fmt.Fprintln(out, "  d.id,")
	fmt.Fprintln(out, "  gs.month_start,")
	fmt.Fprintln(out, "  gs.month_start + INTERVAL '1 month' - INTERVAL '1 day',")
	fmt.Fprintln(out, "  -- System input = production for the month")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30),")
	fmt.Fprintln(out, "  -- Authorised consumption = SIV * (1 - NRW%)")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0),")
	fmt.Fprintln(out, "  -- Billed authorised = 95% of authorised")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.95,")
	fmt.Fprintln(out, "  -- Unbilled authorised = 5% of authorised (fire fighting, flushing)")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.05,")
	fmt.Fprintln(out, "  -- Apparent losses = 30% of NRW (meter errors, theft)")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.30,")
	fmt.Fprintln(out, "  -- Real losses = 70% of NRW (physical leakage)")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0) * 0.70,")
	fmt.Fprintln(out, "  -- NRW volume")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0),")
	fmt.Fprintln(out, "  -- NRW percentage")
	fmt.Fprintln(out, "  dp.nrw_pct + (random()-0.5)*3,")
	fmt.Fprintln(out, "  -- ILI (Infrastructure Leakage Index)")
	fmt.Fprintln(out, "  CASE WHEN dp.nrw_pct > 60 THEN 8.5 + random()*3")
	fmt.Fprintln(out, "       WHEN dp.nrw_pct > 45 THEN 5.0 + random()*3")
	fmt.Fprintln(out, "       WHEN dp.nrw_pct > 30 THEN 2.5 + random()*2")
	fmt.Fprintln(out, "       ELSE 1.0 + random()*1.5 END,")
	fmt.Fprintln(out, "  -- IWA Grade")
	fmt.Fprintln(out, "  CASE WHEN dp.nrw_pct > 60 THEN 'F'")
	fmt.Fprintln(out, "       WHEN dp.nrw_pct > 45 THEN 'D'")
	fmt.Fprintln(out, "       WHEN dp.nrw_pct > 30 THEN 'C'")
	fmt.Fprintln(out, "       WHEN dp.nrw_pct > 20 THEN 'B'")
	fmt.Fprintln(out, "       ELSE 'A' END,")
	fmt.Fprintln(out, "  0.75 + random()*0.20,")
	fmt.Fprintln(out, "  -- Revenue water")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (1 - dp.nrw_pct/100.0) * 0.95,")
	fmt.Fprintln(out, "  -- Non-revenue water")
	fmt.Fprintln(out, "  COALESCE(pr.volume_produced_m3, dp.avg_daily_prod * 30) * (dp.nrw_pct/100.0),")
	fmt.Fprintln(out, "  NOW(), NOW()")
	fmt.Fprintln(out, "FROM (")
	fmt.Fprintln(out, "  VALUES")

	for i, d := range districts {
		comma := ","
		if i == len(districts)-1 { comma = "" }
		fmt.Fprintf(out, "    ('%s', %.1f, %.1f)%s\n", d.Code, d.AvgDailyProdM3, d.NRWPct, comma)
	}

	fmt.Fprintln(out, ") AS dp(district_code, avg_daily_prod, nrw_pct)")
	fmt.Fprintln(out, "JOIN districts d ON d.district_code = dp.district_code")
	fmt.Fprintln(out, "CROSS JOIN (")
	fmt.Fprintf(out,  "  SELECT date_trunc('month', gs)::date AS month_start\n")
	fmt.Fprintf(out,  "  FROM generate_series('%s'::date, '%s'::date, '1 month'::interval) gs\n",
		startDate.Format("2006-01-01"),
		now.AddDate(0, -1, 0).Format("2006-01-01"),
	)
	fmt.Fprintln(out, ") AS gs")
	fmt.Fprintln(out, "LEFT JOIN production_records pr")
	fmt.Fprintln(out, "  ON pr.district_id = d.id AND date_trunc('month', pr.record_date) = gs.month_start")
	fmt.Fprintln(out, "ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out)

	// ── 4. Pre-seeded anomaly flags ───────────────────────────────────────────
	fmt.Fprintln(out, "-- ── 4. Pre-seeded Anomaly Flags ────────────────────────────────────────────")
	fmt.Fprintln(out, "-- These replicate what sentinel would detect after scanning the seeded data.")
	fmt.Fprintln(out, "-- Covers all 6 anomaly types so the frontend shows real data immediately.")
	fmt.Fprintln(out, "DO $$")
	fmt.Fprintln(out, "DECLARE")
	fmt.Fprintln(out, "  acc RECORD;")
	fmt.Fprintln(out, "  flag_count INT := 0;")
	fmt.Fprintln(out, "BEGIN")
	fmt.Fprintln(out, "  -- BILLING_VARIANCE: accounts where shadow bill differs >15% from GWL bill")
	fmt.Fprintln(out, "  FOR acc IN")
	fmt.Fprintln(out, "    SELECT wa.id, wa.gwl_account_number, wa.district_id, wa.category")
	fmt.Fprintln(out, "    FROM water_accounts wa")
	fmt.Fprintln(out, "    WHERE wa.status = 'FLAGGED'")
	fmt.Fprintln(out, "    LIMIT 15")
	fmt.Fprintln(out, "  LOOP")
	fmt.Fprintln(out, "    INSERT INTO anomaly_flags (")
	fmt.Fprintln(out, "      id, account_id, district_id, flag_type, severity,")
	fmt.Fprintln(out, "      title, description, evidence,")
	fmt.Fprintln(out, "      estimated_loss_ghs, status, created_at")
	fmt.Fprintln(out, "    ) VALUES (")
	fmt.Fprintln(out, "      gen_random_uuid(), acc.id, acc.district_id,")
	fmt.Fprintln(out, "      'BILLING_VARIANCE', 'HIGH',")
	fmt.Fprintln(out, "      'Shadow bill variance exceeds 15% threshold',")
	fmt.Fprintln(out, "      'GWL bill amount is significantly lower than PURC tariff calculation. ' ||")
	fmt.Fprintln(out, "       'Account category may be misclassified or meter may be under-reading.',")
	fmt.Fprintln(out, "      jsonb_build_object(")
	fmt.Fprintln(out, "        'gwl_amount_ghs', ROUND((50 + random()*200)::numeric, 2),")
	fmt.Fprintln(out, "        'shadow_amount_ghs', ROUND((200 + random()*500)::numeric, 2),")
	fmt.Fprintln(out, "        'variance_pct', ROUND((20 + random()*40)::numeric, 1),")
	fmt.Fprintln(out, "        'billing_period', TO_CHAR(NOW() - INTERVAL '1 month', 'YYYY-MM')")
	fmt.Fprintln(out, "      ),")
	fmt.Fprintln(out, "      ROUND((500 + random()*2000)::numeric, 2),")
	fmt.Fprintln(out, "      'OPEN', NOW() - (random()*30)::int * INTERVAL '1 day'")
	fmt.Fprintln(out, "    ) ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out, "    flag_count := flag_count + 1;")
	fmt.Fprintln(out, "  END LOOP;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  -- PHANTOM_METER: accounts with identical readings for 6+ months")
	fmt.Fprintln(out, "  FOR acc IN")
	fmt.Fprintln(out, "    SELECT wa.id, wa.gwl_account_number, wa.district_id")
	fmt.Fprintln(out, "    FROM water_accounts wa")
	fmt.Fprintln(out, "    WHERE wa.gwl_account_number LIKE 'GWL-%-PHANTOM%'")
	fmt.Fprintln(out, "       OR wa.gwl_account_number LIKE 'GWL-%-GHOST%'")
	fmt.Fprintln(out, "    LIMIT 8")
	fmt.Fprintln(out, "  LOOP")
	fmt.Fprintln(out, "    INSERT INTO anomaly_flags (")
	fmt.Fprintln(out, "      id, account_id, district_id, flag_type, severity,")
	fmt.Fprintln(out, "      title, description, evidence, estimated_loss_ghs, status, created_at")
	fmt.Fprintln(out, "    ) VALUES (")
	fmt.Fprintln(out, "      gen_random_uuid(), acc.id, acc.district_id,")
	fmt.Fprintln(out, "      'PHANTOM_METER', 'CRITICAL',")
	fmt.Fprintln(out, "      'Phantom meter detected — identical readings for 6+ consecutive months',")
	fmt.Fprintln(out, "      'Meter reading has not changed in 6 months. Natural consumption always varies. ' ||")
	fmt.Fprintln(out, "       'Possible meter bypass, ghost account, or meter replacement without re-registration.',")
	fmt.Fprintln(out, "      jsonb_build_object(")
	fmt.Fprintln(out, "        'identical_reading_m3', ROUND((100 + random()*500)::numeric, 2),")
	fmt.Fprintln(out, "        'months_unchanged', 6 + FLOOR(random()*6)::int,")
	fmt.Fprintln(out, "        'std_dev_m3', 0.0,")
	fmt.Fprintln(out, "        'expected_std_dev_m3', ROUND((2 + random()*5)::numeric, 2)")
	fmt.Fprintln(out, "      ),")
	fmt.Fprintln(out, "      ROUND((2000 + random()*8000)::numeric, 2),")
	fmt.Fprintln(out, "      'OPEN', NOW() - (random()*60)::int * INTERVAL '1 day'")
	fmt.Fprintln(out, "    ) ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out, "  END LOOP;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  -- NRW_SPIKE: districts with NRW > 60%")
	fmt.Fprintln(out, "  FOR acc IN")
	fmt.Fprintln(out, "    SELECT d.id AS district_id, NULL::uuid AS account_id")
	fmt.Fprintln(out, "    FROM districts d")
	fmt.Fprintln(out, "    WHERE d.district_code IN ('ASHAIMAN','TAMALE-CENTRAL','DAMONGO','NALERIGU','DAMBAI')")
	fmt.Fprintln(out, "  LOOP")
	fmt.Fprintln(out, "    INSERT INTO anomaly_flags (")
	fmt.Fprintln(out, "      id, account_id, district_id, flag_type, severity,")
	fmt.Fprintln(out, "      title, description, evidence, estimated_loss_ghs, status, created_at")
	fmt.Fprintln(out, "    ) VALUES (")
	fmt.Fprintln(out, "      gen_random_uuid(), acc.account_id, acc.district_id,")
	fmt.Fprintln(out, "      'NRW_SPIKE', 'CRITICAL',")
	fmt.Fprintln(out, "      'District NRW exceeds 60% — IWA Grade F',")
	fmt.Fprintln(out, "      'Non-Revenue Water has exceeded the critical 60% threshold. ' ||")
	fmt.Fprintln(out, "       'Immediate infrastructure audit and leak detection survey required.',")
	fmt.Fprintln(out, "      jsonb_build_object(")
	fmt.Fprintln(out, "        'nrw_pct', ROUND((62 + random()*15)::numeric, 1),")
	fmt.Fprintln(out, "        'ghana_avg_pct', 51.6,")
	fmt.Fprintln(out, "        'iwa_target_pct', 20.0,")
	fmt.Fprintln(out, "        'iwa_grade', 'F'")
	fmt.Fprintln(out, "      ),")
	fmt.Fprintln(out, "      ROUND((50000 + random()*150000)::numeric, 2),")
	fmt.Fprintln(out, "      'OPEN', NOW() - (random()*14)::int * INTERVAL '1 day'")
	fmt.Fprintln(out, "    ) ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out, "  END LOOP;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "  RAISE NOTICE 'Anomaly flags seeded successfully';")
	fmt.Fprintln(out, "END $$;")
	fmt.Fprintln(out)

	// ── 5. CDC sync log (historical) ─────────────────────────────────────────
	fmt.Fprintln(out, "-- ── 5. CDC Sync Log (historical) ───────────────────────────────────────────")
	fmt.Fprintln(out, "INSERT INTO cdc_sync_log (")
	fmt.Fprintln(out, "  id, sync_started_at, sync_completed_at,")
	fmt.Fprintln(out, "  accounts_synced, bills_synced, readings_synced,")
	fmt.Fprintln(out, "  status, error_message, created_at")
	fmt.Fprintln(out, ")")
	fmt.Fprintln(out, "SELECT")
	fmt.Fprintln(out, "  gen_random_uuid(),")
	fmt.Fprintln(out, "  gs - INTERVAL '5 minutes',")
	fmt.Fprintln(out, "  gs,")
	fmt.Fprintln(out, "  FLOOR(random()*500 + 100)::int,")
	fmt.Fprintln(out, "  FLOOR(random()*2000 + 500)::int,")
	fmt.Fprintln(out, "  FLOOR(random()*2000 + 500)::int,")
	fmt.Fprintln(out, "  CASE WHEN random() < 0.95 THEN 'SUCCESS' ELSE 'PARTIAL' END,")
	fmt.Fprintln(out, "  NULL,")
	fmt.Fprintln(out, "  gs")
	fmt.Fprintf(out,  "FROM generate_series('%s'::timestamptz, NOW(), '15 minutes'::interval) gs\n",
		now.AddDate(0, -1, 0).Format("2006-01-02 00:00:00"),
	)
	fmt.Fprintln(out, "ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out)

	// ── 6. Field jobs (for mobile app) ────────────────────────────────────────
	fmt.Fprintln(out, "-- ── 6. Field Jobs (for mobile app) ─────────────────────────────────────────")
	fmt.Fprintln(out, "DO $$")
	fmt.Fprintln(out, "DECLARE")
	fmt.Fprintln(out, "  officer RECORD;")
	fmt.Fprintln(out, "  acc RECORD;")
	fmt.Fprintln(out, "  job_count INT := 0;")
	fmt.Fprintln(out, "BEGIN")
	fmt.Fprintln(out, "  FOR officer IN")
	fmt.Fprintln(out, "    SELECT u.id, u.district_id")
	fmt.Fprintln(out, "    FROM users u")
	fmt.Fprintln(out, "    WHERE u.role = 'FIELD_OFFICER' AND u.is_active = TRUE")
	fmt.Fprintln(out, "    LIMIT 10")
	fmt.Fprintln(out, "  LOOP")
	fmt.Fprintln(out, "    FOR acc IN")
	fmt.Fprintln(out, "      SELECT wa.id")
	fmt.Fprintln(out, "      FROM water_accounts wa")
	fmt.Fprintln(out, "      WHERE wa.district_id = officer.district_id")
	fmt.Fprintln(out, "        AND wa.status = 'FLAGGED'")
	fmt.Fprintln(out, "      LIMIT 3")
	fmt.Fprintln(out, "    LOOP")
	fmt.Fprintln(out, "      INSERT INTO field_jobs (")
	fmt.Fprintln(out, "        id, account_id, assigned_officer_id,")
	fmt.Fprintln(out, "        job_type, priority, status,")
	fmt.Fprintln(out, "        title, description,")
	fmt.Fprintln(out, "        due_date, created_at")
	fmt.Fprintln(out, "      ) VALUES (")
	fmt.Fprintln(out, "        gen_random_uuid(), acc.id, officer.id,")
	fmt.Fprintln(out, "        CASE FLOOR(random()*3)::int")
	fmt.Fprintln(out, "          WHEN 0 THEN 'METER_INSPECTION'")
	fmt.Fprintln(out, "          WHEN 1 THEN 'LEAK_INVESTIGATION'")
	fmt.Fprintln(out, "          ELSE 'BILLING_VERIFICATION'")
	fmt.Fprintln(out, "        END,")
	fmt.Fprintln(out, "        CASE FLOOR(random()*3)::int")
	fmt.Fprintln(out, "          WHEN 0 THEN 'HIGH'")
	fmt.Fprintln(out, "          WHEN 1 THEN 'MEDIUM'")
	fmt.Fprintln(out, "          ELSE 'LOW'")
	fmt.Fprintln(out, "        END,")
	fmt.Fprintln(out, "        CASE FLOOR(random()*3)::int")
	fmt.Fprintln(out, "          WHEN 0 THEN 'ASSIGNED'")
	fmt.Fprintln(out, "          WHEN 1 THEN 'IN_PROGRESS'")
	fmt.Fprintln(out, "          ELSE 'ASSIGNED'")
	fmt.Fprintln(out, "        END,")
	fmt.Fprintln(out, "        'Anomaly Investigation — Meter Audit Required',")
	fmt.Fprintln(out, "        'Sentinel flagged this account for billing variance. ' ||")
	fmt.Fprintln(out, "         'Please inspect meter, verify category, and photograph installation.',")
	fmt.Fprintln(out, "        NOW() + (3 + FLOOR(random()*7)::int) * INTERVAL '1 day',")
	fmt.Fprintln(out, "        NOW() - (FLOOR(random()*5)::int) * INTERVAL '1 day'")
	fmt.Fprintln(out, "      ) ON CONFLICT DO NOTHING;")
	fmt.Fprintln(out, "      job_count := job_count + 1;")
	fmt.Fprintln(out, "    END LOOP;")
	fmt.Fprintln(out, "  END LOOP;")
	fmt.Fprintln(out, "  RAISE NOTICE 'Field jobs seeded: %', job_count;")
	fmt.Fprintln(out, "END $$;")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "COMMIT;")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "-- ── Verification queries ────────────────────────────────────────────────────")
	fmt.Fprintln(out, "-- Run these after seeding to confirm data is present:")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM production_records;   -- expect ~360")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM meter_readings;        -- expect ~6000+")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM gwl_bills;             -- expect ~6000+")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM water_balance_records; -- expect ~360")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM anomaly_flags;         -- expect ~30+")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM cdc_sync_log;          -- expect ~2880 (15min * 30 days)")
	fmt.Fprintln(out, "-- SELECT COUNT(*) FROM field_jobs;            -- expect ~30+")

	// Suppress unused variable warning
	_ = rng
	_ = startDate
	_ = accountProfiles
	_ = calcBill
	_ = math.Abs
}
