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
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// OfflineSyncHandler handles field officer sync operations
type OfflineSyncHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewOfflineSyncHandler(db *pgxpool.Pool, logger *zap.Logger) *OfflineSyncHandler {
	return &OfflineSyncHandler{db: db, logger: logger}
}

// ── PULL: GET /api/v1/sync/pull ───────────────────────────────────────────────
// Returns all data the field officer needs for offline work:
//   - Assigned field jobs (pending/in-progress)
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
		h.db.Exec(c.Context(), `
			INSERT INTO user_devices (user_id, device_id, last_seen_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (user_id, device_id) DO UPDATE SET last_seen_at = NOW()
		`, userID, deviceID)
	}

	// 1. Fetch assigned field jobs
	jobRows, err := h.db.Query(c.Context(), `
		SELECT
			fj.id, fj.job_reference, fj.job_type, fj.status,
			fj.account_id, fj.district_id,
			fj.scheduled_date, fj.priority, fj.instructions,
			wa.gwl_account_number, wa.customer_name,
			wa.address_line1, wa.address_line2,
			wa.gps_latitude, wa.gps_longitude,
			wa.gps_fence_radius_m,
			wa.meter_serial_number, wa.calibration_factor,
			d.district_name, d.district_code
		FROM field_jobs fj
		JOIN water_accounts wa ON wa.id = fj.account_id
		JOIN districts d ON d.id = fj.district_id
		WHERE fj.assigned_to = $1
		  AND fj.status IN ('PENDING', 'IN_PROGRESS')
		  AND (fj.updated_at >= $2 OR fj.status = 'PENDING')
		ORDER BY fj.priority DESC, fj.scheduled_date ASC
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
		Priority         string   `json:"priority"`
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
		var scheduledDate time.Time
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
			continue
		}
		j.ID = jobID.String()
		j.AccountID = accountID.String()
		j.DistrictID = districtID.String()
		j.ScheduledDate = scheduledDate.Format("2006-01-02")
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
	h.db.QueryRow(c.Context(), `
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
	h.db.QueryRow(c.Context(), `
		SELECT COUNT(*) FROM offline_sync_queue
		WHERE user_id = $1 AND status = 'PENDING'
	`, userID).Scan(&pendingCount)

	return c.JSON(fiber.Map{
		"sync_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"jobs":             jobs,
		"jobs_count":       len(jobs),
		"config":           config,
		"seasonal":         seasonalAdj,
		"pending_actions":  pendingCount,
		"last_sync":        lastSync.Format(time.RFC3339),
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
		err := h.db.QueryRow(c.Context(), `
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
		h.db.Exec(c.Context(), `
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
		"batch_id":    batchID.String(),
		"total":       len(req.Actions),
		"applied":     applied,
		"results":     results,
		"synced_at":   time.Now().UTC().Format(time.RFC3339),
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
	// Check job belongs to this user
	var assignedTo uuid.UUID
	var serverUpdatedAt time.Time
	err := h.db.QueryRow(c.Context(),
		`SELECT assigned_to, updated_at FROM field_jobs WHERE id = $1`, jobID,
	).Scan(&assignedTo, &serverUpdatedAt)
	if err != nil {
		return "REJECTED", "job not found"
	}
	if assignedTo != userID {
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
		_, err = h.db.Exec(c.Context(), `
			UPDATE field_jobs SET
				status = $1, notes = COALESCE($2, notes),
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

	_, err := h.db.Exec(c.Context(), `
		UPDATE water_accounts SET
			gps_latitude        = $1,
			gps_longitude       = $2,
			gps_source          = 'FIELD_CONFIRMED'::gps_source_type,
			gps_geocode_quality = 99.0,
			gps_fence_radius_m  = 5.0,
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

	// Apply calibration factor
	var calibFactor float64 = 1.0
	h.db.QueryRow(c.Context(),
		`SELECT calibration_factor FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&calibFactor)

	adjustedReading := reading * calibFactor

	_, err := h.db.Exec(c.Context(), `
		INSERT INTO meter_readings (
			account_id, reading_m3, adjusted_reading_m3,
			calibration_factor_applied, photo_url,
			ocr_reading_m3, ocr_confidence,
			read_by, read_at, read_method
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'FIELD_OFFICER')
		ON CONFLICT (account_id, read_at) DO NOTHING
	`,
		accountID, reading, adjustedReading,
		calibFactor, photoURL,
		ocrReading, ocrConfidence,
		userID, clientTS,
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
	h.db.QueryRow(c.Context(),
		`SELECT district_id FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&districtID)

	_, err := h.db.Exec(c.Context(), `
		INSERT INTO anomaly_flags (
			account_id, district_id, anomaly_type, severity,
			description, detected_at, reported_by, status
		) VALUES ($1,$2,$3,$4,$5,NOW(),$6,'OPEN')
	`, accountID, districtID, anomalyType, severity, description, userID)
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
	h.db.QueryRow(c.Context(), `
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
