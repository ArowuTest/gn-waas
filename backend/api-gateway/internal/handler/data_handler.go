package handler

// data_handler.go — Read-only endpoints for core data tables:
//   GET /api/v1/production-records   — district production volumes
//   GET /api/v1/meter-readings       — account meter read history
//   GET /api/v1/water-balance        — IWA water balance records
//   GET /api/v1/billing-records      — GWL billing records
//
// These endpoints are intentionally simple: they expose the raw data
// tables that the sentinel and tariff-engine write to, so the admin
// portal and authority portal can display them.

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DataHandler serves read-only queries against core data tables.
type DataHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewDataHandler constructs a DataHandler.
func NewDataHandler(db *pgxpool.Pool, logger *zap.Logger) *DataHandler {
	return &DataHandler{db: db, logger: logger}
}

func (h *DataHandler) q(ctx context.Context) repository.Querier {
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return h.db
}

// ── GET /api/v1/production-records ───────────────────────────────────────────
// Query params: district_id (uuid), from (YYYY-MM-DD), to (YYYY-MM-DD), limit, offset
func (h *DataHandler) ListProductionRecords(c *fiber.Ctx) error {
	ctx := c.UserContext()

	args := []interface{}{}
	conditions := []string{"1=1"}
	idx := 1

	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		conditions = append(conditions, fmt.Sprintf("pr.district_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if f := c.Query("from"); f != "" {
		t, err := time.Parse("2006-01-02", f)
		if err != nil {
			return response.BadRequest(c, "INVALID_FROM", "from must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("pr.recorded_at >= $%d", idx))
		args = append(args, t)
		idx++
	}
	if t := c.Query("to"); t != "" {
		ts, err := time.Parse("2006-01-02", t)
		if err != nil {
			return response.BadRequest(c, "INVALID_TO", "to must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("pr.recorded_at <= $%d", idx))
		args = append(args, ts)
		idx++
	}

	limit := 100
	offset := 0
	if l := c.QueryInt("limit", 100); l > 0 && l <= 500 {
		limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		offset = o
	}

	where := ""
	for i, cond := range conditions {
		if i == 0 {
			where = cond
		} else {
			where += " AND " + cond
		}
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT
			pr.id, pr.district_id, pr.recorded_at, pr.volume_m3,
			pr.source_type, pr.created_at,
			d.district_name, d.district_code
		FROM production_records pr
		JOIN districts d ON d.id = pr.district_id
		WHERE %s
		ORDER BY pr.recorded_at DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)

	rows, err := h.q(ctx).Query(ctx, query, args...)
	if err != nil {
		h.logger.Error("list production records", zap.Error(err))
		return response.InternalError(c, "failed to list production records")
	}
	defer rows.Close()

	type Record struct {
		ID           string    `json:"id"`
		DistrictID   string    `json:"district_id"`
		RecordedAt   time.Time `json:"recorded_at"`
		VolumeM3     float64   `json:"volume_m3"`
		SourceType   string    `json:"source_type"`
		CreatedAt    time.Time `json:"created_at"`
		DistrictName string    `json:"district_name"`
		DistrictCode string    `json:"district_code"`
	}

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(
			&r.ID, &r.DistrictID, &r.RecordedAt, &r.VolumeM3,
			&r.SourceType, &r.CreatedAt,
			&r.DistrictName, &r.DistrictCode,
		); err != nil {
			h.logger.Error("scan production record", zap.Error(err))
			continue
		}
		records = append(records, r)
	}
	if records == nil {
		records = []Record{}
	}
	return response.OK(c, records)
}

// ── GET /api/v1/meter-readings ────────────────────────────────────────────────
// Query params: account_id (uuid), from (YYYY-MM-DD), to (YYYY-MM-DD), limit, offset
func (h *DataHandler) ListMeterReadings(c *fiber.Ctx) error {
	ctx := c.UserContext()

	args := []interface{}{}
	conditions := []string{"1=1"}
	idx := 1

	if a := c.Query("account_id"); a != "" {
		id, err := uuid.Parse(a)
		if err != nil {
			return response.BadRequest(c, "INVALID_ACCOUNT_ID", "invalid account_id")
		}
		conditions = append(conditions, fmt.Sprintf("mr.account_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		conditions = append(conditions, fmt.Sprintf("wa.district_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if f := c.Query("from"); f != "" {
		t, err := time.Parse("2006-01-02", f)
		if err != nil {
			return response.BadRequest(c, "INVALID_FROM", "from must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("mr.reading_date >= $%d", idx))
		args = append(args, t)
		idx++
	}
	if t := c.Query("to"); t != "" {
		ts, err := time.Parse("2006-01-02", t)
		if err != nil {
			return response.BadRequest(c, "INVALID_TO", "to must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("mr.reading_date <= $%d", idx))
		args = append(args, ts)
		idx++
	}

	limit := 100
	offset := 0
	if l := c.QueryInt("limit", 100); l > 0 && l <= 500 {
		limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		offset = o
	}

	where := ""
	for i, cond := range conditions {
		if i == 0 {
			where = cond
		} else {
			where += " AND " + cond
		}
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT
			mr.id, mr.account_id, mr.reading_date, mr.reading_m3,
			mr.consumption_m3, mr.read_method, mr.ocr_confidence,
			mr.is_estimated, mr.created_at,
			wa.gwl_account_number, wa.account_holder_name
		FROM meter_readings mr
		JOIN water_accounts wa ON wa.id = mr.account_id
		WHERE %s
		ORDER BY mr.reading_date DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)

	rows, err := h.q(ctx).Query(ctx, query, args...)
	if err != nil {
		h.logger.Error("list meter readings", zap.Error(err))
		return response.InternalError(c, "failed to list meter readings")
	}
	defer rows.Close()

	type Reading struct {
		ID                string    `json:"id"`
		AccountID         string    `json:"account_id"`
		ReadingDate       time.Time `json:"reading_date"`
		ReadingM3         float64   `json:"reading_m3"`
		ConsumptionM3     *float64  `json:"consumption_m3"`
		ReadMethod        string    `json:"read_method"`
		OCRConfidence     *float64  `json:"ocr_confidence"`
		IsEstimated       bool      `json:"is_estimated"`
		CreatedAt         time.Time `json:"created_at"`
		AccountNumber     string    `json:"account_number"`
		AccountHolderName string    `json:"account_holder_name"`
	}

	var readings []Reading
	for rows.Next() {
		var r Reading
		if err := rows.Scan(
			&r.ID, &r.AccountID, &r.ReadingDate, &r.ReadingM3,
			&r.ConsumptionM3, &r.ReadMethod, &r.OCRConfidence,
			&r.IsEstimated, &r.CreatedAt,
			&r.AccountNumber, &r.AccountHolderName,
		); err != nil {
			h.logger.Error("scan meter reading", zap.Error(err))
			continue
		}
		readings = append(readings, r)
	}
	if readings == nil {
		readings = []Reading{}
	}
	return response.OK(c, readings)
}

// ── GET /api/v1/water-balance ─────────────────────────────────────────────────
// Query params: district_id (uuid), from (YYYY-MM-DD), to (YYYY-MM-DD), limit, offset
func (h *DataHandler) ListWaterBalance(c *fiber.Ctx) error {
	ctx := c.UserContext()

	args := []interface{}{}
	conditions := []string{"1=1"}
	idx := 1

	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		conditions = append(conditions, fmt.Sprintf("wb.district_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if f := c.Query("from"); f != "" {
		t, err := time.Parse("2006-01-02", f)
		if err != nil {
			return response.BadRequest(c, "INVALID_FROM", "from must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("wb.period_start >= $%d", idx))
		args = append(args, t)
		idx++
	}
	if t := c.Query("to"); t != "" {
		ts, err := time.Parse("2006-01-02", t)
		if err != nil {
			return response.BadRequest(c, "INVALID_TO", "to must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("wb.period_end <= $%d", idx))
		args = append(args, ts)
		idx++
	}

	limit := 100
	offset := 0
	if l := c.QueryInt("limit", 100); l > 0 && l <= 500 {
		limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		offset = o
	}

	where := ""
	for i, cond := range conditions {
		if i == 0 {
			where = cond
		} else {
			where += " AND " + cond
		}
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		-- FLOW-07 fix: use correct column names matching water_balance_records schema.
		-- Schema (migration 003, written by sentinel): nrw_percent, ili_score,
		-- data_confidence_score, computed_at, estimated_revenue_recovery_ghs.
		-- The old query used nrw_pct, ili_value, data_confidence_grade, calculated_at,
		-- apparent_loss_value_ghs — none of which exist in the schema.
		SELECT
			wb.id, wb.district_id, wb.period_start, wb.period_end,
			wb.system_input_volume_m3,
			wb.billed_metered_m3,
			wb.billed_unmetered_m3,
			wb.unbilled_metered_m3,
			wb.unbilled_unmetered_m3,
			wb.total_authorised_m3,
			wb.unauthorised_consumption_m3,
			wb.metering_inaccuracies_m3,
			wb.data_handling_errors_m3,
			wb.total_apparent_losses_m3,
			wb.main_leakage_m3,
			wb.storage_overflow_m3,
			wb.service_conn_leakage_m3,
			wb.total_real_losses_m3,
			wb.total_nrw_m3,
			wb.nrw_percent,
			COALESCE(wb.estimated_revenue_recovery_ghs, 0),
			wb.ili_score,
			wb.data_confidence_score,
			wb.computed_at,
			d.district_name, d.district_code
		FROM water_balance_records wb
		JOIN districts d ON d.id = wb.district_id
		WHERE %s
		ORDER BY wb.period_start DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)

	rows, err := h.q(ctx).Query(ctx, query, args...)
	if err != nil {
		h.logger.Error("list water balance", zap.Error(err))
		return response.InternalError(c, "failed to list water balance records")
	}
	defer rows.Close()

	type WBRecord struct {
		ID                        string    `json:"id"`
		DistrictID                string    `json:"district_id"`
		PeriodStart               time.Time `json:"period_start"`
		PeriodEnd                 time.Time `json:"period_end"`
		// Frontend-aligned field names (system_input_m3, nrw_percent, ili, etc.)
		SystemInputM3             float64   `json:"system_input_m3"`
		BilledMeteredM3           float64   `json:"billed_metered_m3"`
		BilledUnmeteredM3         float64   `json:"billed_unmetered_m3"`
		UnbilledMeteredM3         float64   `json:"unbilled_metered_m3"`
		UnbilledUnmeteredM3       float64   `json:"unbilled_unmetered_m3"`
		TotalAuthorisedM3         float64   `json:"total_authorised_m3"`
		UnauthorisedConsumptionM3 float64   `json:"unauthorised_consumption_m3"`
		MeteringInaccuraciesM3    float64   `json:"metering_inaccuracies_m3"`
		DataHandlingErrorsM3      float64   `json:"data_handling_errors_m3"`
		TotalApparentLossesM3     float64   `json:"total_apparent_losses_m3"`
		MainLeakageM3             float64   `json:"main_leakage_m3"`
		StorageOverflowM3         float64   `json:"storage_overflow_m3"`
		ServiceConnectionLeakM3   float64   `json:"service_connection_leak_m3"`
		TotalRealLossesM3         float64   `json:"total_real_losses_m3"`
		TotalWaterLossesM3        float64   `json:"total_water_losses_m3"`   // computed: apparent + real
		NRWM3                     float64   `json:"nrw_m3"`
		NRWPercent                *float64  `json:"nrw_percent"`
		EstimatedRevenueRecovery  float64   `json:"estimated_revenue_recovery_ghs"`
		ILI                       *float64  `json:"ili"`
		IWAGrade                  string    `json:"iwa_grade"`               // computed from ILI
		DataConfidenceScore       *int      `json:"data_confidence_score"`
		ComputedAt                *time.Time `json:"computed_at,omitempty"`
		DistrictName              string    `json:"district_name"`
		DistrictCode              string    `json:"district_code"`
	}

	var records []WBRecord
	for rows.Next() {
		var r WBRecord
		var serviceConnLeakM3 float64
		var dcGrade *int
		if err := rows.Scan(
			&r.ID, &r.DistrictID, &r.PeriodStart, &r.PeriodEnd,
			&r.SystemInputM3,
			&r.BilledMeteredM3,
			&r.BilledUnmeteredM3,
			&r.UnbilledMeteredM3,
			&r.UnbilledUnmeteredM3,
			&r.TotalAuthorisedM3,
			&r.UnauthorisedConsumptionM3,
			&r.MeteringInaccuraciesM3,
			&r.DataHandlingErrorsM3,
			&r.TotalApparentLossesM3,
			&r.MainLeakageM3,
			&r.StorageOverflowM3,
			&serviceConnLeakM3,
			&r.TotalRealLossesM3,
			&r.NRWM3,
			&r.NRWPercent,
			&r.EstimatedRevenueRecovery,
			&r.ILI,
			&dcGrade,
			&r.ComputedAt,
			&r.DistrictName, &r.DistrictCode,
		); err != nil {
			h.logger.Error("scan water balance record", zap.Error(err))
			continue
		}
		r.ServiceConnectionLeakM3 = serviceConnLeakM3
		r.TotalWaterLossesM3 = r.TotalApparentLossesM3 + r.TotalRealLossesM3
		r.DataConfidenceScore = dcGrade
		// Compute IWA grade from ILI
		if r.ILI != nil {
			switch {
			case *r.ILI < 1.5:
				r.IWAGrade = "A"
			case *r.ILI < 2.5:
				r.IWAGrade = "B"
			case *r.ILI < 4.0:
				r.IWAGrade = "C"
			default:
				r.IWAGrade = "D"
			}
		} else {
			r.IWAGrade = "N/A"
		}
		records = append(records, r)
	}
	if records == nil {
		records = []WBRecord{}
	}
	return response.OK(c, records)
}

// ── GET /api/v1/billing-records ───────────────────────────────────────────────
// Query params: account_id (uuid), district_id (uuid), from (YYYY-MM-DD), to (YYYY-MM-DD), limit, offset
func (h *DataHandler) ListBillingRecords(c *fiber.Ctx) error {
	ctx := c.UserContext()

	args := []interface{}{}
	conditions := []string{"1=1"}
	idx := 1

	if a := c.Query("account_id"); a != "" {
		id, err := uuid.Parse(a)
		if err != nil {
			return response.BadRequest(c, "INVALID_ACCOUNT_ID", "invalid account_id")
		}
		conditions = append(conditions, fmt.Sprintf("gb.account_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		conditions = append(conditions, fmt.Sprintf("wa.district_id = $%d", idx))
		args = append(args, id)
		idx++
	}
	if f := c.Query("from"); f != "" {
		t, err := time.Parse("2006-01-02", f)
		if err != nil {
			return response.BadRequest(c, "INVALID_FROM", "from must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("gb.billing_period_start >= $%d", idx))
		args = append(args, t)
		idx++
	}
	if t := c.Query("to"); t != "" {
		ts, err := time.Parse("2006-01-02", t)
		if err != nil {
			return response.BadRequest(c, "INVALID_TO", "to must be YYYY-MM-DD")
		}
		conditions = append(conditions, fmt.Sprintf("gb.billing_period_end <= $%d", idx))
		args = append(args, ts)
		idx++
	}

	limit := 100
	offset := 0
	if l := c.QueryInt("limit", 100); l > 0 && l <= 500 {
		limit = l
	}
	if o := c.QueryInt("offset", 0); o >= 0 {
		offset = o
	}

	where := ""
	for i, cond := range conditions {
		if i == 0 {
			where = cond
		} else {
			where += " AND " + cond
		}
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT
			gb.id, gb.account_id, gb.gwl_bill_id,
			gb.billing_period_start, gb.billing_period_end,
			gb.consumption_m3,
			gb.gwl_category, gb.gwl_amount_ghs, gb.gwl_total_ghs,
			gb.shadow_total_ghs, gb.variance_pct, gb.variance_flag,
			gb.payment_status, gb.payment_amount_ghs,
			gb.created_at,
			wa.gwl_account_number, wa.account_holder_name
		FROM gwl_bills gb
		JOIN water_accounts wa ON wa.id = gb.account_id
		WHERE %s
		ORDER BY gb.billing_period_start DESC
		LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)

	rows, err := h.q(ctx).Query(ctx, query, args...)
	if err != nil {
		h.logger.Error("list billing records", zap.Error(err))
		return response.InternalError(c, "failed to list billing records")
	}
	defer rows.Close()

	type BillRecord struct {
		ID                  string    `json:"id"`
		AccountID           string    `json:"account_id"`
		GWLBillID           string    `json:"gwl_bill_id"`
		PeriodStart         time.Time `json:"billing_period_start"`
		PeriodEnd           time.Time `json:"billing_period_end"`
		ConsumptionM3       float64   `json:"consumption_m3"`
		GWLCategory         string    `json:"gwl_category"`
		GWLAmountGHS        float64   `json:"gwl_amount_ghs"`
		GWLTotalGHS         float64   `json:"gwl_total_ghs"`
		ShadowTotalGHS      *float64  `json:"shadow_total_ghs"`
		VariancePct         *float64  `json:"variance_pct"`
		VarianceFlag        bool      `json:"variance_flag"`
		PaymentStatus       string    `json:"payment_status"`
		PaymentAmountGHS    float64   `json:"payment_amount_ghs"`
		CreatedAt           time.Time `json:"created_at"`
		AccountNumber       string    `json:"account_number"`
		AccountHolderName   string    `json:"account_holder_name"`
	}

	var records []BillRecord
	for rows.Next() {
		var r BillRecord
		if err := rows.Scan(
			&r.ID, &r.AccountID, &r.GWLBillID,
			&r.PeriodStart, &r.PeriodEnd,
			&r.ConsumptionM3,
			&r.GWLCategory, &r.GWLAmountGHS, &r.GWLTotalGHS,
			&r.ShadowTotalGHS, &r.VariancePct, &r.VarianceFlag,
			&r.PaymentStatus, &r.PaymentAmountGHS,
			&r.CreatedAt,
			&r.AccountNumber, &r.AccountHolderName,
		); err != nil {
			h.logger.Error("scan billing record", zap.Error(err))
			continue
		}
		records = append(records, r)
	}
	if records == nil {
		records = []BillRecord{}
	}
	return response.OK(c, records)
}
