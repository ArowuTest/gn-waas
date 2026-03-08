package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/repository/interfaces"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AnomalyFlagRepository implements interfaces.AnomalyFlagRepository using PostgreSQL
type AnomalyFlagRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAnomalyFlagRepository(db *pgxpool.Pool, logger *zap.Logger) *AnomalyFlagRepository {
	return &AnomalyFlagRepository{db: db, logger: logger}
}

func (r *AnomalyFlagRepository) Create(ctx context.Context, flag *entities.AnomalyFlag) (*entities.AnomalyFlag, error) {
	evidenceJSON, err := json.Marshal(flag.EvidenceData)
	if err != nil {
		return nil, fmt.Errorf("marshal evidence failed: %w", err)
	}

	// BE-M02 fix: add explicit enum casts for anomaly_type and alert_level.
	// pgx cannot reliably infer custom enum types from plain strings;
	// without the cast, the INSERT fails with "column is of type anomaly_type
	// but expression is of type text" on some pgx driver versions.
	// Normalise leakage category: if not set, derive from anomaly type
	leakageCategory := flag.LeakageCategory
	if leakageCategory == "" {
		switch flag.AnomalyType {
		case "SHADOW_BILL_VARIANCE", "CATEGORY_MISMATCH", "PHANTOM_METER",
			"DISTRICT_IMBALANCE", "UNMETERED_CONSUMPTION", "METERING_INACCURACY",
			"UNAUTHORISED_CONSUMPTION", "VAT_DISCREPANCY":
			leakageCategory = "REVENUE_LEAKAGE"
		case "OUTAGE_CONSUMPTION":
			leakageCategory = "COMPLIANCE"
		default:
			leakageCategory = "DATA_QUALITY"
		}
	}

	// Annualised = monthly × 12
	annualised := flag.AnnualisedLeakageGHS
	if annualised == 0 && flag.MonthlyLeakageGHS > 0 {
		annualised = flag.MonthlyLeakageGHS * 12
	}

	query := `
		INSERT INTO anomaly_flags (
			account_id, district_id, anomaly_type, alert_level,
			fraud_type, title, description, estimated_loss_ghs,
			billing_period_start, billing_period_end,
			shadow_bill_id, gwl_bill_id, evidence_data,
			status, sentinel_version, detection_hash,
			leakage_category, monthly_leakage_ghs, annualised_leakage_ghs
		) VALUES (
			$1,$2,$3::anomaly_type,$4::alert_level,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,
			$17::leakage_category,$18,$19
		)
		RETURNING id, created_at`

	err = r.db.QueryRow(ctx, query,
		flag.AccountID, flag.DistrictID, flag.AnomalyType, flag.AlertLevel,
		flag.FraudType, flag.Title, flag.Description, flag.EstimatedLossGHS,
		flag.BillingPeriodStart, flag.BillingPeriodEnd,
		flag.ShadowBillID, flag.GWLBillID, evidenceJSON,
		flag.Status, flag.SentinelVersion, flag.DetectionHash,
		leakageCategory, flag.MonthlyLeakageGHS, annualised,
	).Scan(&flag.ID, &flag.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("Create anomaly flag failed: %w", err)
	}

	r.logger.Info("Anomaly flag created",
		zap.String("id", flag.ID.String()),
		zap.String("type", flag.AnomalyType),
		zap.String("level", flag.AlertLevel),
	)

	return flag, nil
}

func (r *AnomalyFlagRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.AnomalyFlag, error) {
	query := `
		SELECT id, account_id, district_id, anomaly_type, alert_level,
		       fraud_type, title, description, estimated_loss_ghs,
		       billing_period_start, billing_period_end,
		       shadow_bill_id, gwl_bill_id, evidence_data,
		       status, sentinel_version, detection_hash, created_at
		FROM anomaly_flags WHERE id = $1`

	row := r.db.QueryRow(ctx, query, id)
	flag, err := r.scanRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("anomaly flag %s not found", id)
		}
		return nil, err
	}
	return flag, nil
}

