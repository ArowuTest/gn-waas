// Package service implements the CDC demo mode synthetic data generator.
//
// When GWL_DB_HOST is not set, the CDC ingestor runs in DEMO mode.
// Instead of connecting to the real GWL billing database, it generates
// synthetic meter readings and billing records that replicate exactly
// what the production pipeline would receive from GWL.
//
// The generated data:
//   - Uses real account numbers from the water_accounts table
//   - Applies PURC 2026 tariff rates (same as tariff-engine)
//   - Generates realistic consumption variance (±15%)
//   - Simulates AMR, manual, and estimated read methods
//   - Publishes NATS events after each sync (same as live mode)
//   - Writes to the same tables as the live CDC sync
//
// This means the entire pipeline works end-to-end in demo mode:
//   Demo Generator → meter_readings / gwl_bills → API endpoints → Frontend

package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DemoSyncService generates synthetic GWL data when no live GWL DB is available.
// It replicates exactly what CDCSyncService.syncAccounts/syncBillingRecords/syncMeterReadings
// would write after receiving data from the real GWL billing system.
type DemoSyncService struct {
	gnwaasDB *pgxpool.Pool
	logger   *zap.Logger
	rng      *rand.Rand
}

func NewDemoSyncService(gnwaasDB *pgxpool.Pool, logger *zap.Logger) *DemoSyncService {
	return &DemoSyncService{
		gnwaasDB: gnwaasDB,
		logger:   logger,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// DemoSyncResult holds the result of a demo sync run
type DemoSyncResult struct {
	ReadingsSynced int
	BillsSynced    int
	AccountsFound  int
	SyncedAt       time.Time
	DistrictCodes  []string
}

// RunDemoSync generates synthetic meter readings and bills for the current month.
// It fetches all active accounts from the GN-WAAS database and generates
// one reading + one bill per account, simulating a monthly GWL billing cycle.
func (s *DemoSyncService) RunDemoSync(ctx context.Context) (*DemoSyncResult, error) {
	s.logger.Info("CDC DEMO MODE: Generating synthetic GWL data for current billing period")

	// Fetch all active accounts
	rows, err := s.gnwaasDB.Query(ctx, `
		SELECT wa.id, wa.gwl_account_number, wa.category,
		       wa.monthly_avg_consumption, wa.district_id,
		       d.district_code
		FROM water_accounts wa
		JOIN districts d ON d.id = wa.district_id
		WHERE wa.status IN ('ACTIVE', 'FLAGGED')
		ORDER BY d.district_code, wa.gwl_account_number
	`)
	if err != nil {
		return nil, fmt.Errorf("fetch accounts for demo sync: %w", err)
	}
	defer rows.Close()

	type accountRow struct {
		ID               uuid.UUID
		GWLAccountNumber string
		Category         string
		MonthlyAvgM3     float64
		DistrictID       uuid.UUID
		DistrictCode     string
	}

	var accounts []accountRow
	for rows.Next() {
		var a accountRow
		if err := rows.Scan(
			&a.ID, &a.GWLAccountNumber, &a.Category,
			&a.MonthlyAvgM3, &a.DistrictID, &a.DistrictCode,
		); err != nil {
			continue
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan accounts: %w", err)
	}

	if len(accounts) == 0 {
		s.logger.Warn("CDC DEMO MODE: No active accounts found — run database seeds first",
			zap.String("hint", "psql $DATABASE_URL < database/seeds/001_system_config.sql && ..."),
		)
		return &DemoSyncResult{SyncedAt: time.Now()}, nil
	}

	s.logger.Info("CDC DEMO MODE: Generating readings for accounts",
		zap.Int("account_count", len(accounts)),
	)

	// Current billing period: previous month
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	periodEnd   := time.Date(now.Year(), now.Month(), 0, 23, 59, 59, 0, time.UTC)
	readingDate := periodEnd

	var readingsSynced, billsSynced int
	districtSet := make(map[string]bool)

	for _, acc := range accounts {
		districtSet[acc.DistrictCode] = true

		// Generate realistic monthly consumption (±15% variance)
		variance := 0.85 + s.rng.Float64()*0.30
		consumption := acc.MonthlyAvgM3 * variance
		if consumption < 0.1 {
			consumption = 0.1
		}

		// Get previous reading to compute cumulative
		var prevReading float64
		err := s.gnwaasDB.QueryRow(ctx, `
			SELECT COALESCE(reading_m3, 0)
			FROM meter_readings
			WHERE account_id = $1
			ORDER BY reading_date DESC
			LIMIT 1
		`, acc.ID).Scan(&prevReading)
		if err != nil {
			// No previous reading — start from a realistic baseline
			prevReading = acc.MonthlyAvgM3 * 24 * (0.8 + s.rng.Float64()*0.4)
		}
		currReading := prevReading + consumption

		// Read method distribution: 70% AMR, 20% manual, 10% estimated
		readMethod, readerID := s.generateReadMethod(acc.DistrictCode)

		// Insert meter reading
		_, err = s.gnwaasDB.Exec(ctx, `
			INSERT INTO meter_readings (
				id, account_id, reading_date, reading_m3,
				flow_rate_m3h, pressure_bar,
				read_method, reader_id, created_at
			) VALUES (
				$1, $2, $3, $4,
				$5, $6,
				$7, $8, NOW()
			)
			ON CONFLICT (account_id, reading_date) DO UPDATE SET
				reading_m3    = EXCLUDED.reading_m3,
				flow_rate_m3h = EXCLUDED.flow_rate_m3h,
				read_method   = EXCLUDED.read_method,
				reader_id     = EXCLUDED.reader_id
		`,
			uuid.New(), acc.ID, readingDate,
			roundTo2(currReading),
			roundTo3(consumption/720.0), // avg flow rate m³/hr
			roundTo2(2.5+s.rng.Float64()*1.5),
			readMethod, readerID,
		)
		if err != nil {
			s.logger.Warn("Failed to insert demo meter reading",
				zap.String("account", acc.GWLAccountNumber),
				zap.Error(err),
			)
			continue
		}
		readingsSynced++

		// Calculate bill using PURC 2026 tariff
		amount, vatAmt, total := calcPURCBill(acc.Category, consumption)

		// Payment status: 75% paid, 15% partial, 10% unpaid
		paymentStatus, paymentDate, paymentAmount := s.generatePayment(total, periodEnd)

		// Insert GWL bill
		gwlBillID := fmt.Sprintf("GWL-BILL-%s-%s", acc.GWLAccountNumber, periodStart.Format("200601"))
		_, err = s.gnwaasDB.Exec(ctx, `
			INSERT INTO gwl_bills (
				id, account_id, gwl_bill_id,
				billing_period_start, billing_period_end,
				previous_reading_m3, current_reading_m3, consumption_m3,
				gwl_category, gwl_amount_ghs, gwl_vat_ghs, gwl_total_ghs,
				gwl_reader_id, gwl_read_date, gwl_read_method,
				payment_status, payment_date, payment_amount_ghs,
				created_at
			) VALUES (
				$1, $2, $3,
				$4, $5,
				$6, $7, $8,
				$9, $10, $11, $12,
				$13, $14, $15,
				$16, $17, $18,
				NOW()
			)
			ON CONFLICT (gwl_bill_id) DO UPDATE SET
				consumption_m3     = EXCLUDED.consumption_m3,
				gwl_amount_ghs     = EXCLUDED.gwl_amount_ghs,
				gwl_vat_ghs        = EXCLUDED.gwl_vat_ghs,
				gwl_total_ghs      = EXCLUDED.gwl_total_ghs,
				payment_status     = EXCLUDED.payment_status,
				payment_date       = EXCLUDED.payment_date,
				payment_amount_ghs = EXCLUDED.payment_amount_ghs
		`,
			uuid.New(), acc.ID, gwlBillID,
			periodStart, periodEnd,
			roundTo2(prevReading), roundTo2(currReading), roundTo2(consumption),
			acc.Category, roundTo2(amount), roundTo2(vatAmt), roundTo2(total),
			readerID, readingDate, readMethod,
			paymentStatus, paymentDate, roundTo2(paymentAmount),
		)
		if err != nil {
			s.logger.Warn("Failed to insert demo GWL bill",
				zap.String("account", acc.GWLAccountNumber),
				zap.Error(err),
			)
			continue
		}
		billsSynced++
	}

	// Also update production records for the current month
	s.generateProductionRecords(ctx, periodStart)

	districtCodes := make([]string, 0, len(districtSet))
	for code := range districtSet {
		districtCodes = append(districtCodes, code)
	}

	result := &DemoSyncResult{
		ReadingsSynced: readingsSynced,
		BillsSynced:    billsSynced,
		AccountsFound:  len(accounts),
		SyncedAt:       time.Now(),
		DistrictCodes:  districtCodes,
	}

	s.logger.Info("CDC DEMO MODE: Sync completed",
		zap.Int("readings_synced", readingsSynced),
		zap.Int("bills_synced", billsSynced),
		zap.Int("districts", len(districtCodes)),
	)

	return result, nil
}

// generateProductionRecords creates/updates production records for the given month.
func (s *DemoSyncService) generateProductionRecords(ctx context.Context, periodStart time.Time) {
	rows, err := s.gnwaasDB.Query(ctx, `
		SELECT id, district_code FROM districts WHERE is_active = true
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	// District production profiles (m³/day)
	prodProfiles := map[string]float64{
		"TEMA-EAST": 12500, "TEMA-WEST": 9200, "ACCRA-WEST": 20800,
		"ACCRA-CENTRAL": 18500, "ACCRA-EAST": 14200, "ASHAIMAN": 10500,
		"LEDZOKUKU": 8300, "ADENTAN": 6500, "AYAWASO": 15600, "ABLEKUMA": 12800,
		"KUMASI-CENTRAL": 25500, "KUMASI-NORTH": 13800, "KUMASI-SOUTH": 9600,
		"TAMALE-CENTRAL": 11400, "TAMALE-SOUTH": 6600,
		"TAKORADI": 13500, "SEKONDI": 8400, "KOFORIDUA": 9600,
	}

	daysInMonth := float64(periodStart.AddDate(0, 1, 0).AddDate(0, 0, -1).Day())

	for rows.Next() {
		var districtID uuid.UUID
		var districtCode string
		if err := rows.Scan(&districtID, &districtCode); err != nil {
			continue
		}

		avgDaily, ok := prodProfiles[districtCode]
		if !ok {
			avgDaily = 3000 // default for smaller districts
		}

		variance := 0.92 + s.rng.Float64()*0.16
		produced := avgDaily * daysInMonth * variance

		// FLOW-01 fix: use correct column names matching production_records schema.
		// Schema (migration 003) uses: recorded_at TIMESTAMPTZ, volume_m3 NUMERIC.
		// Extra columns (volume_treated_m3, pumping_hours, energy_kwh, data_quality_score)
		// are added by migration 022 so the richer data is preserved.
		_, _ = s.gnwaasDB.Exec(ctx, `
			INSERT INTO production_records (
				id, district_id, recorded_at,
				volume_m3, volume_treated_m3,
				pumping_hours, energy_kwh,
				source_type, data_quality_score, created_at
			) VALUES (
				$1, $2, $3,
				$4, $5,
				$6, $7,
				'SURFACE_WATER', $8, NOW()
			)
			ON CONFLICT DO NOTHING
		`,
			uuid.New(), districtID, periodStart,
			roundTo2(produced), roundTo2(produced*0.98),
			roundTo2(produced*0.85/120.0), roundTo2(produced*0.42),
			0.80+s.rng.Float64()*0.15,
		)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (s *DemoSyncService) generateReadMethod(districtCode string) (method, readerID string) {
	r := s.rng.Float64()
	switch {
	case r < 0.70:
		return "AMR", "AMR-GATEWAY-" + districtCode
	case r < 0.90:
		n := s.rng.Intn(50) + 1
		return "MANUAL", fmt.Sprintf("FO-%03d", n)
	default:
		return "ESTIMATED", "EST-SYSTEM"
	}
}

func (s *DemoSyncService) generatePayment(total float64, periodEnd time.Time) (status string, date *time.Time, amount float64) {
	r := s.rng.Float64()
	switch {
	case r < 0.75:
		d := periodEnd.AddDate(0, 0, 5+s.rng.Intn(30))
		return "PAID", &d, total
	case r < 0.90:
		d := periodEnd.AddDate(0, 0, 10+s.rng.Intn(40))
		partial := total * (0.3 + s.rng.Float64()*0.5)
		return "PARTIAL", &d, partial
	default:
		return "UNPAID", nil, 0
	}
}

// calcPURCBill applies PURC 2026 tiered tariff rates.
// Rates match database/seeds/002_tariff_rates.sql (the authoritative source).
// This function is used ONLY by the demo sync service to generate synthetic GWL
// billing data that mirrors what the real GWL system would produce.
//
// PURC 2026 Approved Tariffs (effective 1 Jan 2026):
//   RESIDENTIAL  Tier 1 (0–5 m³):   GH₵ 6.1225/m³  (lifeline rate)
//   RESIDENTIAL  Tier 2 (>5 m³):    GH₵10.8320/m³  (standard rate)
//   COMMERCIAL:                      GH₵18.4500/m³  + GH₵500 service charge
//   INDUSTRIAL:                      GH₵22.1000/m³  + GH₵1,500 service charge
//   PUBLIC_GOVT:                     GH₵15.7372/m³  + GH₵2,000 service charge
//   BOTTLED_WATER:                   GH₵32.7858/m³  + GH₵25,000 service charge
//   VAT: 20% on all categories
func calcPURCBill(category string, m3 float64) (amount, vatAmt, total float64) {
	switch category {
	case "RESIDENTIAL":
		// Two-tier: lifeline (0–5 m³) + standard (>5 m³)
		if m3 <= 5 {
			amount = m3 * 6.1225
		} else {
			amount = 5*6.1225 + (m3-5)*10.8320
		}
	case "COMMERCIAL":
		amount = m3*18.4500 + 500.0 // service charge
	case "INDUSTRIAL":
		amount = m3*22.1000 + 1500.0 // service charge
	case "PUBLIC_GOVT":
		amount = m3*15.7372 + 2000.0 // service charge
	case "BOTTLED_WATER":
		amount = m3*32.7858 + 25000.0 // service charge
	default:
		// Unknown category: use residential tier-2 as conservative estimate
		amount = m3 * 10.8320
	}
	vatAmt = amount * 0.20 // 20% VAT (PURC 2026)
	total = amount + vatAmt
	return
}

func roundTo2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func roundTo3(v float64) float64 {
	return float64(int(v*1000+0.5)) / 1000
}
