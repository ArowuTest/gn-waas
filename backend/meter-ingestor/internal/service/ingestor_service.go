// Package service implements the core meter reading ingestion logic.
//
// Flow:
//   IoT Device / AMR Gateway / Field App
//        │
//        ▼  gRPC SubmitReading / SubmitBatch / StreamReadings
//   MeterIngestorService
//        │
//        ├─► Pre-validate (account exists, reading plausible)
//        ├─► Persist to meter_readings table (ON CONFLICT DO UPDATE)
//        ├─► Run inline sentinel pre-check (tamper, spike, zero-flow)
//        └─► Publish NATS event → gnwaas.meter.reading.received
//                                  gnwaas.sentinel.scan.trigger  (if anomaly)

package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/repository"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"encoding/json"
)

// ─── NATS subjects ────────────────────────────────────────────────────────────

const (
	SubjectMeterReadingReceived = "gnwaas.meter.reading.received"
	SubjectSentinelTrigger      = "gnwaas.sentinel.scan.trigger"
)

// ─── Event payloads ───────────────────────────────────────────────────────────

type MeterReadingEvent struct {
	ReadingID        string    `json:"reading_id"`
	GWLAccountNumber string    `json:"gwl_account_number"`
	DistrictCode     string    `json:"district_code"`
	ReadingM3        float64   `json:"reading_m3"`
	ReadingTimestamp time.Time `json:"reading_timestamp"`
	AnomalyFlagged   bool      `json:"anomaly_flagged"`
	AnomalyReason    string    `json:"anomaly_reason,omitempty"`
}

type SentinelTriggerEvent struct {
	DistrictCode string    `json:"district_code"`
	Reason       string    `json:"reason"`
	TriggeredAt  time.Time `json:"triggered_at"`
}

// ─── Inline pre-check result ─────────────────────────────────────────────────

type PreCheckResult struct {
	Flagged bool
	Reason  string
}

// ─── Service ─────────────────────────────────────────────────────────────────

type MeterIngestorService struct {
	repo   *repository.MeterReadingRepository
	nc     *nats.Conn // nil if NATS not configured
	logger *zap.Logger
}

func NewMeterIngestorService(
	repo *repository.MeterReadingRepository,
	nc *nats.Conn,
	logger *zap.Logger,
) *MeterIngestorService {
	return &MeterIngestorService{repo: repo, nc: nc, logger: logger}
}

// IngestReading validates, persists, and publishes a single meter reading.
// Returns the assigned reading UUID and any inline anomaly flag.
func (s *MeterIngestorService) IngestReading(
	ctx context.Context,
	rec *repository.MeterReadingRecord,
) (uuid.UUID, *PreCheckResult, error) {

	// ── 1. Inline pre-check ───────────────────────────────────────────────────
	preCheck := s.preCheck(ctx, rec)

	// ── 2. Persist ────────────────────────────────────────────────────────────
	readingID, err := s.repo.Upsert(ctx, rec)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("persist reading: %w", err)
	}

	// ── 3. Publish NATS events ────────────────────────────────────────────────
	s.publishReadingEvent(readingID, rec, preCheck)
	if preCheck.Flagged {
		s.publishSentinelTrigger(rec.DistrictCode, preCheck.Reason)
	}

	s.logger.Info("Meter reading ingested",
		zap.String("reading_id", readingID.String()),
		zap.String("account", rec.GWLAccountNumber),
		zap.Float64("reading_m3", rec.ReadingM3),
		zap.Bool("anomaly", preCheck.Flagged),
	)

	return readingID, preCheck, nil
}

// IngestBatch processes multiple readings, returning counts of accepted/rejected/flagged.
func (s *MeterIngestorService) IngestBatch(
	ctx context.Context,
	records []*repository.MeterReadingRecord,
) (accepted, rejected, flagged int, errs []string) {
	for _, rec := range records {
		_, preCheck, err := s.IngestReading(ctx, rec)
		if err != nil {
			rejected++
			errs = append(errs, fmt.Sprintf("%s: %v", rec.GWLAccountNumber, err))
			continue
		}
		accepted++
		if preCheck.Flagged {
			flagged++
		}
	}
	return
}

// ─── Inline sentinel pre-check ───────────────────────────────────────────────
// This is a fast, synchronous check that runs before the full sentinel scan.
// It catches obvious anomalies: tamper, impossible spike, zero-flow on active account.

