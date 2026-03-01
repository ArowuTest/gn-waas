package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// GWLBillingRepository implements interfaces.GWLBillingRepository
type GWLBillingRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewGWLBillingRepository(db *pgxpool.Pool, logger *zap.Logger) *GWLBillingRepository {
	return &GWLBillingRepository{db: db, logger: logger}
}

// GetUnprocessedBills returns bills that haven't been shadow-billed yet
func (r *GWLBillingRepository) GetUnprocessedBills(ctx context.Context, districtID uuid.UUID, limit int) ([]*entities.GWLBillingRecord, error) {
	query := `
		SELECT gbr.id, gbr.gwl_bill_id, gbr.account_id,
		       gbr.billing_period_start, gbr.billing_period_end,
		       gbr.consumption_m3, gbr.gwl_category,
		       gbr.gwl_amount_ghs, gbr.gwl_vat_ghs, gbr.gwl_total_ghs
		FROM gwl_billing_records gbr
		JOIN water_accounts wa ON gbr.account_id = wa.id
		LEFT JOIN shadow_bills sb ON gbr.id = sb.gwl_bill_id
		WHERE wa.district_id = $1
		  AND sb.id IS NULL
		ORDER BY gbr.billing_period_start DESC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, districtID, limit)
	if err != nil {
		return nil, fmt.Errorf("GetUnprocessedBills failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetBillingHistory returns billing history for an account
func (r *GWLBillingRepository) GetBillingHistory(ctx context.Context, accountID uuid.UUID, months int) ([]*entities.GWLBillingRecord, error) {
	query := `
		SELECT id, gwl_bill_id, account_id,
		       billing_period_start, billing_period_end,
		       consumption_m3, gwl_category,
		       gwl_amount_ghs, gwl_vat_ghs, gwl_total_ghs
		FROM gwl_billing_records
		WHERE account_id = $1
		  AND billing_period_start >= NOW() - ($2 || ' months')::INTERVAL
		ORDER BY billing_period_start DESC`

	rows, err := r.db.Query(ctx, query, accountID, months)
	if err != nil {
		return nil, fmt.Errorf("GetBillingHistory failed: %w", err)
	}
	defer rows.Close()

	return r.scanRows(rows)
}

// GetDistrictBillingTotal returns total billed volume for a district/period
func (r *GWLBillingRepository) GetDistrictBillingTotal(ctx context.Context, districtID uuid.UUID, from, to time.Time) (float64, error) {
	var total float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(gbr.consumption_m3), 0)
		FROM gwl_billing_records gbr
		JOIN water_accounts wa ON gbr.account_id = wa.id
		WHERE wa.district_id = $1
		  AND gbr.billing_period_start >= $2
		  AND gbr.billing_period_end <= $3`,
		districtID, from, to,
	).Scan(&total)
	return total, err
}

// GetByID returns a billing record by ID
func (r *GWLBillingRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.GWLBillingRecord, error) {
	query := `
		SELECT id, gwl_bill_id, account_id,
		       billing_period_start, billing_period_end,
		       consumption_m3, gwl_category,
		       gwl_amount_ghs, gwl_vat_ghs, gwl_total_ghs
		FROM gwl_billing_records WHERE id = $1`

	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records, err := r.scanRows(rows)
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("billing record %s not found", id)
	}
	return records[0], nil
}

