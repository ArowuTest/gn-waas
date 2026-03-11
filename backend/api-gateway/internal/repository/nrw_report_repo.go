package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"go.uber.org/zap"
)

// NRWReportRepository handles Non-Revenue Water reporting queries.
// All data is derived from the audit_events, anomaly_flags, and water_accounts tables —
// no hardcoded values.
type NRWReportRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewNRWReportRepository(db *pgxpool.Pool, logger *zap.Logger) *NRWReportRepository {
	return &NRWReportRepository{db: db, logger: logger}
}

// q returns the Querier to use for this request.
// If an RLS-activated transaction is stored in ctx (by rls.Middleware), it is
// returned so that all queries run within that transaction and RLS is enforced.
// Otherwise the connection pool is returned (RLS not enforced — ops alert).
func (r *NRWReportRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NRWSummaryRow represents one district's NRW performance for a reporting period
type NRWSummaryRow struct {
	DistrictID          uuid.UUID `json:"district_id"`
	DistrictCode        string    `json:"district_code"`
	DistrictName        string    `json:"district_name"`
	Region              string    `json:"region"`
	PeriodStart         time.Time `json:"period_start"`
	PeriodEnd           time.Time `json:"period_end"`
	TotalAccounts       int       `json:"total_accounts"`
	FlaggedAccounts     int       `json:"flagged_accounts"`
	OpenAnomalies       int       `json:"open_anomalies"`
	CriticalAnomalies   int       `json:"critical_anomalies"`
	HighAnomalies       int       `json:"high_anomalies"`
	TotalEstimatedLoss  float64   `json:"total_estimated_loss_ghs"`
	TotalConfirmedLoss  float64   `json:"total_confirmed_loss_ghs"`
	TotalRecovered      float64   `json:"total_recovered_ghs"`
	LossRatioPct        *float64  `json:"loss_ratio_pct"`
	DataConfidenceGrade *int      `json:"data_confidence_grade"`
	IsPilotDistrict     bool      `json:"is_pilot_district"`
	Grade               string    `json:"grade"` // A/B/C/D/F derived from loss_ratio_pct
	ZoneType            *string   `json:"zone_type,omitempty"` // FIX: added for frontend NRWSummaryPage
	// Frontend-compatible aliases (NRWAnalysisPage expects these field names)
	NRWPct       float64 `json:"nrw_pct"`       // alias for loss_ratio_pct (0 if null)
	ProductionM3 float64 `json:"production_m3"` // system input volume (placeholder)
	BilledM3     float64 `json:"billed_m3"`     // authorised consumption (placeholder)
}

// GetNRWSummary returns NRW performance per district for the given period.
// If districtID is provided, returns only that district.
// Period defaults to the last 30 days if not specified.
func (r *NRWReportRepository) GetNRWSummary(ctx context.Context, districtID *uuid.UUID, from, to *time.Time) ([]*NRWSummaryRow, error) {
	// Default period: last 30 days
	now := time.Now()
	periodEnd := now
	periodStart := now.AddDate(0, -1, 0)
	if from != nil {
		periodStart = *from
	}
	if to != nil {
		periodEnd = *to
	}

	args := []interface{}{periodStart, periodEnd}
	districtFilter := ""
	if districtID != nil {
		districtFilter = "AND d.id = $3"
		args = append(args, *districtID)
	}

	query := fmt.Sprintf(`
		SELECT
			d.id                                                          AS district_id,
			d.district_code,
			d.district_name,
			d.region,
			$1::timestamptz                                               AS period_start,
			$2::timestamptz                                               AS period_end,
			COALESCE(acc_stats.total_accounts, 0)                        AS total_accounts,
			COALESCE(acc_stats.flagged_accounts, 0)                      AS flagged_accounts,
			COALESCE(flag_stats.open_anomalies, 0)                       AS open_anomalies,
			COALESCE(flag_stats.critical_anomalies, 0)                   AS critical_anomalies,
			COALESCE(flag_stats.high_anomalies, 0)                       AS high_anomalies,
			COALESCE(flag_stats.total_estimated_loss, 0)                 AS total_estimated_loss_ghs,
			COALESCE(audit_stats.total_confirmed_loss, 0)                AS total_confirmed_loss_ghs,
			COALESCE(audit_stats.total_recovered, 0)                     AS total_recovered_ghs,
			d.loss_ratio_pct,
			d.data_confidence_grade,
			d.is_pilot_district,
			d.zone_type
		FROM districts d
		LEFT JOIN (
			SELECT district_id,
			       COUNT(*)                                               AS total_accounts,
			       COUNT(*) FILTER (WHERE is_phantom_flagged = TRUE)     AS flagged_accounts
			FROM water_accounts
			WHERE status = 'ACTIVE'
			GROUP BY district_id
		) acc_stats ON acc_stats.district_id = d.id
		LEFT JOIN (
			SELECT district_id,
			       COUNT(*) FILTER (WHERE status = 'OPEN')               AS open_anomalies,
			       COUNT(*) FILTER (WHERE alert_level = 'CRITICAL' AND status = 'OPEN') AS critical_anomalies,
			       COUNT(*) FILTER (WHERE alert_level = 'HIGH' AND status = 'OPEN')     AS high_anomalies,
			       COALESCE(SUM(estimated_loss_ghs), 0)                  AS total_estimated_loss
			FROM anomaly_flags
			WHERE created_at BETWEEN $1 AND $2
			GROUP BY district_id
		) flag_stats ON flag_stats.district_id = d.id
		LEFT JOIN (
			SELECT district_id,
			       COALESCE(SUM(confirmed_loss_ghs), 0)                  AS total_confirmed_loss,
			       COALESCE(SUM(recovery_invoice_ghs), 0)                AS total_recovered
			FROM audit_events
			WHERE status = 'COMPLETED' AND created_at BETWEEN $1 AND $2
			GROUP BY district_id
		) audit_stats ON audit_stats.district_id = d.id
		WHERE d.is_active = TRUE %s
		ORDER BY COALESCE(flag_stats.total_estimated_loss, 0) DESC`, districtFilter)

	rows, err := r.q(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("NRW summary query failed: %w", err)
	}
	defer rows.Close()

	var results []*NRWSummaryRow
	for rows.Next() {
		row := &NRWSummaryRow{}
		err := rows.Scan(
			&row.DistrictID, &row.DistrictCode, &row.DistrictName, &row.Region,
			&row.PeriodStart, &row.PeriodEnd,
			&row.TotalAccounts, &row.FlaggedAccounts,
			&row.OpenAnomalies, &row.CriticalAnomalies, &row.HighAnomalies,
			&row.TotalEstimatedLoss, &row.TotalConfirmedLoss, &row.TotalRecovered,
			&row.LossRatioPct, &row.DataConfidenceGrade, &row.IsPilotDistrict, &row.ZoneType,
		)
		if err != nil {
			return nil, err
		}
		// Derive IWA grade from loss ratio
		row.Grade = deriveNRWGrade(row.LossRatioPct)
		// Populate frontend-compatible alias fields (NRWAnalysisPage expects nrw_pct)
		if row.LossRatioPct != nil {
			row.NRWPct = *row.LossRatioPct
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

// deriveNRWGrade converts a loss ratio percentage to an IWA/AWWA performance grade
func deriveNRWGrade(lossRatioPct *float64) string {
	if lossRatioPct == nil {
		return "N/A"
	}
	pct := *lossRatioPct
	switch {
	case pct < 15:
		return "A" // World-class
	case pct < 25:
		return "B" // Good
	case pct < 35:
		return "C" // Average
	case pct < 50:
		return "D" // Poor
	default:
		return "F" // Critical — Ghana average is 51.6%
	}
}

// GetDistrictNRWTrend returns monthly NRW trend for a specific district (last 12 months)
func (r *NRWReportRepository) GetDistrictNRWTrend(ctx context.Context, districtID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT
			DATE_TRUNC('month', created_at)                              AS month,
			COUNT(*) FILTER (WHERE status = 'OPEN')                      AS open_flags,
			COUNT(*) FILTER (WHERE status = 'RESOLVED')                  AS resolved_flags,
			COALESCE(SUM(estimated_loss_ghs), 0)                         AS estimated_loss_ghs
		FROM anomaly_flags
		WHERE district_id = $1
		  AND created_at >= NOW() - INTERVAL '12 months'
		GROUP BY DATE_TRUNC('month', created_at)
		ORDER BY month ASC`, districtID)
	if err != nil {
		return nil, fmt.Errorf("NRW trend query failed: %w", err)
	}
	defer rows.Close()

	var trend []map[string]interface{}
	for rows.Next() {
		var month time.Time
		var openFlags, resolvedFlags int
		var estimatedLoss float64
		if err := rows.Scan(&month, &openFlags, &resolvedFlags, &estimatedLoss); err != nil {
			return nil, err
		}
		trend = append(trend, map[string]interface{}{
			"month":              month.Format("Jan 2006"),
			"open_flags":         openFlags,
			"resolved_flags":     resolvedFlags,
			"estimated_loss_ghs": estimatedLoss,
		})
	}
	return trend, rows.Err()
}

// GetMyDistrictSummary returns a summary for a specific district (used by GWL staff portal)
func (r *NRWReportRepository) GetMyDistrictSummary(ctx context.Context, districtID uuid.UUID) (*domain.District, *NRWSummaryRow, error) {
	distRepo := &DistrictRepository{db: r.db, logger: r.logger}
	district, err := distRepo.GetByID(ctx, districtID)
	if err != nil {
		return nil, nil, err
	}

	summaries, err := r.GetNRWSummary(ctx, &districtID, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	if len(summaries) == 0 {
		return district, nil, nil
	}
	return district, summaries[0], nil
}

// GetUserDistrictID returns the district_id assigned to a user.
// Returns an error if the user has no district assigned.
// Uses r.q(ctx) so the query runs inside the RLS-activated transaction.
func (r *NRWReportRepository) GetUserDistrictID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var districtID uuid.UUID
	err := r.q(ctx).QueryRow(ctx,
		"SELECT district_id FROM users WHERE id = $1 AND district_id IS NOT NULL",
		userID,
	).Scan(&districtID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("NRWReportRepository.GetUserDistrictID: %w", err)
	}
	return districtID, nil
}

// GetFirstDistrictID returns the ID of the first active district.
// Used as a fallback for SUPER_ADMIN users who have no district assignment.
func (r *NRWReportRepository) GetFirstDistrictID(ctx context.Context) (uuid.UUID, error) {
	var districtID uuid.UUID
	err := r.q(ctx).QueryRow(ctx,
		"SELECT id FROM districts WHERE is_active = true ORDER BY district_code LIMIT 1",
	).Scan(&districtID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("NRWReportRepository.GetFirstDistrictID: %w", err)
	}
	return districtID, nil
}

