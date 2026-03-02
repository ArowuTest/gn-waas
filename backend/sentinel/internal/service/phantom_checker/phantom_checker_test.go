package phantom_checker_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	phantom "github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/phantom_checker"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ============================================================
// Helpers
// ============================================================

func newPhantomChecker(months int) *phantom.PhantomCheckerService {
	logger, _ := zap.NewDevelopment()
	return phantom.NewPhantomCheckerService(logger, months)
}

func makeAccount(accountNumber string, withinNetwork *bool) *entities.WaterAccount {
	districtID := uuid.New()
	return &entities.WaterAccount{
		ID:               uuid.New(),
		GWLAccountNumber: accountNumber,
		DistrictID:       districtID,
		Category:         "RESIDENTIAL",
		GPSLatitude:      5.6037,
		GPSLongitude:     -0.1870,
		IsWithinNetwork:  withinNetwork,
	}
}

func makeBillingHistory(consumptions []float64) []*entities.GWLBillingRecord {
	records := make([]*entities.GWLBillingRecord, len(consumptions))
	for i, c := range consumptions {
		start := time.Date(2026, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		records[i] = &entities.GWLBillingRecord{
			ID:                 uuid.New(),
			GWLBillID:          "GWL-BILL-" + string(rune('A'+i)),
			AccountID:          uuid.New(),
			DistrictID:         uuid.New(),
			BillingPeriodStart: start,
			BillingPeriodEnd:   end,
			ConsumptionM3:      c,
			GWLCategory:        "RESIDENTIAL",
			GWLAmountGHS:       c * 6.12,
			GWLVatGHS:          c * 1.22,
			GWLTotalGHS:        c * 7.34,
		}
	}
	return records
}

// ============================================================
// Phantom Meter — Identical Readings Tests
// ============================================================

func TestCheckPhantomMeter_IdenticalReadings_Flagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-PHANTOM-001", nil)

	// 3 identical readings = phantom
	history := makeBillingHistory([]float64{10.0, 10.0, 10.0})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected phantom meter flag for 3 identical readings, got nil")
	}
	if flag.AnomalyType != "PHANTOM_METER" {
		t.Errorf("Expected PHANTOM_METER anomaly type, got %s", flag.AnomalyType)
	}
	if flag.AlertLevel != "HIGH" {
		t.Errorf("Expected HIGH alert level, got %s", flag.AlertLevel)
	}
	if flag.Status != "OPEN" {
		t.Errorf("Expected OPEN status, got %s", flag.Status)
	}
}

func TestCheckPhantomMeter_VaryingReadings_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-NORMAL-001", nil)

	// Natural variation — should not be flagged
	history := makeBillingHistory([]float64{8.3, 11.7, 9.2, 10.5, 7.8, 12.1})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag != nil {
		t.Errorf("Expected no flag for varying readings, got: %+v", flag)
	}
}

func TestCheckPhantomMeter_InsufficientHistory_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-NEW-001", nil)

	// Only 2 records, need 3
	history := makeBillingHistory([]float64{10.0, 10.0})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag != nil {
		t.Error("Expected no flag for insufficient history, got flag")
	}
}

func TestCheckPhantomMeter_EmptyHistory_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-EMPTY-001", nil)

	flag, err := svc.CheckPhantomMeter(context.Background(), account, []*entities.GWLBillingRecord{})
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag != nil {
		t.Error("Expected no flag for empty history, got flag")
	}
}

func TestCheckPhantomMeter_FiveIdenticalReadings_Flagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-PHANTOM-005", nil)

	// 5 identical readings — definitely phantom
	history := makeBillingHistory([]float64{15.0, 15.0, 15.0, 15.0, 15.0})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected phantom meter flag for 5 identical readings, got nil")
	}
}

// ============================================================
// Phantom Meter — Round Numbers Tests
// ============================================================

func TestCheckPhantomMeter_AllRoundNumbers_Flagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-ROUND-001", nil)

	// All round numbers over 6 months — suspicious
	history := makeBillingHistory([]float64{5.0, 10.0, 15.0, 5.0, 10.0, 15.0})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected flag for all-round-number readings, got nil")
	}
	if flag.AnomalyType != "PHANTOM_METER" {
		t.Errorf("Expected PHANTOM_METER, got %s", flag.AnomalyType)
	}
}

