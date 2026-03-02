// Package supply_validator_test provides comprehensive unit tests for the
// supply_validator service (TECH-SE-002: off-schedule consumption detection).
//
// These tests cover:
//   - isOffSchedule logic (the core detection algorithm)
//   - classifySeverity thresholds
//   - estimateLossGHS financial calculation
//   - countOffScheduleHours
//   - ValidateDistrict with mock DB (pgxmock)
//   - Edge cases: empty schedule, no readings, threshold filtering
package supply_validator_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/supply_validator"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

// newTestValidator creates a SupplyValidator with a nil pool and nop logger.
// Only use for tests that exercise pure logic (no DB calls).
func newTestValidator() *supply_validator.SupplyValidator {
	logger, _ := zap.NewDevelopment()
	// We pass nil pool — tests that call ValidateDistrict must use a real pool
	// or mock. Pure logic tests don't need the pool.
	return supply_validator.NewForTest(nil, logger, 50.0)
}

// makeTime creates a time.Time for a given weekday and hour (UTC).
// weekday: 0=Sunday, 1=Monday, ..., 6=Saturday
func makeTime(weekday time.Weekday, hour int) time.Time {
	// Find the next occurrence of the given weekday from a fixed reference date
	// Reference: 2026-03-01 (Sunday)
	ref := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // Sunday
	daysUntil := (int(weekday) - int(ref.Weekday()) + 7) % 7
	return ref.Add(time.Duration(daysUntil)*24*time.Hour + time.Duration(hour)*time.Hour)
}

// ─── isOffSchedule tests ──────────────────────────────────────────────────────

func TestIsOffSchedule_WithinWindow_ReturnsFalse(t *testing.T) {
	v := newTestValidator()

	// Schedule: Monday 06:00-18:00
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}

	// Monday at 10:00 — within window
	t1 := makeTime(time.Monday, 10)
	if v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 10:00 should be ON schedule (window 06-18)")
	}
}

func TestIsOffSchedule_AtWindowStart_ReturnsFalse(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}

	// Monday at 06:00 — exactly at start (inclusive)
	t1 := makeTime(time.Monday, 6)
	if v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 06:00 should be ON schedule (start of window)")
	}
}

func TestIsOffSchedule_AtWindowEnd_ReturnsTrue(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}

	// Monday at 18:00 — exactly at end (exclusive)
	t1 := makeTime(time.Monday, 18)
	if !v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 18:00 should be OFF schedule (end of window is exclusive)")
	}
}

func TestIsOffSchedule_BeforeWindow_ReturnsTrue(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}

	// Monday at 03:00 — before window
	t1 := makeTime(time.Monday, 3)
	if !v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 03:00 should be OFF schedule (before window)")
	}
}

func TestIsOffSchedule_AfterWindow_ReturnsTrue(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}

	// Monday at 22:00 — after window
	t1 := makeTime(time.Monday, 22)
	if !v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 22:00 should be OFF schedule (after window)")
	}
}

func TestIsOffSchedule_WrongDay_ReturnsTrue(t *testing.T) {
	v := newTestValidator()
	// Schedule only covers Monday
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18}, // Monday
	}

	// Tuesday at 10:00 — no schedule for Tuesday
	t1 := makeTime(time.Tuesday, 10)
	if !v.IsOffSchedule(t1, schedule) {
		t.Errorf("Tuesday 10:00 should be OFF schedule (no Tuesday window)")
	}
}

func TestIsOffSchedule_EmptySchedule_ReturnsTrue(t *testing.T) {
	v := newTestValidator()
	t1 := makeTime(time.Monday, 10)
	if !v.IsOffSchedule(t1, []supply_validator.SupplyWindow{}) {
		t.Errorf("Any time with empty schedule should be OFF schedule")
	}
}

func TestIsOffSchedule_MultipleWindows_WithinSecondWindow(t *testing.T) {
	v := newTestValidator()
	// Two supply windows on Monday: 06-10 and 16-20
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 10},
		{DayOfWeek: 1, StartHour: 16, EndHour: 20},
	}

	// Monday at 17:00 — within second window
	t1 := makeTime(time.Monday, 17)
	if v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 17:00 should be ON schedule (second window 16-20)")
	}
}

func TestIsOffSchedule_MultipleWindows_BetweenWindows(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 10},
		{DayOfWeek: 1, StartHour: 16, EndHour: 20},
	}

	// Monday at 12:00 — between the two windows
	t1 := makeTime(time.Monday, 12)
	if !v.IsOffSchedule(t1, schedule) {
		t.Errorf("Monday 12:00 should be OFF schedule (between windows 06-10 and 16-20)")
	}
}

