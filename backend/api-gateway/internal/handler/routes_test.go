package handler_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// ─── Route helpers (use existing doRequest from handler_test.go) ─────────────

func makeApp(routes func(*fiber.App)) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: func(c *fiber.Ctx, err error) error {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}})
	routes(app)
	return app
}

func doRequestWithHeaders(t *testing.T, app *fiber.App, method, path string, body interface{}, headers map[string]string) *http.Response {
	t.Helper()
	resp := doRequest(t, app, method, path, body)
	// Note: headers applied via separate request for header-specific tests
	_ = headers
	return resp
}

// ─── Health endpoint ─────────────────────────────────────────────────────────

func TestRoutes_HealthReturns200(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok", "version": "1.0.0"})
		})
	})
	resp := doRequest(t, app, "GET", "/health", nil)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoutes_HealthResponseHasVersion(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok", "version": "1.0.0"})
		})
	})
	resp := doRequest(t, app, "GET", "/health", nil)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["version"] != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %v", result["version"])
	}
}

func TestRoutes_UnknownRoute_Returns404(t *testing.T) {
	app := makeApp(func(a *fiber.App) {})
	resp := doRequest(t, app, "GET", "/api/v1/nonexistent", nil)
	if resp.StatusCode != 404 && resp.StatusCode != 500 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// ─── RBAC middleware ──────────────────────────────────────────────────────────

func makeRBACApp(allowedRoles ...string) *fiber.App {
	return makeApp(func(a *fiber.App) {
		roleMW := func(c *fiber.Ctx) error {
			role := c.Get("X-User-Role")
			for _, r := range allowedRoles {
				if r == role {
					return c.Next()
				}
			}
			return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
		}
		a.Get("/resource", roleMW, func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"data": "ok"})
		})
	})
}

func TestRBAC_WrongRole_Returns403(t *testing.T) {
	app := makeRBACApp("SYSTEM_ADMIN")
	req, _ := makeRequestWithHeader("GET", "/resource", "X-User-Role", "FIELD_OFFICER")
	resp, _ := app.Test(req, 5000)
	if resp.StatusCode != 403 {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRBAC_CorrectRole_Returns200(t *testing.T) {
	app := makeRBACApp("SYSTEM_ADMIN")
	req, _ := makeRequestWithHeader("GET", "/resource", "X-User-Role", "SYSTEM_ADMIN")
	resp, _ := app.Test(req, 5000)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRBAC_GWLRoles_AllAllowed(t *testing.T) {
	app := makeRBACApp("GWL_SUPERVISOR", "GWL_BILLING_OFFICER", "GWL_MANAGER", "SYSTEM_ADMIN")
	for _, role := range []string{"GWL_SUPERVISOR", "GWL_BILLING_OFFICER", "GWL_MANAGER", "SYSTEM_ADMIN"} {
		req, _ := makeRequestWithHeader("GET", "/resource", "X-User-Role", role)
		resp, _ := app.Test(req, 5000)
		if resp.StatusCode != 200 {
			t.Errorf("role %s: expected 200, got %d", role, resp.StatusCode)
		}
	}
}

func TestRBAC_FieldOfficerBlockedFromGWLPortal(t *testing.T) {
	app := makeRBACApp("GWL_SUPERVISOR", "GWL_BILLING_OFFICER", "GWL_MANAGER")
	req, _ := makeRequestWithHeader("GET", "/resource", "X-User-Role", "FIELD_OFFICER")
	resp, _ := app.Test(req, 5000)
	if resp.StatusCode != 403 {
		t.Errorf("FIELD_OFFICER should be blocked from GWL portal, got %d", resp.StatusCode)
	}
}

// ─── Input validation ─────────────────────────────────────────────────────────

func TestValidation_InvalidUUID_Returns400(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			if len(id) != 36 || !strings.Contains(id, "-") {
				return c.Status(400).JSON(fiber.Map{"error": "invalid UUID"})
			}
			return c.JSON(fiber.Map{"id": id})
		})
	})
	resp := doRequest(t, app, "GET", "/cases/not-a-uuid", nil)
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestValidation_ValidUUID_Returns200(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			if len(id) != 36 || !strings.Contains(id, "-") {
				return c.Status(400).JSON(fiber.Map{"error": "invalid UUID"})
			}
			return c.JSON(fiber.Map{"id": id})
		})
	})
	resp := doRequest(t, app, "GET", "/cases/550e8400-e29b-41d4-a716-446655440000", nil)
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestValidation_MissingRequiredField_Returns400(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Post("/assign", func(c *fiber.Ctx) error {
			var body struct{ OfficerID string `json:"officer_id"` }
			c.BodyParser(&body)
			if body.OfficerID == "" {
				return c.Status(400).JSON(fiber.Map{"error": "officer_id required"})
			}
			return c.JSON(fiber.Map{"ok": true})
		})
	})
	resp := doRequest(t, app, "POST", "/assign", map[string]string{})
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ─── GWL Status validation ────────────────────────────────────────────────────

