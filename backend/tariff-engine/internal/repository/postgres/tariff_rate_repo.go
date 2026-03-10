package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/domain/entities"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TariffRateRepository implements interfaces.TariffRateRepository using PostgreSQL
type TariffRateRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewTariffRateRepository(db *pgxpool.Pool, logger *zap.Logger) *TariffRateRepository {
	return &TariffRateRepository{db: db, logger: logger}
}

// GetActiveRatesForCategory returns all active tariff tiers for a category at a given date
// Ordered by min_volume_m3 ASC to ensure correct tier application
func (r *TariffRateRepository) GetActiveRatesForCategory(
	ctx context.Context,
	category string,
	asOf time.Time,
) ([]*entities.TariffRate, error) {

	query := `
		SELECT
			id, category, tier_name,
			min_volume_m3, max_volume_m3,
			rate_per_m3, service_charge_ghs,
			effective_from, effective_to,
			COALESCE(approved_by, ''), COALESCE(regulatory_ref, ''), is_active,
			created_at, updated_at
		FROM tariff_rates
		WHERE category = $1
		  AND is_active = TRUE
		  AND effective_from <= $2
		  AND (effective_to IS NULL OR effective_to > $2)
		ORDER BY min_volume_m3 ASC`

	rows, err := r.db.Query(ctx, query, category, asOf)
	if err != nil {
		return nil, fmt.Errorf("GetActiveRatesForCategory query failed: %w", err)
	}
	defer rows.Close()

	return scanTariffRates(rows)
}

// GetByID returns a specific tariff rate by ID
func (r *TariffRateRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.TariffRate, error) {
	query := `
		SELECT
			id, category, tier_name,
			min_volume_m3, max_volume_m3,
			rate_per_m3, service_charge_ghs,
			effective_from, effective_to,
			COALESCE(approved_by, ''), COALESCE(regulatory_ref, ''), is_active,
			created_at, updated_at
		FROM tariff_rates
		WHERE id = $1`

	row := r.db.QueryRow(ctx, query, id)
	rate, err := scanTariffRate(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("tariff rate %s not found", id)
		}
		return nil, fmt.Errorf("GetByID failed: %w", err)
	}
	return rate, nil
}

// GetAll returns all tariff rates for admin management
func (r *TariffRateRepository) GetAll(ctx context.Context) ([]*entities.TariffRate, error) {
	query := `
		SELECT
			id, category, tier_name,
			min_volume_m3, max_volume_m3,
			rate_per_m3, service_charge_ghs,
			effective_from, effective_to,
			COALESCE(approved_by, ''), COALESCE(regulatory_ref, ''), is_active,
			created_at, updated_at
		FROM tariff_rates
		ORDER BY category, min_volume_m3 ASC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("GetAll tariff rates failed: %w", err)
	}
	defer rows.Close()

	return scanTariffRates(rows)
}

// Create creates a new tariff rate
func (r *TariffRateRepository) Create(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error) {
	query := `
		INSERT INTO tariff_rates (
			category, tier_name, min_volume_m3, max_volume_m3,
			rate_per_m3, service_charge_ghs, effective_from, effective_to,
			COALESCE(approved_by, ''), COALESCE(regulatory_ref, ''), is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, query,
		rate.Category, rate.TierName, rate.MinVolumeM3, rate.MaxVolumeM3,
		rate.RatePerM3, rate.ServiceChargeGHS, rate.EffectiveFrom, rate.EffectiveTo,
		rate.ApprovedBy, rate.RegulatoryRef, rate.IsActive,
	).Scan(&rate.ID, &rate.CreatedAt, &rate.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("Create tariff rate failed: %w", err)
	}

	r.logger.Info("Tariff rate created",
		zap.String("id", rate.ID.String()),
		zap.String("category", rate.Category),
		zap.Float64("rate_per_m3", rate.RatePerM3),
	)

	return rate, nil
}

// Update updates a tariff rate (deactivates old, creates new version)
func (r *TariffRateRepository) Update(ctx context.Context, rate *entities.TariffRate) (*entities.TariffRate, error) {
	query := `
		UPDATE tariff_rates
		SET category = $1, tier_name = $2, min_volume_m3 = $3, max_volume_m3 = $4,
		    rate_per_m3 = $5, service_charge_ghs = $6, effective_from = $7,
		    effective_to = $8, approved_by = $9, regulatory_ref = $10,
		    is_active = $11, updated_at = NOW()
		WHERE id = $12
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, query,
		rate.Category, rate.TierName, rate.MinVolumeM3, rate.MaxVolumeM3,
		rate.RatePerM3, rate.ServiceChargeGHS, rate.EffectiveFrom, rate.EffectiveTo,
		rate.ApprovedBy, rate.RegulatoryRef, rate.IsActive, rate.ID,
	).Scan(&rate.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("Update tariff rate failed: %w", err)
	}

	return rate, nil
}

// Deactivate deactivates a tariff rate by setting effective_to
func (r *TariffRateRepository) Deactivate(ctx context.Context, id uuid.UUID, effectiveTo time.Time) error {
	query := `
		UPDATE tariff_rates
		SET is_active = FALSE, effective_to = $1, updated_at = NOW()
		WHERE id = $2`

	_, err := r.db.Exec(ctx, query, effectiveTo, id)
	if err != nil {
		return fmt.Errorf("Deactivate tariff rate failed: %w", err)
	}
	return nil
}

// scanTariffRate scans a single row into a TariffRate entity
func scanTariffRate(row pgx.Row) (*entities.TariffRate, error) {
	rate := &entities.TariffRate{}
	err := row.Scan(
		&rate.ID, &rate.Category, &rate.TierName,
		&rate.MinVolumeM3, &rate.MaxVolumeM3,
		&rate.RatePerM3, &rate.ServiceChargeGHS,
		&rate.EffectiveFrom, &rate.EffectiveTo,
		&rate.ApprovedBy, &rate.RegulatoryRef, &rate.IsActive,
		&rate.CreatedAt, &rate.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rate, nil
}

// scanTariffRates scans multiple rows into TariffRate entities
func scanTariffRates(rows pgx.Rows) ([]*entities.TariffRate, error) {
	var rates []*entities.TariffRate
	for rows.Next() {
		rate := &entities.TariffRate{}
		err := rows.Scan(
			&rate.ID, &rate.Category, &rate.TierName,
			&rate.MinVolumeM3, &rate.MaxVolumeM3,
			&rate.RatePerM3, &rate.ServiceChargeGHS,
			&rate.EffectiveFrom, &rate.EffectiveTo,
			&rate.ApprovedBy, &rate.RegulatoryRef, &rate.IsActive,
			&rate.CreatedAt, &rate.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan tariff rate failed: %w", err)
		}
		rates = append(rates, rate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration failed: %w", err)
	}
	return rates, nil
}
