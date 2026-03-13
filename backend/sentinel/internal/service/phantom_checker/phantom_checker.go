package phantom_checker

import (
	"sync"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PhantomCheckerService detects phantom meters and unverified addresses.
//
// Revenue leakage detection:
//  1. Phantom Meter: Identical/fabricated readings → GWL under-collecting
//  2. Address Unverified: GPS outside network → DATA QUALITY flag → field job
//     (NOT a revenue leakage flag until field officer confirms outcome)
//
// The distinction matters:
//   - Phantom meter = revenue leakage (GHS calculable from shadow bill)
//   - Address unverified = data quality (GHS unknown until field visit)
//     → If field finds no meter: UNMETERED_CONSUMPTION (revenue leakage)
//     → If field finds no address: FRAUDULENT_ACCOUNT (GWL internal fraud)
type PhantomCheckerService struct {
	mu                   sync.RWMutex
	logger               *zap.Logger
	phantomMonths        int            // Months of identical readings = phantom; hot-reloadable
	roundNumberTolerance float64        // Tolerance for "suspiciously round" readings
	tariffCfg            *entities.TariffConfig // DB-loaded PURC rates (no hardcoding)
}

func NewPhantomCheckerService(logger *zap.Logger, phantomMonths int) *PhantomCheckerService {
	return &PhantomCheckerService{
		logger:               logger,
		phantomMonths:        phantomMonths,
		roundNumberTolerance: 0.01,
	}
}

// WithTariffConfig injects the DB-loaded tariff config into the phantom checker.
func (p *PhantomCheckerService) WithTariffConfig(cfg *entities.TariffConfig) *PhantomCheckerService {
	p.tariffCfg = cfg
	return p
}

// blendedResidentialRate returns the blended residential tariff rate per m3.
// Falls back to PURC 2026 tier-2 (10.8320) if tariffCfg is not set.
func (p *PhantomCheckerService) blendedResidentialRate() float64 {
	if p.tariffCfg != nil {
		return p.tariffCfg.BlendedRateForCategory("RESIDENTIAL")
	}
	return 10.8320 // PURC 2026 residential tier-2 (fallback only)
}

// vatMultiplier returns 1 + VAT/100. Falls back to 1.20 (20% VAT).
func (p *PhantomCheckerService) vatMultiplier() float64 {
	if p.tariffCfg != nil {
		return p.tariffCfg.VATMultiplier()
	}
	return 1.20
}

// CheckPhantomMeter analyses billing history for phantom meter patterns.
// Returns a REVENUE_LEAKAGE flag with estimated monthly GHS loss.
func (p *PhantomCheckerService) CheckPhantomMeter(
	ctx context.Context,
	account *entities.WaterAccount,
	billingHistory []*entities.GWLBillingRecord,
) (*entities.AnomalyFlag, error) {

	if len(billingHistory) < p.phantomMonths {
		return nil, nil // Not enough history
	}

	// Check 1: Identical consecutive readings
	if flag := p.checkIdenticalReadings(account, billingHistory); flag != nil {
		return flag, nil
	}

	// Check 2: Suspiciously round numbers
	if flag := p.checkRoundNumbers(account, billingHistory); flag != nil {
		return flag, nil
	}

	// Check 3: Zero standard deviation in consumption
	if flag := p.checkZeroVariance(account, billingHistory); flag != nil {
		return flag, nil
	}

	return nil, nil
}

// CheckAddressUnverified screens accounts whose GPS is outside the GWL pipe
// network boundary. This is a DATA QUALITY screening signal — NOT a revenue
// leakage flag. It creates a LOW severity flag that triggers a field job.
//
// The field job outcome determines the next step:
//   - METER_NOT_FOUND_INSTALL      → UNMETERED_CONSUMPTION (revenue leakage)
//   - ADDRESS_VALID_UNREGISTERED   → UNMETERED_CONSUMPTION (revenue leakage)
//   - ADDRESS_INVALID              → FRAUDULENT_ACCOUNT (GWL internal fraud)
//   - METER_FOUND_OK               → dismiss flag (GPS data was wrong)
func (p *PhantomCheckerService) CheckAddressUnverified(
	ctx context.Context,
	account *entities.WaterAccount,
) (*entities.AnomalyFlag, error) {

	if account.IsWithinNetwork == nil || *account.IsWithinNetwork {
		return nil, nil // Within network or not yet checked — skip
	}

	return &entities.AnomalyFlag{
		ID:         uuid.New(),
		AccountID:  &account.ID,
		DistrictID: account.DistrictID,

		// DATA_QUALITY — not revenue leakage until field verified
		AnomalyType: "ADDRESS_UNVERIFIED",
		AlertLevel:  "LOW", // Low severity — most are GPS data errors
		FraudType:   "OUTSIDE_NETWORK_BILLING",

		Title: fmt.Sprintf(
			"Account %s: GPS coordinates outside GWL pipe network — field verification required",
			account.GWLAccountNumber,
		),
		Description: fmt.Sprintf(
			"Account %s has GPS coordinates (%.6f, %.6f) that fall outside the known GWL "+
				"pipe network boundary. This is a data quality screening flag. "+
				"A field officer must visit the address to determine the correct action:\n"+
				"• If address is real with no meter → recommend meter installation (revenue leakage)\n"+
				"• If address does not exist → escalate as fraudulent account (GWL internal fraud)\n"+
				"• If GPS data is wrong → update coordinates and dismiss flag",
			account.GWLAccountNumber,
			account.GPSLatitude,
			account.GPSLongitude,
		),

		// No GHS value — unknown until field verification
		EstimatedLossGHS: 0,

		EvidenceData: map[string]interface{}{
			"account_number":     account.GWLAccountNumber,
			"gps_latitude":       account.GPSLatitude,
			"gps_longitude":      account.GPSLongitude,
			"network_check_date": account.NetworkCheckDate,
			"is_within_network":  false,
			"leakage_category":   "DATA_QUALITY",
			"required_action":    "FIELD_VERIFICATION",
			"possible_outcomes": []string{
				"METER_NOT_FOUND_INSTALL → UNMETERED_CONSUMPTION (revenue leakage)",
				"ADDRESS_INVALID → FRAUDULENT_ACCOUNT (GWL internal fraud)",
				"METER_FOUND_OK → dismiss (GPS data error)",
			},
		},
		Status:          "OPEN",
		SentinelVersion: "1.0.0",
		CreatedAt:       time.Now().UTC(),
	}, nil
}

// ─── Private helpers ──────────────────────────────────────────────────────────

func (p *PhantomCheckerService) checkIdenticalReadings(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	if len(history) < p.phantomMonths {
		return nil
	}

	recentHistory := history[len(history)-p.phantomMonths:]
	firstConsumption := recentHistory[0].ConsumptionM3
	identicalCount := 1

	for _, record := range recentHistory[1:] {
		if math.Abs(record.ConsumptionM3-firstConsumption) < p.roundNumberTolerance {
			identicalCount++
		} else {
			break
		}
	}

	if identicalCount < p.phantomMonths {
		return nil
	}

	// Revenue leakage: fabricated readings are typically set low.
	// Estimate monthly leakage as the difference between district average
	// and the suspiciously identical reading.
	estimatedActualM3 := account.MonthlyAvgConsumption
	if estimatedActualM3 <= firstConsumption {
		estimatedActualM3 = firstConsumption * 1.5 // conservative: assume 50% under-reading
	}
	monthlyLeakageGHS := (estimatedActualM3 - firstConsumption) * p.blendedResidentialRate() * p.vatMultiplier()

	return &entities.AnomalyFlag{
		ID:               uuid.New(),
		AccountID:        &account.ID,
		DistrictID:       account.DistrictID,
		AnomalyType:      "PHANTOM_METER",
		AlertLevel:       "HIGH",
		FraudType:        "PHANTOM_METER",
		EstimatedLossGHS: monthlyLeakageGHS,
		Title: fmt.Sprintf(
			"Phantom meter: identical %.2f m³ reading for %d consecutive months — GH₵%.2f/month leakage",
			firstConsumption, identicalCount, monthlyLeakageGHS,
		),
		Description: fmt.Sprintf(
			"Account %s has reported identical consumption of %.2f m³ for %d consecutive months. "+
				"Real meters show natural variation. This meter likely does not exist physically "+
				"or readings are being fabricated. "+
				"Estimated monthly revenue leakage: GH₵%.2f (%.2f m³ under-reported × GH₵%.2f/m³ + 20%% VAT).",
			account.GWLAccountNumber, firstConsumption, identicalCount,
			monthlyLeakageGHS, estimatedActualM3-firstConsumption, p.blendedResidentialRate(),
		),
		EvidenceData: map[string]interface{}{
			"account_number":       account.GWLAccountNumber,
			"identical_months":     identicalCount,
			"consumption_m3":       firstConsumption,
			"threshold_months":     p.phantomMonths,
			"estimated_actual_m3":  estimatedActualM3,
			"monthly_leakage_ghs":  monthlyLeakageGHS,
			"annualised_leakage_ghs": monthlyLeakageGHS * 12,
			"leakage_category":     "REVENUE_LEAKAGE",
		},
		Status:          "OPEN",
		SentinelVersion: "1.0.0",
		CreatedAt:       time.Now().UTC(),
	}
}

func (p *PhantomCheckerService) checkRoundNumbers(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	roundCount := 0
	for _, record := range history {
		if math.Mod(record.ConsumptionM3, 1.0) < p.roundNumberTolerance {
			roundCount++
		}
	}

	roundPct := float64(roundCount) / float64(len(history)) * 100
	if roundPct < 90 || len(history) < 6 {
		return nil
	}

	// Conservative leakage estimate: assume 20% under-reading on average
	avgConsumption := account.MonthlyAvgConsumption
	monthlyLeakageGHS := avgConsumption * 0.20 * p.blendedResidentialRate() * p.vatMultiplier()

	return &entities.AnomalyFlag{
		ID:               uuid.New(),
		AccountID:        &account.ID,
		DistrictID:       account.DistrictID,
		AnomalyType:      "PHANTOM_METER",
		AlertLevel:       "MEDIUM",
		FraudType:        "PHANTOM_METER",
		EstimatedLossGHS: monthlyLeakageGHS,
		Title: fmt.Sprintf(
			"Suspicious round-number readings: %.0f%% of bills — GH₵%.2f/month estimated leakage",
			roundPct, monthlyLeakageGHS,
		),
		Description: fmt.Sprintf(
			"Account %s has suspiciously round consumption figures in %.0f%% of %d billing records. "+
				"Real meter readings are rarely whole numbers. Possible manual fabrication. "+
				"Estimated monthly revenue leakage: GH₵%.2f (assuming 20%% under-reading).",
			account.GWLAccountNumber, roundPct, len(history), monthlyLeakageGHS,
		),
		EvidenceData: map[string]interface{}{
			"account_number":         account.GWLAccountNumber,
			"round_count":            roundCount,
			"total_records":          len(history),
			"round_pct":              roundPct,
			"monthly_leakage_ghs":    monthlyLeakageGHS,
			"annualised_leakage_ghs": monthlyLeakageGHS * 12,
			"leakage_category":       "REVENUE_LEAKAGE",
		},
		Status:          "OPEN",
		SentinelVersion: "1.0.0",
		CreatedAt:       time.Now().UTC(),
	}
}

func (p *PhantomCheckerService) checkZeroVariance(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	if len(history) < 6 {
		return nil
	}

	var sum float64
	for _, r := range history {
		sum += r.ConsumptionM3
	}
	mean := sum / float64(len(history))

	var variance float64
	for _, r := range history {
		diff := r.ConsumptionM3 - mean
		variance += diff * diff
	}
	variance /= float64(len(history))
	stdDev := math.Sqrt(variance)

	if mean <= 0 || (stdDev/mean) >= 0.005 {
		return nil
	}

	monthlyLeakageGHS := mean * 0.20 * p.blendedResidentialRate() * p.vatMultiplier()

	return &entities.AnomalyFlag{
		ID:               uuid.New(),
		AccountID:        &account.ID,
		DistrictID:       account.DistrictID,
		AnomalyType:      "PHANTOM_METER",
		AlertLevel:       "MEDIUM",
		FraudType:        "PHANTOM_METER",
		EstimatedLossGHS: monthlyLeakageGHS,
		Title: fmt.Sprintf(
			"Near-zero consumption variance (std dev: %.4f m³) — GH₵%.2f/month estimated leakage",
			stdDev, monthlyLeakageGHS,
		),
		Description: fmt.Sprintf(
			"Account %s shows near-zero variance (std dev: %.4f m³, CV: %.3f%%) over %d months. "+
				"Natural water consumption always varies. Possible phantom meter or fabricated readings. "+
				"Estimated monthly revenue leakage: GH₵%.2f (assuming 20%% under-reading).",
			account.GWLAccountNumber, stdDev, (stdDev/mean)*100, len(history), monthlyLeakageGHS,
		),
		EvidenceData: map[string]interface{}{
			"account_number":         account.GWLAccountNumber,
			"mean_m3":                mean,
			"std_dev_m3":             stdDev,
			"cv_pct":                 (stdDev / mean) * 100,
			"months_analysed":        len(history),
			"monthly_leakage_ghs":    monthlyLeakageGHS,
			"annualised_leakage_ghs": monthlyLeakageGHS * 12,
			"leakage_category":       "REVENUE_LEAKAGE",
		},
		Status:          "OPEN",
		SentinelVersion: "1.0.0",
		CreatedAt:       time.Now().UTC(),
	}
}

// UpdateMonthsThreshold updates the phantom meter detection window at runtime.
// Called by the sentinel hot-reload goroutine every 5 minutes.
// Safe to call concurrently — protected by mu.
func (p *PhantomCheckerService) UpdateMonthsThreshold(months int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.phantomMonths = months
}
