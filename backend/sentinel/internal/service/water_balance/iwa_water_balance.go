package water_balance

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// IWAWaterBalanceService computes the IWA/AWWA M36 water balance for a district.
//
// The IWA water balance equation:
//
//	System Input Volume
//	  = Authorised Consumption + Water Losses
//
//	Authorised Consumption
//	  = Billed Metered + Billed Unmetered + Unbilled Metered + Unbilled Unmetered
//
//	Water Losses (NRW)
//	  = Apparent Losses + Real Losses
//
//	Apparent Losses (commercial / detectable by GN-WAAS)
//	  = Unauthorised Consumption + Metering Inaccuracies + Data Handling Errors
//
//	Real Losses (physical / statistical estimate)
//	  = Main Leakage + Storage Overflow + Service Connection Leakage
//
// NRW % = (Water Losses / System Input Volume) × 100
type IWAWaterBalanceService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// WaterBalanceInput holds the raw data needed to compute the balance
type WaterBalanceInput struct {
	DistrictID   uuid.UUID
	PeriodStart  time.Time
	PeriodEnd    time.Time

	// System Input (from production_records)
	SystemInputM3 float64

	// Authorised Consumption (from billing_records)
	BilledMeteredM3    float64
	BilledUnmeteredM3  float64
	UnbilledMeteredM3  float64
	UnbilledUnmeteredM3 float64

	// Apparent Losses (from anomaly_flags + audit_events)
	UnauthorisedConsumptionM3 float64 // Theft / fraud detected by Sentinel
	MeteringInaccuraciesM3    float64 // OCR-detected meter errors
	DataHandlingErrorsM3      float64 // Billing data errors

	// Real Losses (statistical estimates — night flow analysis)
	MainLeakageM3            float64 // From night flow analysis
	StorageOverflowM3        float64 // From IoT sensors (if available)
	ServiceConnectionLeakM3  float64 // Statistical estimate
}

// WaterBalanceResult is the computed IWA/AWWA water balance
type WaterBalanceResult struct {
	DistrictID   uuid.UUID `json:"district_id"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`

	// System Input
	SystemInputM3 float64 `json:"system_input_m3"`

	// Authorised Consumption
	BilledMeteredM3     float64 `json:"billed_metered_m3"`
	BilledUnmeteredM3   float64 `json:"billed_unmetered_m3"`
	UnbilledMeteredM3   float64 `json:"unbilled_metered_m3"`
	UnbilledUnmeteredM3 float64 `json:"unbilled_unmetered_m3"`
	TotalAuthorisedM3   float64 `json:"total_authorised_m3"`

	// Apparent Losses
	UnauthorisedConsumptionM3 float64 `json:"unauthorised_consumption_m3"`
	MeteringInaccuraciesM3    float64 `json:"metering_inaccuracies_m3"`
	DataHandlingErrorsM3      float64 `json:"data_handling_errors_m3"`
	TotalApparentLossesM3     float64 `json:"total_apparent_losses_m3"`

	// Real Losses
	MainLeakageM3           float64 `json:"main_leakage_m3"`
	StorageOverflowM3       float64 `json:"storage_overflow_m3"`
	ServiceConnectionLeakM3 float64 `json:"service_connection_leak_m3"`
	TotalRealLossesM3       float64 `json:"total_real_losses_m3"`

	// Totals
	TotalWaterLossesM3 float64 `json:"total_water_losses_m3"`
	NRWM3              float64 `json:"nrw_m3"`
	NRWPercent         float64 `json:"nrw_percent"`
	IWAGrade           string  `json:"iwa_grade"` // A/B/C/D per IWA ILI

	// Infrastructure Leakage Index (ILI) — IWA standard metric
	// ILI = Current Annual Real Losses / Unavoidable Annual Real Losses
	// ILI < 1.0 = A (excellent), 1-2 = B, 2-4 = C, >4 = D
	ILI float64 `json:"ili"`

	// Revenue impact
	EstimatedRevenueRecoveryGHS float64 `json:"estimated_revenue_recovery_ghs"`

	// Data quality
	DataConfidenceScore float64 `json:"data_confidence_score"` // 0-100
	ComputedAt          time.Time `json:"computed_at"`
}

func NewIWAWaterBalanceService(db *pgxpool.Pool, logger *zap.Logger) *IWAWaterBalanceService {
	return &IWAWaterBalanceService{db: db, logger: logger}
}

