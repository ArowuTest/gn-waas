package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"go.uber.org/zap"
)

// AuditEventRepository handles all audit event data access
type AuditEventRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAuditEventRepository(db *pgxpool.Pool, logger *zap.Logger) *AuditEventRepository {
	return &AuditEventRepository{db: db, logger: logger}
}

// q returns the Querier to use for this request.
// If an RLS-activated transaction is stored in ctx (by rls.Middleware), it is
// returned so that all queries run within that transaction and RLS is enforced.
// Otherwise the connection pool is returned (RLS not enforced — ops alert).
func (r *AuditEventRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// Create creates a new audit event with auto-generated reference number
func (r *AuditEventRepository) Create(ctx context.Context, event *domain.AuditEvent) (*domain.AuditEvent, error) {
	// Generate audit reference: AUD-2026-XXXXXX
	var ref string
	err := r.q(ctx).QueryRow(ctx, `
		SELECT 'AUD-' || TO_CHAR(NOW(), 'YYYY') || '-' || LPAD(nextval('audit_ref_seq')::TEXT, 6, '0')
	`).Scan(&ref)
	if err != nil {
		// Fallback reference if sequence doesn't exist
		ref = fmt.Sprintf("AUD-%s-%s", time.Now().Format("2006"), uuid.New().String()[:6])
	}
	event.AuditReference = ref

	query := `
		INSERT INTO audit_events (
			audit_reference, account_id, district_id, anomaly_flag_id,
			status, assigned_officer_id, assigned_supervisor_id,
			due_date, gwl_billed_ghs, shadow_bill_ghs, variance_pct, notes
		) VALUES ($1,$2,$3,$4,$5::audit_status,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`

	err = r.q(ctx).QueryRow(ctx, query,
		event.AuditReference, event.AccountID, event.DistrictID, event.AnomalyFlagID,
		event.Status, event.AssignedOfficerID, event.AssignedSupervisorID,
		event.DueDate, event.GWLBilledGHS, event.ShadowBillGHS, event.VariancePct, event.Notes,
	).Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("Create audit event failed: %w", err)
	}

	r.logger.Info("Audit event created",
		zap.String("id", event.ID.String()),
		zap.String("reference", event.AuditReference),
	)

	return event, nil
}

// GetByID returns an audit event by ID
func (r *AuditEventRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditEvent, error) {
	query := `
		SELECT id, audit_reference, account_id, district_id, anomaly_flag_id,
		       status, assigned_officer_id, assigned_supervisor_id, assigned_at, due_date,
		       field_job_id, meter_photo_url, surroundings_photo_url,
		       ocr_reading_value, manual_reading_value, ocr_status,
		       gps_latitude, gps_longitude, gps_precision_m,
		       tamper_evidence_detected, tamper_evidence_url,
		       gra_status, gra_sdc_id, gra_qr_code_url, gra_signed_at,
		       gwl_billed_ghs, shadow_bill_ghs, variance_pct,
		       confirmed_loss_ghs, recovery_invoice_ghs, success_fee_ghs,
		       is_locked, locked_at, notes, created_at, updated_at
		FROM audit_events WHERE id = $1`

	event := &domain.AuditEvent{}
	err := r.q(ctx).QueryRow(ctx, query, id).Scan(
		&event.ID, &event.AuditReference, &event.AccountID, &event.DistrictID, &event.AnomalyFlagID,
		&event.Status, &event.AssignedOfficerID, &event.AssignedSupervisorID, &event.AssignedAt, &event.DueDate,
		&event.FieldJobID, &event.MeterPhotoURL, &event.SurroundingsPhotoURL,
		&event.OCRReadingValue, &event.ManualReadingValue, &event.OCRStatus,
		&event.GPSLatitude, &event.GPSLongitude, &event.GPSPrecisionM,
		&event.TamperEvidenceDetected, &event.TamperEvidenceURL,
		&event.GRAStatus, &event.GRASDCId, &event.GRAQRCodeURL, &event.GRASignedAt,
		&event.GWLBilledGHS, &event.ShadowBillGHS, &event.VariancePct,
		&event.ConfirmedLossGHS, &event.RecoveryInvoiceGHS, &event.SuccessFeeGHS,
		&event.IsLocked, &event.LockedAt, &event.Notes, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("audit event %s not found", id)
		}
		return nil, fmt.Errorf("GetByID audit event failed: %w", err)
	}
	return event, nil
}