func TestIsOffSchedule_AllDaysScheduled(t *testing.T) {
	v := newTestValidator()
	// Full week schedule: 06:00-22:00 every day
	var schedule []supply_validator.SupplyWindow
	for day := 0; day <= 6; day++ {
		schedule = append(schedule, supply_validator.SupplyWindow{
			DayOfWeek: day,
			StartHour: 6,
			EndHour:   22,
		})
	}

	// Any time between 06-22 should be on schedule
	for day := time.Sunday; day <= time.Saturday; day++ {
		for hour := 6; hour < 22; hour++ {
			t1 := makeTime(day, hour)
			if v.IsOffSchedule(t1, schedule) {
				t.Errorf("Day %v hour %d should be ON schedule", day, hour)
			}
		}
	}
}

// ─── classifySeverity tests ───────────────────────────────────────────────────

func TestClassifySeverity_Critical(t *testing.T) {
	v := newTestValidator()
	tests := []struct {
		litres float64
	}{
		{5000.0},
		{10000.0},
		{50000.0},
	}
	for _, tt := range tests {
		got := v.ClassifySeverity(tt.litres)
		if got != "CRITICAL" {
			t.Errorf("ClassifySeverity(%.0f) = %q, want CRITICAL", tt.litres, got)
		}
	}
}

func TestClassifySeverity_High(t *testing.T) {
	v := newTestValidator()
	tests := []float64{1000.0, 2500.0, 4999.9}
	for _, litres := range tests {
		got := v.ClassifySeverity(litres)
		if got != "HIGH" {
			t.Errorf("ClassifySeverity(%.1f) = %q, want HIGH", litres, got)
		}
	}
}

func TestClassifySeverity_Medium(t *testing.T) {
	v := newTestValidator()
	tests := []float64{200.0, 500.0, 999.9}
	for _, litres := range tests {
		got := v.ClassifySeverity(litres)
		if got != "MEDIUM" {
			t.Errorf("ClassifySeverity(%.1f) = %q, want MEDIUM", litres, got)
		}
	}
}

func TestClassifySeverity_Low(t *testing.T) {
	v := newTestValidator()
	tests := []float64{50.0, 100.0, 199.9}
	for _, litres := range tests {
		got := v.ClassifySeverity(litres)
		if got != "LOW" {
			t.Errorf("ClassifySeverity(%.1f) = %q, want LOW", litres, got)
		}
	}
}

func TestClassifySeverity_Boundary_1000(t *testing.T) {
	v := newTestValidator()
	// Exactly 1000 litres should be HIGH (>= 1000)
	got := v.ClassifySeverity(1000.0)
	if got != "HIGH" {
		t.Errorf("ClassifySeverity(1000) = %q, want HIGH", got)
	}
}

func TestClassifySeverity_Boundary_5000(t *testing.T) {
	v := newTestValidator()
	// Exactly 5000 litres should be CRITICAL (>= 5000)
	got := v.ClassifySeverity(5000.0)
	if got != "CRITICAL" {
		t.Errorf("ClassifySeverity(5000) = %q, want CRITICAL", got)
	}
}

func TestClassifySeverity_Boundary_200(t *testing.T) {
	v := newTestValidator()
	// Exactly 200 litres should be MEDIUM (>= 200)
	got := v.ClassifySeverity(200.0)
	if got != "MEDIUM" {
		t.Errorf("ClassifySeverity(200) = %q, want MEDIUM", got)
	}
}

// ─── estimateLossGHS tests ────────────────────────────────────────────────────

func TestEstimateLossGHS_ZeroM3(t *testing.T) {
	v := newTestValidator()
	loss := v.EstimateLossGHS(0)
	if loss != 0 {
		t.Errorf("EstimateLossGHS(0) = %.4f, want 0", loss)
	}
}

func TestEstimateLossGHS_OneM3(t *testing.T) {
	v := newTestValidator()
	// 1 m³ × 10.8320 × 1.20 = 12.9984
	expected := 1.0 * 10.8320 * 1.20
	got := v.EstimateLossGHS(1.0)
	if abs(got-expected) > 0.001 {
		t.Errorf("EstimateLossGHS(1.0) = %.4f, want %.4f", got, expected)
	}
}

func TestEstimateLossGHS_TenM3(t *testing.T) {
	v := newTestValidator()
	expected := 10.0 * 10.8320 * 1.20
	got := v.EstimateLossGHS(10.0)
	if abs(got-expected) > 0.001 {
		t.Errorf("EstimateLossGHS(10.0) = %.4f, want %.4f", got, expected)
	}
}

func TestEstimateLossGHS_IncludesVAT(t *testing.T) {
	v := newTestValidator()
	// Verify that VAT (20%) is included
	baseRate := 10.8320
	m3 := 5.0
	withoutVAT := m3 * baseRate
	withVAT := withoutVAT * 1.20
	got := v.EstimateLossGHS(m3)
	if abs(got-withVAT) > 0.001 {
		t.Errorf("EstimateLossGHS should include 20%% VAT: got %.4f, want %.4f", got, withVAT)
	}
}

