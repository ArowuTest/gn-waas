package reconciler_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/reconciler"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ============================================================
// Helpers
// ============================================================

func newReconciler(threshold float64) *reconciler.ReconcilerService {
	logger, _ := zap.NewDevelopment()
	return reconciler.NewReconcilerService(logger, threshold)
}

func makeGWLBill(gwlTotal, consumption float64, category string) *entities.GWLBillingRecord {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	return &entities.GWLBillingRecord{
		ID:                 uuid.New(),
		GWLBillID:          "GWL-BILL-001",
		AccountID:          uuid.New(),
		DistrictID:         uuid.New(),
		BillingPeriodStart: start,
		BillingPeriodEnd:   end,
		ConsumptionM3:      consumption,
		GWLCategory:        category,
		GWLAmountGHS:       gwlTotal * 0.833,
		GWLVatGHS:          gwlTotal * 0.167,
		GWLTotalGHS:        gwlTotal,
	}
}

func makeShadowBill(shadowTotal, variancePct float64, category string) *entities.ShadowBillResult {
	return &entities.ShadowBillResult{
		ID:                 uuid.New(),
		CorrectCategory:    category,
		TotalShadowBillGHS: shadowTotal,
		VATAmountGHS:       shadowTotal * 0.167,
		VarianceGHS:        shadowTotal - (shadowTotal / (1 + variancePct/100)),
		VariancePct:        variancePct,
	}
}

// ============================================================
// Core Reconciliation Logic Tests
// ============================================================

func TestReconcileBill_NoAnomalyBelowThreshold(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 10.0, "RESIDENTIAL")
	shadow := makeShadowBill(110.00, 10.0, "RESIDENTIAL") // 10% variance < 15% threshold

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag != nil {
		t.Errorf("Expected no anomaly flag for 10%% variance (threshold 15%%), got flag: %+v", flag)
	}
}

func TestReconcileBill_NoAnomalyAtExactThreshold(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 10.0, "RESIDENTIAL")
	shadow := makeShadowBill(115.00, 15.0, "RESIDENTIAL") // exactly at threshold

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag != nil {
		t.Errorf("Expected no anomaly at exact threshold (15%%), got flag")
	}
}

func TestReconcileBill_FlagAboveThreshold(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL") // 50% variance > 15% threshold

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag for 50%% variance, got nil")
	}
	if flag.Status != "OPEN" {
		t.Errorf("Expected status OPEN, got %s", flag.Status)
	}
	if flag.AnomalyType != "SHADOW_BILL_VARIANCE" {
		t.Errorf("Expected anomaly type SHADOW_BILL_VARIANCE, got %s", flag.AnomalyType)
	}
}

func TestReconcileBill_AlertLevelCritical(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 50.0, "COMMERCIAL")
	shadow := makeShadowBill(300.00, 200.0, "COMMERCIAL") // 200% variance → CRITICAL

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.AlertLevel != "CRITICAL" {
		t.Errorf("Expected CRITICAL alert level for 200%% variance, got %s", flag.AlertLevel)
	}
}

func TestReconcileBill_AlertLevelHigh(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(175.00, 75.0, "RESIDENTIAL") // 75% → HIGH

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.AlertLevel != "HIGH" {
		t.Errorf("Expected HIGH alert level for 75%% variance, got %s", flag.AlertLevel)
	}
}

func TestReconcileBill_AlertLevelMedium(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 15.0, "RESIDENTIAL")
	shadow := makeShadowBill(140.00, 40.0, "RESIDENTIAL") // 40% → MEDIUM

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.AlertLevel != "MEDIUM" {
		t.Errorf("Expected MEDIUM alert level for 40%% variance, got %s", flag.AlertLevel)
	}
}

func TestReconcileBill_AlertLevelLow(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 10.0, "RESIDENTIAL")
	shadow := makeShadowBill(120.00, 20.0, "RESIDENTIAL") // 20% → LOW

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.AlertLevel != "LOW" {
		t.Errorf("Expected LOW alert level for 20%% variance, got %s", flag.AlertLevel)
	}
}

// ============================================================
// Fraud Classification Tests
// ============================================================

func TestReconcileBill_CategoryDowngradeFraud(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 50.0, "RESIDENTIAL")
	shadow := makeShadowBill(200.00, 100.0, "COMMERCIAL") // category mismatch

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.FraudType != "CATEGORY_DOWNGRADE" {
		t.Errorf("Expected CATEGORY_DOWNGRADE fraud type, got %s", flag.FraudType)
	}
	if !strings.Contains(flag.Description, "RESIDENTIAL") {
		t.Errorf("Expected description to mention RESIDENTIAL category, got: %s", flag.Description)
	}
	if !strings.Contains(flag.Description, "COMMERCIAL") {
		t.Errorf("Expected description to mention COMMERCIAL category, got: %s", flag.Description)
	}
}

func TestReconcileBill_VATEvasionFraud(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL") // same category, under-billing

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.FraudType != "VAT_EVASION" {
		t.Errorf("Expected VAT_EVASION fraud type, got %s", flag.FraudType)
	}
}

func TestReconcileBill_ReadingCollusionFraud(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(200.00, 25.0, "RESIDENTIAL")
	// Negative variance: GWL billed MORE than shadow bill
	shadow := &entities.ShadowBillResult{
		ID:                 uuid.New(),
		CorrectCategory:    "RESIDENTIAL",
		TotalShadowBillGHS: 100.00,
		VATAmountGHS:       16.67,
		VarianceGHS:        -100.00,
		VariancePct:        -50.0, // negative = GWL over-billed
	}

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}
	if flag.FraudType != "READING_COLLUSION" {
		t.Errorf("Expected READING_COLLUSION fraud type, got %s", flag.FraudType)
	}
}

