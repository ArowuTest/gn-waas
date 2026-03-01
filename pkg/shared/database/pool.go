package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Pool wraps pgxpool.Pool with GN-WAAS specific helpers
type Pool struct {
	*pgxpool.Pool
	logger *zap.Logger
}

// Config holds database connection configuration
type Config struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultConfig returns sensible defaults for GN-WAAS services
func DefaultConfig() Config {
	return Config{
		SSLMode:           "disable",
		MaxConns:          20,
		MinConns:          2,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
	}
}

// DSN builds the PostgreSQL connection string
func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Name, c.User, c.Password, c.SSLMode,
	)
}

// New creates a new database connection pool
func New(ctx context.Context, cfg Config, logger *zap.Logger) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	// Register custom types (UUID, JSONB, etc.)
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return registerTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection pool established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Name),
		zap.Int32("max_conns", cfg.MaxConns),
	)

	return &Pool{Pool: pool, logger: logger}, nil
}

// registerTypes registers custom PostgreSQL types with pgx
func registerTypes(ctx context.Context, conn *pgx.Conn) error {
	// Register UUID type
	typeNames := []string{
		"uuid", "jsonb", "inet",
		// GN-WAAS custom enums
		"account_category", "account_status", "supply_status",
		"zone_type", "anomaly_type", "alert_level", "fraud_type",
		"audit_status", "field_job_status", "ocr_status",
		"gra_compliance_status", "user_role", "user_status",
		"water_balance_component", "data_confidence_grade",
	}

	for _, typeName := range typeNames {
		dataType, err := conn.LoadType(ctx, typeName)
		if err != nil {
			// Non-fatal: custom enums may not exist in all environments
			continue
		}
		conn.TypeMap().RegisterType(dataType)
	}

	return nil
}

// WithTransaction executes a function within a database transaction
// Automatically commits on success, rolls back on error
func (p *Pool) WithTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			p.logger.Error("Failed to rollback transaction",
				zap.Error(rbErr),
				zap.NamedError("original_error", err),
			)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Stats returns current pool statistics for health monitoring
func (p *Pool) Stats() map[string]interface{} {
	stats := p.Pool.Stat()
	return map[string]interface{}{
		"total_conns":    stats.TotalConns(),
		"idle_conns":     stats.IdleConns(),
		"acquired_conns": stats.AcquiredConns(),
		"max_conns":      stats.MaxConns(),
	}
}
