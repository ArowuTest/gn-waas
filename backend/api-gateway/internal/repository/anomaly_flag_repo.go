package repository

import (
	"fmt"
	"context"
	"time"

	"github.com/google/uuid"
	"strconv"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"go.uber.org/zap"
)

// AnomalyFlag represents a sentinel-detected billing anomaly.
// Field names match the anomaly_flags table schema (migration 004).
type AnomalyFlag struct {
	ID                  uuid.UUID  `json:"id"`
	DistrictID          uuid.UUID  `json:"district_id"`
	AccountID           *uuid.UUID `json:"account_id,omitempty"`
	// APP-3 fix: DB column is anomaly_type, not flag_type
	AnomalyType         string     `json:"anomaly_type"`
	// APP-3 fix: DB column is alert_level, not severity
	AlertLevel          string     `json:"alert_level"`
	FraudType           *string    `json:"fraud_type,omitempty"`
	Title               string     `json:"title"`
	Description         string     `json:"description"`
	EstimatedLossGHS    *float64   `json:"estimated_loss_ghs,omitempty"`
	Status              string     `json:"status"`
	AssignedTo          *uuid.UUID `json:"assigned_to,omitempty"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
	ResolutionNotes     *string    `json:"resolution_notes,omitempty"`
	FalsePositive       *bool      `json:"false_positive,omitempty"`
	ConfirmedFraud         *bool      `json:"confirmed_fraud,omitempty"`
	RecoveredAmountGHS     *float64   `json:"recovered_amount_ghs,omitempty"`
	LeakageCategory        *string    `json:"leakage_category,omitempty"`
	MonthlyLeakageGHS      *float64   `json:"monthly_leakage_ghs,omitempty"`
	AnnualisedLeakageGHS   *float64   `json:"annualised_leakage_ghs,omitempty"`
	ConfirmedLeakageGHS    *float64   `json:"confirmed_leakage_ghs,omitempty"`
	FieldOutcome           *string    `json:"field_outcome,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// DB returns the underlying connection pool (for direct queries in handlers).
func (r *AnomalyFlagRepository) DB() *pgxpool.Pool {
	return r.db
}

// AnomalyFlagRepository handles anomaly flag queries
type AnomalyFlagRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAnomalyFlagRepository(db *pgxpool.Pool, logger *zap.Logger) *AnomalyFlagRepository {
	return &AnomalyFlagRepository{db: db, logger: logger}
}

// q returns the Querier to use for this request.
// If an RLS-activated transaction is stored in ctx (by rls.Middleware), it is
// returned so that all queries run within that transaction and RLS is enforced.
// Otherwise the connection pool is returned (RLS not enforced — ops alert).
func (r *AnomalyFlagRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// selectCols is the canonical SELECT column list matching the AnomalyFlag struct.
// APP-3 fix: uses anomaly_type and alert_level (actual DB column names).
const anomalyFlagSelectCols = `
	id, district_id, account_id, anomaly_type, alert_level, fraud_type,
	title, description, estimated_loss_ghs, status, assigned_to,
	resolved_at, resolution_notes, false_positive, confirmed_fraud,
	recovered_amount_ghs, created_at, updated_at`

func scanAnomalyFlag(row interface {
	Scan(dest ...any) error
}) (AnomalyFlag, error) {
	var f AnomalyFlag
	err := row.Scan(
		&f.ID, &f.DistrictID, &f.AccountID, &f.AnomalyType, &f.AlertLevel, &f.FraudType,
		&f.Title, &f.Description, &f.EstimatedLossGHS, &f.Status, &f.AssignedTo,
		&f.ResolvedAt, &f.ResolutionNotes, &f.FalsePositive, &f.ConfirmedFraud,
		&f.RecoveredAmountGHS, &f.CreatedAt, &f.UpdatedAt,
	)
	return f, err
}

// ListAnomalyFlags returns anomaly flags, optionally filtered by district
func (r *AnomalyFlagRepository) ListAnomalyFlags(
	ctx context.Context,
	districtID *uuid.UUID,
	severity string,
	status string,
	anomalyType string, // FIX: added anomaly_type filter
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
	// APP-3 fix: filter on alert_level (was severity)
	if severity != "" {
		where += " AND alert_level = $" + itoa(argIdx)
		args = append(args, severity)
		argIdx++
	}
	if status != "" {
		where += " AND status = $" + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	// FIX: anomaly_type filter (from frontend type dropdown)
	if anomalyType != "" {
		where += " AND anomaly_type = $" + itoa(argIdx)
		args = append(args, anomalyType)
		argIdx++
	}

	countSQL := `SELECT COUNT(*) FROM anomaly_flags ` + where
	var total int
	if err := r.q(ctx).QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	dataSQL := `SELECT ` + anomalyFlagSelectCols + `
		FROM anomaly_flags
		` + where + `
		ORDER BY created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	rows, err := r.q(ctx).Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flags []AnomalyFlag
	for rows.Next() {
		f, err := scanAnomalyFlag(rows)
		if err != nil {
			r.logger.Error("scan anomaly flag", zap.Error(err))
			continue
		}
		flags = append(flags, f)
	}
	return flags, total, nil
}

// GetByID returns a single anomaly flag by ID
func (r *AnomalyFlagRepository) GetByID(ctx context.Context, id uuid.UUID) (*AnomalyFlag, error) {
	row := r.q(ctx).QueryRow(ctx,
		`SELECT `+anomalyFlagSelectCols+` FROM anomaly_flags WHERE id = $1`, id)
	f, err := scanAnomalyFlag(row)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// itoa converts int to string for SQL placeholder building
func itoa(n int) string {
	return strconv.Itoa(n)
}

// ─── RLS-Activated Variants ───────────────────────────────────────────────────

// ListAnomalyFlagsTx is identical to ListAnomalyFlags but runs inside an
// RLS-activated transaction (via rls.BeginReadOnlyTx).
func (r *AnomalyFlagRepository) ListAnomalyFlagsTx(
	ctx context.Context,
	q Querier,
	districtID *uuid.UUID,
	severity string,
	status string,
	anomalyType string,
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
	// APP-3 fix: filter on alert_level (was severity)
	if severity != "" {
		where += " AND alert_level = $" + itoa(argIdx)
		args = append(args, severity)
		argIdx++
	}
	if status != "" {
		where += " AND status = $" + itoa(argIdx)
		args = append(args, status)
		argIdx++
	}
	// FIX: anomaly_type filter (from frontend type dropdown)
	if anomalyType != "" {
		where += " AND anomaly_type = $" + itoa(argIdx)
		args = append(args, anomalyType)
		argIdx++
	}

	countSQL := `SELECT COUNT(*) FROM anomaly_flags ` + where
	var total int
	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	dataSQL := `SELECT ` + anomalyFlagSelectCols + `
		FROM anomaly_flags
		` + where + `
		ORDER BY created_at DESC
		LIMIT $` + itoa(argIdx) + ` OFFSET $` + itoa(argIdx+1)

	rows, err := q.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var flags []AnomalyFlag
	for rows.Next() {
		f, err := scanAnomalyFlag(rows)
		if err != nil {
			r.logger.Error("scan anomaly flag (tx)", zap.Error(err))
			continue
		}
		flags = append(flags, f)
	}
	return flags, total, nil
}

// CreateAnomalyFlag inserts a new anomaly flag (manual report from field officer or authority portal).
// Uses r.q(ctx) so the INSERT runs inside the RLS-activated transaction.
func (r *AnomalyFlagRepository) CreateAnomalyFlag(ctx context.Context,
	districtID uuid.UUID,
	accountID *uuid.UUID,
	anomalyType, alertLevel, title, description, source string,
	estimatedLossGHS float64,
) (*AnomalyFlag, error) {
	row := r.q(ctx).QueryRow(ctx, `
		INSERT INTO anomaly_flags (
			district_id, account_id, anomaly_type, alert_level,
			title, description, estimated_loss_ghs,
			status, evidence_data, created_at, updated_at
		) VALUES (
			$1, $2, $3::anomaly_type, $4::alert_level,
			$5, $6, $7,
			'OPEN', jsonb_build_object('source', $8), NOW(), NOW()
		) RETURNING `+anomalyFlagSelectCols,
		districtID, accountID, anomalyType, alertLevel,
		title, description, estimatedLossGHS, source,
	)
	flag, err := scanAnomalyFlag(row)
	if err != nil {
		return nil, fmt.Errorf("CreateAnomalyFlag: %w", err)
	}
	return &flag, nil
}
