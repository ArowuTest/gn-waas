package handler

import (
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GWLHandler handles all GWL Case Management Portal API endpoints.
// All routes require the GWL_SUPERVISOR, GWL_BILLING_OFFICER, or GWL_MANAGER role.
type GWLHandler struct {
	caseRepo *repository.GWLCaseRepository
	logger   *zap.Logger
}

func NewGWLHandler(caseRepo *repository.GWLCaseRepository, logger *zap.Logger) *GWLHandler {
	return &GWLHandler{caseRepo: caseRepo, logger: logger}
}

// ── GET /api/v1/gwl/cases/summary ────────────────────────────────────────────
// Returns KPI strip data for the dashboard header
func (h *GWLHandler) GetCaseSummary(c *fiber.Ctx) error {
	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	summary, err := h.caseRepo.GetCaseSummary(c.UserContext(), districtID)
	if err != nil {
		h.logger.Error("GetCaseSummary failed", zap.Error(err))
		return response.InternalError(c, "failed to load case summary")
	}
	return response.OK(c, summary)
}

// ── GET /api/v1/gwl/cases ────────────────────────────────────────────────────
// Returns paginated case queue with filters
func (h *GWLHandler) ListCases(c *fiber.Ctx) error {
	filter := repository.GWLCaseFilter{
		FlagType:  c.Query("flag_type"),
		Severity:  c.Query("severity"),
		GWLStatus: c.Query("gwl_status"),
		Search:    c.Query("search"),
		SortBy:    c.Query("sort_by", "estimated_loss_ghs"),
		SortDir:   c.Query("sort_dir", "DESC"),
		Limit:     c.QueryInt("limit", 50),
		Offset:    c.QueryInt("offset", 0),
	}

	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		filter.DistrictID = &id
	}
	if a := c.Query("assigned_to_id"); a != "" {
		id, err := uuid.Parse(a)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid assigned_to_id")
		}
		filter.AssignedToID = &id
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse("2006-01-02", df)
		if err == nil {
			filter.DateFrom = &t
		}
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse("2006-01-02", dt)
		if err == nil {
			filter.DateTo = &t
		}
	}

	cases, total, err := h.caseRepo.ListCases(c.UserContext(), filter)
	if err != nil {
		h.logger.Error("ListCases failed", zap.Error(err))
		return response.InternalError(c, "failed to list cases")
	}

	return response.OK(c, fiber.Map{
		"cases":  cases,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// ── GET /api/v1/gwl/cases/:id ────────────────────────────────────────────────
// Returns full case detail including evidence, billing comparison, action history
func (h *GWLHandler) GetCase(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	gwlCase, err := h.caseRepo.GetCaseByID(c.UserContext(), id)
	if err != nil {
		h.logger.Error("GetCase failed", zap.Error(err))
		return response.NotFound(c, "case not found")
	}

	actions, err := h.caseRepo.GetCaseActions(c.UserContext(), id)
	if err != nil {
		h.logger.Warn("GetCaseActions failed", zap.Error(err))
	}

	return response.OK(c, fiber.Map{
		"case":    gwlCase,
		"actions": actions,
	})
}

// ── POST /api/v1/gwl/cases/:id/assign ────────────────────────────────────────
// Assigns a case to a field officer and creates a field job
func (h *GWLHandler) AssignToFieldOfficer(c *fiber.Ctx) error {
	flagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	var body struct {
		OfficerID   string `json:"officer_id"`
		AccountID   string `json:"account_id"`
		JobType     string `json:"job_type"`
		Priority    string `json:"priority"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
		PerformedBy string `json:"performed_by"`
		Role        string `json:"role"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid request body")
	}
	if body.OfficerID == "" || body.AccountID == "" {
		return response.BadRequest(c, "BAD_REQUEST", "officer_id and account_id are required")
	}

	officerID, err := uuid.Parse(body.OfficerID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid officer_id")
	}
	accountID, err := uuid.Parse(body.AccountID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid account_id")
	}

	dueDate := time.Now().AddDate(0, 0, 7)
	if body.DueDate != "" {
		if t, err := time.Parse("2006-01-02", body.DueDate); err == nil {
			dueDate = t
		}
	}

	jobType := body.JobType
	if jobType == "" { jobType = "METER_INSPECTION" }
	priority := body.Priority
	if priority == "" { priority = "HIGH" }
	title := body.Title
	if title == "" { title = "Field Verification Required" }
	description := body.Description
	if description == "" { description = "GN-WAAS anomaly detected. Please inspect meter and verify account details." }
	performedBy := body.PerformedBy
	if performedBy == "" { performedBy = "GWL Supervisor" }
	role := body.Role
	if role == "" { role = "GWL_SUPERVISOR" }

	if err := h.caseRepo.AssignToFieldOfficer(
		c.UserContext(), flagID, officerID, accountID,
		priority, jobType, title, description, dueDate,
		performedBy, role,
	); err != nil {
		h.logger.Error("AssignToFieldOfficer failed", zap.Error(err))
		return response.InternalError(c, "failed to assign field officer")
	}

	return response.OK(c, fiber.Map{"message": "Field officer assigned successfully"})
}

// ── PATCH /api/v1/gwl/cases/:id/status ───────────────────────────────────────
// Updates the GWL workflow status of a case
func (h *GWLHandler) UpdateCaseStatus(c *fiber.Ctx) error {
	flagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	var body struct {
		Status      string  `json:"status"`
		Resolution  *string `json:"resolution"`
		Notes       *string `json:"notes"`
		PerformedBy string  `json:"performed_by"`
		Role        string  `json:"role"`
		ActionType  string  `json:"action_type"`
		ActionNotes string  `json:"action_notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid request body")
	}
	if body.Status == "" {
		return response.BadRequest(c, "BAD_REQUEST", "status is required")
	}

	validStatuses := map[string]bool{
		"PENDING_REVIEW": true, "UNDER_INVESTIGATION": true,
		"FIELD_ASSIGNED": true, "EVIDENCE_SUBMITTED": true,
		"APPROVED_FOR_CORRECTION": true, "DISPUTED": true,
		"CORRECTED": true, "CLOSED": true,
	}
	if !validStatuses[body.Status] {
		return response.BadRequest(c, "BAD_REQUEST", "invalid status value")
	}

	performedBy := body.PerformedBy
	if performedBy == "" { performedBy = "GWL User" }
	role := body.Role
	if role == "" { role = "GWL_SUPERVISOR" }
	actionType := body.ActionType
	if actionType == "" { actionType = body.Status }

	if err := h.caseRepo.UpdateCaseStatus(
		c.UserContext(), flagID, body.Status, nil,
		body.Resolution, body.Notes,
		performedBy, role, actionType, body.ActionNotes,
	); err != nil {
		h.logger.Error("UpdateCaseStatus failed", zap.Error(err))
		return response.InternalError(c, "failed to update case status")
	}

	return response.OK(c, fiber.Map{"message": "Case status updated"})
}

// ── POST /api/v1/gwl/cases/:id/reclassify ────────────────────────────────────
// Raises a formal reclassification request for a misclassified account
func (h *GWLHandler) RequestReclassification(c *fiber.Ctx) error {
	flagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	var body struct {
		AccountID           string  `json:"account_id"`
		DistrictID          string  `json:"district_id"`
		CurrentCategory     string  `json:"current_category"`
		RecommendedCategory string  `json:"recommended_category"`
		Justification       string  `json:"justification"`
		MonthlyImpact       float64 `json:"monthly_revenue_impact_ghs"`
		RequestedByName     string  `json:"requested_by_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid request body")
	}
	if body.AccountID == "" || body.CurrentCategory == "" || body.RecommendedCategory == "" {
		return response.BadRequest(c, "BAD_REQUEST", "account_id, current_category, and recommended_category are required")
	}

	accountID, err := uuid.Parse(body.AccountID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid account_id")
	}
	districtID, err := uuid.Parse(body.DistrictID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
	}

	req := &repository.ReclassificationRequest{
		AnomalyFlagID:       flagID,
		AccountID:           accountID,
		DistrictID:          districtID,
		CurrentCategory:     body.CurrentCategory,
		RecommendedCategory: body.RecommendedCategory,
		Justification:       body.Justification,
		MonthlyRevenueImpact: body.MonthlyImpact,
		AnnualRevenueImpact:  body.MonthlyImpact * 12,
		RequestedByName:     body.RequestedByName,
	}

	if err := h.caseRepo.CreateReclassificationRequest(c.UserContext(), req); err != nil {
		h.logger.Error("RequestReclassification failed", zap.Error(err))
		return response.InternalError(c, "failed to create reclassification request")
	}

	return response.OK(c, fiber.Map{"message": "Reclassification request submitted"})
}

// ── GET /api/v1/gwl/reclassifications ────────────────────────────────────────
// Lists all reclassification requests
func (h *GWLHandler) ListReclassifications(c *fiber.Ctx) error {
	status := c.Query("status")
	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	results, err := h.caseRepo.ListReclassificationRequests(c.UserContext(), status, districtID)
	if err != nil {
		h.logger.Error("ListReclassifications failed", zap.Error(err))
		return response.InternalError(c, "failed to list reclassifications")
	}
	return response.OK(c, results)
}

// ── POST /api/v1/gwl/cases/:id/credit ────────────────────────────────────────
// Raises a credit request for an overbilled customer
func (h *GWLHandler) RequestCredit(c *fiber.Ctx) error {
	flagID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	var body struct {
		AccountID          string  `json:"account_id"`
		DistrictID         string  `json:"district_id"`
		BillingPeriodStart string  `json:"billing_period_start"`
		BillingPeriodEnd   string  `json:"billing_period_end"`
		GWLAmountGHS       float64 `json:"gwl_amount_ghs"`
		ShadowAmountGHS    float64 `json:"shadow_amount_ghs"`
		CreditAmountGHS    float64 `json:"credit_amount_ghs"`
		Reason             string  `json:"reason"`
		Notes              *string `json:"notes"`
		RequestedByName    string  `json:"requested_by_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid request body")
	}
	if body.AccountID == "" || body.GWLAmountGHS == 0 {
		return response.BadRequest(c, "BAD_REQUEST", "account_id and gwl_amount_ghs are required")
	}

	accountID, err := uuid.Parse(body.AccountID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid account_id")
	}
	districtID, err := uuid.Parse(body.DistrictID)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
	}

	periodStart, _ := time.Parse("2006-01-02", body.BillingPeriodStart)
	periodEnd, _ := time.Parse("2006-01-02", body.BillingPeriodEnd)
	if periodStart.IsZero() {
		periodStart = time.Now().AddDate(0, -1, 0)
	}
	if periodEnd.IsZero() {
		periodEnd = time.Now()
	}

	overcharge := body.GWLAmountGHS - body.ShadowAmountGHS
	creditAmount := body.CreditAmountGHS
	if creditAmount == 0 {
		creditAmount = overcharge
	}

	req := &repository.CreditRequest{
		AnomalyFlagID:       flagID,
		AccountID:           accountID,
		DistrictID:          districtID,
		BillingPeriodStart:  periodStart,
		BillingPeriodEnd:    periodEnd,
		GWLAmountGHS:        body.GWLAmountGHS,
		ShadowAmountGHS:     body.ShadowAmountGHS,
		OverchargeAmountGHS: overcharge,
		CreditAmountGHS:     creditAmount,
		Reason:              body.Reason,
		Notes:               body.Notes,
		RequestedByName:     body.RequestedByName,
	}

	if err := h.caseRepo.CreateCreditRequest(c.UserContext(), req); err != nil {
		h.logger.Error("RequestCredit failed", zap.Error(err))
		return response.InternalError(c, "failed to create credit request")
	}

	return response.OK(c, fiber.Map{"message": "Credit request submitted"})
}

// ── GET /api/v1/gwl/credits ──────────────────────────────────────────────────
// Lists all credit requests
func (h *GWLHandler) ListCredits(c *fiber.Ctx) error {
	status := c.Query("status")
	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	results, err := h.caseRepo.ListCreditRequests(c.UserContext(), status, districtID)
	if err != nil {
		h.logger.Error("ListCredits failed", zap.Error(err))
		return response.InternalError(c, "failed to list credit requests")
	}
	return response.OK(c, results)
}

// ── GET /api/v1/gwl/reports/monthly ──────────────────────────────────────────
// Returns monthly summary report data
func (h *GWLHandler) GetMonthlyReport(c *fiber.Ctx) error {
	periodStr := c.Query("period", time.Now().AddDate(0, -1, 0).Format("2006-01"))
	period, err := time.Parse("2006-01", periodStr)
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid period format — use YYYY-MM")
	}

	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "BAD_REQUEST", "invalid district_id")
		}
		districtID = &id
	}

	report, err := h.caseRepo.GetMonthlyReport(c.UserContext(), period, districtID)
	if err != nil {
		h.logger.Error("GetMonthlyReport failed", zap.Error(err))
		return response.InternalError(c, "failed to generate monthly report")
	}
	return response.OK(c, report)
}

// ── GET /api/v1/gwl/cases/:id/actions ────────────────────────────────────────
// Returns the full audit trail for a case
func (h *GWLHandler) GetCaseActions(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "BAD_REQUEST", "invalid case id")
	}

	actions, err := h.caseRepo.GetCaseActions(c.UserContext(), id)
	if err != nil {
		h.logger.Error("GetCaseActions failed", zap.Error(err))
		return response.InternalError(c, "failed to get case actions")
	}
	return response.OK(c, actions)
}
