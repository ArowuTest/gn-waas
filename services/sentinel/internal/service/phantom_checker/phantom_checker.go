package phantom_checker

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ArowuTest/gn-waas/services/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PhantomCheckerService detects phantom meters and ghost accounts
// Implements two checks:
// 1. Phantom Meter: Identical readings for N consecutive months
// 2. Ghost Account: Account GPS outside GWL pipe network boundary
type PhantomCheckerService struct {
	logger              *zap.Logger
	phantomMonths       int     // Months of identical readings = phantom (from system_config)
	roundNumberTolerance float64 // Tolerance for "suspiciously round" readings
}

func NewPhantomCheckerService(logger *zap.Logger, phantomMonths int) *PhantomCheckerService {
	return &PhantomCheckerService{
		logger:               logger,
		phantomMonths:        phantomMonths,
		roundNumberTolerance: 0.01,
	}
}

// CheckPhantomMeter analyses billing history for phantom meter patterns
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

// CheckGhostAccount verifies account is within the GWL pipe network boundary
func (p *PhantomCheckerService) CheckGhostAccount(
	ctx context.Context,
	account *entities.WaterAccount,
) (*entities.AnomalyFlag, error) {

	// If network check has been done and account is outside
	if account.IsWithinNetwork != nil && !*account.IsWithinNetwork {
		return &entities.AnomalyFlag{
			ID:          uuid.New(),
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "GHOST_ACCOUNT",
			AlertLevel:  "HIGH",
			FraudType:   "OUTSIDE_NETWORK_BILLING",
			Title:       "Account billed outside GWL pipe network boundary",
			Description: fmt.Sprintf(
				"Account %s (GPS: %.6f, %.6f) is located outside the GWL pipe network boundary. "+
					"This account may be a ghost account generating fraudulent revenue.",
				account.GWLAccountNumber,
				account.GPSLatitude,
				account.GPSLongitude,
			),
			EvidenceData: map[string]interface{}{
				"account_number":    account.GWLAccountNumber,
				"gps_latitude":      account.GPSLatitude,
				"gps_longitude":     account.GPSLongitude,
				"network_check_date": account.NetworkCheckDate,
				"is_within_network": false,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}, nil
	}

	return nil, nil
}

func (p *PhantomCheckerService) checkIdenticalReadings(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	if len(history) < p.phantomMonths {
		return nil
	}

	// Check last N months for identical consumption
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

	if identicalCount >= p.phantomMonths {
		return &entities.AnomalyFlag{
			ID:          uuid.New(),
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "PHANTOM_METER",
			AlertLevel:  "HIGH",
			FraudType:   "PHANTOM_METER",
			Title:       fmt.Sprintf("Phantom meter: identical readings for %d consecutive months", identicalCount),
			Description: fmt.Sprintf(
				"Account %s has reported identical consumption of %.2f m³ for %d consecutive months. "+
					"Real meters show natural variation. This meter likely does not exist physically.",
				account.GWLAccountNumber, firstConsumption, identicalCount,
			),
			EvidenceData: map[string]interface{}{
				"account_number":    account.GWLAccountNumber,
				"identical_months":  identicalCount,
				"consumption_m3":    firstConsumption,
				"threshold_months":  p.phantomMonths,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}
	}

	return nil
}

func (p *PhantomCheckerService) checkRoundNumbers(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	roundCount := 0
	for _, record := range history {
		// Check if consumption is a suspiciously round number (e.g., 5.00, 10.00, 15.00)
		if math.Mod(record.ConsumptionM3, 1.0) < p.roundNumberTolerance {
			roundCount++
		}
	}

	roundPct := float64(roundCount) / float64(len(history)) * 100
	if roundPct >= 90 && len(history) >= 6 {
		return &entities.AnomalyFlag{
			ID:          uuid.New(),
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "PHANTOM_METER",
			AlertLevel:  "MEDIUM",
			FraudType:   "PHANTOM_METER",
			Title:       fmt.Sprintf("Suspicious round-number readings: %.0f%% of bills", roundPct),
			Description: fmt.Sprintf(
				"Account %s has suspiciously round consumption figures in %.0f%% of %d billing records. "+
					"Real meter readings are rarely whole numbers. Possible manual fabrication.",
				account.GWLAccountNumber, roundPct, len(history),
			),
			EvidenceData: map[string]interface{}{
				"account_number": account.GWLAccountNumber,
				"round_count":    roundCount,
				"total_records":  len(history),
				"round_pct":      roundPct,
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}
	}

	return nil
}

func (p *PhantomCheckerService) checkZeroVariance(
	account *entities.WaterAccount,
	history []*entities.GWLBillingRecord,
) *entities.AnomalyFlag {

	if len(history) < 6 {
		return nil
	}

	// Calculate standard deviation of consumption
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

	// If std dev is less than 0.5% of mean, flag as suspicious
	if mean > 0 && (stdDev/mean) < 0.005 {
		return &entities.AnomalyFlag{
			ID:          uuid.New(),
			AccountID:   &account.ID,
			DistrictID:  account.DistrictID,
			AnomalyType: "PHANTOM_METER",
			AlertLevel:  "MEDIUM",
			FraudType:   "PHANTOM_METER",
			Title:       "Near-zero consumption variance detected",
			Description: fmt.Sprintf(
				"Account %s shows near-zero variance (std dev: %.4f m³) over %d months. "+
					"Natural water consumption always varies. Possible phantom meter.",
				account.GWLAccountNumber, stdDev, len(history),
			),
			EvidenceData: map[string]interface{}{
				"account_number": account.GWLAccountNumber,
				"mean_m3":        mean,
				"std_dev_m3":     stdDev,
				"cv_pct":         (stdDev / mean) * 100,
				"months_analysed": len(history),
			},
			Status:          "OPEN",
			SentinelVersion: "1.0.0",
			CreatedAt:       time.Now().UTC(),
		}
	}

	return nil
}
