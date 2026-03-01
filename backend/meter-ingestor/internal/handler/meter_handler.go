// Package handler implements both the gRPC server and the HTTP REST handler
// for the meter ingestor service.
//
// gRPC is used by AMR gateways and IoT concentrators.
// HTTP/REST is used by the field officer mobile app and manual upload tools.

package handler

import (
	"context"
	"time"

	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/repository"
	"github.com/ArowuTest/gn-waas/backend/meter-ingestor/internal/service"
	pb "github.com/ArowuTest/gn-waas/backend/meter-ingestor/proto/meter"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ─── gRPC Server ─────────────────────────────────────────────────────────────

// GRPCHandler implements pb.MeterIngestorServiceServer
type GRPCHandler struct {
	pb.UnimplementedMeterIngestorServiceServer
	svc    *service.MeterIngestorService
	repo   *repository.MeterReadingRepository
	logger *zap.Logger
}

func NewGRPCHandler(
	svc *service.MeterIngestorService,
	repo *repository.MeterReadingRepository,
	logger *zap.Logger,
) *GRPCHandler {
	return &GRPCHandler{svc: svc, repo: repo, logger: logger}
}

// SubmitReading handles a single meter reading submission via gRPC
func (h *GRPCHandler) SubmitReading(
	ctx context.Context,
	req *pb.MeterReading,
) (*pb.MeterReadingResponse, error) {

	if req.GwlAccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "gwl_account_number is required")
	}
	if req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.ReadingM3 < 0 {
		return nil, status.Error(codes.InvalidArgument, "reading_m3 cannot be negative")
	}

	ts := time.Now()
	if req.ReadingTimestamp != nil {
		ts = req.ReadingTimestamp.AsTime()
	}

	rec := &repository.MeterReadingRecord{
		GWLAccountNumber: req.GwlAccountNumber,
		DeviceID:         req.DeviceId,
		ReadingM3:        req.ReadingM3,
		FlowRateM3H:      req.FlowRateM3H,
		PressureBar:      req.PressureBar,
		ReadMethod:       req.ReadMethod,
		ReaderID:         req.ReaderId,
		ReadingTimestamp: ts,
		BatteryVoltage:   req.BatteryVoltage,
		TamperDetected:   req.TamperDetected,
		DistrictCode:     req.DistrictCode,
	}

	readingID, preCheck, err := h.svc.IngestReading(ctx, rec)
	if err != nil {
		h.logger.Error("Failed to ingest reading", zap.Error(err), zap.String("account", req.GwlAccountNumber))
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}

	return &pb.MeterReadingResponse{
		Accepted:      true,
		ReadingId:     readingID.String(),
		Message:       "Reading accepted",
		AnomalyFlagged: preCheck.Flagged,
		AnomalyReason: preCheck.Reason,
	}, nil
}

// SubmitBatch handles a batch of meter readings via gRPC
func (h *GRPCHandler) SubmitBatch(
	ctx context.Context,
	req *pb.BatchMeterReadings,
) (*pb.BatchMeterReadingResponse, error) {

	if len(req.Readings) == 0 {
		return nil, status.Error(codes.InvalidArgument, "readings batch is empty")
	}
	if len(req.Readings) > 1000 {
		return nil, status.Error(codes.InvalidArgument, "batch size exceeds maximum of 1000")
	}

	var records []*repository.MeterReadingRecord
	for _, r := range req.Readings {
		ts := time.Now()
		if r.ReadingTimestamp != nil {
			ts = r.ReadingTimestamp.AsTime()
		}
		records = append(records, &repository.MeterReadingRecord{
			GWLAccountNumber: r.GwlAccountNumber,
			DeviceID:         r.DeviceId,
			ReadingM3:        r.ReadingM3,
			FlowRateM3H:      r.FlowRateM3H,
			PressureBar:      r.PressureBar,
			ReadMethod:       r.ReadMethod,
			ReaderID:         r.ReaderId,
			ReadingTimestamp: ts,
			BatteryVoltage:   r.BatteryVoltage,
			TamperDetected:   r.TamperDetected,
			DistrictCode:     r.DistrictCode,
		})
	}

	accepted, rejected, flagged, errs := h.svc.IngestBatch(ctx, records)

	return &pb.BatchMeterReadingResponse{
		Accepted:      int32(accepted),
		Rejected:      int32(rejected),
		AnomalyFlagged: int32(flagged),
		Errors:        errs,
		BatchId:       req.BatchId,
	}, nil
}

