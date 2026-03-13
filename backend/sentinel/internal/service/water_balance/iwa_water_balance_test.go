package water_balance_test

import (
	"testing"
	"time"

	wb "github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/water_balance"
	"github.com/google/uuid"
)

// TestComputeWaterBalance_NRWCalculation verifies the IWA water balance equation
func TestComputeWaterBalance_NRWCalculation(t *testing.T) {
	svc := wb.NewIWAWaterBalanceService(nil, nil) // nil DB — testing compute() only

	input := &wb.WaterBalanceInput{
		DistrictID:  uuid.New(),
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),

		// System Input: 10,000 m³
		SystemInputM3: 10_000,

		// Authorised Consumption: 5,000 m³
		BilledMeteredM3:     4_500,
		BilledUnmeteredM3:   300,
		UnbilledMeteredM3:   150,
		UnbilledUnmeteredM3: 50,

		// Apparent Losses: 1,500 m³
		UnauthorisedConsumptionM3: 1_200,
		MeteringInaccuraciesM3:    200,
		DataHandlingErrorsM3:      100,

		// Real Losses: 3,500 m³
		MainLeakageM3:           3_000,
		StorageOverflowM3:       300,
		ServiceConnectionLeakM3: 200,
	}

	result := svc.ComputeForTest(input)

	// Authorised = 4500 + 300 + 150 + 50 = 5000
	if result.TotalAuthorisedM3 != 5_000 {
		t.Errorf("TotalAuthorisedM3: expected 5000, got %.1f", result.TotalAuthorisedM3)
	}

	// Apparent Losses = 1200 + 200 + 100 = 1500
	if result.TotalApparentLossesM3 != 1_500 {
		t.Errorf("TotalApparentLossesM3: expected 1500, got %.1f", result.TotalApparentLossesM3)
	}

	// Real Losses = 3000 + 300 + 200 = 3500
	if result.TotalRealLossesM3 != 3_500 {
		t.Errorf("TotalRealLossesM3: expected 3500, got %.1f", result.TotalRealLossesM3)
	}

	// NRW = 1500 + 3500 = 5000
	if result.NRWM3 != 5_000 {
		t.Errorf("NRWM3: expected 5000, got %.1f", result.NRWM3)
	}

	// NRW% = 5000/10000 * 100 = 50%
	if result.NRWPercent != 50.0 {
		t.Errorf("NRWPercent: expected 50.0, got %.1f", result.NRWPercent)
	}

	// IWA Grade D (NRW > 45%)
	if result.IWAGrade != "D" {
		t.Errorf("IWAGrade: expected D for 50%% NRW, got %s", result.IWAGrade)
	}
}

func TestComputeWaterBalance_WorldClassGrade(t *testing.T) {
	svc := wb.NewIWAWaterBalanceService(nil, nil)

	input := &wb.WaterBalanceInput{
		DistrictID:  uuid.New(),
		PeriodStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),

		SystemInputM3:       10_000,
		BilledMeteredM3:     9_300,
		BilledUnmeteredM3:   100,
		UnbilledMeteredM3:   50,
		UnbilledUnmeteredM3: 0,

		// Very low losses — real losses total 300 m³ (3% of system input)
		// With 0.04 proxy: UARL = 400 m³, ILI = 300/400 = 0.75 → Grade A
		UnauthorisedConsumptionM3: 100,
		MeteringInaccuraciesM3:    50,
		DataHandlingErrorsM3:      50,
		MainLeakageM3:             150,
		StorageOverflowM3:         80,
		ServiceConnectionLeakM3:   70,
	}

	result := svc.ComputeForTest(input)

	// NRW = (200 apparent + 300 real) / 10000 = 5%
	if result.NRWPercent > 10 {
		t.Errorf("Expected NRW < 10%%, got %.1f%%", result.NRWPercent)
	}

	// Real losses = 300 m³; UARL proxy = 10000 × 0.04 = 400 m³ → ILI = 0.75
	// ILI < 1.0 with NRW < 20% → Grade A
	// This verifies the ILI formula uses the correct 0.04 proxy (IWA M36).
	if result.IWAGrade != "A" {
		t.Errorf("Expected Grade A (ILI≈0.75, NRW≈5%%); got grade=%s ILI=%.2f NRW=%.1f%%",
			result.IWAGrade, result.ILI, result.NRWPercent)
	}
	// Verify ILI is computed correctly: 300 / (10000 × 0.04) = 0.75 ± tolerance
	if result.ILI < 0.5 || result.ILI > 1.0 {
		t.Errorf("ILI out of expected range [0.5, 1.0] for world-class scenario: got %.3f", result.ILI)
	}
}

func TestComputeWaterBalance_ZeroSystemInput(t *testing.T) {
	svc := wb.NewIWAWaterBalanceService(nil, nil)

	input := &wb.WaterBalanceInput{
		DistrictID:    uuid.New(),
		SystemInputM3: 0, // No production data
	}

	result := svc.ComputeForTest(input)

	// NRW% should be 0 (not NaN or infinity)
	if result.NRWPercent != 0 {
		t.Errorf("Expected NRWPercent=0 when SystemInput=0, got %.1f", result.NRWPercent)
	}

	// Data confidence should be low
	if result.DataConfidenceScore > 60 {
		t.Errorf("Expected low confidence score when no system input, got %.1f", result.DataConfidenceScore)
	}
}

func TestIWAGradeClassification(t *testing.T) {
	tests := []struct {
		ili        float64
		nrwPct     float64
		wantGrade  string
	}{
		{0.5, 15, "A"},  // Excellent
		{1.5, 25, "B"},  // Good
		{3.0, 40, "C"},  // Poor
		{5.0, 55, "D"},  // Very poor
		{0.8, 35, "C"},  // ILI good but NRW high → C
	}

	svc := wb.NewIWAWaterBalanceService(nil, nil)
	for _, tt := range tests {
		got := svc.ClassifyGradeForTest(tt.ili, tt.nrwPct)
		if got != tt.wantGrade {
			t.Errorf("ClassifyGrade(ILI=%.1f, NRW=%.1f%%): expected %s, got %s",
				tt.ili, tt.nrwPct, tt.wantGrade, got)
		}
	}
}

func TestRevenueRecoveryEstimate(t *testing.T) {
	svc := wb.NewIWAWaterBalanceService(nil, nil)

	input := &wb.WaterBalanceInput{
		DistrictID:                uuid.New(),
		SystemInputM3:             10_000,
		BilledMeteredM3:           8_000,
		UnauthorisedConsumptionM3: 500, // 500 m³ of theft
		MeteringInaccuraciesM3:    200,
		DataHandlingErrorsM3:      100,
		MainLeakageM3:             1_200,
	}

	result := svc.ComputeForTest(input)

	// Apparent losses = 800 m³
	// Recovery = 800 × 10.8320 × 1.20 = 10,398.72 GHS
	// Rate: PURC 2026 residential tier-2 fallback (10.8320), VAT 20%
	expectedRecovery := 800.0 * 10.8320 * 1.20
	tolerance := 1.0 // GHS

	if result.EstimatedRevenueRecoveryGHS < expectedRecovery-tolerance ||
		result.EstimatedRevenueRecoveryGHS > expectedRecovery+tolerance {
		t.Errorf("EstimatedRevenueRecoveryGHS: expected ~%.2f, got %.2f",
			expectedRecovery, result.EstimatedRevenueRecoveryGHS)
	}
}