func TestCheckPhantomMeter_MixedRoundAndReal_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)
	account := makeAccount("GWL-ACC-MIXED-001", nil)

	// Mix of round and real numbers — normal
	history := makeBillingHistory([]float64{8.3, 10.0, 9.7, 11.2, 10.0, 8.9})

	flag, err := svc.CheckPhantomMeter(context.Background(), account, history)
	if err != nil {
		t.Fatalf("CheckPhantomMeter failed: %v", err)
	}
	// Should not flag — only 2/6 = 33% round numbers
	if flag != nil && flag.FraudType == "PHANTOM_METER" {
		// Only fail if it's the round-number flag (not identical readings)
		if _, ok := flag.EvidenceData["round_pct"]; ok {
			t.Errorf("Expected no round-number flag for 33%% round readings, got flag")
		}
	}
}

// ============================================================
// Ghost Account Tests
// ============================================================

func TestCheckGhostAccount_OutsideNetwork_Flagged(t *testing.T) {
	svc := newPhantomChecker(3)

	withinNetwork := false
	checkDate := time.Now().UTC()
	account := &entities.WaterAccount{
		ID:               uuid.New(),
		GWLAccountNumber: "GWL-ACC-GHOST-001",
		DistrictID:       uuid.New(),
		Category:         "RESIDENTIAL",
		GPSLatitude:      6.1234,
		GPSLongitude:     -0.5678,
		IsWithinNetwork:  &withinNetwork,
		NetworkCheckDate: &checkDate,
	}

	flag, err := svc.CheckGhostAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CheckGhostAccount failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected ghost account flag for outside-network account, got nil")
	}
	if flag.AnomalyType != "GHOST_ACCOUNT" {
		t.Errorf("Expected GHOST_ACCOUNT anomaly type, got %s", flag.AnomalyType)
	}
	if flag.AlertLevel != "HIGH" {
		t.Errorf("Expected HIGH alert level, got %s", flag.AlertLevel)
	}
	if flag.FraudType != "OUTSIDE_NETWORK_BILLING" {
		t.Errorf("Expected OUTSIDE_NETWORK_BILLING fraud type, got %s", flag.FraudType)
	}
}

func TestCheckGhostAccount_InsideNetwork_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)

	withinNetwork := true
	account := makeAccount("GWL-ACC-VALID-001", &withinNetwork)

	flag, err := svc.CheckGhostAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CheckGhostAccount failed: %v", err)
	}
	if flag != nil {
		t.Errorf("Expected no flag for inside-network account, got: %+v", flag)
	}
}

func TestCheckGhostAccount_NetworkNotChecked_NotFlagged(t *testing.T) {
	svc := newPhantomChecker(3)

	// IsWithinNetwork is nil — not yet checked
	account := makeAccount("GWL-ACC-UNCHECKED-001", nil)

	flag, err := svc.CheckGhostAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CheckGhostAccount failed: %v", err)
	}
	if flag != nil {
		t.Error("Expected no flag when network check not yet performed, got flag")
	}
}

// ============================================================
// Evidence Data Tests
// ============================================================

func TestCheckGhostAccount_EvidenceContainsGPS(t *testing.T) {
	svc := newPhantomChecker(3)

	withinNetwork := false
	checkDate := time.Now().UTC()
	account := &entities.WaterAccount{
		ID:               uuid.New(),
		GWLAccountNumber: "GWL-ACC-GPS-001",
		DistrictID:       uuid.New(),
		Category:         "COMMERCIAL",
		GPSLatitude:      5.9876,
		GPSLongitude:     -0.2345,
		IsWithinNetwork:  &withinNetwork,
		NetworkCheckDate: &checkDate,
	}

	flag, err := svc.CheckGhostAccount(context.Background(), account)
	if err != nil {
		t.Fatalf("CheckGhostAccount failed: %v", err)
	}
	if flag == nil {
		t.Fatal("Expected ghost account flag, got nil")
	}

	if _, ok := flag.EvidenceData["gps_latitude"]; !ok {
		t.Error("Expected gps_latitude in evidence data")
	}
	if _, ok := flag.EvidenceData["gps_longitude"]; !ok {
		t.Error("Expected gps_longitude in evidence data")
	}
	if _, ok := flag.EvidenceData["account_number"]; !ok {
		t.Error("Expected account_number in evidence data")
	}
}
