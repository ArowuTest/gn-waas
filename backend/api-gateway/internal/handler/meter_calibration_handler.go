package handler

// MeterCalibrationHandler tracks meter calibration history and degradation.
//
// GHANA CONTEXT — Why meter calibration matters:
//   Ghana's water meters are often old (10-20+ years), poorly maintained,
//   and subject to:
//   1. UNDER-REGISTRATION: Old meters spin slowly → GWL under-bills → NRW
//   2. OVER-REGISTRATION: Damaged meters over-count → customer over-billed
//   3. TAMPERING: Magnets used to slow meters (common fraud)
//   4. SAND/SEDIMENT: Ghana's water supply has high turbidity in rainy season
//      which clogs and damages meters faster than in developed countries
//
//   IWA/AWWA M36 requires tracking meter accuracy as part of Apparent Losses.
//   A calibration factor of 0.95 means the meter reads 5% low.
//   Adjusted reading = raw reading / calibration_factor
//
// Calibration Methods:
//   - BENCH_TEST: Meter removed and tested in lab (most accurate)
//   - FIELD_TEST: Portable test kit used on-site
//   - STATISTICAL: Inferred from billing patterns (least accurate)
//   - MANUFACTURER: Factory default (used for new meters)

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MeterCalibrationHandler manages meter calibration records
type MeterCalibrationHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewMeterCalibrationHandler(db *pgxpool.Pool, logger *zap.Logger) *MeterCalibrationHandler {
	return &MeterCalibrationHandler{db: db, logger: logger}
}

// ── POST /api/v1/admin/meters/:account_id/calibrations ───────────────────────
// Record a new calibration for a meter

func (h *MeterCalibrationHandler) RecordCalibration(c *fiber.Ctx) error {
	accountID, err := uuid.Parse(c.Params("account_id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid account_id"})
	}

	type calibReq struct {
		CalibrationDate   string  `json:"calibration_date"`   // YYYY-MM-DD
		CalibrationMethod string  `json:"calibration_method"` // BENCH_TEST, FIELD_TEST, etc.
		CalibrationFactor float64 `json:"calibration_factor"` // e.g. 0.95 = 5% under-reads
		MeterCondition    string  `json:"meter_condition"`    // GOOD, FAIR, POOR, FAILED
		MeterSerial       string  `json:"meter_serial"`
		TechnicianName    string  `json:"technician_name"`
		TechnicianID      string  `json:"technician_id"`
		Notes             string  `json:"notes"`
		NextCalibrationDue string `json:"next_calibration_due"` // YYYY-MM-DD
		// Test measurements
		ActualFlowM3      float64 `json:"actual_flow_m3"`
		MeterReadingM3    float64 `json:"meter_reading_m3"`
	}

	var req calibReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
	}

	// Validate calibration factor
	if req.CalibrationFactor <= 0 || req.CalibrationFactor > 2.0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "calibration_factor must be between 0.01 and 2.0",
		})
	}

	// Parse dates
	calibDate, err := time.Parse("2006-01-02", req.CalibrationDate)
	if err != nil {
		calibDate = time.Now()
	}

	var nextDue *time.Time
	if req.NextCalibrationDue != "" {
		t, err := time.Parse("2006-01-02", req.NextCalibrationDue)
		if err == nil {
			nextDue = &t
		}
	}

	// Calculate accuracy percentage
	var accuracyPct *float64
	if req.ActualFlowM3 > 0 && req.MeterReadingM3 > 0 {
		acc := (req.MeterReadingM3 / req.ActualFlowM3) * 100
		accuracyPct = &acc
	}

	userIDStr, _ := c.Locals("user_id").(string)
	userID, _ := uuid.Parse(userIDStr)

	var calibID uuid.UUID
	err = h.db.QueryRow(c.Context(), `
		INSERT INTO meter_calibrations (
			account_id, calibration_date, calibration_method,
			calibration_factor, meter_condition,
			meter_serial_number, technician_name, technician_id,
			actual_flow_m3, meter_reading_m3, accuracy_pct,
			notes, next_calibration_due, recorded_by
		) VALUES ($1,$2,$3,$4,$5::meter_condition,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id
	`,
		accountID, calibDate, req.CalibrationMethod,
		req.CalibrationFactor, req.MeterCondition,
		req.MeterSerial, req.TechnicianName, req.TechnicianID,
		req.ActualFlowM3, req.MeterReadingM3, accuracyPct,
		req.Notes, nextDue, userID,
	).Scan(&calibID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to record calibration: " + err.Error()})
	}

	// Update water_account with new calibration factor and active calibration
	_, err = h.db.Exec(c.Context(), `
		UPDATE water_accounts SET
			calibration_factor      = $1,
			active_calibration_id   = $2,
			meter_serial_number     = COALESCE(NULLIF($3,''), meter_serial_number),
			updated_at              = NOW()
		WHERE id = $4
	`, req.CalibrationFactor, calibID, req.MeterSerial, accountID)
	if err != nil {
		h.logger.Error("Failed to update account calibration factor", zap.Error(err))
	}

	// If meter condition is POOR or FAILED, create an anomaly flag
	if req.MeterCondition == "POOR" || req.MeterCondition == "FAILED" {
		h.createMeterAnomaly(c, accountID, req.MeterCondition, req.CalibrationFactor, calibID)
	}

	h.logger.Info("Meter calibration recorded",
		zap.String("account_id", accountID.String()),
		zap.Float64("calibration_factor", req.CalibrationFactor),
		zap.String("condition", req.MeterCondition),
	)

	return c.Status(201).JSON(fiber.Map{
		"calibration_id":    calibID.String(),
		"calibration_factor": req.CalibrationFactor,
		"meter_condition":   req.MeterCondition,
		"accuracy_pct":      accuracyPct,
	})
}

