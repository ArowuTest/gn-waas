package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// Create creates a new audit event with auto-generated reference number
func (r *AuditEventRepository) Create(ctx context.Context, event *domain.AuditEvent) (*domain.AuditEvent, error) {
	// Generate audit reference: AUD-2026-XXXXXX
	var ref string
	err := r.db.QueryRow(ctx, `
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
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`

	err = r.db.QueryRow(ctx, query,
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
	err := r.db.QueryRow(ctx, query, id).Scan(
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

// GetByDistrict returns audit events for a district with pagination
func (r *AuditEventRepository) GetByDistrict(ctx context.Context, districtID uuid.UUID, status string, limit, offset int) ([]*domain.AuditEvent, int, error) {
	args := []interface{}{districtID}
	where := "district_id = $1"
	argIdx := 2

	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	r.db.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM audit_events WHERE %s", where), args...).Scan(&total)

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, audit_reference, account_id, district_id, anomaly_flag_id,
		       status, assigned_officer_id, gra_status,
		       gwl_billed_ghs, shadow_bill_ghs, variance_pct,
		       is_locked, created_at, updated_at
		FROM audit_events WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1), args...)
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
			&e.IsLocked, &e.CreatedAt, &e.UpdatedAt,
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
	_, err := r.db.Exec(ctx,
		"UPDATE audit_events SET status = $1, updated_at = NOW() WHERE id = $2 AND is_locked = FALSE",
		status, id)
	return err
}

// LockAudit locks an audit event after GRA signing (immutable)
func (r *AuditEventRepository) LockAudit(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE audit_events
		SET is_locked = TRUE, locked_at = NOW(), lock_reason = $1, updated_at = NOW()
		WHERE id = $2`, reason, id)
	return err
}

// UpdateGRAStatus updates GRA compliance status and QR code details
func (r *AuditEventRepository) UpdateGRAStatus(ctx context.Context, id uuid.UUID, sdcID, qrCodeURL string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE audit_events
		SET gra_status = 'SIGNED', gra_sdc_id = $1, gra_qr_code_url = $2,
		    gra_signed_at = NOW(), updated_at = NOW()
		WHERE id = $3`, sdcID, qrCodeURL, id)
	return err
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

	row := r.db.QueryRow(ctx, fmt.Sprintf(`
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

	return stats, err
}
