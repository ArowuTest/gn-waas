package handler

import (
	"strconv"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RevenueRecoveryHandler serves the managed-service monetisation endpoints.
// It exposes the revenue_recovery_events table which tracks every GHS amount
// recovered as a direct result of a GN-WAAS audit flag, and calculates the
// 3% success fee owed to the managed-service operator.
//
// BE-M01 fix: All database queries now go through q(ctx) which retrieves the
// RLS-activated transaction from the request context (set by rls.Middleware).
// This ensures district-level Row-Level Security is enforced on every query.
// Using h.db directly would bypass RLS because the raw pool connection does
// not have the SET LOCAL app.district_id / app.user_role session variables set.
type RevenueRecoveryHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewRevenueRecoveryHandler(db *pgxpool.Pool, logger *zap.Logger) *RevenueRecoveryHandler {
	return &RevenueRecoveryHandler{db: db, logger: logger}
}

// qCtx returns the RLS-activated transaction from the Fiber context if one
// exists (placed there by rls.Middleware), otherwise falls back to the raw
// connection pool. All handler methods MUST use h.qCtx(c) instead of h.db.
func (h *RevenueRecoveryHandler) qCtx(c *fiber.Ctx) repository.Querier {
	ctx := c.UserContext()
	if tx, ok := rls.TxFromContext(ctx); ok {
		return tx
	}
	return h.db
}

// GetSummary godoc
// GET /api/v1/revenue/summary?district_id=&period=
// Returns aggregate recovery stats: total recovered, total success fee, count by type.
func (h *RevenueRecoveryHandler) GetSummary(c *fiber.Ctx) error {
	ctx := c.UserContext()
	q := h.qCtx(c)

	districtFilter := c.Query("district_id")
	period := c.Query("period") // e.g. "2026-03"

	where := "1=1"
	args := []interface{}{}
	idx := 1

	if districtFilter != "" {
		if _, err := uuid.Parse(districtFilter); err == nil {
			where += " AND district_id = $" + strconv.Itoa(idx)
			args = append(args, districtFilter)
			idx++
		}
	}
	if period != "" {
		where += " AND TO_CHAR(created_at, 'YYYY-MM') = $" + strconv.Itoa(idx)
		args = append(args, period)
		idx++
	}

	type Summary struct {
		TotalEvents        int     `json:"total_events"`
		TotalVarianceGHS   float64 `json:"total_variance_ghs"`
		TotalRecoveredGHS  float64 `json:"total_recovered_ghs"`
		TotalSuccessFeeGHS float64 `json:"total_success_fee_ghs"`
		PendingCount       int     `json:"pending_count"`
		ConfirmedCount     int     `json:"confirmed_count"`
		CollectedCount     int     `json:"collected_count"`
		ByType             []struct {
			RecoveryType  string  `json:"recovery_type"`
			Count         int     `json:"count"`
			RecoveredGHS  float64 `json:"recovered_ghs"`
			SuccessFeeGHS float64 `json:"success_fee_ghs"`
		} `json:"by_type"`
	}

	var s Summary

	row := q.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(variance_ghs), 0),
			COALESCE(SUM(recovered_ghs), 0),
			COALESCE(SUM(success_fee_ghs), 0),
			COUNT(*) FILTER (WHERE status = 'PENDING')::int,
			COUNT(*) FILTER (WHERE status = 'CONFIRMED')::int,
			COUNT(*) FILTER (WHERE status = 'COLLECTED')::int
		FROM revenue_recovery_events
		WHERE `+where, args...,
	)
	if err := row.Scan(
		&s.TotalEvents, &s.TotalVarianceGHS, &s.TotalRecoveredGHS,
		&s.TotalSuccessFeeGHS, &s.PendingCount, &s.ConfirmedCount, &s.CollectedCount,
	); err != nil {
		h.logger.Error("GetRevenueSummary scan failed", zap.Error(err))
		return response.InternalError(c, "Failed to load revenue summary")
	}

	// By-type breakdown
	rows, err := q.Query(ctx, `
		SELECT recovery_type,
		       COUNT(*)::int,
		       COALESCE(SUM(recovered_ghs), 0),
		       COALESCE(SUM(success_fee_ghs), 0)
		FROM revenue_recovery_events
		WHERE `+where+`
		GROUP BY recovery_type
		ORDER BY SUM(recovered_ghs) DESC`, args...,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var bt struct {
				RecoveryType  string  `json:"recovery_type"`
				Count         int     `json:"count"`
				RecoveredGHS  float64 `json:"recovered_ghs"`
				SuccessFeeGHS float64 `json:"success_fee_ghs"`
			}
			if err := rows.Scan(&bt.RecoveryType, &bt.Count, &bt.RecoveredGHS, &bt.SuccessFeeGHS); err == nil {
				s.ByType = append(s.ByType, bt)
			}
		}
	}

	return response.OK(c, s)
}

// ListEvents godoc
// GET /api/v1/revenue/events?district_id=&status=&limit=&offset=
func (h *RevenueRecoveryHandler) ListEvents(c *fiber.Ctx) error {
	ctx := c.UserContext()
	q := h.qCtx(c)
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 {
		limit = 100
	}

	where := "1=1"
	args := []interface{}{}
	idx := 1

	if d := c.Query("district_id"); d != "" {
		if _, err := uuid.Parse(d); err == nil {
			where += " AND rr.district_id = $" + strconv.Itoa(idx)
			args = append(args, d)
			idx++
		}
	}
	if s := c.Query("status"); s != "" {
		where += " AND rr.status = $" + strconv.Itoa(idx)
		args = append(args, s)
		idx++
	}

	type Event struct {
		ID            uuid.UUID  `json:"id"`
		AuditEventID  uuid.UUID  `json:"audit_event_id"`
		DistrictName  string     `json:"district_name"`
		AccountNumber *string    `json:"account_number,omitempty"`
		AccountHolder *string    `json:"account_holder,omitempty"`
		VarianceGHS   float64    `json:"variance_ghs"`
		RecoveredGHS  float64    `json:"recovered_ghs"`
		SuccessFeeGHS float64    `json:"success_fee_ghs"`
		RecoveryType  string     `json:"recovery_type"`
		Status        string     `json:"status"`
		ConfirmedAt   *time.Time `json:"confirmed_at,omitempty"`
		CollectedAt   *time.Time `json:"collected_at,omitempty"`
		CreatedAt     time.Time  `json:"created_at"`
	}

	var total int
	q.QueryRow(ctx, `SELECT COUNT(*) FROM revenue_recovery_events rr WHERE `+where, args...).Scan(&total)

	args = append(args, limit, offset)

	rows, err := q.Query(ctx, `
		SELECT rr.id, rr.audit_event_id,
		       d.district_name, wa.gwl_account_number, wa.account_holder_name,
		       rr.variance_ghs, rr.recovered_ghs, rr.success_fee_ghs,
		       rr.recovery_type, rr.status, rr.confirmed_at, rr.collected_at, rr.created_at
		FROM revenue_recovery_events rr
		JOIN districts d ON rr.district_id = d.id
		LEFT JOIN water_accounts wa ON rr.account_id = wa.id
		WHERE `+where+`
		ORDER BY rr.created_at DESC
		LIMIT $`+strconv.Itoa(idx)+` OFFSET $`+strconv.Itoa(idx+1), args...,
	)
	if err != nil {
		h.logger.Error("ListRevenueEvents query failed", zap.Error(err))
		return response.InternalError(c, "Failed to list revenue events")
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID, &e.AuditEventID,
			&e.DistrictName, &e.AccountNumber, &e.AccountHolder,
			&e.VarianceGHS, &e.RecoveredGHS, &e.SuccessFeeGHS,
			&e.RecoveryType, &e.Status, &e.ConfirmedAt, &e.CollectedAt, &e.CreatedAt,
		); err == nil {
			events = append(events, e)
		}
	}

	intTotal := total
	return response.OKWithMeta(c, events, &response.Meta{Total: &intTotal})
}

// ConfirmRecovery godoc
// PATCH /api/v1/revenue/events/:id/confirm
// Marks a recovery event as CONFIRMED with the actual recovered amount.
// Transitions: PENDING | FIELD_VERIFIED → CONFIRMED
func (h *RevenueRecoveryHandler) ConfirmRecovery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	q := h.qCtx(c)
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid event ID")
	}

	var req struct {
		RecoveredGHS float64 `json:"recovered_ghs"`
		Notes        string  `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.RecoveredGHS <= 0 {
		return response.BadRequest(c, "INVALID_AMOUNT", "recovered_ghs must be positive")
	}

	confirmedBy, _ := c.Locals("user_id").(string)

	tag, err := q.Exec(ctx, `
		UPDATE revenue_recovery_events
		SET status        = 'CONFIRMED',
		    recovered_ghs = $1,
		    confirmed_at  = NOW(),
		    confirmed_by  = $2::uuid,
		    notes         = COALESCE(NULLIF($3,''), notes),
		    updated_at    = NOW()
		WHERE id = $4
		  AND status IN ('PENDING', 'FIELD_VERIFIED')`,
		req.RecoveredGHS, confirmedBy, req.Notes, id,
	)
	if err != nil {
		h.logger.Error("ConfirmRecovery failed", zap.Error(err))
		return response.InternalError(c, "Failed to confirm recovery")
	}
	if tag.RowsAffected() == 0 {
		return response.BadRequest(c, "INVALID_STATE",
			"Recovery event not found or already past CONFIRMED state")
	}
	return response.OK(c, fiber.Map{
		"message": "Recovery confirmed",
		"id":      id,
		"status":  "CONFIRMED",
	})
}