// GetByDistrict returns audit events for a district with pagination.
// If districtID is uuid.Nil, returns events for ALL districts (admin use).
func (r *AuditEventRepository) GetByDistrict(ctx context.Context, districtID uuid.UUID, status, graStatus string, limit, offset int) ([]*domain.AuditEvent, int, error) {
	var args []interface{}
	var where string
	argIdx := 1
	if districtID != uuid.Nil {
		args = append(args, districtID)
		where = "district_id = $1"
		argIdx = 2
	} else {
		where = "1=1" // all districts
	}

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d::audit_status", argIdx)
		args = append(args, status)
		argIdx++
	}
	if graStatus != "" {
		where += fmt.Sprintf(" AND gra_status = $%d::gra_compliance_status", argIdx)
		args = append(args, graStatus)
		argIdx++
	}

	// Qualify district_id in WHERE to avoid ambiguity with JOINed tables
	qualifiedWhere := strings.ReplaceAll(where, "district_id", "ae.district_id")
	qualifiedWhere = strings.ReplaceAll(qualifiedWhere, "status", "ae.status")
	qualifiedWhere = strings.ReplaceAll(qualifiedWhere, "gra_status", "ae.gra_status")

	var total int
	r.q(ctx).QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM audit_events ae WHERE %s", qualifiedWhere), args...).Scan(&total)

	args = append(args, limit, offset)
	// FIX: expanded SELECT to include fields required by GRACompliancePage and AuditsPage:
	// account_number, account_holder (from water_accounts), district_name (from districts),
	// gra_sdc_id (receipt number), gra_qr_code_url, gra_signed_at, confirmed_loss_ghs, success_fee_ghs
	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT ae.id, ae.audit_reference, ae.account_id, ae.district_id, ae.anomaly_flag_id,
		       ae.status, ae.assigned_officer_id, ae.gra_status,
		       ae.gwl_billed_ghs, ae.shadow_bill_ghs, ae.variance_pct,
		       ae.confirmed_loss_ghs, ae.success_fee_ghs,
		       ae.gra_sdc_id, ae.gra_qr_code_url, ae.gra_signed_at,
		       ae.is_locked, ae.created_at, ae.updated_at,
		       COALESCE(wa.gwl_account_number, '')  AS account_number,
		       COALESCE(wa.account_holder_name, '')   AS account_holder,
		       COALESCE(d.district_name, '')    AS district_name
		FROM audit_events ae
		LEFT JOIN water_accounts wa ON wa.id = ae.account_id
		LEFT JOIN districts d ON d.id = ae.district_id
		WHERE %s
		ORDER BY ae.created_at DESC
		LIMIT $%d OFFSET $%d`, qualifiedWhere, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("GetByDistrict failed: %w", err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		e := &domain.AuditEvent{}
		err := rows.Scan(
			&e.ID, &e.AuditReference, &e.AccountID, &e.DistrictID, &e.AnomalyFlagID,
			&e.Status, &e.AssignedOfficerID, &e.GRAStatus,
			&e.GWLBilledGHS, &e.ShadowBillGHS, &e.VariancePct,
			&e.ConfirmedLossGHS, &e.SuccessFeeGHS,
			&e.GRASDCId, &e.GRAQRCodeURL, &e.GRASignedAt,
			&e.IsLocked, &e.CreatedAt, &e.UpdatedAt,
			&e.AccountNumber, &e.AccountHolder, &e.DistrictName,
		)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}

	return events, total, rows.Err()
}

// UpdateStatus updates the status of an audit event
func (r *AuditEventRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.q(ctx).Exec(ctx,
		"UPDATE audit_events SET status = $1::audit_status, updated_at = NOW() WHERE id = $2 AND is_locked = FALSE",
		status, id)
	return err
}

// LockAudit locks an audit event after GRA signing (immutable)
func (r *AuditEventRepository) LockAudit(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE audit_events
		SET is_locked = TRUE, locked_at = NOW(), lock_reason = $1, updated_at = NOW()
		WHERE id = $2`, reason, id)
	return err
}