func (r *GWLBillingRepository) scanRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*entities.GWLBillingRecord, error) {
	var records []*entities.GWLBillingRecord
	for rows.Next() {
		rec := &entities.GWLBillingRecord{}
		err := rows.Scan(
			&rec.ID, &rec.GWLBillID, &rec.AccountID,
			&rec.BillingPeriodStart, &rec.BillingPeriodEnd,
			&rec.ConsumptionM3, &rec.GWLCategory,
			&rec.GWLAmountGHS, &rec.GWLVatGHS, &rec.GWLTotalGHS,
		)
		if err != nil {
			return nil, fmt.Errorf("scan billing record: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// WaterAccountRepository implements interfaces.WaterAccountRepository
type WaterAccountRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewWaterAccountRepository(db *pgxpool.Pool, logger *zap.Logger) *WaterAccountRepository {
	return &WaterAccountRepository{db: db, logger: logger}
}

func (r *WaterAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.WaterAccount, error) {
	query := `
		SELECT id, gwl_account_number, district_id, category,
		       gps_latitude, gps_longitude, is_within_network,
		       network_check_date, monthly_avg_consumption, is_phantom_flagged
		FROM water_accounts WHERE id = $1`

	acc := &entities.WaterAccount{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&acc.ID, &acc.GWLAccountNumber, &acc.DistrictID, &acc.Category,
		&acc.GPSLatitude, &acc.GPSLongitude, &acc.IsWithinNetwork,
		&acc.NetworkCheckDate, &acc.MonthlyAvgConsumption, &acc.IsPhantomFlagged,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByID account failed: %w", err)
	}
	return acc, nil
}

func (r *WaterAccountRepository) GetByDistrict(ctx context.Context, districtID uuid.UUID, limit, offset int) ([]*entities.WaterAccount, int, error) {
	var total int
	r.db.QueryRow(ctx, "SELECT COUNT(*) FROM water_accounts WHERE district_id = $1 AND status != 'INACTIVE'", districtID).Scan(&total)

	rows, err := r.db.Query(ctx, `
		SELECT id, gwl_account_number, district_id, category,
		       gps_latitude, gps_longitude, is_within_network,
		       network_check_date, monthly_avg_consumption, is_phantom_flagged
		FROM water_accounts
		WHERE district_id = $1 AND status != 'INACTIVE'
		ORDER BY gwl_account_number
		LIMIT $2 OFFSET $3`, districtID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	accounts, err := r.scanAccountRows(rows)
	return accounts, total, err
}

func (r *WaterAccountRepository) GetOutsideNetwork(ctx context.Context, districtID uuid.UUID) ([]*entities.WaterAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, gwl_account_number, district_id, category,
		       gps_latitude, gps_longitude, is_within_network,
		       network_check_date, monthly_avg_consumption, is_phantom_flagged
		FROM water_accounts
		WHERE district_id = $1 AND is_within_network = FALSE`, districtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAccountRows(rows)
}

func (r *WaterAccountRepository) GetHighConsumptionResidential(ctx context.Context, districtID uuid.UUID, thresholdM3 float64) ([]*entities.WaterAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, gwl_account_number, district_id, category,
		       gps_latitude, gps_longitude, is_within_network,
		       network_check_date, monthly_avg_consumption, is_phantom_flagged
		FROM water_accounts
		WHERE district_id = $1
		  AND category = 'RESIDENTIAL'
		  AND monthly_avg_consumption >= $2
		ORDER BY monthly_avg_consumption DESC`, districtID, thresholdM3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAccountRows(rows)
}

func (r *WaterAccountRepository) UpdatePhantomFlag(ctx context.Context, id uuid.UUID, flagged bool, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE water_accounts
		SET is_phantom_flagged = $1, phantom_flag_reason = $2,
		    phantom_flag_date = CASE WHEN $1 THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $3`, flagged, reason, id)
	return err
}

func (r *WaterAccountRepository) GetAll(ctx context.Context, districtID uuid.UUID) ([]*entities.WaterAccount, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, gwl_account_number, district_id, category,
		       gps_latitude, gps_longitude, is_within_network,
		       network_check_date, monthly_avg_consumption, is_phantom_flagged
		FROM water_accounts
		WHERE district_id = $1 AND status != 'INACTIVE'
		ORDER BY gwl_account_number`, districtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanAccountRows(rows)
}

func (r *WaterAccountRepository) scanAccountRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*entities.WaterAccount, error) {
	var accounts []*entities.WaterAccount
	for rows.Next() {
		acc := &entities.WaterAccount{}
		err := rows.Scan(
			&acc.ID, &acc.GWLAccountNumber, &acc.DistrictID, &acc.Category,
			&acc.GPSLatitude, &acc.GPSLongitude, &acc.IsWithinNetwork,
			&acc.NetworkCheckDate, &acc.MonthlyAvgConsumption, &acc.IsPhantomFlagged,
		)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, acc)
	}
	return accounts, rows.Err()
}

// DistrictRepository implements interfaces.DistrictRepository
type DistrictRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewDistrictRepository(db *pgxpool.Pool, logger *zap.Logger) *DistrictRepository {
	return &DistrictRepository{db: db, logger: logger}
}

// DB returns the underlying database pool for direct queries
func (r *DistrictRepository) DB() *pgxpool.Pool { return r.db }

func (r *DistrictRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.District, error) {
	d := &entities.District{}
	err := r.db.QueryRow(ctx,
		"SELECT id, district_code, district_name, region FROM districts WHERE id = $1", id,
	).Scan(&d.ID, &d.DistrictCode, &d.DistrictName, &d.Region)
	if err != nil {
		return nil, fmt.Errorf("GetByID district failed: %w", err)
	}
	return d, nil
}

func (r *DistrictRepository) GetAll(ctx context.Context) ([]*entities.District, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, district_code, district_name, region FROM districts WHERE is_active = TRUE ORDER BY district_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanDistrictRows(rows)
}

func (r *DistrictRepository) GetPilotDistricts(ctx context.Context) ([]*entities.District, error) {
	rows, err := r.db.Query(ctx,
		"SELECT id, district_code, district_name, region FROM districts WHERE is_pilot_district = TRUE AND is_active = TRUE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanDistrictRows(rows)
}

func (r *DistrictRepository) GetProductionTotal(ctx context.Context, districtID uuid.UUID, from, to time.Time) (float64, error) {
	var total float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(volume_m3), 0)
		FROM production_records
		WHERE district_id = $1 AND recorded_at BETWEEN $2 AND $3`,
		districtID, from, to,
	).Scan(&total)
	return total, err
}

func (r *DistrictRepository) scanDistrictRows(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]*entities.District, error) {
	var districts []*entities.District
	for rows.Next() {
		d := &entities.District{}
		if err := rows.Scan(&d.ID, &d.DistrictCode, &d.DistrictName, &d.Region); err != nil {
			return nil, err
		}
		districts = append(districts, d)
	}
	return districts, rows.Err()
}