// ── GET /api/v1/admin/meters/:account_id/calibrations ────────────────────────
// Get calibration history for a meter

func (h *MeterCalibrationHandler) GetCalibrationHistory(c *fiber.Ctx) error {
	accountID, err := uuid.Parse(c.Params("account_id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid account_id"})
	}

	rows, err := h.db.Query(c.Context(), `
		SELECT
			mc.id, mc.calibration_date, mc.calibration_method,
			mc.calibration_factor, mc.meter_condition::text,
			mc.meter_serial_number, mc.technician_name,
			mc.actual_flow_m3, mc.meter_reading_m3, mc.accuracy_pct,
			mc.notes, mc.next_calibration_due,
			u.full_name AS recorded_by_name,
			mc.created_at
		FROM meter_calibrations mc
		LEFT JOIN users u ON u.id = mc.recorded_by
		WHERE mc.account_id = $1
		ORDER BY mc.calibration_date DESC
		LIMIT 50
	`, accountID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch calibrations"})
	}
	defer rows.Close()

	type calibRow struct {
		ID                string   `json:"id"`
		CalibrationDate   string   `json:"calibration_date"`
		CalibrationMethod string   `json:"calibration_method"`
		CalibrationFactor float64  `json:"calibration_factor"`
		MeterCondition    string   `json:"meter_condition"`
		MeterSerial       string   `json:"meter_serial"`
		TechnicianName    string   `json:"technician_name"`
		ActualFlowM3      *float64 `json:"actual_flow_m3"`
		MeterReadingM3    *float64 `json:"meter_reading_m3"`
		AccuracyPct       *float64 `json:"accuracy_pct"`
		Notes             string   `json:"notes"`
		NextCalibrationDue *string `json:"next_calibration_due"`
		RecordedByName    string   `json:"recorded_by_name"`
		CreatedAt         string   `json:"created_at"`
	}

	var calibrations []calibRow
	for rows.Next() {
		var cr calibRow
		var cID uuid.UUID
		var calibDate, createdAt time.Time
		var nextDue *time.Time

		if err := rows.Scan(
			&cID, &calibDate, &cr.CalibrationMethod,
			&cr.CalibrationFactor, &cr.MeterCondition,
			&cr.MeterSerial, &cr.TechnicianName,
			&cr.ActualFlowM3, &cr.MeterReadingM3, &cr.AccuracyPct,
			&cr.Notes, &nextDue,
			&cr.RecordedByName, &createdAt,
		); err != nil {
			continue
		}
		cr.ID = cID.String()
		cr.CalibrationDate = calibDate.Format("2006-01-02")
		cr.CreatedAt = createdAt.Format(time.RFC3339)
		if nextDue != nil {
			s := nextDue.Format("2006-01-02")
			cr.NextCalibrationDue = &s
		}
		calibrations = append(calibrations, cr)
	}

	// Get current account calibration info
	var currentFactor float64
	var meterSerial string
	var installDate *time.Time
	h.db.QueryRow(c.Context(), `
		SELECT calibration_factor, meter_serial_number, meter_install_date
		FROM water_accounts WHERE id = $1
	`, accountID).Scan(&currentFactor, &meterSerial, &installDate)

	var installDateStr *string
	if installDate != nil {
		s := installDate.Format("2006-01-02")
		installDateStr = &s
	}

	return c.JSON(fiber.Map{
		"account_id":         accountID.String(),
		"current_factor":     currentFactor,
		"meter_serial":       meterSerial,
		"meter_install_date": installDateStr,
		"calibrations":       calibrations,
		"total":              len(calibrations),
	})
}

