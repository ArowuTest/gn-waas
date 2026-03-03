package handler

import (
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// WorkforceHandler provides GPS breadcrumb tracking and workforce oversight.
// Field officers submit their location via the mobile app; the Admin Portal
// "Workforce Oversight" view uses this data to verify officers are physically
// visiting flagged properties (anti-desk-audit control).
type WorkforceHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewWorkforceHandler(db *pgxpool.Pool, logger *zap.Logger) *WorkforceHandler {
	return &WorkforceHandler{db: db, logger: logger}
}

// RecordLocation godoc
// POST /api/v1/workforce/location
// Called by the mobile app every 5 minutes while a field job is active.
func (h *WorkforceHandler) RecordLocation(c *fiber.Ctx) error {
	ctx := c.UserContext()

	officerID, ok := c.Locals("user_id").(string)
	if !ok || officerID == "" {
		return response.Unauthorized(c, "Officer ID not found in token")
	}

	var req struct {
		FieldJobID *string  `json:"field_job_id"`
		Latitude   float64  `json:"latitude"`
		Longitude  float64  `json:"longitude"`
		AccuracyM  *float64 `json:"accuracy_m"`
		DeviceID   string   `json:"device_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.Latitude == 0 && req.Longitude == 0 {
		return response.BadRequest(c, "INVALID_COORDS", "Latitude and longitude are required")
	}

	var jobID *uuid.UUID
	if req.FieldJobID != nil && *req.FieldJobID != "" {
		if id, err := uuid.Parse(*req.FieldJobID); err == nil {
			jobID = &id
		}
	}

	_, err := h.db.Exec(ctx, `
		INSERT INTO officer_gps_tracks
		    (officer_id, field_job_id, latitude, longitude, accuracy_m, device_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)`,
		officerID, jobID, req.Latitude, req.Longitude, req.AccuracyM, req.DeviceID,
	)
	if err != nil {
		h.logger.Error("RecordLocation failed", zap.Error(err))
		return response.InternalError(c, "Failed to record location")
	}
	return response.OK(c, fiber.Map{"recorded": true})
}

// GetOfficerTrack godoc
// GET /api/v1/workforce/officers/:id/track?from=&to=
// Returns GPS breadcrumbs for a specific officer in a time window.
func (h *WorkforceHandler) GetOfficerTrack(c *fiber.Ctx) error {
	ctx := c.UserContext()
	officerID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid officer ID")
	}

	from := time.Now().Add(-8 * time.Hour)
	to := time.Now()
	if f := c.Query("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = t
		}
	}
	if t := c.Query("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = parsed
		}
	}

	type Track struct {
		Latitude   float64    `json:"latitude"`
		Longitude  float64    `json:"longitude"`
		AccuracyM  *float64   `json:"accuracy_m,omitempty"`
		FieldJobID *uuid.UUID `json:"field_job_id,omitempty"`
		RecordedAt time.Time  `json:"recorded_at"`
	}

	rows, err := h.db.Query(ctx, `
		SELECT latitude, longitude, accuracy_m, field_job_id, recorded_at
		FROM officer_gps_tracks
		WHERE officer_id = $1
		  AND recorded_at BETWEEN $2 AND $3
		ORDER BY recorded_at ASC
		LIMIT 2000`,
		officerID, from, to,
	)
	if err != nil {
		h.logger.Error("GetOfficerTrack query failed", zap.Error(err))
		return response.InternalError(c, "Failed to load officer track")
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		var t Track
		if err := rows.Scan(&t.Latitude, &t.Longitude, &t.AccuracyM, &t.FieldJobID, &t.RecordedAt); err == nil {
			tracks = append(tracks, t)
		}
	}
	return response.OK(c, tracks)
}

// GetActiveOfficers godoc
// GET /api/v1/workforce/active?district_id=
// Returns all officers who have submitted a GPS ping in the last 30 minutes.
func (h *WorkforceHandler) GetActiveOfficers(c *fiber.Ctx) error {
	ctx := c.UserContext()

	districtFilter := c.Query("district_id")
	args := []interface{}{time.Now().Add(-30 * time.Minute)}
	districtJoin := ""
	districtWhere := ""
	if districtFilter != "" {
		if _, err := uuid.Parse(districtFilter); err == nil {
			districtJoin = " JOIN field_jobs fj ON t.field_job_id = fj.id"
			districtWhere = " AND fj.district_id = $2::uuid"
			args = append(args, districtFilter)
		}
	}

	type ActiveOfficer struct {
		OfficerID    uuid.UUID  `json:"officer_id"`
		FullName     string     `json:"full_name"`
		EmployeeID   string     `json:"employee_id"`
		Latitude     float64    `json:"latitude"`
		Longitude    float64    `json:"longitude"`
		FieldJobID   *uuid.UUID `json:"field_job_id,omitempty"`
		LastSeenAt   time.Time  `json:"last_seen_at"`
	}

	rows, err := h.db.Query(ctx, `
		SELECT DISTINCT ON (t.officer_id)
		       t.officer_id, u.full_name, u.employee_id,
		       t.latitude, t.longitude, t.field_job_id, t.recorded_at
		FROM officer_gps_tracks t
		JOIN users u ON t.officer_id = u.id`+districtJoin+`
		WHERE t.recorded_at >= $1`+districtWhere+`
		ORDER BY t.officer_id, t.recorded_at DESC`,
		args...,
	)
	if err != nil {
		h.logger.Error("GetActiveOfficers query failed", zap.Error(err))
		return response.InternalError(c, "Failed to load active officers")
	}
	defer rows.Close()

	var officers []ActiveOfficer
	for rows.Next() {
		var o ActiveOfficer
		if err := rows.Scan(
			&o.OfficerID, &o.FullName, &o.EmployeeID,
			&o.Latitude, &o.Longitude, &o.FieldJobID, &o.LastSeenAt,
		); err == nil {
			officers = append(officers, o)
		}
	}
	return response.OK(c, officers)
}

// GetWorkforceSummary godoc
// GET /api/v1/workforce/summary
// Returns aggregate workforce stats for the Admin Dashboard.
func (h *WorkforceHandler) GetWorkforceSummary(c *fiber.Ctx) error {
	ctx := c.UserContext()

	type Summary struct {
		TotalFieldOfficers  int `json:"total_field_officers"`
		ActiveNow           int `json:"active_now"`           // GPS ping < 30 min
		OnActiveJob         int `json:"on_active_job"`         // has open field_job
		IdleOfficers        int `json:"idle_officers"`
		JobsCompletedToday  int `json:"jobs_completed_today"`
	}

	var s Summary

	// Total field officers
	h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM users
		WHERE role = 'FIELD_OFFICER'::user_role AND status = 'ACTIVE'::user_status`,
	).Scan(&s.TotalFieldOfficers)

	// Active now (GPS ping in last 30 min)
	h.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT officer_id) FROM officer_gps_tracks
		WHERE recorded_at >= NOW() - INTERVAL '30 minutes'`,
	).Scan(&s.ActiveNow)

	// On active job
	h.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT assigned_to) FROM field_jobs
		WHERE status IN ('ASSIGNED','IN_PROGRESS') AND assigned_to IS NOT NULL`,
	).Scan(&s.OnActiveJob)

	// Jobs completed today
	h.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM field_jobs
		WHERE status = 'COMPLETED' AND DATE(updated_at) = CURRENT_DATE`,
	).Scan(&s.JobsCompletedToday)

	s.IdleOfficers = s.TotalFieldOfficers - s.ActiveNow
	if s.IdleOfficers < 0 {
		s.IdleOfficers = 0
	}

	return response.OK(c, s)
}
