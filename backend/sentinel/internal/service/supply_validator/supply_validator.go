// Package supply_validator implements TECH-SE-002: off-schedule consumption detection.
//
// The sentinel service uses this module to detect water consumption that occurs
// during scheduled supply outages — a key fraud vector where illegal connections
// or bypasses allow consumption when the main supply is supposed to be off.
//
// How it works:
//  1. Load the supply schedule for a district (from supply_schedules table)
//  2. Fetch meter readings for the district over the analysis window
//  3. Flag any readings that show significant consumption during off-schedule hours
//  4. Return anomaly flags for the sentinel orchestrator to persist
package supply_validator

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SupplyWindow represents a scheduled supply period for a district.
type SupplyWindow struct {
	DayOfWeek int // 0=Sunday, 6=Saturday
	StartHour int // 0-23
	EndHour   int // 1-24
}

// SupplyValidator checks for off-schedule consumption anomalies.
// It queries the supply_schedules and meter_readings tables directly.
type SupplyValidator struct {
	db     *pgxpool.Pool
	logger *zap.Logger

	// Minimum consumption threshold to flag (litres).
	// Small amounts may be residual pressure, not active consumption.
	minFlagThresholdLitres float64
}

// New creates a new SupplyValidator.
func New(db *pgxpool.Pool, logger *zap.Logger) *SupplyValidator {
	return &SupplyValidator{
		db:                     db,
		logger:                 logger,
		minFlagThresholdLitres: 50.0, // 50 litres minimum to flag
	}
}