// StreamReadings handles a client-side streaming RPC from AMR gateways.
// The new grpc API: server calls SendAndClose to return the response.
func (h *GRPCHandler) StreamReadings(
	stream pb.MeterIngestorService_StreamReadingsServer,
) error {
	var accepted, rejected, flagged int
	var errs []string

	for {
		req, err := stream.Recv()
		if err != nil {
			// io.EOF means client finished streaming — send summary response
			break
		}

		ts := time.Now()
		if req.ReadingTimestamp != nil {
			ts = req.ReadingTimestamp.AsTime()
		}

		rec := &repository.MeterReadingRecord{
			GWLAccountNumber: req.GwlAccountNumber,
			DeviceID:         req.DeviceId,
			ReadingM3:        req.ReadingM3,
			FlowRateM3H:      req.FlowRateM3H,
			PressureBar:      req.PressureBar,
			ReadMethod:       req.ReadMethod,
			ReaderID:         req.ReaderId,
			ReadingTimestamp: ts,
			BatteryVoltage:   req.BatteryVoltage,
			TamperDetected:   req.TamperDetected,
			DistrictCode:     req.DistrictCode,
		}

		_, preCheck, ingestErr := h.svc.IngestReading(stream.Context(), rec)
		if ingestErr != nil {
			rejected++
			errs = append(errs, ingestErr.Error())
		} else {
			accepted++
			if preCheck.Flagged {
				flagged++
			}
		}
	}

	return stream.SendAndClose(&pb.BatchMeterReadingResponse{
		Accepted:       int32(accepted),
		Rejected:       int32(rejected),
		AnomalyFlagged: int32(flagged),
		Errors:         errs,
	})
}

// GetLatestReading returns the most recent reading for an account
func (h *GRPCHandler) GetLatestReading(
	ctx context.Context,
	req *pb.GetLatestReadingRequest,
) (*pb.MeterReading, error) {
	if req.GwlAccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "gwl_account_number is required")
	}

	rec, err := h.repo.GetLatest(ctx, req.GwlAccountNumber)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no readings found for account %s", req.GwlAccountNumber)
	}

	return &pb.MeterReading{
		GwlAccountNumber: rec.GWLAccountNumber,
		DeviceId:         rec.DeviceID,
		ReadingM3:        rec.ReadingM3,
		FlowRateM3H:      rec.FlowRateM3H,
		PressureBar:      rec.PressureBar,
		ReadMethod:       rec.ReadMethod,
		ReaderId:         rec.ReaderID,
		ReadingTimestamp: timestamppb.New(rec.ReadingTimestamp),
		BatteryVoltage:   rec.BatteryVoltage,
		TamperDetected:   rec.TamperDetected,
		DistrictCode:     rec.DistrictCode,
	}, nil
}

// GetReadingHistory returns reading history for an account
func (h *GRPCHandler) GetReadingHistory(
	ctx context.Context,
	req *pb.GetReadingHistoryRequest,
) (*pb.ReadingHistoryResponse, error) {
	if req.GwlAccountNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "gwl_account_number is required")
	}

	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now()
	if req.From != nil {
		from = req.From.AsTime()
	}
	if req.To != nil {
		to = req.To.AsTime()
	}

	records, err := h.repo.GetHistory(ctx, req.GwlAccountNumber, from, to, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get history: %v", err)
	}

	var readings []*pb.MeterReading
	for _, rec := range records {
		readings = append(readings, &pb.MeterReading{
			GwlAccountNumber: rec.GWLAccountNumber,
			DeviceId:         rec.DeviceID,
			ReadingM3:        rec.ReadingM3,
			FlowRateM3H:      rec.FlowRateM3H,
			PressureBar:      rec.PressureBar,
			ReadMethod:       rec.ReadMethod,
			ReaderId:         rec.ReaderID,
			ReadingTimestamp: timestamppb.New(rec.ReadingTimestamp),
			BatteryVoltage:   rec.BatteryVoltage,
			TamperDetected:   rec.TamperDetected,
			DistrictCode:     rec.DistrictCode,
		})
	}

	return &pb.ReadingHistoryResponse{
		Readings: readings,
		Total:    int32(len(readings)),
	}, nil
}

// ─── HTTP REST Handler ────────────────────────────────────────────────────────
// Used by the field officer mobile app and manual upload tools.

type HTTPHandler struct {
	svc    *service.MeterIngestorService
	repo   *repository.MeterReadingRepository
	logger *zap.Logger
}

func NewHTTPHandler(
	svc *service.MeterIngestorService,
	repo *repository.MeterReadingRepository,
	logger *zap.Logger,
) *HTTPHandler {
	return &HTTPHandler{svc: svc, repo: repo, logger: logger}
}

