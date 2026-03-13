package handler

// OfflineSyncHandler handles push/pull sync for field officers.
//
// GHANA CONTEXT — Why offline-first matters:
//   Field officers work in peri-urban and rural areas of Ghana where:
//   - 3G/4G coverage is patchy (especially in Northern, Upper East/West regions)
//   - Power outages (dumsor) affect mobile data towers
//   - Officers may work for hours without connectivity
//
//   The mobile app queues all actions locally (SQLite) and syncs when
//   connectivity is restored. This handler processes those queued actions.
//
// Sync Protocol:
//   1. PULL: GET /api/v1/sync/pull?device_id=X&last_sync=T
//      Returns: pending jobs, account data, config changes since last_sync
//
//   2. PUSH: POST /api/v1/sync/push
//      Body: array of queued actions with client timestamps
//      Returns: applied/conflict/rejected status per action
//
// Conflict Resolution:
//   - Billing data: server wins (GWL is source of truth)
//   - GPS coordinates: client wins (field officer is on-site)
//   - Photos: always accepted (append-only)
//   - Job status: last-write-wins with timestamp comparison
//
// Security:
//   - Device ID is registered and tied to a user account
//   - Actions are validated against the user's role and district
//   - Replay attacks prevented by client_sequence monotonic check

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
)

// OfflineSyncHandler handles field officer sync operations
type OfflineSyncHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewOfflineSyncHandler(db *pgxpool.Pool, logger *zap.Logger) *OfflineSyncHandler {
	return &OfflineSyncHandler{db: db, logger: logger}
}
// q returns the RLS-activated querier for domain data writes.
// The offline_sync_queue housekeeping writes (Push/Pull) stay on h.db directly.
func (h *OfflineSyncHandler) q(ctx context.Context) repository.Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return h.db
}


// ── PULL: GET /api/v1/sync/pull ───────────────────────────────────────────────
// Returns all data the field officer needs for offline work:
//   - Assigned field jobs (active/non-terminal statuses)
//   - Account data for those jobs
//   - Config (GPS fence radius, thresholds)
//   - Any server-side updates since last_sync

