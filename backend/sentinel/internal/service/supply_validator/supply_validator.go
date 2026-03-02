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
//  4. Create anomaly flags for the sentinel orchestrator to process
package supply_validator

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SupplyWindow represents a scheduled supply period for a district.
type SupplyWindow struct {
	DayOfWeek int // 0=Sunday, 6=Saturday
	StartHour int // 0-23
	EndHour   int // 1-24
}

// MeterReading represents a single meter reading data point.
type MeterReading struct {
	AccountID   string
	MeterID     string
	ReadingM3   float64
	ReadingTime time.Time
}

// OffScheduleViolation represents detected off-schedule consumption.
type OffScheduleViolation struct {
	AccountID          string
	MeterID            string
	ConsumptionM3      float64
	ViolationStart     time.Time
	ViolationEnd       time.Time
	ScheduledOffHours  int
	EstimatedLossGHS   float64
	Severity           string // CRITICAL, HIGH, MEDIUM, LOW
	Description        string
}

// SupplyScheduleRepository defines the data access interface.
type SupplyScheduleRepository interface {
	GetScheduleForDistrict(ctx context.Context, districtID string) ([]SupplyWindow, error)
	GetMeterReadings(ctx context.Context, districtID string, from, to time.Time) ([]MeterReading, error)
}

// SupplyValidator checks for off-schedule consumption anomalies.
type SupplyValidator struct {
	repo   SupplyScheduleRepository
	logger *zap.Logger

	// Minimum consumption threshold to flag (litres)
	// Small amounts may be residual pressure, not active consumption
	minFlagThresholdLitres float64
}

// New creates a new SupplyValidator.
func New(repo SupplyScheduleRepository, logger *zap.Logger) *SupplyValidator {
	return &SupplyValidator{
		repo:                   repo,
		logger:                 logger,
		minFlagThresholdLitres: 50.0, // 50 litres minimum to flag
	}
}

// ValidateDistrict checks all meters in a district for off-schedule consumption
// over the given time window. Returns violations found.
func (v *SupplyValidator) ValidateDistrict(
	ctx context.Context,
	districtID string,
	from, to time.Time,
) ([]OffScheduleViolation, error) {
	// Load supply schedule
	schedule, err := v.repo.GetScheduleForDistrict(ctx, districtID)
	if err != nil {
		return nil, fmt.Errorf("loading supply schedule for district %s: %w", districtID, err)
	}
	if len(schedule) == 0 {
		v.logger.Debug("No supply schedule defined for district, skipping validation",
			zap.String("district_id", districtID))
		return nil, nil
	}

	// Load meter readings for the window
	readings, err := v.repo.GetMeterReadings(ctx, districtID, from, to)
	if err != nil {
		return nil, fmt.Errorf("loading meter readings for district %s: %w", districtID, err)
	}
	if len(readings) == 0 {
		return nil, nil
	}

	v.logger.Info("Validating supply schedule compliance",
		zap.String("district_id", districtID),
		zap.Int("schedule_windows", len(schedule)),
		zap.Int("readings", len(readings)),
		zap.Time("from", from),
		zap.Time("to", to),
	)

	var violations []OffScheduleViolation

	for _, reading := range readings {
		if reading.ReadingM3 <= 0 {
			continue
		}

		// Check if this reading falls in an off-schedule period
		if v.isOffSchedule(reading.ReadingTime, schedule) {
			consumptionLitres := reading.ReadingM3 * 1000
			if consumptionLitres < v.minFlagThresholdLitres {
				continue // Below threshold — likely residual pressure
			}

			severity := v.classifySeverity(consumptionLitres)
			violation := OffScheduleViolation{
				AccountID:         reading.AccountID,
				MeterID:           reading.MeterID,
				ConsumptionM3:     reading.ReadingM3,
				ViolationStart:    reading.ReadingTime,
				ViolationEnd:      reading.ReadingTime.Add(time.Hour),
				ScheduledOffHours: v.countOffScheduleHours(reading.ReadingTime, schedule),
				EstimatedLossGHS:  v.estimateLossGHS(reading.ReadingM3),
				Severity:          severity,
				Description: fmt.Sprintf(
					"Off-schedule consumption detected: %.3f m³ (%.0f L) at %s during scheduled outage. "+
						"This may indicate an illegal bypass or tampered meter.",
					reading.ReadingM3,
					consumptionLitres,
					reading.ReadingTime.Format("2006-01-02 15:04"),
				),
			}
			violations = append(violations, violation)
		}
	}

	v.logger.Info("Supply schedule validation complete",
		zap.String("district_id", districtID),
		zap.Int("violations_found", len(violations)),
	)

	return violations, nil
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
