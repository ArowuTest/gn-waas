package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SystemConfigRepository reads runtime configuration from the system_config table.
// This allows administrators to change thresholds (e.g. variance threshold) via the
// admin portal without redeploying the service.
type SystemConfigRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewSystemConfigRepository(db *pgxpool.Pool, logger *zap.Logger) *SystemConfigRepository {
	return &SystemConfigRepository{db: db, logger: logger}
}

// GetFloat64 returns a float64 config value by key.
// Falls back to defaultVal if the key is not found or cannot be parsed.
func (r *SystemConfigRepository) GetFloat64(
	ctx context.Context,
	key string,
	defaultVal float64,
) (float64, error) {
	var strVal string
	err := r.db.QueryRow(ctx,
		`SELECT config_value FROM system_config WHERE config_key = $1 AND is_active = true`,
		key,
	).Scan(&strVal)
	if err != nil {
		// Key not found — return default silently
		r.logger.Debug("system_config key not found, using default",
			zap.String("key", key),
			zap.Float64("default", defaultVal),
		)
		return defaultVal, nil
	}

	val, err := strconv.ParseFloat(strVal, 64)
	if err != nil {
		return defaultVal, fmt.Errorf("system_config[%s] value %q is not a valid float: %w", key, strVal, err)
	}
	return val, nil
}

// GetString returns a string config value by key.
// Falls back to defaultVal if the key is not found.
func (r *SystemConfigRepository) GetString(
	ctx context.Context,
	key string,
	defaultVal string,
) (string, error) {
	var strVal string
	err := r.db.QueryRow(ctx,
		`SELECT config_value FROM system_config WHERE config_key = $1 AND is_active = true`,
		key,
	).Scan(&strVal)
	if err != nil {
		r.logger.Debug("system_config key not found, using default",
			zap.String("key", key),
			zap.String("default", defaultVal),
		)
		return defaultVal, nil
	}
	return strVal, nil
}
