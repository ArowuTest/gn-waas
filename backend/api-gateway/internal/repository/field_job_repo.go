package repository

import (
	"context"
	"encoding/json"
	"strings"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"go.uber.org/zap"
)

// FieldJobRepository handles field job data access
type FieldJobRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewFieldJobRepository(db *pgxpool.Pool, logger *zap.Logger) *FieldJobRepository {
	return &FieldJobRepository{db: db, logger: logger}
}

// q returns the Querier for this request (RLS tx if present, else pool).
func (r *FieldJobRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *FieldJobRepository) Create(ctx context.Context, job *domain.FieldJob) (*domain.FieldJob, error) {
	// Generate job reference: FJ-2026-XXXXXX
	job.JobReference = fmt.Sprintf("FJ-%s-%s", time.Now().Format("2006"), uuid.New().String()[:6])

	query := `
		INSERT INTO field_jobs (
			job_reference, audit_event_id, account_id, district_id,
			assigned_officer_id, status, is_blind_audit,
			target_gps_lat, target_gps_lng, gps_fence_radius_m,
			priority, requires_security_escort, notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at`

	err := r.q(ctx).QueryRow(ctx, query,
		job.JobReference, job.AuditEventID, job.AccountID, job.DistrictID,
		job.AssignedOfficerID, job.Status, job.IsBlindAudit,
		job.TargetGPSLat, job.TargetGPSLng, job.GPSFenceRadiusM,
		job.Priority, job.RequiresSecurityEscort, job.Notes,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("Create field job failed: %w", err)
	}

	r.logger.Info("Field job created",
		zap.String("id", job.ID.String()),
		zap.String("reference", job.JobReference),
	)

	return job, nil
}

func (r *FieldJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.FieldJob, error) {
	query := `
		SELECT id, job_reference, audit_event_id, account_id, district_id,
		       assigned_officer_id, status, is_blind_audit,
		       target_gps_lat, target_gps_lng, gps_fence_radius_m,
		       dispatched_at, arrived_at, completed_at,
		       officer_gps_lat, officer_gps_lng, gps_verified,
		       biometric_verified, priority, requires_security_escort,
		       sos_triggered, sos_triggered_at, notes, created_at, updated_at
		FROM field_jobs WHERE id = $1`

	job := &domain.FieldJob{}
	err := r.q(ctx).QueryRow(ctx, query, id).Scan(
		&job.ID, &job.JobReference, &job.AuditEventID, &job.AccountID, &job.DistrictID,
		&job.AssignedOfficerID, &job.Status, &job.IsBlindAudit,
		&job.TargetGPSLat, &job.TargetGPSLng, &job.GPSFenceRadiusM,
		&job.DispatchedAt, &job.ArrivedAt, &job.CompletedAt,
		&job.OfficerGPSLat, &job.OfficerGPSLng, &job.GPSVerified,
		&job.BiometricVerified, &job.Priority, &job.RequiresSecurityEscort,
		&job.SOSTriggered, &job.SOSTriggeredAt, &job.Notes, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("field job %s not found", id)
		}
		return nil, fmt.Errorf("GetByID field job failed: %w", err)
	}
	return job, nil
}

