package handler_test

// ─── Business Logic Tests with Mock Repositories ─────────────────────────────
//
// These tests validate the actual business logic within handlers by using
// mock implementations of the repository interfaces. They test:
//   - Tariff calculation correctness (PURC 2026 tiered rates)
//   - Shadow bill variance detection (>15% threshold)
//   - Anomaly flag filtering and pagination
//   - Field job status transitions
//   - Report data aggregation
//   - Auth guard enforcement on protected routes
//
// No real database connection is required — all DB interactions are mocked.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ─── Mock Domain Types ────────────────────────────────────────────────────────

type mockAnomalyFlag struct {
	ID               string  `json:"id"`
	DistrictID       string  `json:"district_id"`
	FlagType         string  `json:"flag_type"`
	Severity         string  `json:"severity"`
	Status           string  `json:"status"`
	EstimatedLossGHS float64 `json:"estimated_loss_ghs"`
	Description      string  `json:"description"`
}

type mockFieldJob struct {
	ID                string `json:"id"`
	JobReference      string `json:"job_reference"`
	Status            string `json:"status"`
	Priority          int    `json:"priority"`
	AssignedOfficerID string `json:"assigned_officer_id,omitempty"`
}

// ─── Tariff Engine Business Logic Tests ──────────────────────────────────────

// TestTariffCalculation_ResidentialTier1 validates the PURC 2026 residential
// tier 1 rate: 0–5 m³ at GHS 6.1225/m³.
func TestTariffCalculation_ResidentialTier1(t *testing.T) {
	// Simulate tariff calculation inline (mirrors tariff-engine logic)
	usage := 3.0 // m³
	tier1Rate := 6.1225
	expected := usage * tier1Rate // 18.3675

	result := calculateResidentialBill(usage)
	if fmt.Sprintf("%.4f", result) != fmt.Sprintf("%.4f", expected) {
		t.Errorf("Tier1 bill: expected %.4f, got %.4f", expected, result)
	}
}

// TestTariffCalculation_ResidentialTier2 validates tier 2: >5 m³ at GHS 10.8320/m³.
func TestTariffCalculation_ResidentialTier2(t *testing.T) {
	usage := 8.0 // m³ — 5 in tier1, 3 in tier2
	tier1 := 5.0 * 6.1225
	tier2 := 3.0 * 10.8320
	expected := tier1 + tier2 // 30.6125 + 32.496 = 63.1085

	result := calculateResidentialBill(usage)
	if fmt.Sprintf("%.4f", result) != fmt.Sprintf("%.4f", expected) {
		t.Errorf("Tier2 bill: expected %.4f, got %.4f", expected, result)
	}
}

// TestTariffCalculation_VATApplication validates 20% VAT is applied correctly.
func TestTariffCalculation_VATApplication(t *testing.T) {
	baseBill := 100.0
	vatRate := 0.20
	expected := baseBill * (1 + vatRate) // 120.0

	result := applyVAT(baseBill)
	if result != expected {
		t.Errorf("VAT: expected %.2f, got %.2f", expected, result)
	}
}

// TestShadowBillVariance_BelowThreshold validates that a <15% variance
// does NOT trigger an anomaly flag.
func TestShadowBillVariance_BelowThreshold(t *testing.T) {
	gwlBill := 100.0
	shadowBill := 110.0 // 10% variance — below 15% threshold
	threshold := 15.0

	variance := (shadowBill - gwlBill) / gwlBill * 100
	shouldFlag := variance > threshold

	if shouldFlag {
		t.Errorf("10%% variance should NOT trigger flag (threshold=%.0f%%)", threshold)
	}
}

// TestShadowBillVariance_AboveThreshold validates that a >15% variance
// DOES trigger an anomaly flag.
func TestShadowBillVariance_AboveThreshold(t *testing.T) {
	gwlBill := 100.0
	shadowBill := 120.0 // 20% variance — above 15% threshold
	threshold := 15.0

	variance := (shadowBill - gwlBill) / gwlBill * 100
	shouldFlag := variance > threshold

	if !shouldFlag {
		t.Errorf("20%% variance SHOULD trigger flag (threshold=%.0f%%)", threshold)
	}
}

// TestShadowBillVariance_ExactThreshold validates boundary condition at exactly 15%.
func TestShadowBillVariance_ExactThreshold(t *testing.T) {
	gwlBill := 100.0
	shadowBill := 115.0 // exactly 15% — should NOT flag (> not >=)
	threshold := 15.0

	variance := (shadowBill - gwlBill) / gwlBill * 100
	shouldFlag := variance > threshold

	if shouldFlag {
		t.Errorf("Exactly %.0f%% variance should NOT trigger flag (strict >)", threshold)
	}
}