func (r *AnomalyFlagRepository) GetByAccount(ctx context.Context, accountID uuid.UUID) ([]*entities.AnomalyFlag, error) {
	query := `
		SELECT id, account_id, district_id, anomaly_type, alert_level,
		       fraud_type, title, description, estimated_loss_ghs,
		       billing_period_start, billing_period_end,
		       shadow_bill_id, gwl_bill_id, evidence_data,
		       status, sentinel_version, detection_hash, created_at
		FROM anomaly_flags
		WHERE account_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("GetByAccount failed: %w", err)
	}
	defer rows.Close()
	return r.scanRows(rows)
}

func (r *AnomalyFlagRepository) GetOpenByDistrict(ctx context.Context, districtID uuid.UUID, limit, offset int) ([]*entities.AnomalyFlag, int, error) {
	var total int
	if err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM anomaly_flags WHERE district_id = $1 AND status = 'OPEN'",
		districtID,
	).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count failed: %w", err)
	}

	query := `
		SELECT id, account_id, district_id, anomaly_type, alert_level,
		       fraud_type, title, description, estimated_loss_ghs,
		       billing_period_start, billing_period_end,
		       shadow_bill_id, gwl_bill_id, evidence_data,
		       status, sentinel_version, detection_hash, created_at
		FROM anomaly_flags
		WHERE district_id = $1 AND status = 'OPEN'
		ORDER BY
			CASE alert_level WHEN 'CRITICAL' THEN 1 WHEN 'HIGH' THEN 2 WHEN 'MEDIUM' THEN 3 ELSE 4 END,
			created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, districtID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("GetOpenByDistrict failed: %w", err)
	}
	defer rows.Close()

	flags, err := r.scanRows(rows)
	return flags, total, err
}

func (r *AnomalyFlagRepository) GetByCriteria(ctx context.Context, filter interfaces.AnomalyFilter) ([]*entities.AnomalyFlag, int, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filter.DistrictID != nil {
		conditions = append(conditions, fmt.Sprintf("district_id = $%d", argIdx))
		args = append(args, *filter.DistrictID)
		argIdx++
	}
	if filter.AccountID != nil {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", argIdx))
		args = append(args, *filter.AccountID)
		argIdx++
	}
	if filter.AnomalyType != nil {
		conditions = append(conditions, fmt.Sprintf("anomaly_type = $%d", argIdx))
		args = append(args, *filter.AnomalyType)
		argIdx++
	}
	if filter.AlertLevel != nil {
		conditions = append(conditions, fmt.Sprintf("alert_level = $%d", argIdx))
		args = append(args, *filter.AlertLevel)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM anomaly_flags WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count failed: %w", err)
	}

	limit := filter.Limit
	if limit == 0 {
		limit = 20
	}
	args = append(args, limit, filter.Offset)

	dataQuery := fmt.Sprintf(`
		SELECT id, account_id, district_id, anomaly_type, alert_level,
		       fraud_type, title, description, estimated_loss_ghs,
		       billing_period_start, billing_period_end,
		       shadow_bill_id, gwl_bill_id, evidence_data,
		       status, sentinel_version, detection_hash, created_at
		FROM anomaly_flags WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("GetByCriteria failed: %w", err)
	}
	defer rows.Close()

	flags, err := r.scanRows(rows)
	return flags, total, err
}

func (r *AnomalyFlagRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID, notes string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE anomaly_flags
		SET status = $1, assigned_to = $2, resolution_notes = $3,
		    resolved_at = CASE WHEN $1 IN ('RESOLVED','CLOSED','FALSE_POSITIVE') THEN NOW() ELSE NULL END,
		    updated_at = NOW()
		WHERE id = $4`, status, resolvedBy, notes, id)
	return err
}