// UpdateGRAStatus updates GRA compliance status and QR code details.
// Also propagates GRA_SIGNED status to the linked revenue_recovery_event
// (belt-and-suspenders alongside the DB trigger trg_gra_sign_recovery).
func (r *AuditEventRepository) UpdateGRAStatus(ctx context.Context, id uuid.UUID, sdcID, qrCodeURL string) error {
	// Step 1: Update the audit event
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE audit_events
		SET gra_status     = 'SIGNED',
		    gra_sdc_id     = $1,
		    gra_qr_code_url = $2,
		    gra_signed_at  = NOW(),
		    updated_at     = NOW()
		WHERE id = $3`, sdcID, qrCodeURL, id)
	if err != nil {
		return err
	}

	// Step 2: Propagate GRA_SIGNED to the linked revenue_recovery_event.
	// The DB trigger trg_gra_sign_recovery also does this, but we do it here
	// too so the pipeline view updates immediately within the same transaction.
	_, _ = r.q(ctx).Exec(ctx, `
		UPDATE revenue_recovery_events rre
		SET status     = 'GRA_SIGNED',
		    updated_at = NOW()
		FROM audit_events ae
		WHERE ae.id              = $1
		  AND rre.anomaly_flag_id = ae.anomaly_flag_id
		  AND rre.status IN ('PENDING', 'FIELD_VERIFIED', 'CONFIRMED')`,
		id,
	)
	return nil
}

// GetDashboardStats returns high-level statistics for the dashboard
func (r *AuditEventRepository) GetDashboardStats(ctx context.Context, districtID *uuid.UUID) (map[string]interface{}, error) {
	args := []interface{}{}
	where := "1=1"
	if districtID != nil {
		where = "district_id = $1"
		args = append(args, *districtID)
	}

	stats := make(map[string]interface{})

	row := r.q(ctx).QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'PENDING') AS pending,
			COUNT(*) FILTER (WHERE status = 'IN_PROGRESS') AS in_progress,
			COUNT(*) FILTER (WHERE status = 'COMPLETED') AS completed,
			COUNT(*) FILTER (WHERE gra_status = 'SIGNED') AS gra_signed,
			COALESCE(SUM(confirmed_loss_ghs), 0) AS total_confirmed_loss,
			COALESCE(SUM(success_fee_ghs), 0) AS total_success_fees
		FROM audit_events WHERE %s`, where), args...,
	)
	var total, pending, inProgress, completed, graSigned int64
	var totalLoss, totalFees float64
	err := row.Scan(&total, &pending, &inProgress, &completed, &graSigned, &totalLoss, &totalFees)
	stats["total"] = total
	stats["pending"] = pending
	stats["in_progress"] = inProgress
	stats["completed"] = completed
	stats["gra_signed"] = graSigned
	stats["total_confirmed_loss_ghs"] = totalLoss
	stats["total_success_fees_ghs"] = totalFees

	// FE-DASH-02 fix: also count field jobs awaiting assignment (status = QUEUED).
	// The admin dashboard "Pending Assignment" card reads this field.
	// A separate query is used because field_jobs is a different table.
	fjArgs := []interface{}{}
	fjWhere := "status = 'QUEUED'"
	if districtID != nil {
		fjWhere += " AND district_id = $1"
		fjArgs = append(fjArgs, *districtID)
	}
	var pendingAssignment int64
	_ = r.q(ctx).QueryRow(ctx, fmt.Sprintf(
		"SELECT COUNT(*) FROM field_jobs WHERE %s", fjWhere), fjArgs...,
	).Scan(&pendingAssignment)
	stats["pending_assignment"] = pendingAssignment

	return stats, err
}

