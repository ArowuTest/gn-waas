package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/repository/interfaces"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/service"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ---- Mock Repositories ----

type mockTariffRateRepo struct{}

func (m *mockTariffRateRepo) GetActiveRatesForCategory(ctx context.Context, category string, asOf time.Time) ([]*entities.TariffRate, error) {
	rates := map[string][]*entities.TariffRate{
		"RESIDENTIAL": {
			{ID: uuid.New(), Category: "RESIDENTIAL", TierName: "Tier 1", MinVolumeM3: 0, MaxVolumeM3: floatPtr(5), RatePerM3: 6.1225, ServiceChargeGHS: 0, IsActive: true},
			{ID: uuid.New(), Category: "RESIDENTIAL", TierName: "Tier 2", MinVolumeM3: 5, MaxVolumeM3: nil, RatePerM3: 10.8320, ServiceChargeGHS: 0, IsActive: true},
		},
		"COMMERCIAL": {
			{ID: uuid.New(), Category: "COMMERCIAL", TierName: "Commercial", MinVolumeM3: 0, MaxVolumeM3: nil, RatePerM3: 15.50, ServiceChargeGHS: 120.00, IsActive: true},
		},
	}
	if r, ok := rates[category]; ok {
		return r, nil
	}
	return []*entities.TariffRate{}, nil
}

func (m *mockTariffRateRepo) GetByID(ctx context.Context, id uuid.UUID) (*entities.TariffRate, error) {
	return nil, nil
}

func (m *mockTariffRateRepo) GetAll(ctx context.Context) ([]*entities.TariffRate, error) {
	return nil, nil
}

func (m *mockTariffRateRepo) Create(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error) {
	return rate, nil
}

func (m *mockTariffRateRepo) Update(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error) {
	return rate, nil
}

func (m *mockTariffRateRepo) Deactivate(ctx context.Context, id uuid.UUID, effectiveTo time.Time) error {
	return nil
}

type mockVATRepo struct{}

func (m *mockVATRepo) GetActiveConfig(ctx context.Context, asOf time.Time) (*entities.VATConfig, error) {
	return &entities.VATConfig{
		ID:             uuid.New(),
		RatePercentage: 20.0,
		IsActive:       true,
	}, nil
}

func (m *mockVATRepo) GetAll(ctx context.Context) ([]*entities.VATConfig, error) {
	return nil, nil
}

func (m *mockVATRepo) Create(ctx context.Context, config *entities.VATConfig) (*entities.VATConfig, error) {
	return config, nil
}

type mockShadowBillRepo struct{}

func (m *mockShadowBillRepo) Create(ctx context.Context, bill *entities.ShadowBillCalculation) error {
	return nil
}

func (m *mockShadowBillRepo) GetByGWLBillID(ctx context.Context, gwlBillID uuid.UUID) (*entities.ShadowBillCalculation, error) {
	return nil, nil
}

func (m *mockShadowBillRepo) GetFlaggedBills(ctx context.Context, districtID uuid.UUID, from, to time.Time) ([]*entities.ShadowBillCalculation, error) {
	return nil, nil
}

func (m *mockShadowBillRepo) GetVarianceSummary(ctx context.Context, districtID uuid.UUID, from, to time.Time) (*interfaces.VarianceSummary, error) {
	return nil, nil
}

// mockSystemConfigRepo returns the default threshold (15%) for all tests
type mockSystemConfigRepo struct{}

func (m *mockSystemConfigRepo) GetFloat64(_ context.Context, _ string, defaultVal float64) (float64, error) {
	return defaultVal, nil
}
func (m *mockSystemConfigRepo) GetString(_ context.Context, _ string, defaultVal string) (string, error) {
	return defaultVal, nil
}

func floatPtr(f float64) *float64 { return &f }

func newTestService() *service.TariffService {
	logger, _ := zap.NewDevelopment()
	return service.NewTariffService(&mockTariffRateRepo{}, &mockVATRepo{}, &mockShadowBillRepo{}, &mockSystemConfigRepo{}, logger)
}

// ---- Tests ----

