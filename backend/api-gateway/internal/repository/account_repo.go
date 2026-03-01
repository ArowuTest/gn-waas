package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AccountRepository handles water account data access
type AccountRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAccountRepository(db *pgxpool.Pool, logger *zap.Logger) *AccountRepository {
	return &AccountRepository{db: db, logger: logger}
}

// Search performs a full-text search across account number, holder name, address, and meter number.
// Supports optional district filter and pagination.
func (r *AccountRepository) Search(ctx context.Context, query string, districtID *uuid.UUID, limit, offset int) ([]*domain.WaterAccount, int, error) {
	args := []interface{}{}
	conditions := []string{}
	argIdx := 1

	if query != "" {
		// Use ILIKE for case-insensitive partial match across key fields
		conditions = append(conditions, fmt.Sprintf(
			"(gwl_account_number ILIKE $%d OR account_holder_name ILIKE $%d OR address_line1 ILIKE $%d OR meter_number ILIKE $%d)",
			argIdx, argIdx, argIdx, argIdx,
		))
		args = append(args, "%"+query+"%")
		argIdx++
	}

	if districtID != nil {
		conditions = append(conditions, fmt.Sprintf("district_id = $%d", argIdx))
		args = append(args, *districtID)
		argIdx++
	}

	where := "1=1"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	// Count total
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM water_accounts WHERE %s", where), countArgs...).Scan(&total)

	// Fetch page
	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, gwl_account_number, account_holder_name, account_holder_tin,
		       category, status, district_id, meter_number, address_line1,
		       gps_latitude, gps_longitude, is_within_network,
		       monthly_avg_consumption, is_phantom_flagged, created_at, updated_at
		FROM water_accounts
		WHERE %s
		ORDER BY account_holder_name ASC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("account search failed: %w", err)
	}
	defer rows.Close()

	var accounts []*domain.WaterAccount
	for rows.Next() {
		a := &domain.WaterAccount{}
		err := rows.Scan(
			&a.ID, &a.GWLAccountNumber, &a.AccountHolderName, &a.AccountHolderTIN,
			&a.Category, &a.Status, &a.DistrictID, &a.MeterNumber, &a.AddressLine1,
			&a.GPSLatitude, &a.GPSLongitude, &a.IsWithinNetwork,
			&a.MonthlyAvgConsumption, &a.IsPhantomFlagged, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, a)
	}

	return accounts, total, rows.Err()
}

// GetByID returns a single water account by UUID
func (r *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.WaterAccount, error) {
	a := &domain.WaterAccount{}
	err := r.db.QueryRow(ctx, `
		SELECT id, gwl_account_number, account_holder_name, account_holder_tin,
		       category, status, district_id, meter_number, address_line1,
		       gps_latitude, gps_longitude, is_within_network,
		       monthly_avg_consumption, is_phantom_flagged, created_at, updated_at
		FROM water_accounts WHERE id = $1`, id,
	).Scan(
		&a.ID, &a.GWLAccountNumber, &a.AccountHolderName, &a.AccountHolderTIN,
		&a.Category, &a.Status, &a.DistrictID, &a.MeterNumber, &a.AddressLine1,
		&a.GPSLatitude, &a.GPSLongitude, &a.IsWithinNetwork,
		&a.MonthlyAvgConsumption, &a.IsPhantomFlagged, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("account %s not found", id)
		}
		return nil, fmt.Errorf("GetByID account failed: %w", err)
	}
	return a, nil
}

// GetByGWLNumber returns a water account by its GWL account number
func (r *AccountRepository) GetByGWLNumber(ctx context.Context, gwlNumber string) (*domain.WaterAccount, error) {
	a := &domain.WaterAccount{}
	err := r.db.QueryRow(ctx, `
		SELECT id, gwl_account_number, account_holder_name, account_holder_tin,
		       category, status, district_id, meter_number, address_line1,
		       gps_latitude, gps_longitude, is_within_network,
		       monthly_avg_consumption, is_phantom_flagged, created_at, updated_at
		FROM water_accounts WHERE gwl_account_number = $1`, gwlNumber,
	).Scan(
		&a.ID, &a.GWLAccountNumber, &a.AccountHolderName, &a.AccountHolderTIN,
		&a.Category, &a.Status, &a.DistrictID, &a.MeterNumber, &a.AddressLine1,
		&a.GPSLatitude, &a.GPSLongitude, &a.IsWithinNetwork,
		&a.MonthlyAvgConsumption, &a.IsPhantomFlagged, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("account %s not found", gwlNumber)
		}
		return nil, fmt.Errorf("GetByGWLNumber failed: %w", err)
	}
	return a, nil
}

// GetByDistrict returns all accounts for a district with pagination
func (r *AccountRepository) GetByDistrict(ctx context.Context, districtID uuid.UUID, limit, offset int) ([]*domain.WaterAccount, int, error) {
	var total int
	r.db.QueryRow(ctx, "SELECT COUNT(*) FROM water_accounts WHERE district_id = $1", districtID).Scan(&total)

	rows, err := r.db.Query(ctx, `
		SELECT id, gwl_account_number, account_holder_name, account_holder_tin,
		       category, status, district_id, meter_number, address_line1,
		       gps_latitude, gps_longitude, is_within_network,
		       monthly_avg_consumption, is_phantom_flagged, created_at, updated_at
		FROM water_accounts
		WHERE district_id = $1
		ORDER BY account_holder_name ASC
		LIMIT $2 OFFSET $3`, districtID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var accounts []*domain.WaterAccount
	for rows.Next() {
		a := &domain.WaterAccount{}
		err := rows.Scan(
			&a.ID, &a.GWLAccountNumber, &a.AccountHolderName, &a.AccountHolderTIN,
			&a.Category, &a.Status, &a.DistrictID, &a.MeterNumber, &a.AddressLine1,
			&a.GPSLatitude, &a.GPSLongitude, &a.IsWithinNetwork,
			&a.MonthlyAvgConsumption, &a.IsPhantomFlagged, &a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, a)
	}
	return accounts, total, rows.Err()
}