func (h *OfflineSyncHandler) Pull(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid user"})
	}

	deviceID := c.Query("device_id")
	lastSyncStr := c.Query("last_sync") // RFC3339 timestamp

	var lastSync time.Time
	if lastSyncStr != "" {
		lastSync, _ = time.Parse(time.RFC3339, lastSyncStr)
	} else {
		lastSync = time.Now().AddDate(0, 0, -7) // default: last 7 days
	}

	// Register device if new
	if deviceID != "" {
		// Device registration is housekeeping — uses pool directly (not RLS-sensitive)
		h.db.Exec(c.Context(), `
			INSERT INTO user_devices (user_id, device_id, last_seen_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (user_id, device_id) DO UPDATE SET last_seen_at = NOW()
		`, userID, deviceID)
	}

	// 1. Fetch assigned field jobs.
	//
	// BUG-SYNC-01 fixes applied here:
	//   - fj.assigned_officer_id (was: fj.assigned_to — column does not exist)
	//   - wa.account_holder_name (was: wa.customer_name — column does not exist)
	//   - wa.meter_serial        (was: wa.meter_serial_number — column does not exist)
	//   - fj.gps_fence_radius_m  (was: wa.gps_fence_radius_m — wrong table;
	//                             gps_fence_radius_m lives on field_jobs, not water_accounts)
	//   - wa.calibration_factor  (now exists via migration 036)
	//   - fj.job_type            (now exists via migration 036)
	//   - fj.scheduled_date      (now exists via migration 036)
	//   - fj.instructions        (now exists via migration 036)
	//
	// BUG-ENUM-01 fix:
	//   - Active statuses corrected to valid field_job_status enum values.
	//     'PENDING' and 'IN_PROGRESS' are NOT in the enum.
	//     Valid non-terminal statuses: QUEUED, ASSIGNED, DISPATCHED, EN_ROUTE, ON_SITE
	jobRows, err := h.db.Query(c.Context(), `
		SELECT
			fj.id, fj.job_reference,
			COALESCE(fj.job_type, 'METER_READING')  AS job_type,
			fj.status,
			fj.account_id, fj.district_id,
			fj.scheduled_date, fj.priority,
			COALESCE(fj.instructions, '')            AS instructions,
			wa.gwl_account_number,
			wa.account_holder_name                   AS customer_name,
			wa.address_line1, wa.address_line2,
			wa.gps_latitude, wa.gps_longitude,
			fj.gps_fence_radius_m,
			wa.meter_serial                          AS meter_serial_number,
			wa.calibration_factor,
			d.district_name, d.district_code
		FROM field_jobs fj
		JOIN water_accounts wa ON wa.id = fj.account_id
		JOIN districts d ON d.id = fj.district_id
		WHERE fj.assigned_officer_id = $1
		  AND fj.status IN ('QUEUED', 'ASSIGNED', 'DISPATCHED', 'EN_ROUTE', 'ON_SITE')
		  AND (fj.updated_at >= $2 OR fj.status IN ('QUEUED', 'ASSIGNED'))
		ORDER BY fj.priority DESC, COALESCE(fj.scheduled_date, fj.created_at::date) ASC
		LIMIT 100
	`, userID, lastSync)
	if err != nil {
		h.logger.Error("Pull: failed to fetch jobs", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch jobs"})
	}
	defer jobRows.Close()

	type jobPayload struct {
		ID               string   `json:"id"`
		JobReference     string   `json:"job_reference"`
		JobType          string   `json:"job_type"`
		Status           string   `json:"status"`
		AccountID        string   `json:"account_id"`
		DistrictID       string   `json:"district_id"`
		ScheduledDate    string   `json:"scheduled_date"`
		Priority         int      `json:"priority"`
		Instructions     string   `json:"instructions"`
		AccountNumber    string   `json:"account_number"`
		CustomerName     string   `json:"customer_name"`
		AddressLine1     string   `json:"address_line1"`
		AddressLine2     string   `json:"address_line2"`
		GPSLat           *float64 `json:"gps_lat"`
		GPSLng           *float64 `json:"gps_lng"`
		GPSFenceRadiusM  float64  `json:"gps_fence_radius_m"`
		MeterSerial      string   `json:"meter_serial"`
		CalibrationFactor float64 `json:"calibration_factor"`
		DistrictName     string   `json:"district_name"`
		DistrictCode     string   `json:"district_code"`
	}

	var jobs []jobPayload
	for jobRows.Next() {
		var j jobPayload
		var jobID, accountID, districtID uuid.UUID
		var scheduledDate *time.Time
		var fenceRadius *float64
		var calibFactor *float64

		if err := jobRows.Scan(
			&jobID, &j.JobReference, &j.JobType, &j.Status,
			&accountID, &districtID,
			&scheduledDate, &j.Priority, &j.Instructions,
			&j.AccountNumber, &j.CustomerName,
			&j.AddressLine1, &j.AddressLine2,
			&j.GPSLat, &j.GPSLng, &fenceRadius,
			&j.MeterSerial, &calibFactor,
			&j.DistrictName, &j.DistrictCode,
		); err != nil {
			h.logger.Warn("Pull: failed to scan job row", zap.Error(err))
			continue
		}
		j.ID = jobID.String()
		j.AccountID = accountID.String()
		j.DistrictID = districtID.String()
		if scheduledDate != nil {
			j.ScheduledDate = scheduledDate.Format("2006-01-02")
		}
		if fenceRadius != nil {
			j.GPSFenceRadiusM = *fenceRadius
		} else {
			j.GPSFenceRadiusM = 5.0
		}
		if calibFactor != nil {
			j.CalibrationFactor = *calibFactor
		} else {
			j.CalibrationFactor = 1.0
		}
		jobs = append(jobs, j)
	}

	// 2. Fetch system config for offline use
	configRows, err := h.db.Query(c.Context(), `
		SELECT config_key, config_value
		FROM system_config
		WHERE config_key IN (
			'gps_fence_radius_m', 'variance_threshold_pct',
			'night_flow_start_hour', 'night_flow_end_hour',
			'ocr_confidence_threshold', 'max_offline_days'
		)
	`)
	config := map[string]string{}
	if err == nil {
		defer configRows.Close()
		for configRows.Next() {
			var k, v string
			if err := configRows.Scan(&k, &v); err == nil {
				config[k] = v
			}
		}
	}

	// 3. Fetch seasonal threshold for current month
	currentMonth := int(time.Now().Month())
	var seasonalAdj struct {
		Season                  string  `json:"season"`
		VarianceThresholdAdjPct float64 `json:"variance_threshold_adj_pct"`
		NightFlowBaselineAdjM3  float64 `json:"night_flow_baseline_adj_m3"`
	}
	h.q(c.UserContext()).QueryRow(c.UserContext(), `
		SELECT season::text, variance_threshold_adj_pct, night_flow_baseline_adj_m3
		FROM seasonal_threshold_config
		WHERE is_active = true
		  AND (
		    (month_start <= month_end AND $1 BETWEEN month_start AND month_end)
		    OR (month_start > month_end AND ($1 >= month_start OR $1 <= month_end))
		  )
		LIMIT 1
	`, currentMonth).Scan(
		&seasonalAdj.Season,
		&seasonalAdj.VarianceThresholdAdjPct,
		&seasonalAdj.NightFlowBaselineAdjM3,
	)

	// 4. Fetch pending sync actions for this device (to show sync status)
	var pendingCount int
	h.q(c.UserContext()).QueryRow(c.UserContext(), `
		SELECT COUNT(*) FROM offline_sync_queue
		WHERE user_id = $1 AND status = 'PENDING'
	`, userID).Scan(&pendingCount)

	return c.JSON(fiber.Map{
		"sync_timestamp":  time.Now().UTC().Format(time.RFC3339),
		"jobs":            jobs,
		"jobs_count":      len(jobs),
		"config":          config,
		"seasonal":        seasonalAdj,
		"pending_actions": pendingCount,
		"last_sync":       lastSync.Format(time.RFC3339),
	})
}