// ── GET /api/v1/admin/meters/due-calibration ─────────────────────────────────
// List meters due for calibration (overdue or within 30 days)

func (h *MeterCalibrationHandler) GetDueCalibrations(c *fiber.Ctx) error {
	districtCode := c.Query("district_code")
	daysAhead := c.QueryInt("days_ahead", 30)

	query := `
		SELECT
			wa.id, wa.gwl_account_number, wa.customer_name,
			wa.meter_serial_number, wa.calibration_factor,
			d.district_name, d.district_code,
			mc.next_calibration_due, mc.meter_condition::text,
			mc.calibration_date AS last_calibration_date
		FROM water_accounts wa
		JOIN districts d ON d.id = wa.district_id
		LEFT JOIN meter_calibrations mc ON mc.id = wa.active_calibration_id
		WHERE mc.next_calibration_due <= NOW() + ($1 || ' days')::INTERVAL
		   OR mc.next_calibration_due IS NULL
	`
	args := []interface{}{daysAhead}

	if districtCode != "" {
		query += " AND d.district_code = $2"
		args = append(args, districtCode)
	}

	query += " ORDER BY mc.next_calibration_due ASC NULLS FIRST LIMIT 200"

	rows, err := h.db.Query(c.Context(), query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch due calibrations"})
	}
	defer rows.Close()

	type dueRow struct {
		AccountID          string  `json:"account_id"`
		AccountNumber      string  `json:"account_number"`
		CustomerName       string  `json:"customer_name"`
		MeterSerial        string  `json:"meter_serial"`
		CalibrationFactor  float64 `json:"calibration_factor"`
		DistrictName       string  `json:"district_name"`
		DistrictCode       string  `json:"district_code"`
		NextCalibrationDue *string `json:"next_calibration_due"`
		MeterCondition     string  `json:"meter_condition"`
		LastCalibrationDate *string `json:"last_calibration_date"`
		IsOverdue          bool    `json:"is_overdue"`
	}

	var due []dueRow
	for rows.Next() {
		var dr dueRow
		var aID uuid.UUID
		var nextDue, lastCalib *time.Time

		if err := rows.Scan(
			&aID, &dr.AccountNumber, &dr.CustomerName,
			&dr.MeterSerial, &dr.CalibrationFactor,
			&dr.DistrictName, &dr.DistrictCode,
			&nextDue, &dr.MeterCondition, &lastCalib,
		); err != nil {
			continue
		}
		dr.AccountID = aID.String()
		if nextDue != nil {
			s := nextDue.Format("2006-01-02")
			dr.NextCalibrationDue = &s
			dr.IsOverdue = nextDue.Before(time.Now())
		} else {
			dr.IsOverdue = true // Never calibrated = overdue
		}
		if lastCalib != nil {
			s := lastCalib.Format("2006-01-02")
			dr.LastCalibrationDate = &s
		}
		due = append(due, dr)
	}

	return c.JSON(fiber.Map{
		"due_calibrations": due,
		"total":            len(due),
		"days_ahead":       daysAhead,
	})
}

// createMeterAnomaly creates an anomaly flag for poor/failed meter condition
func (h *MeterCalibrationHandler) createMeterAnomaly(
	c *fiber.Ctx,
	accountID uuid.UUID,
	condition string,
	calibFactor float64,
	calibID uuid.UUID,
) {
	var districtID uuid.UUID
	h.db.QueryRow(c.Context(),
		`SELECT district_id FROM water_accounts WHERE id = $1`, accountID,
	).Scan(&districtID)

	severity := "MEDIUM"
	if condition == "FAILED" {
		severity = "HIGH"
	}

	h.db.Exec(c.Context(), `
		INSERT INTO anomaly_flags (
			account_id, district_id, anomaly_type, severity,
			description, detected_at, status
		) VALUES ($1,$2,'METER_CONDITION',$3,$4,NOW(),'OPEN')
	`,
		accountID, districtID, severity,
		"Meter condition: "+condition+". Calibration factor: "+
			formatFloat(calibFactor)+". Calibration ID: "+calibID.String(),
	)
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 4, 64)
}