// ============================================================
// Detection Hash Tests (SHA-256 integrity)
// ============================================================

func TestReconcileBill_DetectionHashIsSHA256(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL")

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}

	// SHA-256 hex string is always 64 characters
	if len(flag.DetectionHash) != 64 {
		t.Errorf("Expected 64-char SHA-256 hex hash, got %d chars: %s",
			len(flag.DetectionHash), flag.DetectionHash)
	}

	// Verify it's valid hex
	for _, c := range flag.DetectionHash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("DetectionHash contains non-hex character: %c", c)
			break
		}
	}
}

func TestReconcileBill_DetectionHashIsDeterministic(t *testing.T) {
	svc := newReconciler(15.0)

	gwlID := uuid.New()
	gwl := &entities.GWLBillingRecord{
		ID:                 gwlID,
		GWLBillID:          "GWL-BILL-DET-001",
		AccountID:          uuid.New(),
		DistrictID:         uuid.New(),
		BillingPeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BillingPeriodEnd:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		ConsumptionM3:      25.0,
		GWLCategory:        "RESIDENTIAL",
		GWLAmountGHS:       83.30,
		GWLVatGHS:          16.70,
		GWLTotalGHS:        100.00,
	}
	shadow := &entities.ShadowBillResult{
		ID:                 uuid.New(),
		CorrectCategory:    "RESIDENTIAL",
		TotalShadowBillGHS: 150.00,
		VATAmountGHS:       25.00,
		VarianceGHS:        50.00,
		VariancePct:        50.0,
	}

	flag1, _ := svc.ReconcileBill(context.Background(), gwl, shadow)
	flag2, _ := svc.ReconcileBill(context.Background(), gwl, shadow)

	if flag1 == nil || flag2 == nil {
		t.Fatal("Expected anomaly flags, got nil")
	}
	if flag1.DetectionHash != flag2.DetectionHash {
		t.Errorf("DetectionHash is not deterministic: %s != %s",
			flag1.DetectionHash, flag2.DetectionHash)
	}

	// Manually verify the hash
	input := fmt.Sprintf("%s|%.4f|%.4f|%.4f",
		gwlID.String(),
		100.00,
		150.00,
		50.0,
	)
	h := sha256.Sum256([]byte(input))
	expected := fmt.Sprintf("%x", h[:])
	if flag1.DetectionHash != expected {
		t.Errorf("DetectionHash mismatch.\nExpected: %s\nGot:      %s", expected, flag1.DetectionHash)
	}
}

func TestReconcileBill_DifferentBillsDifferentHashes(t *testing.T) {
	svc := newReconciler(15.0)

	gwl1 := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	gwl2 := makeGWLBill(200.00, 25.0, "RESIDENTIAL") // different amount
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL")

	flag1, _ := svc.ReconcileBill(context.Background(), gwl1, shadow)
	flag2, _ := svc.ReconcileBill(context.Background(), gwl2, shadow)

	if flag1 == nil || flag2 == nil {
		t.Fatal("Expected anomaly flags, got nil")
	}
	if flag1.DetectionHash == flag2.DetectionHash {
		t.Error("Expected different hashes for different bills, got same hash")
	}
}

// ============================================================
// Evidence Data Tests
// ============================================================

func TestReconcileBill_EvidenceDataContainsRequiredFields(t *testing.T) {
	svc := newReconciler(15.0)

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL")

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected anomaly flag, got nil")
	}

	requiredFields := []string{
		"gwl_bill_id", "gwl_category", "gwl_total_ghs",
		"shadow_category", "shadow_total_ghs", "variance_ghs",
		"variance_pct", "consumption_m3", "threshold_pct",
	}
	for _, field := range requiredFields {
		if _, ok := flag.EvidenceData[field]; !ok {
			t.Errorf("Missing required evidence field: %s", field)
		}
	}
}

// ============================================================
// Nil Input Tests
// ============================================================

func TestReconcileBill_NilGWLBillReturnsError(t *testing.T) {
	svc := newReconciler(15.0)
	shadow := makeShadowBill(150.00, 50.0, "RESIDENTIAL")

	_, err := svc.ReconcileBill(context.Background(), nil, shadow)
	if err == nil {
		t.Error("Expected error for nil GWL bill, got nil")
	}
}

func TestReconcileBill_NilShadowBillReturnsError(t *testing.T) {
	svc := newReconciler(15.0)
	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")

	_, err := svc.ReconcileBill(context.Background(), gwl, nil)
	if err == nil {
		t.Error("Expected error for nil shadow bill, got nil")
	}
}

// ============================================================
// Threshold Configuration Tests
// ============================================================

func TestReconcileBill_CustomThreshold5Pct(t *testing.T) {
	svc := newReconciler(5.0) // strict 5% threshold

	gwl := makeGWLBill(100.00, 10.0, "RESIDENTIAL")
	shadow := makeShadowBill(110.00, 10.0, "RESIDENTIAL") // 10% > 5% threshold

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag == nil {
		t.Error("Expected anomaly flag for 10%% variance with 5%% threshold, got nil")
	}
}

func TestReconcileBill_CustomThreshold50Pct(t *testing.T) {
	svc := newReconciler(50.0) // lenient 50% threshold

	gwl := makeGWLBill(100.00, 25.0, "RESIDENTIAL")
	shadow := makeShadowBill(130.00, 30.0, "RESIDENTIAL") // 30% < 50% threshold

	flag, err := svc.ReconcileBill(context.Background(), gwl, shadow)
	if err != nil {
		t.Fatalf("ReconcileBill failed: %v", err)
	}
	if flag != nil {
		t.Error("Expected no anomaly flag for 30%% variance with 50%% threshold, got flag")
	}
}