// ComputeAndPersist computes the IWA water balance for a district/period
// and upserts the result into water_balance_records.
func (s *IWAWaterBalanceService) ComputeAndPersist(
	ctx context.Context,
	districtID uuid.UUID,
	periodStart, periodEnd time.Time,
) (*WaterBalanceResult, error) {
	s.logger.Info("Computing IWA water balance",
		zap.String("district_id", districtID.String()),
		zap.Time("period_start", periodStart),
		zap.Time("period_end", periodEnd),
	)

	// Gather all inputs from the database
	input, err := s.gatherInputs(ctx, districtID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to gather water balance inputs: %w", err)
	}

	// Compute the balance
	result := s.compute(input)

	// Persist to water_balance_records
	if err := s.persist(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to persist water balance: %w", err)
	}

	s.logger.Info("Water balance computed and persisted",
		zap.String("district_id", districtID.String()),
		zap.Float64("nrw_percent", result.NRWPercent),
		zap.String("iwa_grade", result.IWAGrade),
		zap.Float64("ili", result.ILI),
	)

	return result, nil
}

// gatherInputs queries all data sources needed for the water balance
func (s *IWAWaterBalanceService) gatherInputs(
	ctx context.Context,
	districtID uuid.UUID,
	from, to time.Time,
) (*WaterBalanceInput, error) {
	input := &WaterBalanceInput{
		DistrictID:  districtID,
		PeriodStart: from,
		PeriodEnd:   to,
	}

	// 1. System Input Volume — from production_records
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(volume_m3), 0)
		FROM production_records
		WHERE district_id = $1
		  AND recorded_at >= $2
		  AND recorded_at < $3`,
		districtID, from, to,
	).Scan(&input.SystemInputM3)
	if err != nil {
		return nil, fmt.Errorf("query production_records: %w", err)
	}

	// 2. Billed Metered — from billing_records (metered accounts)
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(br.consumption_m3), 0)
		FROM billing_records br
		JOIN water_accounts wa ON wa.id = br.account_id
		WHERE wa.district_id = $1
		  AND br.billing_period_start >= $2
		  AND br.billing_period_end <= $3
		  AND wa.meter_serial_number IS NOT NULL
		  AND br.bill_status IN ('ISSUED', 'PAID', 'OVERDUE')`,
		districtID, from, to,
	).Scan(&input.BilledMeteredM3)
	if err != nil {
		return nil, fmt.Errorf("query billed_metered: %w", err)
	}

	// 3. Billed Unmetered — flat-rate accounts
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(br.consumption_m3), 0)
		FROM billing_records br
		JOIN water_accounts wa ON wa.id = br.account_id
		WHERE wa.district_id = $1
		  AND br.billing_period_start >= $2
		  AND br.billing_period_end <= $3
		  AND wa.meter_serial_number IS NULL
		  AND br.bill_status IN ('ISSUED', 'PAID', 'OVERDUE')`,
		districtID, from, to,
	).Scan(&input.BilledUnmeteredM3)
	if err != nil {
		return nil, fmt.Errorf("query billed_unmetered: %w", err)
	}

	// 4. Unbilled Metered — active metered accounts with no bill this period
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(mr.reading_m3), 0)
		FROM meter_readings mr
		JOIN water_accounts wa ON wa.id = mr.account_id
		WHERE wa.district_id = $1
		  AND mr.reading_timestamp >= $2
		  AND mr.reading_timestamp < $3
		  AND NOT EXISTS (
			SELECT 1 FROM billing_records br
			WHERE br.account_id = mr.account_id
			  AND br.billing_period_start >= $2
			  AND br.billing_period_end <= $3
		  )`,
		districtID, from, to,
	).Scan(&input.UnbilledMeteredM3)
	if err != nil {
		return nil, fmt.Errorf("query unbilled_metered: %w", err)
	}

	// 5. Apparent Losses — from anomaly_flags (THEFT / FRAUD detected by Sentinel)
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(
			CASE
				WHEN anomaly_type IN ('SHADOW_BILL_VARIANCE', 'CATEGORY_MISMATCH')
				THEN estimated_loss_ghs / 10.83  -- Convert GHS to m³ at residential rate
				ELSE 0
			END
		), 0)
		FROM anomaly_flags af
		JOIN water_accounts wa ON wa.id = af.account_id
		WHERE wa.district_id = $1
		  AND af.created_at >= $2
		  AND af.created_at < $3
		  AND af.status IN ('OPEN', 'CONFIRMED')`,
		districtID, from, to,
	).Scan(&input.UnauthorisedConsumptionM3)
	if err != nil {
		return nil, fmt.Errorf("query apparent_losses: %w", err)
	}

	// 6. Metering Inaccuracies — from OCR audit events with low confidence
	err = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(
			CASE WHEN ae.ocr_confidence < 0.70 THEN ae.consumption_variance_m3 ELSE 0 END
		), 0)
		FROM audit_events ae
		JOIN water_accounts wa ON wa.id = ae.account_id
		WHERE wa.district_id = $1
		  AND ae.created_at >= $2
		  AND ae.created_at < $3
		  AND ae.ocr_confidence IS NOT NULL`,
		districtID, from, to,
	).Scan(&input.MeteringInaccuraciesM3)
	if err != nil {
		// Non-fatal — OCR data may not be available yet
		s.logger.Warn("Could not query metering inaccuracies", zap.Error(err))
		input.MeteringInaccuraciesM3 = 0
	}

	// 7. Real Losses — statistical estimate using night flow analysis
	// UARL formula (IWA): UARL = (0.8 × Lm + 25 × Nc + 0.02 × Lp) × P × 0.001
	// Where: Lm = mains length (km), Nc = service connections, Lp = service pipe length (m), P = pressure (m)
	// For GN-WAAS, we use a simplified estimate: 15% of system input as real losses baseline
	// This is refined by night flow analysis when IoT data is available
	nightFlowLeakage, err := s.estimateRealLossesFromNightFlow(ctx, districtID, from, to)
	if err != nil {
		s.logger.Warn("Night flow data unavailable, using statistical estimate", zap.Error(err))
		// Fallback: 15% of system input as real losses (Ghana average)
		nightFlowLeakage = input.SystemInputM3 * 0.15
	}
	input.MainLeakageM3 = nightFlowLeakage
	input.ServiceConnectionLeakM3 = input.SystemInputM3 * 0.02 // 2% statistical estimate

	return input, nil
}