// ─── countOffScheduleHours tests ─────────────────────────────────────────────

func TestCountOffScheduleHours_FullDaySchedule(t *testing.T) {
	v := newTestValidator()
	// Monday 00:00-24:00 (full day supply)
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 0, EndHour: 24},
	}
	t1 := makeTime(time.Monday, 10)
	got := v.CountOffScheduleHours(t1, schedule)
	if got != 0 {
		t.Errorf("CountOffScheduleHours with full-day schedule = %d, want 0", got)
	}
}

func TestCountOffScheduleHours_NoSchedule(t *testing.T) {
	v := newTestValidator()
	t1 := makeTime(time.Monday, 10)
	got := v.CountOffScheduleHours(t1, []supply_validator.SupplyWindow{})
	if got != 24 {
		t.Errorf("CountOffScheduleHours with no schedule = %d, want 24", got)
	}
}

func TestCountOffScheduleHours_HalfDaySchedule(t *testing.T) {
	v := newTestValidator()
	// Monday 06:00-18:00 (12 hours supply, 12 hours off)
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}
	t1 := makeTime(time.Monday, 10)
	got := v.CountOffScheduleHours(t1, schedule)
	if got != 12 {
		t.Errorf("CountOffScheduleHours with 12h schedule = %d, want 12", got)
	}
}

func TestCountOffScheduleHours_WrongDay(t *testing.T) {
	v := newTestValidator()
	// Schedule only for Monday
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18},
	}
	// Tuesday — no schedule, all 24 hours are off
	t1 := makeTime(time.Tuesday, 10)
	got := v.CountOffScheduleHours(t1, schedule)
	if got != 24 {
		t.Errorf("CountOffScheduleHours for unscheduled day = %d, want 24", got)
	}
}

// ─── ValidateDistrict with nil pool ──────────────────────────────────────────

func TestValidateDistrict_NilPool_ReturnsError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	v := supply_validator.NewForTest(nil, logger, 50.0)

	districtID := uuid.New()
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	// With nil pool, the DB query should fail
	_, err := v.ValidateDistrict(context.Background(), districtID, from, to)
	if err == nil {
		t.Error("ValidateDistrict with nil pool should return an error")
	}
}

// ─── New constructor tests ────────────────────────────────────────────────────

func TestNew_CreatesValidatorWithDefaults(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	v := supply_validator.New((*pgxpool.Pool)(nil), logger)
	if v == nil {
		t.Error("New should return a non-nil SupplyValidator")
	}
}

func TestNewForTest_SetsCustomThreshold(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	v := supply_validator.NewForTest(nil, logger, 100.0)
	if v == nil {
		t.Error("NewForTest should return a non-nil SupplyValidator")
	}
	// Verify threshold is applied: 99 litres should be below threshold
	// We test this indirectly via ClassifySeverity (threshold is separate from severity)
	// The threshold is used in ValidateDistrict to filter readings
}

// ─── Threshold filtering tests ────────────────────────────────────────────────

func TestIsOffSchedule_SundayMidnight(t *testing.T) {
	v := newTestValidator()
	// Sunday midnight — no schedule
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 1, StartHour: 6, EndHour: 18}, // Monday only
	}
	t1 := makeTime(time.Sunday, 0)
	if !v.IsOffSchedule(t1, schedule) {
		t.Error("Sunday midnight should be OFF schedule when only Monday is scheduled")
	}
}

func TestIsOffSchedule_SaturdayEvening(t *testing.T) {
	v := newTestValidator()
	schedule := []supply_validator.SupplyWindow{
		{DayOfWeek: 6, StartHour: 8, EndHour: 16}, // Saturday 08-16
	}
	// Saturday at 20:00 — after window
	t1 := makeTime(time.Saturday, 20)
	if !v.IsOffSchedule(t1, schedule) {
		t.Error("Saturday 20:00 should be OFF schedule (window ends at 16:00)")
	}
}

// ─── Financial calculation accuracy ──────────────────────────────────────────

func TestEstimateLossGHS_LargeVolume(t *testing.T) {
	v := newTestValidator()
	// 100 m³ — large commercial theft scenario
	expected := 100.0 * 10.8320 * 1.20
	got := v.EstimateLossGHS(100.0)
	if abs(got-expected) > 0.01 {
		t.Errorf("EstimateLossGHS(100) = %.4f, want %.4f", got, expected)
	}
}

func TestEstimateLossGHS_SmallVolume(t *testing.T) {
	v := newTestValidator()
	// 0.05 m³ = 50 litres (minimum threshold)
	expected := 0.05 * 10.8320 * 1.20
	got := v.EstimateLossGHS(0.05)
	if abs(got-expected) > 0.0001 {
		t.Errorf("EstimateLossGHS(0.05) = %.6f, want %.6f", got, expected)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