func (r *AnomalyFlagRepository) MarkFalsePositive(ctx context.Context, id uuid.UUID, resolvedBy uuid.UUID, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE anomaly_flags
		SET status = 'FALSE_POSITIVE', false_positive = TRUE,
		    assigned_to = $1, resolution_notes = $2,
		    resolved_at = NOW(), updated_at = NOW()
		WHERE id = $3`, resolvedBy, reason, id)
	return err
}

func (r *AnomalyFlagRepository) GetSummaryByDistrict(ctx context.Context, districtID uuid.UUID, from, to time.Time) (*interfaces.AnomalySummary, error) {
	summary := &interfaces.AnomalySummary{
		DistrictID:  districtID,
		ByType:      make(map[string]int),
		ByFraudType: make(map[string]int),
	}

	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'OPEN'),
			COUNT(*) FILTER (WHERE alert_level = 'CRITICAL' AND status = 'OPEN'),
			COUNT(*) FILTER (WHERE alert_level = 'HIGH' AND status = 'OPEN'),
			COUNT(*) FILTER (WHERE alert_level = 'MEDIUM' AND status = 'OPEN'),
			COUNT(*) FILTER (WHERE alert_level = 'LOW' AND status = 'OPEN'),
			COALESCE(SUM(estimated_loss_ghs) FILTER (WHERE status = 'OPEN'), 0)
		FROM anomaly_flags
		WHERE district_id = $1 AND created_at BETWEEN $2 AND $3`,
		districtID, from, to,
	).Scan(
		&summary.TotalOpen, &summary.TotalCritical, &summary.TotalHigh,
		&summary.TotalMedium, &summary.TotalLow, &summary.TotalEstimatedLoss,
	)
	if err != nil {
		return nil, fmt.Errorf("GetSummaryByDistrict failed: %w", err)
	}

	rows, err := r.db.Query(ctx, `
		SELECT anomaly_type, COUNT(*) FROM anomaly_flags
		WHERE district_id = $1 AND status = 'OPEN' AND created_at BETWEEN $2 AND $3
		GROUP BY anomaly_type`, districtID, from, to)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var cnt int
			if err := rows.Scan(&t, &cnt); err == nil {
				summary.ByType[t] = cnt
			}
		}
	}

	return summary, nil
}

func (r *AnomalyFlagRepository) ExistsByDetectionHash(ctx context.Context, hash string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM anomaly_flags WHERE detection_hash = $1)", hash,
	).Scan(&exists)
	return exists, err
}

func (r *AnomalyFlagRepository) scanRow(row pgx.Row) (*entities.AnomalyFlag, error) {
	flag := &entities.AnomalyFlag{}
	var evidenceJSON []byte
	err := row.Scan(
		&flag.ID, &flag.AccountID, &flag.DistrictID, &flag.AnomalyType, &flag.AlertLevel,
		&flag.FraudType, &flag.Title, &flag.Description, &flag.EstimatedLossGHS,
		&flag.BillingPeriodStart, &flag.BillingPeriodEnd,
		&flag.ShadowBillID, &flag.GWLBillID, &evidenceJSON,
		&flag.Status, &flag.SentinelVersion, &flag.DetectionHash, &flag.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(evidenceJSON) > 0 {
		_ = json.Unmarshal(evidenceJSON, &flag.EvidenceData)
	}
	return flag, nil
}

func (r *AnomalyFlagRepository) scanRows(rows pgx.Rows) ([]*entities.AnomalyFlag, error) {
	var flags []*entities.AnomalyFlag
	for rows.Next() {
		flag := &entities.AnomalyFlag{}
		var evidenceJSON []byte
		err := rows.Scan(
			&flag.ID, &flag.AccountID, &flag.DistrictID, &flag.AnomalyType, &flag.AlertLevel,
			&flag.FraudType, &flag.Title, &flag.Description, &flag.EstimatedLossGHS,
			&flag.BillingPeriodStart, &flag.BillingPeriodEnd,
			&flag.ShadowBillID, &flag.GWLBillID, &evidenceJSON,
			&flag.Status, &flag.SentinelVersion, &flag.DetectionHash, &flag.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan anomaly flag: %w", err)
		}
		if len(evidenceJSON) > 0 {
			_ = json.Unmarshal(evidenceJSON, &flag.EvidenceData)
		}
		flags = append(flags, flag)
	}
	return flags, rows.Err()
}
