package night_flow

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NightFlowAnalyser implements the 2-4 AM night flow analysis
// Night flow (2-4 AM) is the minimum flow period when legitimate consumption is near zero
// High night flow = leakage OR theft (water flowing but not being billed)
//
// Without IoT meters, this is implemented statistically:
// - Compare district production records vs billed consumption
// - Flag districts where production >> billing (implies unaccounted flow)
type NightFlowAnalyser struct {
	logger              *zap.Logger
	nightFlowThreshold  float64 // % of daily average that triggers flag (from system_config)
}

func NewNightFlowAnalyser(logger *zap.Logger, nightFlowThresholdPct float64) *NightFlowAnalyser {
	return &NightFlowAnalyser{
		logger:             logger,
		nightFlowThreshold: nightFlowThresholdPct,
	}
}

// AnalyseDistrictBalance performs statistical night-flow equivalent analysis
// using production vs billing records (software-only mode, no IoT)
func (n *NightFlowAnalyser) AnalyseDistrictBalance(
	ctx context.Context,
	district *entities.District,
	productionM3 float64,
	billedM3 float64,
	period time.Time,
) (*entities.AnomalyFlag, error) {

	if productionM3 <= 0 {
		return nil, nil // No production data
	}

	// Calculate unaccounted water
	unaccountedM3 := productionM3 - billedM3
	unaccountedPct := (unaccountedM3 / productionM3) * 100

	n.logger.Debug("District balance analysis",
		zap.String("district", district.DistrictCode),
		zap.Float64("production_m3", productionM3),
		zap.Float64("billed_m3", billedM3),
		zap.Float64("unaccounted_pct", unaccountedPct),
	)

	if unaccountedPct <= n.nightFlowThreshold {
		return nil, nil // Within acceptable range
	}

	// Classify severity
	alertLevel := "LOW"
	switch {
	case unaccountedPct >= 70:
		alertLevel = "CRITICAL"
	case unaccountedPct >= 50:
		alertLevel = "HIGH"
	case unaccountedPct >= 35:
		alertLevel = "MEDIUM"
	}

	return &entities.AnomalyFlag{
		ID:          uuid.New(),
		DistrictID:  district.ID,
		AnomalyType: "DISTRICT_IMBALANCE",
		AlertLevel:  alertLevel,
		FraudType:   "DISTRICT_IMBALANCE",
		Title: fmt.Sprintf(
			"%s: %.1f%% of water produced is unaccounted",
			district.DistrictName, unaccountedPct,
		),
		Description: fmt.Sprintf(
			"District %s produced %.2f m³ but only %.2f m³ was billed (%.1f%% unaccounted). "+
				"This exceeds the %.1f%% threshold. Possible causes: "+
				"(1) Phantom billing in unserved areas, "+
				"(2) Commercial accounts billed as residential, "+
				"(3) Physical leakage, "+
				"(4) Meter tampering. "+
				"Recommend field audit of top 20 accounts by consumption.",
			district.DistrictName, productionM3, billedM3, unaccountedPct, n.nightFlowThreshold,
		),
		EvidenceData: map[string]interface{}{
			"district_code":    district.DistrictCode,
			"district_name":    district.DistrictName,
			"production_m3":    productionM3,
			"billed_m3":        billedM3,
			"unaccounted_m3":   unaccountedM3,
			"unaccounted_pct":  unaccountedPct,
			"threshold_pct":    n.nightFlowThreshold,
			"analysis_period":  period.Format("2006-01"),
			"analysis_type":    "STATISTICAL_BALANCE",
			"iot_available":    false,
		},
		Status:          "OPEN",
		SentinelVersion: "1.0.0",
		CreatedAt:       time.Now().UTC(),
	}, nil
}

// AnalyseRationingAnomaly detects accounts with high consumption during supply outages
// If an area had no water supply but bills show consumption, it's phantom billing
func (n *NightFlowAnalyser) AnalyseRationingAnomaly(
	ctx context.Context,
	account *entities.WaterAccount,
	billingRecord *entities.GWLBillingRecord,
	supplySchedule *entities.SupplySchedule,
) (*entities.AnomalyFlag, error) {

	if supplySchedule == nil || billingRecord == nil {
		return nil, nil
	}

	// Calculate expected supply days in billing period
	supplyDays := supplySchedule.SupplyDaysPerWeek * 4 // Approximate monthly
	totalDays := 30.0
	supplyRatio := float64(supplyDays) / totalDays

	// Expected consumption should be proportional to supply days
	expectedMaxConsumption := account.MonthlyAvgConsumption * supplyRatio * 1.2 // 20% tolerance

	if billingRecord.ConsumptionM3 > expectedMaxConsumption && supplyRatio < 0.5 {
		return &entities.AnomalyFlag{
			ID:          uuid.New(),
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "RATIONING_ANOMALY",
			AlertLevel:  "HIGH",
			FraudType:   "PHANTOM_BILLING",
			Title: fmt.Sprintf(
				"Impossible consumption during rationing: %.2f m³ with only %d supply days",
				billingRecord.ConsumptionM3, supplyDays,
			),
			Description: fmt.Sprintf(
				"Account %s reported %.2f m³ consumption during a period with only %d supply days/week. "+
					"Expected maximum: %.2f m³. This is mathematically impossible without storage. "+
					"Possible phantom billing or meter reading fabrication.",
				account.GWLAccountNumber,
				billingRecord.ConsumptionM3,
				supplyDays,
				expectedMaxConsumption,
			),
			EvidenceData: map[string]interface{}{
				"account_number":          account.GWLAccountNumber,
				"reported_consumption_m3": billingRecord.ConsumptionM3,
				"expected_max_m3":         expectedMaxConsumption,
				"supply_days_per_week":    supplyDays,
				"supply_ratio":            supplyRatio,
				"avg_monthly_m3":          account.MonthlyAvgConsumption,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}, nil
	}

	return nil, nil
}