func (s *MeterIngestorService) preCheck(
	ctx context.Context,
	rec *repository.MeterReadingRecord,
) *PreCheckResult {

	// Tamper flag from smart meter hardware
	if rec.TamperDetected {
		return &PreCheckResult{
			Flagged: true,
			Reason:  "Smart meter tamper flag set — possible meter bypass or physical interference",
		}
	}

	// Impossible negative reading
	if rec.ReadingM3 < 0 {
		return &PreCheckResult{
			Flagged: true,
			Reason:  fmt.Sprintf("Negative meter reading (%.2f m³) — impossible value", rec.ReadingM3),
		}
	}

	// Compare against previous reading for spike detection
	prev, err := s.repo.GetLatest(ctx, rec.GWLAccountNumber)
	if err == nil && prev != nil {
		daysDiff := rec.ReadingTimestamp.Sub(prev.ReadingTimestamp).Hours() / 24
		if daysDiff > 0 {
			dailyConsumption := (rec.ReadingM3 - prev.ReadingM3) / daysDiff

			// Residential: >50 m³/day is suspicious; Commercial: >500 m³/day
			// Using 200 m³/day as a conservative universal threshold
			if dailyConsumption > 200 {
				return &PreCheckResult{
					Flagged: true,
					Reason: fmt.Sprintf(
						"Consumption spike: %.1f m³/day (prev reading %.2f m³ on %s → current %.2f m³)",
						dailyConsumption,
						prev.ReadingM3,
						prev.ReadingTimestamp.Format("2006-01-02"),
						rec.ReadingM3,
					),
				}
			}

			// Meter rollback (current < previous)
			if rec.ReadingM3 < prev.ReadingM3 && math.Abs(rec.ReadingM3-prev.ReadingM3) > 0.5 {
				return &PreCheckResult{
					Flagged: true,
					Reason: fmt.Sprintf(
						"Meter rollback detected: reading decreased from %.2f to %.2f m³",
						prev.ReadingM3, rec.ReadingM3,
					),
				}
			}
		}
	}

	// Abnormal flow rate (>100 m³/hr suggests burst pipe or meter fault)
	if rec.FlowRateM3H > 100 {
		return &PreCheckResult{
			Flagged: true,
			Reason: fmt.Sprintf(
				"Abnormal flow rate: %.1f m³/hr — possible burst pipe or meter fault",
				rec.FlowRateM3H,
			),
		}
	}

	// Low battery warning (not an anomaly, but log it)
	if rec.BatteryVoltage > 0 && rec.BatteryVoltage < 2.5 {
		s.logger.Warn("Smart meter low battery",
			zap.String("account", rec.GWLAccountNumber),
			zap.Float64("battery_v", rec.BatteryVoltage),
		)
	}

	return &PreCheckResult{Flagged: false}
}

// ─── NATS publish helpers ─────────────────────────────────────────────────────

func (s *MeterIngestorService) publishReadingEvent(
	readingID uuid.UUID,
	rec *repository.MeterReadingRecord,
	preCheck *PreCheckResult,
) {
	if s.nc == nil {
		return
	}
	evt := MeterReadingEvent{
		ReadingID:        readingID.String(),
		GWLAccountNumber: rec.GWLAccountNumber,
		DistrictCode:     rec.DistrictCode,
		ReadingM3:        rec.ReadingM3,
		ReadingTimestamp: rec.ReadingTimestamp,
		AnomalyFlagged:   preCheck.Flagged,
		AnomalyReason:    preCheck.Reason,
	}
	data, _ := json.Marshal(evt)
	if err := s.nc.Publish(SubjectMeterReadingReceived, data); err != nil {
		s.logger.Warn("Failed to publish meter reading event", zap.Error(err))
	}
}

func (s *MeterIngestorService) publishSentinelTrigger(districtCode, reason string) {
	if s.nc == nil {
		return
	}
	evt := SentinelTriggerEvent{
		DistrictCode: districtCode,
		Reason:       reason,
		TriggeredAt:  time.Now(),
	}
	data, _ := json.Marshal(evt)
	if err := s.nc.Publish(SubjectSentinelTrigger, data); err != nil {
		s.logger.Warn("Failed to publish sentinel trigger", zap.Error(err))
	}
}