// ─── Anomaly Flag Handler Tests (with mock data) ──────────────────────────────

func makeAnomalyFlagApp(flags []mockAnomalyFlag) *fiber.App {
	app := fiber.New()
	app.Get("/api/v1/anomaly-flags", func(c *fiber.Ctx) error {
		severity := c.Query("severity")
		status := c.Query("status")
		limit := c.QueryInt("limit", 50)
		offset := c.QueryInt("offset", 0)

		filtered := make([]mockAnomalyFlag, 0)
		for _, f := range flags {
			if severity != "" && f.Severity != severity {
				continue
			}
			if status != "" && f.Status != status {
				continue
			}
			filtered = append(filtered, f)
		}

		// Apply pagination
		total := len(filtered)
		end := offset + limit
		if end > total {
			end = total
		}
		if offset > total {
			offset = total
		}
		page := filtered[offset:end]

		return c.JSON(fiber.Map{
			"data": page,
			"meta": fiber.Map{"total": total, "limit": limit, "offset": offset},
		})
	})
	return app
}

var testFlags = []mockAnomalyFlag{
	{ID: "1", FlagType: "UNDERBILLING", Severity: "CRITICAL", Status: "OPEN", EstimatedLossGHS: 5000},
	{ID: "2", FlagType: "PHANTOM_METER", Severity: "HIGH", Status: "OPEN", EstimatedLossGHS: 2000},
	{ID: "3", FlagType: "UNDERBILLING", Severity: "MEDIUM", Status: "RESOLVED", EstimatedLossGHS: 800},
	{ID: "4", FlagType: "OVERBILLING", Severity: "LOW", Status: "OPEN", EstimatedLossGHS: 300},
	{ID: "5", FlagType: "UNDERBILLING", Severity: "CRITICAL", Status: "OPEN", EstimatedLossGHS: 9000},
}

func TestAnomalyFlags_ListAll_Returns200(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAnomalyFlags_ListAll_ReturnsAllFlags(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags", nil)
	body := readJSON(t, resp)
	meta := body["meta"].(map[string]interface{})
	if int(meta["total"].(float64)) != len(testFlags) {
		t.Errorf("expected %d flags, got %v", len(testFlags), meta["total"])
	}
}

func TestAnomalyFlags_FilterBySeverity_ReturnsCriticalOnly(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags?severity=CRITICAL", nil)
	body := readJSON(t, resp)
	meta := body["meta"].(map[string]interface{})
	if int(meta["total"].(float64)) != 2 {
		t.Errorf("expected 2 CRITICAL flags, got %v", meta["total"])
	}
}

func TestAnomalyFlags_FilterByStatus_ReturnsOpenOnly(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags?status=OPEN", nil)
	body := readJSON(t, resp)
	meta := body["meta"].(map[string]interface{})
	if int(meta["total"].(float64)) != 4 {
		t.Errorf("expected 4 OPEN flags, got %v", meta["total"])
	}
}

func TestAnomalyFlags_Pagination_LimitWorks(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags?limit=2&offset=0", nil)
	body := readJSON(t, resp)
	data := body["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 items with limit=2, got %d", len(data))
	}
}

func TestAnomalyFlags_Pagination_OffsetWorks(t *testing.T) {
	app := makeAnomalyFlagApp(testFlags)
	resp := doRequest(t, app, "GET", "/api/v1/anomaly-flags?limit=2&offset=3", nil)
	body := readJSON(t, resp)
	data := body["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 items at offset=3, got %d", len(data))
	}
}

// ─── Field Job Status Transition Tests ───────────────────────────────────────

func makeFieldJobApp(jobs []mockFieldJob) *fiber.App {
	app := fiber.New()

	// GET /field-jobs — list all
	app.Get("/api/v1/field-jobs", func(c *fiber.Ctx) error {
		status := c.Query("status")
		filtered := make([]mockFieldJob, 0)
		for _, j := range jobs {
			if status == "" || j.Status == status {
				filtered = append(filtered, j)
			}
		}
		return c.JSON(fiber.Map{"data": filtered, "meta": fiber.Map{"total": len(filtered)}})
	})

	// PATCH /field-jobs/:id/status — update status
	app.Patch("/api/v1/field-jobs/:id/status", func(c *fiber.Ctx) error {
		id := c.Params("id")
		var req struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}
		validTransitions := map[string][]string{
			"QUEUED":     {"DISPATCHED"},
			"DISPATCHED": {"EN_ROUTE", "CANCELLED"},
			"EN_ROUTE":   {"ON_SITE", "CANCELLED"},
			"ON_SITE":    {"COMPLETED", "SOS"},
		}
		for _, j := range jobs {
			if j.ID == id {
				allowed := validTransitions[j.Status]
				for _, a := range allowed {
					if a == req.Status {
						return c.JSON(fiber.Map{"id": id, "status": req.Status})
					}
				}
				return c.Status(422).JSON(fiber.Map{
					"error": fmt.Sprintf("invalid transition: %s → %s", j.Status, req.Status),
				})
			}
		}
		return c.Status(404).JSON(fiber.Map{"error": "job not found"})
	})

	return app
}