type submitReadingRequest struct {
	GWLAccountNumber string  `json:"gwl_account_number"`
	DeviceID         string  `json:"device_id"`
	ReadingM3        float64 `json:"reading_m3"`
	FlowRateM3H      float64 `json:"flow_rate_m3h"`
	PressureBar      float64 `json:"pressure_bar"`
	ReadMethod       string  `json:"read_method"`
	ReaderID         string  `json:"reader_id"`
	ReadingTimestamp string  `json:"reading_timestamp"` // RFC3339
	BatteryVoltage   float64 `json:"battery_voltage"`
	TamperDetected   bool    `json:"tamper_detected"`
	DistrictCode     string  `json:"district_code"`
}

// SubmitReading handles POST /api/v1/readings
func (h *HTTPHandler) SubmitReading(c *fiber.Ctx) error {
	var req submitReadingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "invalid request body"})
	}
	if req.GWLAccountNumber == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "gwl_account_number is required"})
	}

	ts := time.Now()
	if req.ReadingTimestamp != "" {
		if parsed, err := time.Parse(time.RFC3339, req.ReadingTimestamp); err == nil {
			ts = parsed
		}
	}

	rec := &repository.MeterReadingRecord{
		GWLAccountNumber: req.GWLAccountNumber,
		DeviceID:         req.DeviceID,
		ReadingM3:        req.ReadingM3,
		FlowRateM3H:      req.FlowRateM3H,
		PressureBar:      req.PressureBar,
		ReadMethod:       req.ReadMethod,
		ReaderID:         req.ReaderID,
		ReadingTimestamp: ts,
		BatteryVoltage:   req.BatteryVoltage,
		TamperDetected:   req.TamperDetected,
		DistrictCode:     req.DistrictCode,
	}

	readingID, preCheck, err := h.svc.IngestReading(c.Context(), rec)
	if err != nil {
		h.logger.Error("HTTP ingest failed", zap.Error(err))
		return c.Status(500).JSON(fiber.Map{"success": false, "error": err.Error()})
	}

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"reading_id":      readingID.String(),
			"accepted":        true,
			"anomaly_flagged": preCheck.Flagged,
			"anomaly_reason":  preCheck.Reason,
		},
	})
}

type batchReadingRequest struct {
	Readings []submitReadingRequest `json:"readings"`
	BatchID  string                 `json:"batch_id"`
	Source   string                 `json:"source"`
}

// SubmitBatch handles POST /api/v1/readings/batch
func (h *HTTPHandler) SubmitBatch(c *fiber.Ctx) error {
	var req batchReadingRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "invalid request body"})
	}
	if len(req.Readings) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "readings array is empty"})
	}
	if len(req.Readings) > 1000 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "batch size exceeds 1000"})
	}

	var records []*repository.MeterReadingRecord
	for _, r := range req.Readings {
		ts := time.Now()
		if r.ReadingTimestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, r.ReadingTimestamp); err == nil {
				ts = parsed
			}
		}
		records = append(records, &repository.MeterReadingRecord{
			GWLAccountNumber: r.GWLAccountNumber,
			DeviceID:         r.DeviceID,
			ReadingM3:        r.ReadingM3,
			FlowRateM3H:      r.FlowRateM3H,
			PressureBar:      r.PressureBar,
			ReadMethod:       r.ReadMethod,
			ReaderID:         r.ReaderID,
			ReadingTimestamp: ts,
			BatteryVoltage:   r.BatteryVoltage,
			TamperDetected:   r.TamperDetected,
			DistrictCode:     r.DistrictCode,
		})
	}

	accepted, rejected, flagged, errs := h.svc.IngestBatch(c.Context(), records)

	return c.Status(200).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"batch_id":       req.BatchID,
			"accepted":       accepted,
			"rejected":       rejected,
			"anomaly_flagged": flagged,
			"errors":         errs,
		},
	})
}

// GetLatestReading handles GET /api/v1/readings/:account_number/latest
func (h *HTTPHandler) GetLatestReading(c *fiber.Ctx) error {
	accountNumber := c.Params("account_number")
	if accountNumber == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": "account_number is required"})
	}

	rec, err := h.repo.GetLatest(c.Context(), accountNumber)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": "no readings found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"gwl_account_number": rec.GWLAccountNumber,
			"device_id":          rec.DeviceID,
			"reading_m3":         rec.ReadingM3,
			"flow_rate_m3h":      rec.FlowRateM3H,
			"pressure_bar":       rec.PressureBar,
			"read_method":        rec.ReadMethod,
			"reading_timestamp":  rec.ReadingTimestamp.Format(time.RFC3339),
			"tamper_detected":    rec.TamperDetected,
		},
	})
}

// HealthCheck handles GET /health
func (h *HTTPHandler) HealthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"service": "meter-ingestor",
			"status":  "healthy",
			"version": "1.0.0",
		},
	})
}