// IllegalConnectionReport holds the data for a new illegal connection report.
// Mirrors the request body in audit_handler.go ReportIllegalConnection.
type IllegalConnectionReport struct {
	OfficerID                string
	DistrictID               string   // SEC-H01 fix: district of the reporting officer
	JobID                    string
	ConnectionType           string
	Severity                 string
	Description              string
	EstimatedDailyLossLitres float64
	Address                  string
	AccountNumber            string
	Latitude                 float64
	Longitude                float64
	GPSAccuracy              float64
	PhotoCount               int
	PhotoHashes              []string
}

// CreateIllegalConnection inserts a new illegal connection report and returns its UUID.
// Uses r.q(ctx) so the INSERT runs inside the RLS-activated transaction.
func (r *AuditEventRepository) CreateIllegalConnection(ctx context.Context, rep *IllegalConnectionReport) (uuid.UUID, error) {
	var reportID uuid.UUID
	// SEC-H01 fix: include district_id so RLS policy can enforce district isolation.
	// district_id is populated from the reporting officer's JWT district claim.
	err := r.q(ctx).QueryRow(ctx, `
		INSERT INTO illegal_connections (
			officer_id, district_id, job_id, connection_type, severity, description,
			estimated_daily_loss_litres, address, account_number,
			latitude, longitude, gps_accuracy, photo_count, photo_hashes, reported_at
		) VALUES (
			$1::uuid, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13, $14, NOW()
		) RETURNING id`,
		rep.OfficerID, rep.DistrictID, rep.JobID, rep.ConnectionType, rep.Severity, rep.Description,
		rep.EstimatedDailyLossLitres, rep.Address, rep.AccountNumber,
		rep.Latitude, rep.Longitude, rep.GPSAccuracy, rep.PhotoCount, rep.PhotoHashes,
	).Scan(&reportID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("AuditEventRepository.CreateIllegalConnection: %w", err)
	}
	return reportID, nil
}

// ─── RLS-Activated Variants ───────────────────────────────────────────────────
// These methods accept a pgx.Tx that has already had SET LOCAL rls.* executed
// (via the rls.BeginReadOnlyTx helper). They enforce Row-Level Security by
// running queries inside the RLS-activated transaction.

// GetByDistrictTx is identical to GetByDistrict but runs inside an RLS transaction.
// Use this for all user-facing list operations to enforce district isolation.
func (r *AuditEventRepository) GetByDistrictTx(
	ctx context.Context,
	q Querier,
	districtID interface{},
	status, graStatus string,
	limit, offset int,
) ([]*domain.AuditEvent, int, error) {
	args := []interface{}{districtID}
	where := "district_id = $1"
	argIdx := 2

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d::audit_status", argIdx)
		args = append(args, status)
		argIdx++
	}
	if graStatus != "" {
		where += fmt.Sprintf(" AND gra_status = $%d::gra_compliance_status", argIdx)
		args = append(args, graStatus)
		argIdx++
	}

	var total int
	q.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM audit_events WHERE %s", where), args...).Scan(&total)

	args = append(args, limit, offset)
	rows, err := q.Query(ctx, fmt.Sprintf(`
		SELECT id, audit_reference, account_id, district_id, anomaly_flag_id,
		       status, assigned_officer_id, gra_status,
		       gwl_billed_ghs, shadow_bill_ghs, variance_pct,
		       is_locked, created_at, updated_at
		FROM audit_events WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("GetByDistrictTx failed: %w", err)
	}
	defer rows.Close()

	var events []*domain.AuditEvent
	for rows.Next() {
		e := &domain.AuditEvent{}
		err := rows.Scan(
			&e.ID, &e.AuditReference, &e.AccountID, &e.DistrictID, &e.AnomalyFlagID,
			&e.Status, &e.AssignedOfficerID, &e.GRAStatus,
			&e.GWLBilledGHS, &e.ShadowBillGHS, &e.VariancePct,
			&e.IsLocked, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}