// CollectRecovery godoc
// PATCH /api/v1/revenue/events/:id/collect
// Marks a recovery event as COLLECTED — money has been physically received.
// This is the final stage of the revenue leakage pipeline.
// Transitions: GRA_SIGNED | CONFIRMED → COLLECTED
func (h *RevenueRecoveryHandler) CollectRecovery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	q := h.qCtx(c)
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid event ID")
	}

	var req struct {
		CollectedGHS    float64 `json:"collected_ghs"`    // actual amount received
		PaymentRef      string  `json:"payment_ref"`      // GWL payment reference
		CollectionNotes string  `json:"collection_notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.CollectedGHS <= 0 {
		return response.BadRequest(c, "INVALID_AMOUNT", "collected_ghs must be positive")
	}

	collectedBy, _ := c.Locals("user_id").(string)

	tag, err := q.Exec(ctx, `
		UPDATE revenue_recovery_events
		SET status        = 'COLLECTED',
		    recovered_ghs = $1,
		    collected_at  = NOW(),
		    notes         = COALESCE(NULLIF($2,''), NULLIF($3,''), notes),
		    updated_at    = NOW()
		WHERE id = $4
		  AND status IN ('CONFIRMED', 'GRA_SIGNED')`,
		req.CollectedGHS,
		req.PaymentRef,
		req.CollectionNotes,
		id,
	)
	if err != nil {
		h.logger.Error("CollectRecovery failed", zap.Error(err))
		return response.InternalError(c, "Failed to mark recovery as collected")
	}
	if tag.RowsAffected() == 0 {
		return response.BadRequest(c, "INVALID_STATE",
			"Recovery event not found or not in CONFIRMED/GRA_SIGNED state")
	}

	h.logger.Info("Revenue recovery collected",
		zap.String("event_id", id.String()),
		zap.Float64("collected_ghs", req.CollectedGHS),
		zap.String("collected_by", collectedBy),
	)

	return response.OK(c, fiber.Map{
		"message":       "Revenue collected — pipeline complete",
		"id":            id,
		"status":        "COLLECTED",
		"collected_ghs": req.CollectedGHS,
	})
}

// GetLeakagePipeline godoc
// GET /api/v1/revenue/pipeline?district_id=
//
// Returns the full revenue leakage pipeline in GHS at each stage:
//   Detected → Field Verified → Confirmed → GRA Signed → Collected
//
// This is the primary dashboard metric for GN-WAAS.
// Every GHS figure represents money GWL should be collecting but isn't.
//
// Also returns compliance flags (OUTAGE_CONSUMPTION) separately —
// these are PURC violations, not revenue leakage.
func (h *RevenueRecoveryHandler) GetLeakagePipeline(c *fiber.Ctx) error {
	ctx := c.UserContext()
	q := h.qCtx(c)

	districtFilter := c.Query("district_id")

	type PipelineStage struct {
		Count  int     `json:"count"`
		GHS    float64 `json:"ghs"`
	}

	type Pipeline struct {
		// Revenue leakage pipeline (primary mission)
		Detected      PipelineStage `json:"detected"`
		FieldVerified PipelineStage `json:"field_verified"`
		Confirmed     PipelineStage `json:"confirmed"`
		GRASigned     PipelineStage `json:"gra_signed"`
		Collected     PipelineStage `json:"collected"`

		// Compliance flags (separate — not revenue leakage)
		ComplianceOpen    int `json:"compliance_flags_open"`
		DataQualityOpen   int `json:"data_quality_flags_open"`

		// Summary
		TotalDetectedMonthlyGHS    float64 `json:"total_detected_monthly_ghs"`
		TotalDetectedAnnualGHS     float64 `json:"total_detected_annual_ghs"`
		TotalCollectedGHS          float64 `json:"total_collected_ghs"`
		RecoveryRatePct            float64 `json:"recovery_rate_pct"`
		DistrictID                 *string `json:"district_id,omitempty"`
	}

	var p Pipeline
	if districtFilter != "" {
		p.DistrictID = &districtFilter
	}

	// Use the v_leakage_pipeline view if district filter is provided,
	// otherwise aggregate across all districts
	var whereClause string
	var args []interface{}
	if districtFilter != "" {
		if _, err := uuid.Parse(districtFilter); err == nil {
			whereClause = "WHERE district_id = $1"
			args = append(args, districtFilter)
		}
	}

	// Stage 1: Detected (open revenue leakage flags)
	q.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(monthly_leakage_ghs), 0)
		FROM anomaly_flags
		`+whereClause+`
		`+func() string {
		if whereClause == "" {
			return "WHERE leakage_category = 'REVENUE_LEAKAGE' AND status = 'OPEN'"
		}
		return "AND leakage_category = 'REVENUE_LEAKAGE' AND status = 'OPEN'"
	}(), args...,
	).Scan(&p.Detected.Count, &p.Detected.GHS)

	// Stage 2: Field verified (field outcome recorded, not yet confirmed)
	q.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(confirmed_leakage_ghs), 0)
		FROM anomaly_flags
		`+whereClause+`
		`+func() string {
		if whereClause == "" {
			return "WHERE leakage_category = 'REVENUE_LEAKAGE' AND field_outcome IS NOT NULL AND confirmed_fraud IS NULL"
		}
		return "AND leakage_category = 'REVENUE_LEAKAGE' AND field_outcome IS NOT NULL AND confirmed_fraud IS NULL"
	}(), args...,
	).Scan(&p.FieldVerified.Count, &p.FieldVerified.GHS)

	// Stage 3: Confirmed (confirmed_fraud = TRUE)
	q.QueryRow(ctx, `
		SELECT
			COUNT(*)::int,
			COALESCE(SUM(confirmed_leakage_ghs), 0)
		FROM anomaly_flags
		`+whereClause+`
		`+func() string {
		if whereClause == "" {
			return "WHERE confirmed_fraud = TRUE AND leakage_category = 'REVENUE_LEAKAGE'"
		}
		return "AND confirmed_fraud = TRUE AND leakage_category = 'REVENUE_LEAKAGE'"
	}(), args...,
	).Scan(&p.Confirmed.Count, &p.Confirmed.GHS)

	// Stages 4 & 5: GRA signed and collected (from revenue_recovery_events)
	var rreArgs []interface{}
	var rreWhere string
	if districtFilter != "" {
		if _, err := uuid.Parse(districtFilter); err == nil {
			rreWhere = "WHERE district_id = $1"
			rreArgs = append(rreArgs, districtFilter)
		}
	}

	q.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status IN ('GRA_SIGNED', 'COLLECTED'))::int,
			COALESCE(SUM(recovered_ghs) FILTER (WHERE status IN ('GRA_SIGNED', 'COLLECTED')), 0),
			COUNT(*) FILTER (WHERE status = 'COLLECTED')::int,
			COALESCE(SUM(recovered_ghs) FILTER (WHERE status = 'COLLECTED'), 0)
		FROM revenue_recovery_events
		`+rreWhere, rreArgs...,
	).Scan(
		&p.GRASigned.Count, &p.GRASigned.GHS,
		&p.Collected.Count, &p.Collected.GHS,
	)

	// Compliance and data quality flags
	q.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE leakage_category = 'COMPLIANCE' AND status = 'OPEN')::int,
			COUNT(*) FILTER (WHERE leakage_category = 'DATA_QUALITY' AND status = 'OPEN')::int
		FROM anomaly_flags
		`+whereClause, args...,
	).Scan(&p.ComplianceOpen, &p.DataQualityOpen)

	// Summary calculations
	p.TotalDetectedMonthlyGHS = p.Detected.GHS
	p.TotalDetectedAnnualGHS = p.Detected.GHS * 12
	p.TotalCollectedGHS = p.Collected.GHS
	if p.Confirmed.GHS > 0 {
		p.RecoveryRatePct = (p.Collected.GHS / p.Confirmed.GHS) * 100
	}

	return response.OK(c, p)
}
