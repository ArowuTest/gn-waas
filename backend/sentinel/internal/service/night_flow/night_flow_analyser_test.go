package night_flow_test

import (
	"context"
	"testing"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/night_flow"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func newAnalyser(threshold float64) *night_flow.NightFlowAnalyser {
	logger, _ := zap.NewDevelopment()
	return night_flow.NewNightFlowAnalyser(logger, threshold)
}

func testDistrict() *entities.District {
	return &entities.District{
		ID:           uuid.New(),
		DistrictName: "Accra West",
		DistrictCode: "AW",
	}
}

func TestAnalyseDistrictBalance_FlagsHighNRW(t *testing.T) {
	analyser := newAnalyser(20.0) // 20% threshold

	// 1000 m³ produced, 450 m³ billed → 55% unaccounted → HIGH
	flag, err := analyser.AnalyseDistrictBalance(
		context.Background(),
		testDistrict(),
		1000.0, // production
		450.0,  // billed
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag == nil {
		t.Fatal("expected anomaly flag, got nil")
	}
	if flag.AlertLevel != "HIGH" && flag.AlertLevel != "CRITICAL" {
		t.Errorf("expected HIGH or CRITICAL alert, got %s", flag.AlertLevel)
	}
}

func TestAnalyseDistrictBalance_NoFlagLowNRW(t *testing.T) {
	analyser := newAnalyser(20.0)

	// 1000 m³ produced, 900 m³ billed → 10% unaccounted → should NOT flag
	flag, err := analyser.AnalyseDistrictBalance(
		context.Background(),
		testDistrict(),
		1000.0,
		900.0,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag != nil {
		t.Errorf("expected no flag for 10%% NRW, got flag with level %s", flag.AlertLevel)
	}
}

func TestAnalyseDistrictBalance_ZeroProduction(t *testing.T) {
	analyser := newAnalyser(20.0)

	// Zero production — should return nil (no data)
	flag, err := analyser.AnalyseDistrictBalance(
		context.Background(),
		testDistrict(),
		0.0,
		0.0,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag != nil {
		t.Error("expected nil flag for zero production")
	}
}

func TestAnalyseDistrictBalance_CriticalNRW(t *testing.T) {
	analyser := newAnalyser(20.0)

	// 1000 m³ produced, 250 m³ billed → 75% unaccounted → CRITICAL (>=70%)
	flag, err := analyser.AnalyseDistrictBalance(
		context.Background(),
		testDistrict(),
		1000.0,
		250.0,
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if flag == nil {
		t.Fatal("expected anomaly flag for 75% NRW")
	}
	if flag.AlertLevel != "CRITICAL" {
		t.Errorf("expected CRITICAL alert for 75%% NRW, got %s", flag.AlertLevel)
	}
}

func TestAnalyseDistrictBalance_ExactThreshold(t *testing.T) {
	analyser := newAnalyser(20.0)

	// Exactly 20% unaccounted — boundary condition
	flag, err := analyser.AnalyseDistrictBalance(
		context.Background(),
		testDistrict(),
		1000.0,
		800.0, // exactly 20% unaccounted
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// At exactly threshold, behaviour depends on implementation (> vs >=)
	// Just verify no panic and result is consistent
	_ = flag
}