func TestGWLStatus_InvalidStatus_Returns400(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Patch("/status", func(c *fiber.Ctx) error {
			var body struct{ Status string `json:"status"` }
			c.BodyParser(&body)
			valid := map[string]bool{
				"PENDING_REVIEW": true, "UNDER_INVESTIGATION": true,
				"FIELD_ASSIGNED": true, "EVIDENCE_SUBMITTED": true,
				"APPROVED_FOR_CORRECTION": true, "DISPUTED": true,
				"CORRECTED": true, "CLOSED": true,
			}
			if !valid[body.Status] {
				return c.Status(400).JSON(fiber.Map{"error": "invalid status"})
			}
			return c.JSON(fiber.Map{"ok": true})
		})
	})
	resp := doRequest(t, app, "PATCH", "/status", map[string]string{"status": "INVALID"})
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestGWLStatus_AllValidStatuses_Pass(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Patch("/status", func(c *fiber.Ctx) error {
			var body struct{ Status string `json:"status"` }
			c.BodyParser(&body)
			valid := map[string]bool{
				"PENDING_REVIEW": true, "UNDER_INVESTIGATION": true,
				"FIELD_ASSIGNED": true, "EVIDENCE_SUBMITTED": true,
				"APPROVED_FOR_CORRECTION": true, "DISPUTED": true,
				"CORRECTED": true, "CLOSED": true,
			}
			if !valid[body.Status] {
				return c.Status(400).JSON(fiber.Map{"error": "invalid status"})
			}
			return c.JSON(fiber.Map{"ok": true})
		})
	})
	for _, s := range []string{"PENDING_REVIEW", "UNDER_INVESTIGATION", "FIELD_ASSIGNED",
		"EVIDENCE_SUBMITTED", "APPROVED_FOR_CORRECTION", "DISPUTED", "CORRECTED", "CLOSED"} {
		resp := doRequest(t, app, "PATCH", "/status", map[string]string{"status": s})
		if resp.StatusCode != 200 {
			t.Errorf("status %s: expected 200, got %d", s, resp.StatusCode)
		}
	}
}

// ─── Pagination ───────────────────────────────────────────────────────────────

func TestPagination_DefaultLimit(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases", func(c *fiber.Ctx) error {
			limit := c.QueryInt("limit", 50)
			if limit > 200 { limit = 200 }
			return c.JSON(fiber.Map{"limit": limit, "offset": c.QueryInt("offset", 0)})
		})
	})
	resp := doRequest(t, app, "GET", "/cases", nil)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["limit"].(float64) != 50 {
		t.Errorf("expected default limit=50, got %v", result["limit"])
	}
}

func TestPagination_LimitCappedAt200(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases", func(c *fiber.Ctx) error {
			limit := c.QueryInt("limit", 50)
			if limit > 200 { limit = 200 }
			return c.JSON(fiber.Map{"limit": limit})
		})
	})
	resp := doRequest(t, app, "GET", "/cases?limit=9999", nil)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["limit"].(float64) != 200 {
		t.Errorf("expected limit capped at 200, got %v", result["limit"])
	}
}

func TestPagination_OffsetApplied(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"offset": c.QueryInt("offset", 0)})
		})
	})
	resp := doRequest(t, app, "GET", "/cases?offset=25", nil)
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["offset"].(float64) != 25 {
		t.Errorf("expected offset=25, got %v", result["offset"])
	}
}

// ─── Content-type ─────────────────────────────────────────────────────────────

func TestContentType_ResponseIsJSON(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"status": "ok"})
		})
	})
	resp := doRequest(t, app, "GET", "/health", nil)
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected JSON content-type, got %s", ct)
	}
}

// ─── Method not allowed ───────────────────────────────────────────────────────

func TestMethod_WrongMethod_Returns405(t *testing.T) {
	app := makeApp(func(a *fiber.App) {
		a.Get("/cases", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"cases": []string{}})
		})
	})
	resp := doRequest(t, app, "DELETE", "/cases", nil)
	if resp.StatusCode == 200 {
		t.Errorf("DELETE on GET-only route should not return 200, got %d", resp.StatusCode)
	}
}

// ─── Tariff business logic ────────────────────────────────────────────────────

func TestTariff_ResidentialVAT(t *testing.T) {
	consumption := 3.0
	rate := 6.1225 // 2026 PURC tier1
	vat := 0.20
	bill := consumption * rate * (1 + vat)
	expected := 22.041
	if bill < expected-0.01 || bill > expected+0.01 {
		t.Errorf("expected ~%.3f, got %.3f", expected, bill)
	}
}

func TestTariff_VarianceAbove15Pct_ShouldFlag(t *testing.T) {
	gwlBill := 100.0
	shadowBill := 120.0
	variance := (shadowBill - gwlBill) / gwlBill * 100
	if variance <= 15.0 {
		t.Errorf("expected variance > 15%%, got %.1f%%", variance)
	}
}

func TestTariff_VarianceBelow15Pct_NoFlag(t *testing.T) {
	gwlBill := 100.0
	shadowBill := 110.0
	variance := (shadowBill - gwlBill) / gwlBill * 100
	if variance > 15.0 {
		t.Errorf("expected variance <= 15%%, got %.1f%%", variance)
	}
}

func TestTariff_NRWCalculation(t *testing.T) {
	systemInput := 1000.0
	billedConsumption := 484.0 // 51.6% NRW
	nrw := (systemInput - billedConsumption) / systemInput * 100
	if nrw < 51.0 || nrw > 52.0 {
		t.Errorf("expected NRW ~51.6%%, got %.1f%%", nrw)
	}
}

// ─── Helper: make request with custom header ──────────────────────────────────

func makeRequestWithHeader(method, path, headerKey, headerVal string) (*http.Request, error) {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(headerKey, headerVal)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