func TestCalculateShadowBill_ResidentialTier1Only(t *testing.T) {
	svc := newTestService()
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "RESIDENTIAL",
		ConsumptionM3: 3.0,
		GWLTotalGHS:   20.0,
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 × 6.1225 = 18.3675
	expectedBase := 3.0 * 6.1225
	if absF(result.SubtotalGHS-expectedBase) > 0.01 {
		t.Errorf("SubtotalGHS: got %.4f, want %.4f", result.SubtotalGHS, expectedBase)
	}

	// VAT = 18.3675 × 0.20 = 3.6735
	expectedVAT := expectedBase * 0.20
	if absF(result.VATAmountGHS-expectedVAT) > 0.01 {
		t.Errorf("VATAmountGHS: got %.4f, want %.4f", result.VATAmountGHS, expectedVAT)
	}

	// Total = base + VAT = 22.041
	expectedTotal := expectedBase + expectedVAT
	if absF(result.TotalShadowBillGHS-expectedTotal) > 0.01 {
		t.Errorf("TotalShadowBillGHS: got %.4f, want %.4f", result.TotalShadowBillGHS, expectedTotal)
	}
}

func TestCalculateShadowBill_ResidentialCrossTier(t *testing.T) {
	svc := newTestService()
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "RESIDENTIAL",
		ConsumptionM3: 8.0,
		GWLTotalGHS:   50.0,
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5 × 6.1225 + 3 × 10.8320 = 30.6125 + 32.496 = 63.1085
	expectedBase := 5.0*6.1225 + 3.0*10.8320
	if absF(result.SubtotalGHS-expectedBase) > 0.01 {
		t.Errorf("SubtotalGHS: got %.4f, want %.4f", result.SubtotalGHS, expectedBase)
	}
}

func TestCalculateShadowBill_CommercialWithFixedCharge(t *testing.T) {
	svc := newTestService()
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "COMMERCIAL",
		ConsumptionM3: 10.0,
		GWLTotalGHS:   200.0,
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 120 (fixed) + 10 × 15.50 = 275
	expectedBase := 120.0 + 10.0*15.50
	if absF(result.SubtotalGHS-expectedBase) > 0.01 {
		t.Errorf("SubtotalGHS: got %.4f, want %.4f", result.SubtotalGHS, expectedBase)
	}
}

func TestCalculateShadowBill_ZeroConsumption(t *testing.T) {
	svc := newTestService()
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "RESIDENTIAL",
		ConsumptionM3: 0.0,
		GWLTotalGHS:   0.0,
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SubtotalGHS != 0 {
		t.Errorf("expected 0 subtotal for zero consumption, got %.4f", result.SubtotalGHS)
	}
}

func TestVarianceFlagging_Above15Pct(t *testing.T) {
	svc := newTestService()
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "RESIDENTIAL",
		ConsumptionM3: 8.0,
		GWLTotalGHS:   40.0, // Under-billed — shadow will be ~75.73
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsFlagged {
		t.Errorf("expected IsFlagged=true when GWL bill is significantly under shadow bill (variance=%.2f%%)", result.VariancePct)
	}
}

func TestVATCalculation_20Percent(t *testing.T) {
	base := 100.0
	vatRate := 20.0 / 100.0
	vat := base * vatRate
	total := base + vat

	if absF(vat-20.0) > 0.001 {
		t.Errorf("VAT: got %.4f, want 20.0000", vat)
	}
	if absF(total-120.0) > 0.001 {
		t.Errorf("Total: got %.4f, want 120.0000", total)
	}
}

func TestTierBreakdown_Tier1Boundary(t *testing.T) {
	svc := newTestService()
	// Exactly 5 m³ — all in Tier 1
	req := &entities.TariffCalculationRequest{
		AccountID:     uuid.New(),
		Category:      "RESIDENTIAL",
		ConsumptionM3: 5.0,
		GWLTotalGHS:   30.0,
		BillingDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	result, err := svc.CalculateShadowBill(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5 × 6.1225 = 30.6125
	expectedBase := 5.0 * 6.1225
	if absF(result.SubtotalGHS-expectedBase) > 0.01 {
		t.Errorf("SubtotalGHS at tier boundary: got %.4f, want %.4f", result.SubtotalGHS, expectedBase)
	}

	// Tier 2 should be 0
	if result.Tier2VolumeM3 != 0 {
		t.Errorf("Tier2VolumeM3 should be 0 at boundary, got %.4f", result.Tier2VolumeM3)
	}
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
