package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

	err := r.db.QueryRow(ctx, query,
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
	err := r.db.QueryRow(ctx, query, id).Scan(
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
		where += " AND status = $2"
		args = append(args, status)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
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
	case "ARRIVED":
		arrivedAt = &now
	case "COMPLETED":
		completedAt = &now
	}

	_, err := r.db.Exec(ctx, `
		UPDATE field_jobs
		SET status = $1,
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
	_, err := r.db.Exec(ctx, `
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

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(ctx, `
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
	err := r.db.QueryRow(ctx, `
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
	where := "role = $1 AND status = 'ACTIVE'"
	if districtID != nil {
		where += " AND district_id = $2"
		args = append(args, *districtID)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
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
	_, err := r.db.Exec(ctx, "UPDATE users SET last_login_at = NOW() WHERE id = $1", id)
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

func (r *DistrictRepository) GetAll(ctx context.Context) ([]*domain.District, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, district_code, district_name, region,
		       population_estimate, total_connections, supply_status, zone_type,
		       loss_ratio_pct, data_confidence_grade, is_pilot_district, is_active, created_at
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
			&d.LossRatioPct, &d.DataConfidenceGrade, &d.IsPilotDistrict, &d.IsActive, &d.CreatedAt,
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
	err := r.db.QueryRow(ctx, `
		SELECT id, district_code, district_name, region,
		       population_estimate, total_connections, supply_status, zone_type,
		       loss_ratio_pct, data_confidence_grade, is_pilot_district, is_active, created_at
		FROM districts WHERE id = $1`, id,
	).Scan(
		&d.ID, &d.DistrictCode, &d.DistrictName, &d.Region,
		&d.PopulationEstimate, &d.TotalConnections, &d.SupplyStatus, &d.ZoneType,
		&d.LossRatioPct, &d.DataConfidenceGrade, &d.IsPilotDistrict, &d.IsActive, &d.CreatedAt,
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

func (r *SystemConfigRepository) GetByKey(ctx context.Context, key string) (*domain.SystemConfig, error) {
	cfg := &domain.SystemConfig{}
	err := r.db.QueryRow(ctx, `
		SELECT id, config_key, config_value, config_type, description, category, updated_at
		FROM system_config WHERE config_key = $1`, key,
	).Scan(&cfg.ID, &cfg.ConfigKey, &cfg.ConfigValue, &cfg.ConfigType, &cfg.Description, &cfg.Category, &cfg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("GetByKey config failed: %w", err)
	}
	return cfg, nil
}

func (r *SystemConfigRepository) GetByCategory(ctx context.Context, category string) ([]*domain.SystemConfig, error) {
	rows, err := r.db.Query(ctx, `
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
	_, err := r.db.Exec(ctx, `
		UPDATE system_config
		SET config_value = $1, updated_at = NOW(), updated_by = $2
		WHERE config_key = $3`, value, updatedBy, key)
	return err
}