func (r *FieldJobRepository) GetByOfficer(ctx context.Context, officerID uuid.UUID, status string) ([]*domain.FieldJob, error) {
	args := []interface{}{officerID}
	where := "assigned_officer_id = $1"
	if status != "" {
		where += " AND status = $2::field_job_status"
		args = append(args, status)
	}

	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT id, job_reference, audit_event_id, account_id, district_id,
		       assigned_officer_id, status, is_blind_audit,
		       target_gps_lat, target_gps_lng, gps_fence_radius_m,
		       dispatched_at, priority, requires_security_escort, created_at, updated_at
		FROM field_jobs WHERE %s ORDER BY priority DESC, created_at ASC`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*domain.FieldJob
	for rows.Next() {
		job := &domain.FieldJob{}
		err := rows.Scan(
			&job.ID, &job.JobReference, &job.AuditEventID, &job.AccountID, &job.DistrictID,
			&job.AssignedOfficerID, &job.Status, &job.IsBlindAudit,
			&job.TargetGPSLat, &job.TargetGPSLng, &job.GPSFenceRadiusM,
			&job.DispatchedAt, &job.Priority, &job.RequiresSecurityEscort,
			&job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *FieldJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, officerLat, officerLng *float64) error {
	now := time.Now()
	var arrivedAt, completedAt *time.Time

	switch status {
	case "ON_SITE":
		arrivedAt = &now
	case "COMPLETED":
		completedAt = &now
	}

	_, err := r.q(ctx).Exec(ctx, `
		UPDATE field_jobs
		SET status = $1::field_job_status,
		    arrived_at = COALESCE($2, arrived_at),
		    completed_at = COALESCE($3, completed_at),
		    officer_gps_lat = COALESCE($4, officer_gps_lat),
		    officer_gps_lng = COALESCE($5, officer_gps_lng),
		    updated_at = NOW()
		WHERE id = $6`,
		status, arrivedAt, completedAt, officerLat, officerLng, id)
	return err
}

func (r *FieldJobRepository) TriggerSOS(ctx context.Context, id uuid.UUID, officerLat, officerLng float64) error {
	now := time.Now()
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE field_jobs
		SET sos_triggered = TRUE, sos_triggered_at = $1,
		    officer_gps_lat = $2, officer_gps_lng = $3, updated_at = NOW()
		WHERE id = $4`, now, officerLat, officerLng, id)
	return err
}

// UserRepository handles user data access
type UserRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewUserRepository(db *pgxpool.Pool, logger *zap.Logger) *UserRepository {
	return &UserRepository{db: db, logger: logger}
}