// estimateRealLossesFromNightFlow uses night flow bulk meter data to estimate real losses
func (s *IWAWaterBalanceService) estimateRealLossesFromNightFlow(
	ctx context.Context,
	districtID uuid.UUID,
	from, to time.Time,
) (float64, error) {
	// Night flow (2-4 AM) represents minimum legitimate demand + leakage
	// Minimum Night Flow (MNF) method: Real Losses = (MNF - Legitimate Night Use) × 24 × days
	// Legitimate Night Use ≈ 1.7 L/connection/hour (IWA standard)
	var avgNightFlowM3PerHour float64
	var connectionCount int

	err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(AVG(flow_rate_m3_per_hour), 0) AS avg_night_flow,
			(SELECT COUNT(*) FROM water_accounts WHERE district_id = $1 AND status = 'ACTIVE') AS connections
		FROM bulk_meter_readings
		WHERE district_id = $1
		  AND reading_timestamp >= $2
		  AND reading_timestamp < $3
		  AND EXTRACT(HOUR FROM reading_timestamp) BETWEEN 2 AND 4`,
		districtID, from, to,
	).Scan(&avgNightFlowM3PerHour, &connectionCount)
	if err != nil {
		return 0, err
	}

	if avgNightFlowM3PerHour == 0 || connectionCount == 0 {
		return 0, fmt.Errorf("insufficient night flow data")
	}

	// Legitimate Night Use = 1.7 L/connection/hour = 0.0017 m³/connection/hour
	legitimateNightUseM3PerHour := float64(connectionCount) * 0.0017
	realLossRateM3PerHour := avgNightFlowM3PerHour - legitimateNightUseM3PerHour
	if realLossRateM3PerHour < 0 {
		realLossRateM3PerHour = 0
	}

	// Extrapolate to full period
	days := to.Sub(from).Hours() / 24
	return realLossRateM3PerHour * 24 * days, nil
}

// compute calculates the IWA water balance from inputs
func (s *IWAWaterBalanceService) compute(input *WaterBalanceInput) *WaterBalanceResult {
	r := &WaterBalanceResult{
		DistrictID:  input.DistrictID,
		PeriodStart: input.PeriodStart,
		PeriodEnd:   input.PeriodEnd,
		ComputedAt:  time.Now().UTC(),

		SystemInputM3:       input.SystemInputM3,
		BilledMeteredM3:     input.BilledMeteredM3,
		BilledUnmeteredM3:   input.BilledUnmeteredM3,
		UnbilledMeteredM3:   input.UnbilledMeteredM3,
		UnbilledUnmeteredM3: input.UnbilledUnmeteredM3,

		UnauthorisedConsumptionM3: input.UnauthorisedConsumptionM3,
		MeteringInaccuraciesM3:    input.MeteringInaccuraciesM3,
		DataHandlingErrorsM3:      input.DataHandlingErrorsM3,

		MainLeakageM3:           input.MainLeakageM3,
		StorageOverflowM3:       input.StorageOverflowM3,
		ServiceConnectionLeakM3: input.ServiceConnectionLeakM3,
	}

	// Authorised Consumption
	r.TotalAuthorisedM3 = r.BilledMeteredM3 + r.BilledUnmeteredM3 +
		r.UnbilledMeteredM3 + r.UnbilledUnmeteredM3

	// Apparent Losses
	r.TotalApparentLossesM3 = r.UnauthorisedConsumptionM3 +
		r.MeteringInaccuraciesM3 + r.DataHandlingErrorsM3

	// Real Losses
	r.TotalRealLossesM3 = r.MainLeakageM3 + r.StorageOverflowM3 + r.ServiceConnectionLeakM3

	// Total Water Losses = NRW
	r.TotalWaterLossesM3 = r.TotalApparentLossesM3 + r.TotalRealLossesM3
	r.NRWM3 = r.TotalWaterLossesM3

	// NRW %
	if r.SystemInputM3 > 0 {
		r.NRWPercent = (r.NRWM3 / r.SystemInputM3) * 100
	}

	// IWA Infrastructure Leakage Index (ILI)
	// Simplified: ILI = Current Annual Real Losses / (SystemInput × 0.08)
	// (8% of system input is the IWA "unavoidable" real loss benchmark for Ghana)
	unavoidableRealLossM3 := r.SystemInputM3 * 0.08
	if unavoidableRealLossM3 > 0 {
		r.ILI = r.TotalRealLossesM3 / unavoidableRealLossM3
	}

	// IWA Grade based on ILI
	r.IWAGrade = classifyIWAGrade(r.ILI, r.NRWPercent)

	// Revenue recovery estimate (apparent losses only — recoverable)
	// Using blended tariff of ₵10.83/m³ + 20% VAT
	r.EstimatedRevenueRecoveryGHS = r.TotalApparentLossesM3 * 10.83 * 1.20

	// Data confidence score (0-100)
	r.DataConfidenceScore = s.computeConfidenceScore(input)

	return r
}

// classifyIWAGrade returns the IWA performance grade
// Based on IWA ILI thresholds adapted for developing-country context
func classifyIWAGrade(ili, nrwPct float64) string {
	switch {
	case ili < 1.0 && nrwPct < 20:
		return "A" // Excellent — world-class
	case ili < 2.0 && nrwPct < 30:
		return "B" // Good — acceptable
	case ili < 4.0 && nrwPct < 45:
		return "C" // Poor — needs improvement
	default:
		return "D" // Very poor — urgent action required
	}
}

// computeConfidenceScore rates the quality of the input data (0-100)
func (s *IWAWaterBalanceService) computeConfidenceScore(input *WaterBalanceInput) float64 {
	score := 100.0

	// Deduct for missing system input data
	if input.SystemInputM3 == 0 {
		score -= 40 // Critical — can't compute NRW without this
	}

	// Deduct for missing billing data
	if input.BilledMeteredM3 == 0 && input.BilledUnmeteredM3 == 0 {
		score -= 30
	}

	// Deduct for using statistical real loss estimate (no night flow data)
	if input.MainLeakageM3 == input.SystemInputM3*0.15 {
		score -= 15 // Using fallback estimate
	}

	// Deduct for no OCR metering data
	if input.MeteringInaccuraciesM3 == 0 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}
	return score
}

// persist upserts the water balance result into water_balance_records
func (s *IWAWaterBalanceService) persist(ctx context.Context, r *WaterBalanceResult) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO water_balance_records (
			district_id, period_start, period_end,
			system_input_volume_m3, input_data_source,
			billed_metered_m3, billed_unmetered_m3,
			unbilled_metered_m3, unbilled_unmetered_m3,
			unauthorised_consumption_m3, metering_inaccuracies_m3, data_handling_errors_m3,
			main_leakage_m3, storage_overflow_m3,
			nrw_percent, ili_score, iwa_grade,
			estimated_revenue_recovery_ghs, data_confidence_score,
			computed_at
		) VALUES (
			$1, $2, $3,
			$4, 'GWL_PRODUCTION',
			$5, $6,
			$7, $8,
			$9, $10, $11,
			$12, $13,
			$14, $15, $16,
			$17, $18,
			NOW()
		)
		ON CONFLICT (district_id, period_start, period_end)
		DO UPDATE SET
			system_input_volume_m3        = EXCLUDED.system_input_volume_m3,
			billed_metered_m3             = EXCLUDED.billed_metered_m3,
			billed_unmetered_m3           = EXCLUDED.billed_unmetered_m3,
			unbilled_metered_m3           = EXCLUDED.unbilled_metered_m3,
			unauthorised_consumption_m3   = EXCLUDED.unauthorised_consumption_m3,
			metering_inaccuracies_m3      = EXCLUDED.metering_inaccuracies_m3,
			main_leakage_m3               = EXCLUDED.main_leakage_m3,
			nrw_percent                   = EXCLUDED.nrw_percent,
			ili_score                     = EXCLUDED.ili_score,
			iwa_grade                     = EXCLUDED.iwa_grade,
			estimated_revenue_recovery_ghs = EXCLUDED.estimated_revenue_recovery_ghs,
			data_confidence_score         = EXCLUDED.data_confidence_score,
			computed_at                   = NOW()`,
		r.DistrictID, r.PeriodStart, r.PeriodEnd,
		r.SystemInputM3,
		r.BilledMeteredM3, r.BilledUnmeteredM3,
		r.UnbilledMeteredM3, r.UnbilledUnmeteredM3,
		r.UnauthorisedConsumptionM3, r.MeteringInaccuraciesM3, r.DataHandlingErrorsM3,
		r.MainLeakageM3, r.StorageOverflowM3,
		r.NRWPercent, r.ILI, r.IWAGrade,
		r.EstimatedRevenueRecoveryGHS, r.DataConfidenceScore,
	)
	return err
}

