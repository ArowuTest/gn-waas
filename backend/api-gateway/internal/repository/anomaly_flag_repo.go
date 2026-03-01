package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"strconv"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AnomalyFlag represents a sentinel-detected billing anomaly
type AnomalyFlag struct {
	ID                  uuid.UUID  `json:"id"`
	DistrictID          uuid.UUID  `json:"district_id"`
	AccountID           *uuid.UUID `json:"account_id,omitempty"`
	FlagType            string     `json:"flag_type"`
	Severity            string     `json:"severity"`
	Status              string     `json:"status"`
	GWLStatus           *string    `json:"gwl_status,omitempty"`
	EstimatedLossGHS    float64    `json:"estimated_loss_ghs"`
	ConfirmedLossGHS    *float64   `json:"confirmed_loss_ghs,omitempty"`
	RecoveredGHS        *float64   `json:"recovered_ghs,omitempty"`
	AlertLevel          *string    `json:"alert_level,omitempty"`
	AnomalyType         *string    `json:"anomaly_type,omitempty"`
	Description         string     `json:"description"`
	DetectedAt          time.Time  `json:"detected_at"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

// AnomalyFlagRepository handles anomaly flag queries
type AnomalyFlagRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAnomalyFlagRepository(db *pgxpool.Pool, logger *zap.Logger) *AnomalyFlagRepository {
	return &AnomalyFlagRepository{db: db, logger: logger}
}

// ListAnomalyFlags returns anomaly flags, optionally filtered by district
func (r *AnomalyFlagRepository) ListAnomalyFlags(
	ctx context.Context,
	districtID *uuid.UUID,
	severity string,
	status string,
	limit, offset int,
) ([]AnomalyFlag, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	args := []interface{}{}
	argIdx := 1
	where := "WHERE 1=1"

	if districtID != nil {
		where += " AND district_id = $" + itoa(argIdx)
		args = append(args, *districtID)
		argIdx++
	}
	if severity != "" {
		where += " AND severity = $" + itoa(argIdx)
		args = append(args, severity)
		argIdx++
	}
	if status != "" {
		where += " AND status = $" + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	countSQL := `SELECT COUNT(*) FROM anomaly_flags ` + where
	var total int
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	dataSQL := `
		SELECT id, district_id, account_id, flag_type, severity, status,
		       gwl_status, estimated_loss_ghs, confirmed_loss_ghs, recovered_ghs,
		       description, detected_at, resolved_at, created_at
		FROM anomaly_flags
		` + where + `
		ORDER BY detected_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	rows, err := r.db.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flags []AnomalyFlag
	for rows.Next() {
		var f AnomalyFlag
		if err := rows.Scan(
			&f.ID, &f.DistrictID, &f.AccountID, &f.FlagType, &f.Severity, &f.Status,
			&f.GWLStatus, &f.EstimatedLossGHS, &f.ConfirmedLossGHS, &f.RecoveredGHS,
			&f.Description, &f.DetectedAt, &f.ResolvedAt, &f.CreatedAt,
		); err != nil {
			r.logger.Error("scan anomaly flag", zap.Error(err))
			continue
		}
		flags = append(flags, f)
	}
	return flags, total, nil
}

// GetByID returns a single anomaly flag by ID
func (r *AnomalyFlagRepository) GetByID(ctx context.Context, id uuid.UUID) (*AnomalyFlag, error) {
	var f AnomalyFlag
	err := r.db.QueryRow(ctx, `
		SELECT id, district_id, account_id, flag_type, severity, status,
		       gwl_status, estimated_loss_ghs, confirmed_loss_ghs, recovered_ghs,
		       description, detected_at, resolved_at, created_at
		FROM anomaly_flags WHERE id = $1`, id,
	).Scan(
		&f.ID, &f.DistrictID, &f.AccountID, &f.FlagType, &f.Severity, &f.Status,
		&f.GWLStatus, &f.EstimatedLossGHS, &f.ConfirmedLossGHS, &f.RecoveredGHS,
		&f.Description, &f.DetectedAt, &f.ResolvedAt, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// itoa converts int to string for SQL placeholder building
func itoa(n int) string {
	return strconv.Itoa(n)
}