// q returns the Querier for this request (RLS tx if present, else pool).
func (r *UserRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user := &domain.User{}
	err := r.q(ctx).QueryRow(ctx, `
		SELECT id, email, full_name, phone_number, role, status,
		       organisation, employee_id, district_id, keycloak_id, last_login_at, created_at, updated_at
		FROM users WHERE id = $1`, id,
	).Scan(
		&user.ID, &user.Email, &user.FullName, &user.PhoneNumber, &user.Role, &user.Status,
		&user.Organisation, &user.EmployeeID, &user.DistrictID, &user.KeycloakID,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByID user failed: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.q(ctx).QueryRow(ctx, `
		SELECT id, email, full_name, phone_number, role, status,
		       organisation, employee_id, district_id, keycloak_id, last_login_at, created_at, updated_at
		FROM users WHERE email = $1`, email,
	).Scan(
		&user.ID, &user.Email, &user.FullName, &user.PhoneNumber, &user.Role, &user.Status,
		&user.Organisation, &user.EmployeeID, &user.DistrictID, &user.KeycloakID,
		&user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByEmail user failed: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetFieldOfficers(ctx context.Context, districtID *uuid.UUID) ([]*domain.User, error) {
	args := []interface{}{"FIELD_OFFICER"}
	where := "role = $1::user_role AND status = 'ACTIVE'::user_status"
	if districtID != nil {
		where += " AND district_id = $2"
		args = append(args, *districtID)
	}

	rows, err := r.q(ctx).Query(ctx, fmt.Sprintf(`
		SELECT id, email, full_name, phone_number, role, status,
		       organisation, employee_id, district_id, created_at, updated_at
		FROM users WHERE %s ORDER BY full_name`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		err := rows.Scan(
			&u.ID, &u.Email, &u.FullName, &u.PhoneNumber, &u.Role, &u.Status,
			&u.Organisation, &u.EmployeeID, &u.DistrictID, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, "UPDATE users SET last_login_at = NOW() WHERE id = $1", id)
	return err
}

// DistrictRepository handles district data access
type DistrictRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewDistrictRepository(db *pgxpool.Pool, logger *zap.Logger) *DistrictRepository {
	return &DistrictRepository{db: db, logger: logger}
}

// q returns the Querier for this request (RLS tx if present, else pool).
func (r *DistrictRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *DistrictRepository) GetAll(ctx context.Context) ([]*domain.District, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, district_code, district_name, region,
		       population_estimate, total_connections, supply_status, zone_type,
		       geographic_zone, loss_ratio_pct, data_confidence_grade, is_pilot_district, is_active, created_at
		FROM districts WHERE is_active = TRUE ORDER BY district_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var districts []*domain.District
	for rows.Next() {
		d := &domain.District{}
		err := rows.Scan(
			&d.ID, &d.DistrictCode, &d.DistrictName, &d.Region,
			&d.PopulationEstimate, &d.TotalConnections, &d.SupplyStatus, &d.ZoneType,
			&d.GeographicZone, &d.LossRatioPct, &d.DataConfidenceGrade, &d.IsPilotDistrict, &d.IsActive, &d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		districts = append(districts, d)
	}
	return districts, rows.Err()
}

func (r *DistrictRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.District, error) {
	d := &domain.District{}
	err := r.q(ctx).QueryRow(ctx, `
		SELECT id, district_code, district_name, region,
		       population_estimate, total_connections, supply_status, zone_type,
		       geographic_zone, loss_ratio_pct, data_confidence_grade, is_pilot_district, is_active, created_at
		FROM districts WHERE id = $1`, id,
	).Scan(
		&d.ID, &d.DistrictCode, &d.DistrictName, &d.Region,
		&d.PopulationEstimate, &d.TotalConnections, &d.SupplyStatus, &d.ZoneType,
		&d.GeographicZone, &d.LossRatioPct, &d.DataConfidenceGrade, &d.IsPilotDistrict, &d.IsActive, &d.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByID district failed: %w", err)
	}
	return d, nil
}

// SystemConfigRepository handles system configuration data access
type SystemConfigRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewSystemConfigRepository(db *pgxpool.Pool, logger *zap.Logger) *SystemConfigRepository {
	return &SystemConfigRepository{db: db, logger: logger}
}

// q returns the Querier for this request (RLS tx if present, else pool).
func (r *SystemConfigRepository) q(ctx context.Context) Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

func (r *SystemConfigRepository) GetByKey(ctx context.Context, key string) (*domain.SystemConfig, error) {
	cfg := &domain.SystemConfig{}
	err := r.q(ctx).QueryRow(ctx, `
		SELECT id, config_key, config_value, config_type, description, category, updated_at
		FROM system_config WHERE config_key = $1`, key,
	).Scan(&cfg.ID, &cfg.ConfigKey, &cfg.ConfigValue, &cfg.ConfigType, &cfg.Description, &cfg.Category, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("GetByKey config failed: %w", err)
	}
	return cfg, nil
}

func (r *SystemConfigRepository) GetByCategory(ctx context.Context, category string) ([]*domain.SystemConfig, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT id, config_key, config_value, config_type, description, category, updated_at
		FROM system_config WHERE category = $1 ORDER BY config_key`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*domain.SystemConfig
	for rows.Next() {
		cfg := &domain.SystemConfig{}
		err := rows.Scan(&cfg.ID, &cfg.ConfigKey, &cfg.ConfigValue, &cfg.ConfigType, &cfg.Description, &cfg.Category, &cfg.UpdatedAt)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, rows.Err()
}

func (r *SystemConfigRepository) Update(ctx context.Context, key, value string, updatedBy uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE system_config
		SET config_value = $1, updated_at = NOW(), updated_by = $2
		WHERE config_key = $3`, value, updatedBy, key)
	return err
}

// WriteEvidence writes field officer evidence to the linked audit_event record.
// Called after SubmitJobEvidence to persist OCR reading, GPS, and photo hashes.
func (r *FieldJobRepository) WriteEvidence(ctx context.Context, jobID uuid.UUID, ev *domain.FieldJobEvidence) error {
	photoURLsJSON, _ := json.Marshal(ev.PhotoURLs)
	photoHashesJSON, _ := json.Marshal(ev.PhotoHashes)

	_, err := r.q(ctx).Exec(ctx, `
		UPDATE audit_events
		SET
			ocr_reading_value       = $1,
			ocr_status              = $2::ocr_status,
			gps_latitude            = $3,
			gps_longitude           = $4,
			gps_precision_m         = $5,
			meter_photo_url         = COALESCE(($6::jsonb)->0, NULL)::text,
			evidence_data           = evidence_data ||
				jsonb_build_object(
					'photo_urls',    $6::jsonb,
					'photo_hashes',  $7::jsonb,
					'ocr_confidence', $8,
					'officer_notes',  $9
				),
			updated_at              = NOW()
		WHERE field_job_id = $10`,
		ev.OCRReadingValue,
		ev.OCRStatus,
		ev.GPSLat,
		ev.GPSLng,
		ev.GPSAccuracyM,
		string(photoURLsJSON),
		string(photoHashesJSON),
		ev.OCRConfidence,
		ev.Notes,
		jobID,
	)
	return err
}

// ListAll returns all field jobs with optional filters for admin/supervisor view.
func (r *FieldJobRepository) ListAll(ctx context.Context, status, alertLevel, districtID string) ([]*EnrichedFieldJob, error) {
	query := `
		SELECT
			fj.id, fj.job_reference, fj.audit_event_id, fj.account_id, fj.district_id,
			fj.assigned_officer_id, fj.status, fj.is_blind_audit,
			fj.target_gps_lat, fj.target_gps_lng, fj.gps_fence_radius_m,
			fj.dispatched_at, fj.arrived_at, fj.completed_at,
			fj.officer_gps_lat, fj.officer_gps_lng, fj.gps_verified,
			fj.biometric_verified, fj.priority, fj.requires_security_escort,
			fj.sos_triggered, fj.sos_triggered_at, fj.notes, fj.created_at, fj.updated_at,
			COALESCE(wa.account_holder_name, 'Unknown Customer') AS customer_name,
			COALESCE(wa.gwl_account_number, '')                  AS account_number,
			COALESCE(wa.address_line1, '')                       AS address,
			af.anomaly_type,
			af.alert_level::text,
			af.estimated_loss_ghs
		FROM field_jobs fj
		LEFT JOIN water_accounts wa ON wa.id = fj.account_id
		LEFT JOIN LATERAL (
			SELECT anomaly_type, alert_level, estimated_loss_ghs
			FROM anomaly_flags
			WHERE account_id = fj.account_id AND status = 'OPEN'
			ORDER BY created_at DESC LIMIT 1
		) af ON true
		WHERE ($1 = '' OR fj.status = $1::field_job_status)
		  AND ($2 = '' OR af.alert_level::text = $2)
		  AND ($3 = '' OR fj.district_id::text = $3)
		ORDER BY
			CASE fj.status WHEN 'SOS' THEN 0 WHEN 'ON_SITE' THEN 1 WHEN 'EN_ROUTE' THEN 2
			WHEN 'DISPATCHED' THEN 3 WHEN 'QUEUED' THEN 4 ELSE 5 END,
			fj.priority DESC,
			fj.created_at DESC
		LIMIT 500`

	rows, err := r.q(ctx).Query(ctx, query, status, alertLevel, districtID)
	if err != nil {
		return nil, fmt.Errorf("ListAll field jobs failed: %w", err)
	}
	defer rows.Close()

	var jobs []*EnrichedFieldJob
	for rows.Next() {
		j := &EnrichedFieldJob{}
		if err := rows.Scan(
			&j.ID, &j.JobReference, &j.AuditEventID, &j.AccountID, &j.DistrictID,
			&j.AssignedOfficerID, &j.Status, &j.IsBlindAudit,
			&j.TargetGPSLat, &j.TargetGPSLng, &j.GPSFenceRadiusM,
			&j.DispatchedAt, &j.ArrivedAt, &j.CompletedAt,
			&j.OfficerGPSLat, &j.OfficerGPSLng, &j.GPSVerified,
			&j.BiometricVerified, &j.Priority, &j.RequiresSecurityEscort,
			&j.SOSTriggered, &j.SOSTriggeredAt, &j.Notes, &j.CreatedAt, &j.UpdatedAt,
			&j.AccountHolderName, &j.GWLAccountNumber, &j.AddressLine1,
			&j.AnomalyType, &j.AlertLevel, &j.EstimatedLossGHS,
		); err != nil {
			return nil, fmt.Errorf("ListAll scan failed: %w", err)
		}
		// Populate GPS aliases for Flutter/admin portal compatibility
		j.GpsLat = j.TargetGPSLat
		j.GpsLng = j.TargetGPSLng
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// AssignOfficer assigns a field officer to a job and sets status to DISPATCHED.
func (r *FieldJobRepository) AssignOfficer(ctx context.Context, jobID, officerID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx, `
		UPDATE field_jobs
		SET assigned_officer_id = $1,
		    status              = 'DISPATCHED',
		    dispatched_at       = NOW(),
		    updated_at          = NOW()
		WHERE id = $2`,
		officerID, jobID,
	)
	return err
}

// GetFloat64 returns a float64 config value by key, with a default fallback.
// Used by the /api/v1/config/mobile endpoint.
func (r *SystemConfigRepository) GetFloat64(ctx context.Context, key string, defaultVal float64) (float64, error) {
	cfg, err := r.GetByKey(ctx, key)
	if err != nil || cfg == nil {
		return defaultVal, nil
	}
	var v float64
	if _, err := fmt.Sscanf(cfg.ConfigValue, "%f", &v); err != nil {
		return defaultVal, nil
	}
	return v, nil
}

// GetString returns a string config value by key, with a default fallback.
func (r *SystemConfigRepository) GetString(ctx context.Context, key string, defaultVal string) (string, error) {
	cfg, err := r.GetByKey(ctx, key)
	if err != nil || cfg == nil {
		return defaultVal, nil
	}
	return cfg.ConfigValue, nil
}

// EnrichedFieldJob extends FieldJob with joined account data for the mobile app
type EnrichedFieldJob struct {
	domain.FieldJob
	AccountHolderName string   `json:"customer_name"`
	GWLAccountNumber  string   `json:"account_number"`
	AddressLine1      string   `json:"address"`
	AnomalyType       *string  `json:"anomaly_type,omitempty"`
	AlertLevel        *string  `json:"alert_level,omitempty"`
	EstimatedLossGHS  *float64 `json:"estimated_variance_ghs,omitempty"`
	// GPS aliases: Flutter and admin portal read gps_lat/gps_lng;
	// domain.FieldJob stores them as target_gps_lat/target_gps_lng.
	// These fields are populated after scanning so both names work.
	GpsLat float64 `json:"gps_lat"`
	GpsLng float64 `json:"gps_lng"`
}

// GetByOfficerEnriched returns field jobs for an officer with joined account data.
// This is the endpoint used by the mobile app.
func (r *FieldJobRepository) GetByOfficerEnriched(ctx context.Context, officerID uuid.UUID) ([]*EnrichedFieldJob, error) {
	rows, err := r.q(ctx).Query(ctx, `
		SELECT
			fj.id, fj.job_reference, fj.audit_event_id, fj.account_id, fj.district_id,
			fj.assigned_officer_id, fj.status, fj.is_blind_audit,
			fj.target_gps_lat, fj.target_gps_lng, fj.gps_fence_radius_m,
			fj.dispatched_at, fj.arrived_at, fj.completed_at,
			fj.officer_gps_lat, fj.officer_gps_lng, fj.gps_verified,
			fj.biometric_verified, fj.priority, fj.requires_security_escort,
			fj.sos_triggered, fj.sos_triggered_at, fj.notes, fj.created_at, fj.updated_at,
			-- Joined account data
			COALESCE(wa.account_holder_name, 'Unknown Customer')  AS customer_name,
			COALESCE(wa.gwl_account_number, '')                   AS account_number,
			COALESCE(wa.address_line1, '')                        AS address,
			-- Joined anomaly data (most recent open flag for this account)
			af.anomaly_type,
			af.alert_level,
			af.estimated_loss_ghs
		FROM field_jobs fj
		LEFT JOIN water_accounts wa ON wa.id = fj.account_id
		LEFT JOIN LATERAL (
			SELECT anomaly_type, alert_level::text, estimated_loss_ghs
			FROM anomaly_flags
			WHERE account_id = fj.account_id AND status = 'OPEN'
			ORDER BY created_at DESC
			LIMIT 1
		) af ON TRUE
		WHERE fj.assigned_officer_id = $1
		  AND fj.status NOT IN ('COMPLETED', 'CANCELLED')
		ORDER BY fj.priority ASC, fj.created_at ASC`,
		officerID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByOfficerEnriched failed: %w", err)
	}
	defer rows.Close()

	var jobs []*EnrichedFieldJob
	for rows.Next() {
		j := &EnrichedFieldJob{}
		err := rows.Scan(
			&j.ID, &j.JobReference, &j.AuditEventID, &j.AccountID, &j.DistrictID,
			&j.AssignedOfficerID, &j.Status, &j.IsBlindAudit,
			&j.TargetGPSLat, &j.TargetGPSLng, &j.GPSFenceRadiusM,
			&j.DispatchedAt, &j.ArrivedAt, &j.CompletedAt,
			&j.OfficerGPSLat, &j.OfficerGPSLng, &j.GPSVerified,
			&j.BiometricVerified, &j.Priority, &j.RequiresSecurityEscort,
			&j.SOSTriggered, &j.SOSTriggeredAt, &j.Notes, &j.CreatedAt, &j.UpdatedAt,
			&j.AccountHolderName, &j.GWLAccountNumber, &j.AddressLine1,
			&j.AnomalyType, &j.AlertLevel, &j.EstimatedLossGHS,
		)
		if err != nil {
			r.logger.Warn("Failed to scan enriched field job", zap.Error(err))
			continue
		}
		// Populate GPS aliases so Flutter (gps_lat/gps_lng) and admin portal
		// both receive the target coordinates under the expected JSON keys.
		j.GpsLat = j.TargetGPSLat
		j.GpsLng = j.TargetGPSLng
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// Create inserts a new district and returns its generated UUID.
// Uses r.q(ctx) so the INSERT runs inside the RLS-activated transaction.
func (r *DistrictRepository) Create(ctx context.Context, d *domain.District) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.q(ctx).QueryRow(ctx, `
		INSERT INTO districts
			(district_code, district_name, region, population_estimate,
			 total_connections, supply_status, zone_type, is_pilot_district, is_active)
		VALUES ($1,$2,$3,$4,$5,$6::supply_status,$7::district_zone_type,$8,$9)
		RETURNING id`,
		d.DistrictCode, d.DistrictName, d.Region,
		d.PopulationEstimate, d.TotalConnections,
		d.SupplyStatus, d.ZoneType,
		d.IsPilotDistrict, d.IsActive,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("DistrictRepository.Create: %w", err)
	}
	return id, nil
}

// UpdateFields applies a partial update to a district row.
// Only non-nil fields in the map are updated.
// Uses r.q(ctx) so the UPDATE runs inside the RLS-activated transaction.
func (r *DistrictRepository) UpdateFields(ctx context.Context, id uuid.UUID, fields map[string]interface{}) (int64, error) {
	if len(fields) == 0 {
		return 0, nil
	}
	// enumCasts maps column names that require explicit PostgreSQL enum casts.
	// Without these casts, pgx will reject string values for enum-typed columns.
	enumCasts := map[string]string{
		"supply_status": "::supply_status",
		"zone_type":          "::district_zone_type",
		"geographic_zone": "::geographic_zone_type",
	}
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	idx := 1
	for col, val := range fields {
		cast := enumCasts[col] // empty string if not an enum column
		setClauses = append(setClauses, fmt.Sprintf("%s=$%d%s", col, idx, cast))
		args = append(args, val)
		idx++
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE districts SET %s WHERE id=$%d",
		strings.Join(setClauses, ", "), idx)

	tag, err := r.q(ctx).Exec(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("DistrictRepository.UpdateFields: %w", err)
	}
	return tag.RowsAffected(), nil
}

// LogAdminAction writes an immutable audit trail entry for an admin mutation.
// Uses r.q(ctx) so the INSERT runs inside the RLS-activated transaction.
// Non-fatal: errors are returned but callers may choose to log-and-continue.
func (r *DistrictRepository) LogAdminAction(ctx context.Context, entityType, entityID, action, changedBy string, oldVal, newVal interface{}) error {
	oldJSON, _ := json.Marshal(oldVal)
	newJSON, _ := json.Marshal(newVal)
	_, err := r.q(ctx).Exec(ctx, `
		INSERT INTO audit_trail (entity_type, entity_id, action, changed_by, old_values, new_values)
		VALUES ($1, $2, $3, $4::uuid, $5, $6)`,
		entityType, entityID, action, changedBy, string(oldJSON), string(newJSON),
	)
	if err != nil {
		return fmt.Errorf("DistrictRepository.LogAdminAction: %w", err)
	}
	return nil
}

// ─── RLS-Activated Variants ───────────────────────────────────────────────────

// ListAllTx is identical to ListAll but runs inside an RLS-activated transaction.
func (r *FieldJobRepository) ListAllTx(ctx context.Context, q Querier, status, alertLevel, districtID string) ([]*EnrichedFieldJob, error) {
	query := `
		SELECT
			fj.id, fj.job_reference, fj.audit_event_id, fj.account_id, fj.district_id,
			fj.assigned_officer_id, fj.status, fj.is_blind_audit,
			fj.target_gps_lat, fj.target_gps_lng, fj.gps_fence_radius_m,
			fj.dispatched_at, fj.arrived_at, fj.completed_at,
			fj.officer_gps_lat, fj.officer_gps_lng, fj.gps_verified,
			fj.biometric_verified, fj.priority, fj.requires_security_escort,
			fj.sos_triggered, fj.sos_triggered_at, fj.notes, fj.created_at, fj.updated_at,
			COALESCE(wa.account_holder_name, 'Unknown Customer') AS customer_name,
			COALESCE(wa.gwl_account_number, '')                  AS account_number,
			COALESCE(wa.address_line1, '')                       AS address,
			af.anomaly_type,
			af.alert_level::text,
			af.estimated_loss_ghs
		FROM field_jobs fj
		LEFT JOIN water_accounts wa ON wa.id = fj.account_id
		LEFT JOIN LATERAL (
			SELECT anomaly_type, alert_level, estimated_loss_ghs
			FROM anomaly_flags
			WHERE account_id = fj.account_id AND status = 'OPEN'
			ORDER BY created_at DESC LIMIT 1
		) af ON true
		WHERE ($1 = '' OR fj.status = $1::field_job_status)
		  AND ($2 = '' OR af.alert_level::text = $2)
		  AND ($3 = '' OR fj.district_id::text = $3)
		ORDER BY
			CASE fj.status WHEN 'SOS' THEN 0 WHEN 'ON_SITE' THEN 1 WHEN 'EN_ROUTE' THEN 2
			WHEN 'DISPATCHED' THEN 3 WHEN 'QUEUED' THEN 4 ELSE 5 END,
			fj.priority DESC,
			fj.created_at DESC
		LIMIT 500`

	rows, err := q.Query(ctx, query, status, alertLevel, districtID)
	if err != nil {
		return nil, fmt.Errorf("ListAllTx field jobs failed: %w", err)
	}
	defer rows.Close()

	var jobs []*EnrichedFieldJob
	for rows.Next() {
		j := &EnrichedFieldJob{}
		if err := rows.Scan(
			&j.ID, &j.JobReference, &j.AuditEventID, &j.AccountID, &j.DistrictID,
			&j.AssignedOfficerID, &j.Status, &j.IsBlindAudit,
			&j.TargetGPSLat, &j.TargetGPSLng, &j.GPSFenceRadiusM,
			&j.DispatchedAt, &j.ArrivedAt, &j.CompletedAt,
			&j.OfficerGPSLat, &j.OfficerGPSLng, &j.GPSVerified,
			&j.BiometricVerified, &j.Priority, &j.RequiresSecurityEscort,
			&j.SOSTriggered, &j.SOSTriggeredAt, &j.Notes, &j.CreatedAt, &j.UpdatedAt,
			&j.AccountHolderName, &j.GWLAccountNumber, &j.AddressLine1,
			&j.AnomalyType, &j.AlertLevel, &j.EstimatedLossGHS,
		); err != nil {
			return nil, fmt.Errorf("ListAllTx scan failed: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// GetByOfficerEnrichedTx is identical to GetByOfficerEnriched but runs inside
// an RLS-activated transaction.
func (r *FieldJobRepository) GetByOfficerEnrichedTx(ctx context.Context, q Querier, officerID uuid.UUID) ([]*EnrichedFieldJob, error) {
	rows, err := q.Query(ctx, `
		SELECT
			fj.id, fj.job_reference, fj.audit_event_id, fj.account_id, fj.district_id,
			fj.assigned_officer_id, fj.status, fj.is_blind_audit,
			fj.target_gps_lat, fj.target_gps_lng, fj.gps_fence_radius_m,
			fj.dispatched_at, fj.arrived_at, fj.completed_at,
			fj.officer_gps_lat, fj.officer_gps_lng, fj.gps_verified,
			fj.biometric_verified, fj.priority, fj.requires_security_escort,
			fj.sos_triggered, fj.sos_triggered_at, fj.notes, fj.created_at, fj.updated_at,
			COALESCE(wa.account_holder_name, 'Unknown Customer') AS customer_name,
			COALESCE(wa.gwl_account_number, '')                  AS account_number,
			COALESCE(wa.address_line1, '')                       AS address,
			af.anomaly_type,
			af.alert_level::text,
			af.estimated_loss_ghs
		FROM field_jobs fj
		LEFT JOIN water_accounts wa ON wa.id = fj.account_id
		LEFT JOIN LATERAL (
			SELECT anomaly_type, alert_level, estimated_loss_ghs
			FROM anomaly_flags
			WHERE account_id = fj.account_id AND status = 'OPEN'
			ORDER BY created_at DESC LIMIT 1
		) af ON true
		WHERE fj.assigned_officer_id = $1
		ORDER BY
			CASE fj.status WHEN 'SOS' THEN 0 WHEN 'ON_SITE' THEN 1 WHEN 'EN_ROUTE' THEN 2
			WHEN 'DISPATCHED' THEN 3 WHEN 'QUEUED' THEN 4 ELSE 5 END,
			fj.priority DESC,
			fj.created_at DESC`,
		officerID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetByOfficerEnrichedTx failed: %w", err)
	}
	defer rows.Close()

	var jobs []*EnrichedFieldJob
	for rows.Next() {
		j := &EnrichedFieldJob{}
		if err := rows.Scan(
			&j.ID, &j.JobReference, &j.AuditEventID, &j.AccountID, &j.DistrictID,
			&j.AssignedOfficerID, &j.Status, &j.IsBlindAudit,
			&j.TargetGPSLat, &j.TargetGPSLng, &j.GPSFenceRadiusM,
			&j.DispatchedAt, &j.ArrivedAt, &j.CompletedAt,
			&j.OfficerGPSLat, &j.OfficerGPSLng, &j.GPSVerified,
			&j.BiometricVerified, &j.Priority, &j.RequiresSecurityEscort,
			&j.SOSTriggered, &j.SOSTriggeredAt, &j.Notes, &j.CreatedAt, &j.UpdatedAt,
			&j.AccountHolderName, &j.GWLAccountNumber, &j.AddressLine1,
			&j.AnomalyType, &j.AlertLevel, &j.EstimatedLossGHS,
		); err != nil {
			return nil, fmt.Errorf("GetByOfficerEnrichedTx scan failed: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

