package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// TariffRateRepository loads PURC tariff rates from the database.
// Sentinel uses these for leakage GHS calculations instead of hardcoded values.
type TariffRateRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewTariffRateRepository(db *pgxpool.Pool, logger *zap.Logger) *TariffRateRepository {
	return &TariffRateRepository{db: db, logger: logger}
}

// LoadActiveTariffConfig loads all active tariff rates and the current VAT rate.
// Returns a TariffConfig that can be passed to sentinel services.
// Falls back to PURC 2026 defaults if the DB is unavailable.
func (r *TariffRateRepository) LoadActiveTariffConfig(ctx context.Context) (*entities.TariffConfig, error) {
	cfg := &entities.TariffConfig{
		LoadedAt: time.Now().UTC(),
	}

	// Load VAT rate
	err := r.db.QueryRow(ctx, `
		SELECT rate_percentage
		FROM vat_config
		WHERE is_active = TRUE
		ORDER BY effective_from DESC
		LIMIT 1`,
	).Scan(&cfg.VATRate)
	if err != nil {
		r.logger.Warn("Failed to load VAT rate from DB, using PURC 2026 default 20%",
			zap.Error(err))
		cfg.VATRate = 20.0
	}

	// Load all active tariff rates
	rows, err := r.db.Query(ctx, `
		SELECT category, tier_name,
		       COALESCE(min_volume_m3, 0),
		       COALESCE(max_volume_m3, 0),
		       rate_per_m3,
		       COALESCE(service_charge_ghs, 0)
		FROM tariff_rates
		WHERE is_active = TRUE
		ORDER BY category, min_volume_m3 ASC`)
	if err != nil {
		return nil, fmt.Errorf("LoadActiveTariffConfig: query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tr entities.TariffRate
		if err := rows.Scan(
			&tr.Category, &tr.TierName,
			&tr.MinVolumeM3, &tr.MaxVolumeM3,
			&tr.RatePerM3, &tr.ServiceCharge,
		); err != nil {
			r.logger.Warn("Failed to scan tariff rate row", zap.Error(err))
			continue
		}
		cfg.Rates = append(cfg.Rates, tr)
	}

	if len(cfg.Rates) == 0 {
		r.logger.Warn("No active tariff rates found in DB — using PURC 2026 hardcoded defaults")
		cfg.Rates = purc2026Defaults()
	}

	r.logger.Info("Tariff config loaded",
		zap.Int("rate_count", len(cfg.Rates)),
		zap.Float64("vat_rate_pct", cfg.VATRate),
	)
	return cfg, nil
}

// purc2026Defaults returns the PURC 2026 tariff rates as a fallback.
// These match the seed data in database/seeds/002_tariff_rates.sql.
// They are ONLY used when the database is unreachable at startup.
func purc2026Defaults() []entities.TariffRate {
	return []entities.TariffRate{
		{Category: "RESIDENTIAL",   TierName: "Tier 1 - Lifeline",    MinVolumeM3: 0,  MaxVolumeM3: 5,    RatePerM3: 6.1225,  ServiceCharge: 0},
		{Category: "RESIDENTIAL",   TierName: "Tier 2 - Standard",    MinVolumeM3: 5,  MaxVolumeM3: 0,    RatePerM3: 10.8320, ServiceCharge: 0},
		{Category: "COMMERCIAL",    TierName: "Standard Commercial",   MinVolumeM3: 0,  MaxVolumeM3: 0,    RatePerM3: 18.4500, ServiceCharge: 500},
		{Category: "INDUSTRIAL",    TierName: "Industrial Rate",       MinVolumeM3: 0,  MaxVolumeM3: 0,    RatePerM3: 22.1000, ServiceCharge: 1500},
		{Category: "PUBLIC_GOVT",   TierName: "Flat Rate",             MinVolumeM3: 0,  MaxVolumeM3: 0,    RatePerM3: 15.7372, ServiceCharge: 2000},
		{Category: "BOTTLED_WATER", TierName: "Bottled Water Rate",    MinVolumeM3: 0,  MaxVolumeM3: 0,    RatePerM3: 32.7858, ServiceCharge: 25000},
	}
}
