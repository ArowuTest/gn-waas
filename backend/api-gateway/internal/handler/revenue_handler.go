package handler

import (
	"strconv"
	"time"

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
type RevenueRecoveryHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewRevenueRecoveryHandler(db *pgxpool.Pool, logger *zap.Logger) *RevenueRecoveryHandler {
	return &RevenueRecoveryHandler{db: db, logger: logger}
}

// GetSummary godoc
// GET /api/v1/revenue/summary?district_id=&period=
// Returns aggregate recovery stats: total recovered, total success fee, count by type.
func (h *RevenueRecoveryHandler) GetSummary(c *fiber.Ctx) error {
	ctx := c.UserContext()

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
		TotalEvents       int     `json:"total_events"`
		TotalVarianceGHS  float64 `json:"total_variance_ghs"`
		TotalRecoveredGHS float64 `json:"total_recovered_ghs"`
		TotalSuccessFeeGHS float64 `json:"total_success_fee_ghs"`
		PendingCount      int     `json:"pending_count"`
		ConfirmedCount    int     `json:"confirmed_count"`
		CollectedCount    int     `json:"collected_count"`
		ByType            []struct {
			RecoveryType   string  `json:"recovery_type"`
			Count          int     `json:"count"`
			RecoveredGHS   float64 `json:"recovered_ghs"`
			SuccessFeeGHS  float64 `json:"success_fee_ghs"`
		} `json:"by_type"`
	}

	var s Summary

	row := h.db.QueryRow(ctx, `
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
	rows, err := h.db.Query(ctx, `
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
		ID             uuid.UUID  `json:"id"`
		AuditEventID   uuid.UUID  `json:"audit_event_id"`
		DistrictName   string     `json:"district_name"`
		AccountNumber  string     `json:"account_number"`
		AccountHolder  string     `json:"account_holder"`
		VarianceGHS    float64    `json:"variance_ghs"`
		RecoveredGHS   float64    `json:"recovered_ghs"`
		SuccessFeeGHS  float64    `json:"success_fee_ghs"`
		RecoveryType   string     `json:"recovery_type"`
		Status         string     `json:"status"`
		ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
		CollectedAt    *time.Time `json:"collected_at,omitempty"`
		CreatedAt      time.Time  `json:"created_at"`
	}

	var total int
	h.db.QueryRow(ctx, `SELECT COUNT(*) FROM revenue_recovery_events rr WHERE `+where, args...).Scan(&total)

	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	args = append(args, limit, offset)

	rows, err := h.db.Query(ctx, `
		SELECT rr.id, rr.audit_event_id,
		       d.district_name, wa.gwl_account_number, wa.account_holder_name,
		       rr.variance_ghs, rr.recovered_ghs, rr.success_fee_ghs,
		       rr.recovery_type, rr.status, rr.confirmed_at, rr.collected_at, rr.created_at
		FROM revenue_recovery_events rr
		JOIN districts d ON rr.district_id = d.id
		JOIN water_accounts wa ON rr.account_id = wa.id
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
func (h *RevenueRecoveryHandler) ConfirmRecovery(c *fiber.Ctx) error {
	ctx := c.UserContext()
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

	_, err = h.db.Exec(ctx, `
		UPDATE revenue_recovery_events
		SET status        = 'CONFIRMED',
		    recovered_ghs = $1,
		    confirmed_at  = NOW(),
		    confirmed_by  = $2::uuid,
		    notes         = COALESCE(NULLIF($3,''), notes),
		    updated_at    = NOW()
		WHERE id = $4 AND status = 'PENDING'`,
		req.RecoveredGHS, confirmedBy, req.Notes, id,
	)
	if err != nil {
		h.logger.Error("ConfirmRecovery failed", zap.Error(err))
		return response.InternalError(c, "Failed to confirm recovery")
	}
	return response.OK(c, fiber.Map{"message": "Recovery confirmed"})
}
