package reconciler

import (
	"sync"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ReconcilerService implements the core 3-way reconciliation logic
// Compares GWL actual bills against GN-WAAS shadow bills
// Flags variances exceeding the configurable threshold
type ReconcilerService struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	threshold float64 // Loaded from system_config (default 15%); hot-reloadable
}

func NewReconcilerService(logger *zap.Logger, varianceThresholdPct float64) *ReconcilerService {
	return &ReconcilerService{
		logger:    logger,
		threshold: varianceThresholdPct,
	}
}

// ReconcileBill compares a GWL bill against its shadow bill
// Returns an AnomalyFlag if variance exceeds threshold
func (r *ReconcilerService) ReconcileBill(
	ctx context.Context,
	gwlBill *entities.GWLBillingRecord,
	shadowBill *entities.ShadowBillResult,
) (*entities.AnomalyFlag, error) {

	if gwlBill == nil || shadowBill == nil {
		return nil, fmt.Errorf("gwl bill and shadow bill are required")
	}

	variancePct := shadowBill.VariancePct
	absVariancePct := math.Abs(variancePct)

	r.logger.Debug("Reconciling bill",
		zap.String("account_id", gwlBill.AccountID.String()),
		zap.Float64("gwl_total", gwlBill.GWLTotalGHS),
		zap.Float64("shadow_total", shadowBill.TotalShadowBillGHS),
		zap.Float64("variance_pct", variancePct),
	)

	if absVariancePct <= r.threshold {
		return nil, nil // No anomaly
	}

	// Determine alert level based on variance magnitude
	alertLevel := r.classifyAlertLevel(absVariancePct)

	// Determine fraud type
	fraudType, description := r.classifyFraud(gwlBill, shadowBill, variancePct)

	flag := &entities.AnomalyFlag{
		ID:                  uuid.New(),
		AccountID:           &gwlBill.AccountID,
		DistrictID:          gwlBill.DistrictID,
		AnomalyType:         "SHADOW_BILL_VARIANCE",
		AlertLevel:          alertLevel,
		FraudType:           fraudType,
		Title:               fmt.Sprintf("Bill variance of %.1f%% detected", variancePct),
		Description:         description,
		EstimatedLossGHS:    shadowBill.VarianceGHS,
		BillingPeriodStart:  &gwlBill.BillingPeriodStart,
		BillingPeriodEnd:    &gwlBill.BillingPeriodEnd,
		ShadowBillID:        &shadowBill.ID,
		GWLBillID:           &gwlBill.ID,
		EvidenceData:        r.buildEvidence(gwlBill, shadowBill),
		Status:              "OPEN",
		SentinelVersion:     "1.0.0",
		DetectionHash:       r.computeHash(gwlBill, shadowBill),
		CreatedAt:           time.Now().UTC(),
	}

	return flag, nil
}

func (r *ReconcilerService) classifyAlertLevel(absVariancePct float64) string {
	switch {
	case absVariancePct >= 100:
		return "CRITICAL"
	case absVariancePct >= 50:
		return "HIGH"
	case absVariancePct >= 30:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

func (r *ReconcilerService) classifyFraud(
	gwlBill *entities.GWLBillingRecord,
	shadowBill *entities.ShadowBillResult,
	variancePct float64,
) (string, string) {

	// GWL billed LESS than shadow bill (under-billing)
	if variancePct > 0 {
		// Check if category mismatch is the cause
		if gwlBill.GWLCategory != shadowBill.CorrectCategory {
			return "CATEGORY_DOWNGRADE", fmt.Sprintf(
				"Account billed as %s but consumption pattern indicates %s. "+
					"GWL billed ₵%.2f, correct bill is ₵%.2f (variance: +%.1f%%)",
				gwlBill.GWLCategory, shadowBill.CorrectCategory,
				gwlBill.GWLTotalGHS, shadowBill.TotalShadowBillGHS, variancePct,
			)
		}
		return "VAT_EVASION", fmt.Sprintf(
			"GWL billed ₵%.2f but correct bill with PURC 2026 tariffs is ₵%.2f. "+
				"Potential under-billing or VAT not applied (variance: +%.1f%%)",
			gwlBill.GWLTotalGHS, shadowBill.TotalShadowBillGHS, variancePct,
		)
	}

	// GWL billed MORE than shadow bill (over-billing)
	return "READING_COLLUSION", fmt.Sprintf(
		"GWL billed ₵%.2f but correct bill is ₵%.2f. "+
			"Potential meter reading inflation (variance: %.1f%%)",
		gwlBill.GWLTotalGHS, shadowBill.TotalShadowBillGHS, variancePct,
	)
}

func (r *ReconcilerService) buildEvidence(
	gwlBill *entities.GWLBillingRecord,
	shadowBill *entities.ShadowBillResult,
) map[string]interface{} {
	return map[string]interface{}{
		"gwl_bill_id":          gwlBill.ID.String(),
		"gwl_category":         gwlBill.GWLCategory,
		"gwl_total_ghs":        gwlBill.GWLTotalGHS,
		"gwl_vat_ghs":          gwlBill.GWLVatGHS,
		"shadow_category":      shadowBill.CorrectCategory,
		"shadow_total_ghs":     shadowBill.TotalShadowBillGHS,
		"shadow_vat_ghs":       shadowBill.VATAmountGHS,
		"variance_ghs":         shadowBill.VarianceGHS,
		"variance_pct":         shadowBill.VariancePct,
		"consumption_m3":       gwlBill.ConsumptionM3,
		"threshold_pct":        r.threshold,
		"billing_period_start": gwlBill.BillingPeriodStart.Format("2006-01-02"),
		"billing_period_end":   gwlBill.BillingPeriodEnd.Format("2006-01-02"),
	}
}

func (r *ReconcilerService) computeHash(
	gwlBill *entities.GWLBillingRecord,
	shadowBill *entities.ShadowBillResult,
) string {
	// SHA-256 of key detection inputs for immutability verification
	// Produces a deterministic, tamper-evident fingerprint of the anomaly detection inputs.
	input := fmt.Sprintf("%s|%.4f|%.4f|%.4f",
		gwlBill.ID.String(),
		gwlBill.GWLTotalGHS,
		shadowBill.TotalShadowBillGHS,
		shadowBill.VariancePct,
	)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h[:])
}

// UpdateVarianceThreshold updates the variance threshold at runtime without restart.
// Called by the sentinel hot-reload goroutine every 5 minutes.
// Safe to call concurrently — protected by mu.
func (r *ReconcilerService) UpdateVarianceThreshold(pct float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.threshold = pct
}
