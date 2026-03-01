package interfaces

import (
	"context"
	"time"

	"github.com/ArowuTest/gn-waas/services/tariff-engine/internal/domain/entities"
	"github.com/google/uuid"
)

// TariffRateRepository defines the contract for tariff rate data access
type TariffRateRepository interface {
	// GetActiveRatesForCategory returns all active tariff tiers for a category at a given date
	GetActiveRatesForCategory(ctx context.Context, category string, asOf time.Time) ([]*entities.TariffRate, error)

	// GetByID returns a specific tariff rate by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.TariffRate, error)

	// GetAll returns all tariff rates (for admin management)
	GetAll(ctx context.Context) ([]*entities.TariffRate, error)

	// Create creates a new tariff rate (admin action)
	Create(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error)

	// Update updates a tariff rate (admin action - creates new version)
	Update(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error)

	// Deactivate deactivates a tariff rate
	Deactivate(ctx context.Context, id uuid.UUID, effectiveTo time.Time) error
}

// VATConfigRepository defines the contract for VAT configuration data access
type VATConfigRepository interface {
	// GetActiveConfig returns the active VAT configuration at a given date
	GetActiveConfig(ctx context.Context, asOf time.Time) (*entities.VATConfig, error)

	// GetAll returns all VAT configurations
	GetAll(ctx context.Context) ([]*entities.VATConfig, error)

	// Create creates a new VAT configuration
	Create(ctx context.Context, config *entities.VATConfig) (*entities.VATConfig, error)
}

// ShadowBillRepository defines the contract for shadow bill data access
type ShadowBillRepository interface {
	// Create persists a shadow bill calculation
	Create(ctx context.Context, bill *entities.ShadowBillCalculation) error

	// GetByGWLBillID returns the shadow bill for a GWL bill
	GetByGWLBillID(ctx context.Context, gwlBillID uuid.UUID) (*entities.ShadowBillCalculation, error)

	// GetFlaggedBills returns all flagged shadow bills for a district/period
	GetFlaggedBills(ctx context.Context, districtID uuid.UUID, from, to time.Time) ([]*entities.ShadowBillCalculation, error)

	// GetVarianceSummary returns variance statistics for a district
	GetVarianceSummary(ctx context.Context, districtID uuid.UUID, from, to time.Time) (*VarianceSummary, error)
}

// VarianceSummary is a reporting aggregate
type VarianceSummary struct {
	DistrictID        uuid.UUID `json:"district_id"`
	TotalBills        int       `json:"total_bills"`
	FlaggedBills      int       `json:"flagged_bills"`
	TotalVarianceGHS  float64   `json:"total_variance_ghs"`
	AvgVariancePct    float64   `json:"avg_variance_pct"`
	MaxVariancePct    float64   `json:"max_variance_pct"`
	EstimatedLossGHS  float64   `json:"estimated_loss_ghs"`
}