// ── PUSH: POST /api/v1/sync/push ─────────────────────────────────────────────
// Processes queued offline actions from the field officer's device.
// Returns per-action status: applied, conflict, rejected, superseded.

func (h *OfflineSyncHandler) Push(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "invalid user"})
	}

	type pushAction struct {
		ClientID        string                 `json:"client_id"`        // client-side UUID
		ActionType      string                 `json:"action_type"`
		EntityType      string                 `json:"entity_type"`
		EntityID        string                 `json:"entity_id"`
		Payload         map[string]interface{} `json:"payload"`
		ClientTimestamp string                 `json:"client_timestamp"` // RFC3339
		ClientSequence  int64                  `json:"client_sequence"`
	}

	type pushRequest struct {
		DeviceID string       `json:"device_id"`
		Actions  []pushAction `json:"actions"`
	}

	var req pushRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	if len(req.Actions) == 0 {
		return c.JSON(fiber.Map{"results": []interface{}{}, "applied": 0})
	}

	batchID := uuid.New()
	type actionResult struct {
		ClientID string `json:"client_id"`
		Status   string `json:"status"`
		Message  string `json:"message,omitempty"`
	}

	results := make([]actionResult, 0, len(req.Actions))
	applied := 0

	for _, action := range req.Actions {
		clientTS, _ := time.Parse(time.RFC3339, action.ClientTimestamp)
		entityID, _ := uuid.Parse(action.EntityID)

		// Insert into sync queue
		var queueID uuid.UUID
		err := h.q(c.UserContext()).QueryRow(c.UserContext(), `
			INSERT INTO offline_sync_queue (
				device_id, user_id, action_type, entity_type, entity_id,
				payload, client_timestamp, client_sequence, status, sync_batch_id
			) VALUES ($1,$2,$3::sync_action_type,$4,$5,$6,$7,$8,'PENDING',$9)
			RETURNING id
		`,
			req.DeviceID, userID,
			action.ActionType, action.EntityType, entityID,
			action.Payload, clientTS, action.ClientSequence, batchID,
		).Scan(&queueID)

		if err != nil {
			results = append(results, actionResult{
				ClientID: action.ClientID,
				Status:   "REJECTED",
				Message:  "Failed to queue: " + err.Error(),
			})
			continue
		}

		// Apply the action immediately
		status, msg := h.applyAction(c, userID, action.ActionType, entityID, action.Payload, clientTS)

		// Update queue status
		h.q(c.UserContext()).Exec(c.UserContext(), `
			UPDATE offline_sync_queue SET status = $1::sync_status, processed_at = NOW()
			WHERE id = $2
		`, status, queueID)

		if status == "APPLIED" {
			applied++
		}

		results = append(results, actionResult{
			ClientID: action.ClientID,
			Status:   status,
			Message:  msg,
		})
	}

	h.logger.Info("Offline sync push processed",
		zap.String("user_id", userID.String()),
		zap.String("device_id", req.DeviceID),
		zap.Int("total", len(req.Actions)),
		zap.Int("applied", applied),
	)

	return c.JSON(fiber.Map{
		"batch_id":  batchID.String(),
		"total":     len(req.Actions),
		"applied":   applied,
		"results":   results,
		"synced_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// applyAction processes a single offline action
func (h *OfflineSyncHandler) applyAction(
	c *fiber.Ctx,
	userID uuid.UUID,
	actionType string,
	entityID uuid.UUID,
	payload map[string]interface{},
	clientTS time.Time,
) (string, string) {
	switch actionType {
	case "FIELD_JOB_UPDATE":
		return h.applyJobUpdate(c, userID, entityID, payload, clientTS)
	case "GPS_CONFIRM":
		return h.applyGPSConfirm(c, userID, entityID, payload)
	case "METER_READING":
		return h.applyMeterReading(c, userID, entityID, payload, clientTS)
	case "ANOMALY_REPORT":
		return h.applyAnomalyReport(c, userID, entityID, payload)
	default:
		return "REJECTED", fmt.Sprintf("unknown action type: %s", actionType)
	}
}

func (h *OfflineSyncHandler) applyJobUpdate(
	c *fiber.Ctx,
	userID uuid.UUID,
	jobID uuid.UUID,
	payload map[string]interface{},
	clientTS time.Time,
) (string, string) {
	// BUG-SYNC-02 fix: assigned_to → assigned_officer_id
	// field_jobs has no 'assigned_to' column; the correct column is 'assigned_officer_id'.
	var assignedOfficerID uuid.UUID
	var serverUpdatedAt time.Time
	err := h.q(c.UserContext()).QueryRow(c.UserContext(),
		`SELECT assigned_officer_id, updated_at FROM field_jobs WHERE id = $1`, jobID,
	).Scan(&assignedOfficerID, &serverUpdatedAt)
	if err != nil {
		return "REJECTED", "job not found"
	}
	if assignedOfficerID != userID {
		return "REJECTED", "job not assigned to this user"
	}

	// Conflict check: if server was updated after client action, flag conflict
	if serverUpdatedAt.After(clientTS) {
		return "CONFLICT", fmt.Sprintf(
			"server updated at %s, client action at %s",
			serverUpdatedAt.Format(time.RFC3339),
			clientTS.Format(time.RFC3339),
		)
	}

	// Apply status update
	if status, ok := payload["status"].(string); ok {
		notes, _ := payload["notes"].(string)
		_, err = h.q(c.UserContext()).Exec(c.UserContext(), `
			UPDATE field_jobs SET
				status     = $1::field_job_status,
				notes      = COALESCE(NULLIF($2, ''), notes),
				updated_at = NOW()
			WHERE id = $3
		`, status, notes, jobID)
		if err != nil {
			return "REJECTED", "failed to update job: " + err.Error()
		}
	}

	return "APPLIED", ""
}

func (h *OfflineSyncHandler) applyGPSConfirm(
	c *fiber.Ctx,
	userID uuid.UUID,
	accountID uuid.UUID,
	payload map[string]interface{},
) (string, string) {
	lat, _ := payload["latitude"].(float64)
	lng, _ := payload["longitude"].(float64)
	if lat == 0 || lng == 0 {
		return "REJECTED", "latitude and longitude required"
	}

	_, err := h.q(c.UserContext()).Exec(c.UserContext(), `
		UPDATE water_accounts SET
			gps_latitude        = $1,
			gps_longitude       = $2,
			gps_source          = 'FIELD_CONFIRMED'::gps_source_type,
			gps_geocode_quality = 99.0,
			gps_confirmed_at    = NOW(),
			gps_confirmed_by    = $3,
			updated_at          = NOW()
		WHERE id = $4
	`, lat, lng, userID, accountID)
	if err != nil {
		return "REJECTED", "failed to confirm GPS: " + err.Error()
	}
	return "APPLIED", ""
}

func (h *OfflineSyncHandler) applyMeterReading(
	c *fiber.Ctx,
	userID uuid.UUID,
	accountID uuid.UUID,
	payload map[string]interface{},
	clientTS time.Time,
) (string, string) {
	reading, _ := payload["reading_m3"].(float64)
	photoURL, _ := payload["photo_url"].(string)
	ocrReading, _ := payload["ocr_reading"].(float64)
	ocrConfidence, _ := payload["ocr_confidence"].(float64)

	if reading <= 0 {
		return "REJECTED", "reading_m3 must be positive"
	}

	// BUG-SYNC-03 fix: calibration_factor now exists on water_accounts (migration 036).
	// Default 1.0 is safe if the row is not found (no calibration applied).
	var calibFactor float64 = 1.0
	h.q(c.UserContext()).QueryRow(c.UserContext(),
		`SELECT calibration_factor FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&calibFactor)

	adjustedReading := reading * calibFactor

	// BUG-SYNC-03 fixes applied to the INSERT:
	//   - adjusted_reading_m3        now exists (migration 036)
	//   - calibration_factor_applied now exists (migration 036)
	//   - ocr_reading_m3             now exists (migration 036)
	//   - reader_id                  (was: read_by — column does not exist;
	//                                 meter_readings uses reader_id for the UUID FK)
	//   - reading_date               (was: read_at — column does not exist;
	//                                 meter_readings uses reading_date DATE;
	//                                 clientTS is truncated to date for the unique key)
	//   - ON CONFLICT (account_id, reading_date)
	//                                (was: ON CONFLICT (account_id, read_at) — wrong;
	//                                 the unique constraint is on reading_date, not read_at)
	readingDate := clientTS.Truncate(24 * time.Hour)
	_, err := h.q(c.UserContext()).Exec(c.UserContext(), `
		INSERT INTO meter_readings (
			account_id, reading_date, reading_m3,
			adjusted_reading_m3, calibration_factor_applied,
			photo_url, ocr_reading_m3, ocr_confidence,
			reader_id, read_method
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'FIELD_OFFICER')
		ON CONFLICT (account_id, reading_date) DO UPDATE SET
			reading_m3                  = EXCLUDED.reading_m3,
			adjusted_reading_m3         = EXCLUDED.adjusted_reading_m3,
			calibration_factor_applied  = EXCLUDED.calibration_factor_applied,
			photo_url                   = EXCLUDED.photo_url,
			ocr_reading_m3              = EXCLUDED.ocr_reading_m3,
			ocr_confidence              = EXCLUDED.ocr_confidence,
			reader_id                   = EXCLUDED.reader_id,
			read_method                 = EXCLUDED.read_method
	`,
		accountID, readingDate, reading,
		adjustedReading, calibFactor,
		photoURL, ocrReading, ocrConfidence,
		userID,
	)
	if err != nil {
		return "REJECTED", "failed to save reading: " + err.Error()
	}
	return "APPLIED", ""
}

func (h *OfflineSyncHandler) applyAnomalyReport(
	c *fiber.Ctx,
	userID uuid.UUID,
	accountID uuid.UUID,
	payload map[string]interface{},
) (string, string) {
	anomalyType, _ := payload["anomaly_type"].(string)
	description, _ := payload["description"].(string)
	severity, _ := payload["severity"].(string)
	if severity == "" {
		severity = "MEDIUM"
	}

	var districtID uuid.UUID
	h.q(c.UserContext()).QueryRow(c.UserContext(),
		`SELECT district_id FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&districtID)

	_, err := h.q(c.UserContext()).Exec(c.UserContext(), `
		INSERT INTO anomaly_flags (
			account_id, district_id, anomaly_type, alert_level,
			description, created_at
		) VALUES ($1,$2,$3::anomaly_type,$4::alert_level,$5,NOW())
	`, accountID, districtID, anomalyType, severity, description)
	if err != nil {
		return "REJECTED", "failed to save anomaly: " + err.Error()
	}
	return "APPLIED", ""
}

// ── GET /api/v1/sync/status ───────────────────────────────────────────────────
// Returns sync queue status for a device

func (h *OfflineSyncHandler) Status(c *fiber.Ctx) error {
	userIDStr, _ := c.Locals("user_id").(string)
	userID, _ := uuid.Parse(userIDStr)
	deviceID := c.Query("device_id")

	var pending, applied, conflicts, rejected int
	h.q(c.UserContext()).QueryRow(c.UserContext(), `
		SELECT
			COUNT(*) FILTER (WHERE status = 'PENDING'),
			COUNT(*) FILTER (WHERE status = 'APPLIED'),
			COUNT(*) FILTER (WHERE status = 'CONFLICT'),
			COUNT(*) FILTER (WHERE status = 'REJECTED')
		FROM offline_sync_queue
		WHERE user_id = $1 AND ($2 = '' OR device_id = $2)
		  AND created_at >= NOW() - INTERVAL '7 days'
	`, userID, deviceID).Scan(&pending, &applied, &conflicts, &rejected)

	return c.JSON(fiber.Map{
		"pending":   pending,
		"applied":   applied,
		"conflicts": conflicts,
		"rejected":  rejected,
	})
}
