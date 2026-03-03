package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/handler"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// mockDBPinger implements the Ping interface for testing
type mockDBPinger struct{}
func (m *mockDBPinger) Ping(_ context.Context) error { return nil }


func newTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		// Suppress error output in tests
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		},
	})
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, path string, body interface{}) *http.Response {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	return resp
}

func readJSON(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}
	return m
}

// ─── Health Handler Tests ─────────────────────────────────────────────────────

func TestHealthHandler_Returns200(t *testing.T) {
	app := newTestApp()
	h := handler.NewHealthHandler(&mockDBPinger{})
	app.Get("/health", h.HealthCheck)

	resp := doRequest(t, app, "GET", "/health", nil)
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	body := readJSON(t, resp)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data object in response, got: %v", body)
	}
	if data["service"] != "api-gateway" {
		t.Errorf("Expected service=api-gateway, got %v", data["service"])
	}
	if data["status"] != "alive" {
		t.Errorf("Expected status=alive, got %v", data["status"])
	}
}

func TestHealthHandler_ResponseStructure(t *testing.T) {
	app := newTestApp()
	h := handler.NewHealthHandler(&mockDBPinger{})
	app.Get("/health", h.HealthCheck)

	resp := doRequest(t, app, "GET", "/health", nil)
	body := readJSON(t, resp)

	// Must have success:true and data object
	if body["success"] != true {
		t.Errorf("Expected success=true, got %v", body["success"])
	}
	if body["data"] == nil {
		t.Error("Expected data field in response")
	}
}

// ─── Admin User Handler Tests (no DB — auth guard only) ───────────────────────

func TestAdminUserHandler_ListUsers_RequiresSystemAdmin(t *testing.T) {
	app := newTestApp()
	h := handler.NewAdminUserHandler(nil, zap.NewNop())

	// Inject a non-admin role via locals middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "DISTRICT_MANAGER")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Get("/api/v1/admin/users", h.ListUsers)

	resp := doRequest(t, app, "GET", "/api/v1/admin/users", nil)
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for non-admin, got %d", resp.StatusCode)
	}
}

func TestAdminUserHandler_CreateUser_RequiresSystemAdmin(t *testing.T) {
	app := newTestApp()
	h := handler.NewAdminUserHandler(nil, zap.NewNop())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "FIELD_OFFICER")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Post("/api/v1/admin/users", h.CreateUser)

	resp := doRequest(t, app, "POST", "/api/v1/admin/users", map[string]string{
		"email": "test@gwl.gov.gh", "full_name": "Test User", "role": "FIELD_OFFICER",
	})
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for non-admin, got %d", resp.StatusCode)
	}
}

func TestAdminUserHandler_UpdateUser_InvalidUUID(t *testing.T) {
	app := newTestApp()
	h := handler.NewAdminUserHandler(nil, zap.NewNop())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "SYSTEM_ADMIN")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Patch("/api/v1/admin/users/:id", h.UpdateUser)

	resp := doRequest(t, app, "PATCH", "/api/v1/admin/users/not-a-uuid", map[string]bool{"is_active": false})
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for invalid UUID, got %d", resp.StatusCode)
	}
}

func TestAdminUserHandler_ResetPassword_RequiresSystemAdmin(t *testing.T) {
	app := newTestApp()
	h := handler.NewAdminUserHandler(nil, zap.NewNop())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "AUDIT_MANAGER")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Post("/api/v1/admin/users/:id/reset-password", h.ResetPassword)

	resp := doRequest(t, app, "POST", "/api/v1/admin/users/00000000-0000-0000-0000-000000000002/reset-password", nil)
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for non-admin, got %d", resp.StatusCode)
	}
}

// ─── District Handler Tests (no DB — auth guard only) ─────────────────────────

func TestDistrictHandler_CreateDistrict_RequiresSystemAdmin(t *testing.T) {
	app := newTestApp()
	// nil repo — we only test the auth guard, which fires before DB access
	h := handler.NewDistrictHandler(nil, zap.NewNop())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "READONLY_VIEWER")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Post("/api/v1/admin/districts", h.CreateDistrict)

	resp := doRequest(t, app, "POST", "/api/v1/admin/districts", map[string]string{
		"district_code": "TEST-001", "district_name": "Test District",
	})
	if resp.StatusCode != 401 {
		t.Errorf("Expected 401 for non-admin, got %d", resp.StatusCode)
	}
}

func TestDistrictHandler_UpdateDistrict_InvalidUUID(t *testing.T) {
	app := newTestApp()
	h := handler.NewDistrictHandler(nil, zap.NewNop())

	app.Use(func(c *fiber.Ctx) error {
		c.Locals("rls_user_role", "SYSTEM_ADMIN")
		c.Locals("user_id", "00000000-0000-0000-0000-000000000001")
		return c.Next()
	})
	app.Patch("/api/v1/admin/districts/:id", h.UpdateDistrict)

	resp := doRequest(t, app, "PATCH", "/api/v1/admin/districts/bad-id", map[string]string{"district_name": "New Name"})
	if resp.StatusCode != 400 {
		t.Errorf("Expected 400 for invalid UUID, got %d", resp.StatusCode)
	}
}

// ─── Response Structure Tests ─────────────────────────────────────────────────

func TestHealthHandler_ContentTypeJSON(t *testing.T) {
	app := newTestApp()
	h := handler.NewHealthHandler(&mockDBPinger{})
	app.Get("/health", h.HealthCheck)

	resp := doRequest(t, app, "GET", "/health", nil)
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		t.Error("Expected Content-Type header")
	}
	// Should contain application/json
	if len(ct) < 16 || ct[:16] != "application/json" {
		t.Errorf("Expected application/json content type, got %s", ct)
	}
}

func TestHealthHandler_Version(t *testing.T) {
	app := newTestApp()
	h := handler.NewHealthHandler(&mockDBPinger{})
	app.Get("/health", h.HealthCheck)

	resp := doRequest(t, app, "GET", "/health", nil)
	body := readJSON(t, resp)
	data := body["data"].(map[string]interface{})

	if data["version"] == nil || data["version"] == "" {
		t.Error("Expected version field in health response")
	}
}