// ValidateDistrict checks all meters in a district for off-schedule consumption
// over the given time window. Returns anomaly flags for any violations found.
func (v *SupplyValidator) ValidateDistrict(
	ctx context.Context,
	districtID uuid.UUID,
	from, to time.Time,
) ([]*entities.AnomalyFlag, error) {
	if v.db == nil {
		return nil, fmt.Errorf("supply_validator: database pool is nil")
	}
	// Load supply schedule windows for this district
	schedule, err := v.loadSchedule(ctx, districtID)
	if err != nil {
		return nil, fmt.Errorf("loading supply schedule for district %s: %w", districtID, err)
	}
	if len(schedule) == 0 {
		v.logger.Debug("No supply schedule defined for district, skipping validation",
			zap.String("district_id", districtID.String()))
		return nil, nil
	}

	// Load meter readings for the analysis window
	type meterReading struct {
		accountID   uuid.UUID
		meterID     string
		readingM3   float64
		readingTime time.Time
	}

	// FLOW-04 fix: reading_time → reading_date (DATE column in meter_readings schema).
	// FLOW-05 fix: meter_serial is on water_accounts, not meter_readings.
	//   Use wa.meter_serial (joined) with fallback to mr.id::text for unmetered accounts.
	rows, err := v.db.Query(ctx, `
		SELECT
			mr.account_id,
			COALESCE(wa.meter_serial, mr.id::text) AS meter_id,
			mr.reading_m3,
			mr.reading_date
		FROM meter_readings mr
		JOIN water_accounts wa ON wa.id = mr.account_id
		WHERE wa.district_id = $1
		  AND mr.reading_date BETWEEN $2 AND $3
		  AND mr.reading_m3 > 0
		ORDER BY mr.reading_date`,
		districtID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("loading meter readings for district %s: %w", districtID, err)
	}
	defer rows.Close()

	var readings []meterReading
	for rows.Next() {
		var r meterReading
		if err := rows.Scan(&r.accountID, &r.meterID, &r.readingM3, &r.readingTime); err != nil {
			v.logger.Warn("scan meter reading", zap.Error(err))
			continue
		}
		readings = append(readings, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating meter readings: %w", err)
	}

	if len(readings) == 0 {
		return nil, nil
	}

	v.logger.Info("Validating supply schedule compliance",
		zap.String("district_id", districtID.String()),
		zap.Int("schedule_windows", len(schedule)),
		zap.Int("readings", len(readings)),
		zap.Time("from", from),
		zap.Time("to", to),
	)

	var flags []*entities.AnomalyFlag

	for _, reading := range readings {
		if v.isOffSchedule(reading.readingTime, schedule) {
			consumptionLitres := reading.readingM3 * 1000
			if consumptionLitres < v.minFlagThresholdLitres {
				continue // Below threshold — likely residual pressure
			}

			severity := v.classifySeverity(consumptionLitres)
			estimatedLoss := v.estimateLossGHS(reading.readingM3)

			flag := &entities.AnomalyFlag{
				AccountID:   &reading.accountID,
				DistrictID:  districtID,
				AnomalyType: "OFF_SCHEDULE_CONSUMPTION",
				AlertLevel:  severity,
				FraudType:   "ILLEGAL_CONNECTION",
				Title: fmt.Sprintf(
					"Off-schedule consumption: %.3f m³ (%.0f L) at %s",
					reading.readingM3,
					consumptionLitres,
					reading.readingTime.Format("2006-01-02 15:04"),
				),
				Description: fmt.Sprintf(
					"Meter %s recorded %.3f m³ (%.0f L) at %s during a scheduled supply outage. "+
						"This may indicate an illegal bypass, tampered meter, or unauthorised connection. "+
						"Recommend field investigation.",
					reading.meterID,
					reading.readingM3,
					consumptionLitres,
					reading.readingTime.Format("2006-01-02 15:04 MST"),
				),
				EstimatedLossGHS: estimatedLoss,
				EvidenceData: map[string]interface{}{
					"meter_id":              reading.meterID,
					"reading_m3":            reading.readingM3,
					"consumption_litres":    consumptionLitres,
					"reading_time":          reading.readingTime.Format(time.RFC3339),
					"scheduled_off_hours":   v.countOffScheduleHours(reading.readingTime, schedule),
					"detection_method":      "SUPPLY_SCHEDULE_VALIDATOR",
				},
				Status:          "OPEN",
				SentinelVersion: "1.0.0",
				CreatedAt:       time.Now().UTC(),
			}
			flags = append(flags, flag)
		}
	}

	v.logger.Info("Supply schedule validation complete",
		zap.String("district_id", districtID.String()),
		zap.Int("violations_found", len(flags)),
	)

	return flags, nil
}

// loadSchedule loads the supply windows for a district from the supply_schedules table.
func (v *SupplyValidator) loadSchedule(ctx context.Context, districtID uuid.UUID) ([]SupplyWindow, error) {
	rows, err := v.db.Query(ctx, `
		SELECT day_of_week, start_hour, end_hour
		FROM supply_schedules
		WHERE district_id = $1 AND is_active = true
		ORDER BY day_of_week, start_hour`,
		districtID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var windows []SupplyWindow
	for rows.Next() {
		var w SupplyWindow
		if err := rows.Scan(&w.DayOfWeek, &w.StartHour, &w.EndHour); err != nil {
			return nil, err
		}
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

// isOffSchedule returns true if the given time falls outside all supply windows.
func (v *SupplyValidator) isOffSchedule(t time.Time, schedule []SupplyWindow) bool {
	dayOfWeek := int(t.Weekday()) // 0=Sunday
	hour := t.Hour()

	for _, window := range schedule {
		if window.DayOfWeek == dayOfWeek &&
			hour >= window.StartHour &&
			hour < window.EndHour {
			return false // Within a supply window — on schedule
		}
	}
	return true // Not in any supply window — off schedule
}

// countOffScheduleHours counts how many hours in the day are off-schedule.
func (v *SupplyValidator) countOffScheduleHours(t time.Time, schedule []SupplyWindow) int {
	dayOfWeek := int(t.Weekday())
	offHours := 24

	for _, window := range schedule {
		if window.DayOfWeek == dayOfWeek {
			offHours -= (window.EndHour - window.StartHour)
		}
	}
	if offHours < 0 {
		offHours = 0
	}
	return offHours
}

// classifySeverity classifies the severity of an off-schedule consumption event.
func (v *SupplyValidator) classifySeverity(litres float64) string {
	switch {
	case litres >= 5000: // 5 m³+
		return "CRITICAL"
	case litres >= 1000: // 1 m³+
		return "HIGH"
	case litres >= 200: // 200 L+
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// estimateLossGHS estimates the financial loss in Ghana Cedis.
// Uses the 2026 PURC residential tier-2 rate as a conservative estimate.
func (v *SupplyValidator) estimateLossGHS(m3 float64) float64 {
	const ratePerM3 = 10.8320 // 2026 PURC residential tier-2 rate
	const vatRate = 0.20
	return m3 * ratePerM3 * (1 + vatRate)
}

// ─── Exported test helpers ────────────────────────────────────────────────────
// These exported wrappers allow unit tests to exercise internal logic without
// requiring a real database connection.

// NewForTest creates a SupplyValidator with a custom minimum flag threshold.
// Use in tests to control the threshold without modifying the production default.
func NewForTest(db *pgxpool.Pool, logger *zap.Logger, minFlagThresholdLitres float64) *SupplyValidator {
	return &SupplyValidator{
		db:                     db,
		logger:                 logger,
		minFlagThresholdLitres: minFlagThresholdLitres,
	}
}

// IsOffSchedule is an exported wrapper for the internal isOffSchedule method.
// Used in unit tests to verify the core detection algorithm.
func (v *SupplyValidator) IsOffSchedule(t time.Time, schedule []SupplyWindow) bool {
	return v.isOffSchedule(t, schedule)
}

// ClassifySeverity is an exported wrapper for the internal classifySeverity method.
// Used in unit tests to verify severity thresholds.
func (v *SupplyValidator) ClassifySeverity(litres float64) string {
	return v.classifySeverity(litres)
}

// EstimateLossGHS is an exported wrapper for the internal estimateLossGHS method.
// Used in unit tests to verify financial calculations.
func (v *SupplyValidator) EstimateLossGHS(m3 float64) float64 {
	return v.estimateLossGHS(m3)
}

// CountOffScheduleHours is an exported wrapper for the internal countOffScheduleHours method.
// Used in unit tests to verify off-schedule hour counting.
func (v *SupplyValidator) CountOffScheduleHours(t time.Time, schedule []SupplyWindow) int {
	return v.countOffScheduleHours(t, schedule)
}
