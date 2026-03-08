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
	logger             *zap.Logger
	nightFlowThreshold float64                // % of daily average that triggers flag (from system_config)
	tariffCfg          *entities.TariffConfig // DB-loaded PURC rates (no hardcoding)
}

func NewNightFlowAnalyser(logger *zap.Logger, nightFlowThresholdPct float64) *NightFlowAnalyser {
	return &NightFlowAnalyser{
		logger:             logger,
		nightFlowThreshold: nightFlowThresholdPct,
	}
}

// WithTariffConfig injects the DB-loaded tariff config.
func (n *NightFlowAnalyser) WithTariffConfig(cfg *entities.TariffConfig) *NightFlowAnalyser {
	n.tariffCfg = cfg
	return n
}

// blendedResidentialRate returns the blended residential tariff rate per m3.
func (n *NightFlowAnalyser) blendedResidentialRate() float64 {
	if n.tariffCfg != nil {
		return n.tariffCfg.BlendedRateForCategory("RESIDENTIAL")
	}
	return 10.8320 // PURC 2026 residential tier-2 (fallback only)
}

// vatMultiplier returns 1 + VAT/100.
func (n *NightFlowAnalyser) vatMultiplier() float64 {
	if n.tariffCfg != nil {
		return n.tariffCfg.VATMultiplier()
	}
	return 1.20
}

// AnalyseDistrictBalance performs statistical night-flow equivalent analysis
// using production vs billing records (software-only mode, no IoT).
//
// Revenue leakage calculation:
//   Unaccounted water = production - billed
//   After subtracting estimated real losses (IWA standard: 15% of production),
//   the remainder is APPARENT LOSS = unregistered consumption = revenue leakage.
//
//   Monthly leakage GHS = apparent_loss_m3 × district_avg_tariff × 1.20 (VAT)
//
// This is the district-level signal. Account-level signals come from:
//   - SHADOW_BILL_VARIANCE (individual bill reconciliation)
//   - CATEGORY_MISMATCH (individual account category check)
//   - ADDRESS_UNVERIFIED → UNMETERED_CONSUMPTION (field verification)
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

	// IWA/AWWA standard: ~15% of production is acceptable real loss (physical leakage).
	// Anything above that is apparent loss = revenue leakage.
	const (
		realLossBaselinePct = 15.0 // IWA standard baseline for Ghana infrastructure
	)

	// Use DB-loaded tariff rates (no hardcoding)
	districtAvgTariffGHS := n.blendedResidentialRate()
	vatMult              := n.vatMultiplier()

	realLossM3 := productionM3 * (realLossBaselinePct / 100.0)
	apparentLossM3 := unaccountedM3 - realLossM3
	if apparentLossM3 < 0 {
		apparentLossM3 = 0
	}

	// Monthly revenue leakage = apparent loss × tariff × VAT
	monthlyLeakageGHS := apparentLossM3 * districtAvgTariffGHS * vatMult
	annualisedLeakageGHS := monthlyLeakageGHS * 12

	// Classify severity based on unaccounted percentage
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
		ID:               uuid.New(),
		DistrictID:       district.ID,
		AnomalyType:      "DISTRICT_IMBALANCE",
		AlertLevel:       alertLevel,
		FraudType:        "DISTRICT_IMBALANCE",
		EstimatedLossGHS: monthlyLeakageGHS,
		Title: fmt.Sprintf(
			"%s: %.1f%% unaccounted water - GHC%.2f/month apparent loss (GHC%.2f/year)",
			district.DistrictName, unaccountedPct, monthlyLeakageGHS, annualisedLeakageGHS,
		),
		Description: fmt.Sprintf(
			"District %s produced %.2f m3 but only %.2f m3 was billed (%.1f%% unaccounted). "+
				"This exceeds the %.1f%% threshold.\n\n"+
				"Revenue leakage breakdown (IWA/AWWA method):\n"+
				"  Total unaccounted:    %.2f m3 (%.1f%% of production)\n"+
				"  Real loss baseline:  %.2f m3 (%.0f%% IWA standard)\n"+
				"  Apparent loss:       %.2f m3 (unregistered consumption)\n"+
				"  Monthly leakage:     GHC%.2f (%.2f m3 x GHC%.2f/m3 x %.2f VAT)\n"+
				"  Annual leakage:      GHC%.2f\n\n"+
				"Possible causes:\n"+
				"  (1) Unregistered connections - addresses consuming with no billing account\n"+
				"  (2) Category fraud - commercial premises billed as residential\n"+
				"  (3) Meter tampering - readings manipulated below actual consumption\n"+
				"  (4) Physical leakage above IWA baseline (infrastructure issue)\n\n"+
				"Recommended action: Field audit of top 20 accounts by consumption in this district.",
			district.DistrictName, productionM3, billedM3, unaccountedPct, n.nightFlowThreshold,
			unaccountedM3, unaccountedPct,
			realLossM3, realLossBaselinePct,
			apparentLossM3,
			monthlyLeakageGHS, apparentLossM3, districtAvgTariffGHS, vatMult,
			annualisedLeakageGHS,
		),
		EvidenceData: map[string]interface{}{
			"district_code":           district.DistrictCode,
			"district_name":           district.DistrictName,
			"production_m3":           productionM3,
			"billed_m3":               billedM3,
			"unaccounted_m3":          unaccountedM3,
			"unaccounted_pct":         unaccountedPct,
			"real_loss_baseline_m3":   realLossM3,
			"apparent_loss_m3":        apparentLossM3,
			"monthly_leakage_ghs":     monthlyLeakageGHS,
			"annualised_leakage_ghs":  annualisedLeakageGHS,
			"district_avg_tariff_ghs": districtAvgTariffGHS,
			"threshold_pct":           n.nightFlowThreshold,
			"analysis_period":         period.Format("2006-01"),
			"analysis_type":           "STATISTICAL_BALANCE",
			"iot_available":           false,
			"leakage_category":        "REVENUE_LEAKAGE",
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
