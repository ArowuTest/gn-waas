package interfaces

import (
	"context"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AnomalyFlagRepository defines the contract for anomaly flag data access
type AnomalyFlagRepository interface {
	// Create persists a new anomaly flag (immutable - no updates)
	Create(ctx context.Context, flag *entities.AnomalyFlag) (*entities.AnomalyFlag, error)

	// GetByID returns an anomaly flag by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.AnomalyFlag, error)

	// GetByAccount returns all anomaly flags for an account
	GetByAccount(ctx context.Context, accountID uuid.UUID) ([]*entities.AnomalyFlag, error)

	// GetOpenByDistrict returns open anomaly flags for a district
	GetOpenByDistrict(ctx context.Context, districtID uuid.UUID, limit, offset int) ([]*entities.AnomalyFlag, int, error)

	// GetByCriteria returns anomaly flags matching filter criteria
	GetByCriteria(ctx context.Context, filter AnomalyFilter) ([]*entities.AnomalyFlag, int, error)

	// UpdateStatus updates the status of an anomaly flag
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID, notes string) error

	// MarkFalsePositive marks an anomaly as a false positive
	MarkFalsePositive(ctx context.Context, id uuid.UUID, resolvedBy uuid.UUID, reason string) error

	// GetSummaryByDistrict returns anomaly counts grouped by type for a district
	GetSummaryByDistrict(ctx context.Context, districtID uuid.UUID, from, to time.Time) (*AnomalySummary, error)

	// ExistsByDetectionHash checks if an anomaly with this hash already exists (deduplication)
	ExistsByDetectionHash(ctx context.Context, hash string) (bool, error)
}

// AnomalyFilter holds filter criteria for anomaly queries
type AnomalyFilter struct {
	DistrictID  *uuid.UUID
	AccountID   *uuid.UUID
	AnomalyType *string
	AlertLevel  *string
	Status      *string
	FraudType   *string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

// AnomalySummary holds aggregated anomaly statistics
type AnomalySummary struct {
	DistrictID          uuid.UUID              `json:"district_id"`
	TotalOpen           int                    `json:"total_open"`
	TotalCritical       int                    `json:"total_critical"`
	TotalHigh           int                    `json:"total_high"`
	TotalMedium         int                    `json:"total_medium"`
	TotalLow            int                    `json:"total_low"`
	TotalEstimatedLoss  float64                `json:"total_estimated_loss_ghs"`
	ByType              map[string]int         `json:"by_type"`
	ByFraudType         map[string]int         `json:"by_fraud_type"`
}

// GWLBillingRepository defines the contract for GWL billing data access
type GWLBillingRepository interface {
	// GetUnprocessedBills returns bills that haven't been shadow-billed yet
	GetUnprocessedBills(ctx context.Context, districtID uuid.UUID, limit int) ([]*entities.GWLBillingRecord, error)

	// GetBillingHistory returns billing history for an account
	GetBillingHistory(ctx context.Context, accountID uuid.UUID, months int) ([]*entities.GWLBillingRecord, error)

	// GetDistrictBillingTotal returns total billed volume for a district/period
	GetDistrictBillingTotal(ctx context.Context, districtID uuid.UUID, from, to time.Time) (float64, error)

	// GetByID returns a billing record by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.GWLBillingRecord, error)
}

// WaterAccountRepository defines the contract for water account data access
type WaterAccountRepository interface {
	// GetByID returns a water account by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.WaterAccount, error)

	// GetByDistrict returns all accounts in a district
	GetByDistrict(ctx context.Context, districtID uuid.UUID, limit, offset int) ([]*entities.WaterAccount, int, error)

	// GetOutsideNetwork returns accounts flagged as outside the pipe network
	GetOutsideNetwork(ctx context.Context, districtID uuid.UUID) ([]*entities.WaterAccount, error)

	// GetHighConsumptionResidential returns residential accounts with commercial-level consumption
	GetHighConsumptionResidential(ctx context.Context, districtID uuid.UUID, thresholdM3 float64) ([]*entities.WaterAccount, error)

	// UpdatePhantomFlag updates the phantom flag on an account
	UpdatePhantomFlag(ctx context.Context, id uuid.UUID, flagged bool, reason string) error

	// GetAll returns all accounts for batch processing
	GetAll(ctx context.Context, districtID uuid.UUID) ([]*entities.WaterAccount, error)
}

// DistrictRepository defines the contract for district data access
type DistrictRepository interface {
	// DB returns the underlying database pool for direct queries
	DB() *pgxpool.Pool

	// GetByID returns a district by ID
	GetByID(ctx context.Context, id uuid.UUID) (*entities.District, error)

	// GetAll returns all active districts
	GetAll(ctx context.Context) ([]*entities.District, error)

	// GetPilotDistricts returns districts marked as pilot
	GetPilotDistricts(ctx context.Context) ([]*entities.District, error)

	// GetProductionTotal returns total production volume for a district/period
	GetProductionTotal(ctx context.Context, districtID uuid.UUID, from, to time.Time) (float64, error)
}

// SupplyScheduleRepository defines the contract for supply schedule data access
type SupplyScheduleRepository interface {
	// GetActiveSchedule returns the active supply schedule for a district
	GetActiveSchedule(ctx context.Context, districtID uuid.UUID, asOf time.Time) (*entities.SupplySchedule, error)
}