// RunAllDistricts computes the water balance for all active districts
func (s *IWAWaterBalanceService) RunAllDistricts(ctx context.Context, periodStart, periodEnd time.Time) ([]*WaterBalanceResult, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM districts WHERE is_active = true ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list districts: %w", err)
	}
	defer rows.Close()

	var districtIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		districtIDs = append(districtIDs, id)
	}

	var results []*WaterBalanceResult
	for _, id := range districtIDs {
		result, err := s.ComputeAndPersist(ctx, id, periodStart, periodEnd)
		if err != nil {
			s.logger.Error("Failed to compute water balance for district",
				zap.String("district_id", id.String()),
				zap.Error(err),
			)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetLatestBalance retrieves the most recent water balance for a district
func (s *IWAWaterBalanceService) GetLatestBalance(ctx context.Context, districtID uuid.UUID) (*entities.WaterBalanceSummary, error) {
	row := s.db.QueryRow(ctx, `
		SELECT
			district_id, period_start, period_end,
			system_input_volume_m3,
			total_authorised_m3,
			total_apparent_losses_m3,
			total_real_losses_m3,
			nrw_percent, ili_score, iwa_grade,
			estimated_revenue_recovery_ghs,
			data_confidence_score,
			computed_at
		FROM water_balance_records
		WHERE district_id = $1
		ORDER BY period_start DESC
		LIMIT 1`,
		districtID,
	)

	var b entities.WaterBalanceSummary
	err := row.Scan(
		&b.DistrictID, &b.PeriodStart, &b.PeriodEnd,
		&b.SystemInputM3,
		&b.TotalAuthorisedM3,
		&b.TotalApparentLossesM3,
		&b.TotalRealLossesM3,
		&b.NRWPercent, &b.ILI, &b.IWAGrade,
		&b.EstimatedRevenueRecoveryGHS,
		&b.DataConfidenceScore,
		&b.ComputedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("GetLatestBalance: %w", err)
	}
	return &b, nil
}

// ─── Test Helpers ─────────────────────────────────────────────────────────────
// These methods expose internal logic for unit testing without a database.

// ComputeForTest runs the compute() function directly (no DB required)
func (s *IWAWaterBalanceService) ComputeForTest(input *WaterBalanceInput) *WaterBalanceResult {
	return s.compute(input)
}

// ClassifyGradeForTest exposes the IWA grade classifier for unit testing
func (s *IWAWaterBalanceService) ClassifyGradeForTest(ili, nrwPct float64) string {
	return classifyIWAGrade(ili, nrwPct)
}