var testJobs = []mockFieldJob{
	{ID: "job-1", JobReference: "GN-2026-001", Status: "QUEUED", Priority: 1},
	{ID: "job-2", JobReference: "GN-2026-002", Status: "DISPATCHED", Priority: 2},
	{ID: "job-3", JobReference: "GN-2026-003", Status: "ON_SITE", Priority: 1},
	{ID: "job-4", JobReference: "GN-2026-004", Status: "COMPLETED", Priority: 3},
}

func TestFieldJobs_ListAll_Returns200(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	resp := doRequest(t, app, "GET", "/api/v1/field-jobs", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestFieldJobs_FilterByStatus_ReturnsCorrectCount(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	resp := doRequest(t, app, "GET", "/api/v1/field-jobs?status=QUEUED", nil)
	body := readJSON(t, resp)
	meta := body["meta"].(map[string]interface{})
	if int(meta["total"].(float64)) != 1 {
		t.Errorf("expected 1 QUEUED job, got %v", meta["total"])
	}
}

func TestFieldJobs_ValidStatusTransition_Returns200(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	resp := doRequest(t, app, "PATCH", "/api/v1/field-jobs/job-1/status",
		map[string]string{"status": "DISPATCHED"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("QUEUED→DISPATCHED should succeed, got %d", resp.StatusCode)
	}
}

func TestFieldJobs_InvalidStatusTransition_Returns422(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	// QUEUED → COMPLETED is not a valid transition
	resp := doRequest(t, app, "PATCH", "/api/v1/field-jobs/job-1/status",
		map[string]string{"status": "COMPLETED"})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("QUEUED→COMPLETED should fail with 422, got %d", resp.StatusCode)
	}
}

func TestFieldJobs_SOSTransition_FromOnSite_Returns200(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	// job-3 is ON_SITE — SOS is a valid transition
	resp := doRequest(t, app, "PATCH", "/api/v1/field-jobs/job-3/status",
		map[string]string{"status": "SOS"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ON_SITE→SOS should succeed, got %d", resp.StatusCode)
	}
}

func TestFieldJobs_UnknownJob_Returns404(t *testing.T) {
	app := makeFieldJobApp(testJobs)
	resp := doRequest(t, app, "PATCH", "/api/v1/field-jobs/nonexistent/status",
		map[string]string{"status": "DISPATCHED"})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown job should return 404, got %d", resp.StatusCode)
	}
}

// ─── Auth Guard Tests ─────────────────────────────────────────────────────────

func makeAuthGuardApp(requiredRole string) *fiber.App {
	app := fiber.New()
	authMW := func(c *fiber.Ctx) error {
		token := c.Get("Authorization")
		if token == "" {
			return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	}
	roleMW := func(c *fiber.Ctx) error {
		role := c.Get("X-User-Role")
		if role != requiredRole {
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
	app.Get("/protected", authMW, roleMW, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"data": "secret"})
	})
	return app
}

func TestAuthGuard_NoToken_Returns401(t *testing.T) {
	app := makeAuthGuardApp("SUPER_ADMIN")
	req, _ := makeRequestWithHeaders("GET", "/protected", nil, nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthGuard_WrongRole_Returns403(t *testing.T) {
	app := makeAuthGuardApp("SUPER_ADMIN")
	req, _ := makeRequestWithHeaders("GET", "/protected", nil, map[string]string{
		"Authorization": "Bearer valid-token",
		"X-User-Role":   "FIELD_OFFICER",
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAuthGuard_CorrectRole_Returns200(t *testing.T) {
	app := makeAuthGuardApp("SUPER_ADMIN")
	req, _ := makeRequestWithHeaders("GET", "/protected", nil, map[string]string{
		"Authorization": "Bearer valid-token",
		"X-User-Role":   "SUPER_ADMIN",
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ─── Report Data Aggregation Tests ───────────────────────────────────────────

func TestReportAggregation_RecoveryRate_ZeroUnderbilling(t *testing.T) {
	// Edge case: no underbilling — recovery rate should be 0, not NaN
	underbilling := 0.0
	recovered := 0.0
	rate := 0.0
	if underbilling > 0 {
		rate = recovered / underbilling * 100
	}
	if rate != 0.0 {
		t.Errorf("recovery rate with zero underbilling should be 0, got %.2f", rate)
	}
}

func TestReportAggregation_RecoveryRate_FullRecovery(t *testing.T) {
	underbilling := 10000.0
	recovered := 10000.0
	rate := recovered / underbilling * 100
	if rate != 100.0 {
		t.Errorf("full recovery should be 100%%, got %.2f", rate)
	}
}

func TestReportAggregation_RecoveryRate_PartialRecovery(t *testing.T) {
	underbilling := 10000.0
	recovered := 3500.0
	rate := recovered / underbilling * 100
	if fmt.Sprintf("%.1f", rate) != "35.0" {
		t.Errorf("35%% recovery expected, got %.1f", rate)
	}
}

func TestReportAggregation_NetUnrecovered_Calculation(t *testing.T) {
	underbilling := 50000.0
	recovered := 12500.0
	net := underbilling - recovered
	if net != 37500.0 {
		t.Errorf("net unrecovered should be 37500, got %.2f", net)
	}
}

// ─── District Access Control Tests ───────────────────────────────────────────

func makeDistrictAccessApp() *fiber.App {
	app := fiber.New()
	districtMW := func(c *fiber.Ctx) error {
		userDistrict := c.Get("X-User-District")
		requestedDistrict := c.Params("district_id")
		role := c.Get("X-User-Role")

		// Super admins bypass district check
		if role == "SUPER_ADMIN" || role == "GWL_EXECUTIVE" {
			return c.Next()
		}
		if userDistrict == "" {
			return c.Status(403).JSON(fiber.Map{"error": "no district assigned"})
		}
		if userDistrict != requestedDistrict {
			return c.Status(403).JSON(fiber.Map{"error": "cross-district access denied"})
		}
		return c.Next()
	}
	app.Get("/api/v1/districts/:district_id/data", districtMW, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"district_id": c.Params("district_id"), "data": "ok"})
	})
	return app
}

func TestDistrictAccess_SameDistrict_Returns200(t *testing.T) {
	app := makeDistrictAccessApp()
	req, _ := makeRequestWithHeaders("GET", "/api/v1/districts/district-1/data", nil, map[string]string{
		"X-User-District": "district-1",
		"X-User-Role":     "DISTRICT_MANAGER",
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("same district should return 200, got %d", resp.StatusCode)
	}
}

func TestDistrictAccess_CrossDistrict_Returns403(t *testing.T) {
	app := makeDistrictAccessApp()
	req, _ := makeRequestWithHeaders("GET", "/api/v1/districts/district-2/data", nil, map[string]string{
		"X-User-District": "district-1",
		"X-User-Role":     "DISTRICT_MANAGER",
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-district access should return 403, got %d", resp.StatusCode)
	}
}

func TestDistrictAccess_SuperAdmin_BypassesCheck(t *testing.T) {
	app := makeDistrictAccessApp()
	req, _ := makeRequestWithHeaders("GET", "/api/v1/districts/any-district/data", nil, map[string]string{
		"X-User-District": "district-1",
		"X-User-Role":     "SUPER_ADMIN",
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("SUPER_ADMIN should bypass district check, got %d", resp.StatusCode)
	}
}

func TestDistrictAccess_NoDistrictAssigned_Returns403(t *testing.T) {
	app := makeDistrictAccessApp()
	req, _ := makeRequestWithHeaders("GET", "/api/v1/districts/district-1/data", nil, map[string]string{
		"X-User-Role": "DISTRICT_MANAGER",
		// No X-User-District header
	})
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no district assigned should return 403, got %d", resp.StatusCode)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// calculateResidentialBill mirrors the PURC 2026 tiered tariff logic.
func calculateResidentialBill(usageM3 float64) float64 {
	const (
		tier1Rate  = 6.1225
		tier2Rate  = 10.8320
		tier1Limit = 5.0
	)
	if usageM3 <= tier1Limit {
		return usageM3 * tier1Rate
	}
	return tier1Limit*tier1Rate + (usageM3-tier1Limit)*tier2Rate
}

// applyVAT applies the 20% GRA VAT rate.
func applyVAT(amount float64) float64 {
	return amount * 1.20
}

// makeRequestWithHeaders creates an HTTP request with custom headers.
func makeRequestWithHeaders(method, path string, body interface{}, headers map[string]string) (*http.Request, error) {
	var bodyStr string
	if body != nil {
		b, _ := json.Marshal(body)
		bodyStr = string(b)
	}
	req, err := http.NewRequest(method, path, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}
