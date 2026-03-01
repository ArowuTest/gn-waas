package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/domain/entities"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// VATConfigRepository implements interfaces.VATConfigRepository using PostgreSQL
type VATConfigRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewVATConfigRepository(db *pgxpool.Pool, logger *zap.Logger) *VATConfigRepository {
	return &VATConfigRepository{db: db, logger: logger}
}

// GetActiveConfig returns the active VAT configuration at a given date
func (r *VATConfigRepository) GetActiveConfig(
	ctx context.Context,
	asOf time.Time,
) (*entities.VATConfig, error) {

	query := `
		SELECT
			id, rate_percentage, components,
			effective_from, effective_to,
			regulatory_ref, is_active, created_at
		FROM vat_config
		WHERE is_active = TRUE
		  AND effective_from <= $1
		  AND (effective_to IS NULL OR effective_to > $1)
		ORDER BY effective_from DESC
		LIMIT 1`

	row := r.db.QueryRow(ctx, query, asOf)

	cfg := &entities.VATConfig{}
	var componentsJSON []byte

	err := row.Scan(
		&cfg.ID, &cfg.RatePercentage, &componentsJSON,
		&cfg.EffectiveFrom, &cfg.EffectiveTo,
		&cfg.RegulatoryRef, &cfg.IsActive, &cfg.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("no active VAT configuration found for date %s", asOf.Format("2006-01-02"))
		}
		return nil, fmt.Errorf("GetActiveConfig failed: %w", err)
	}

	if err := json.Unmarshal(componentsJSON, &cfg.Components); err != nil {
		return nil, fmt.Errorf("failed to parse VAT components: %w", err)
	}

	return cfg, nil
}

// GetAll returns all VAT configurations
func (r *VATConfigRepository) GetAll(ctx context.Context) ([]*entities.VATConfig, error) {
	query := `
		SELECT
			id, rate_percentage, components,
			effective_from, effective_to,
			regulatory_ref, is_active, created_at
		FROM vat_config
		ORDER BY effective_from DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("GetAll VAT configs failed: %w", err)
	}
	defer rows.Close()

	var configs []*entities.VATConfig
	for rows.Next() {
		cfg := &entities.VATConfig{}
		var componentsJSON []byte

		err := rows.Scan(
			&cfg.ID, &cfg.RatePercentage, &componentsJSON,
			&cfg.EffectiveFrom, &cfg.EffectiveTo,
			&cfg.RegulatoryRef, &cfg.IsActive, &cfg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan VAT config failed: %w", err)
		}

		if err := json.Unmarshal(componentsJSON, &cfg.Components); err != nil {
			return nil, fmt.Errorf("failed to parse VAT components: %w", err)
		}

		configs = append(configs, cfg)
	}

	return configs, rows.Err()
}

// Create creates a new VAT configuration
func (r *VATConfigRepository) Create(ctx context.Context, cfg *entities.VATConfig) (*entities.VATConfig, error) {
	componentsJSON, err := json.Marshal(cfg.Components)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VAT components: %w", err)
	}

	query := `
		INSERT INTO vat_config (
			rate_percentage, components, effective_from,
			effective_to, regulatory_ref, is_active
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	err = r.db.QueryRow(ctx, query,
		cfg.RatePercentage, componentsJSON, cfg.EffectiveFrom,
		cfg.EffectiveTo, cfg.RegulatoryRef, cfg.IsActive,
	).Scan(&cfg.ID, &cfg.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("Create VAT config failed: %w", err)
	}

	r.logger.Info("VAT config created",
		zap.String("id", cfg.ID.String()),
		zap.Float64("rate_pct", cfg.RatePercentage),
	)

	return cfg, nil
}
