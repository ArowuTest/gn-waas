package rls_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// ─── Context helpers ──────────────────────────────────────────────────────────

func TestWithTxInContext_StoresAndRetrieves(t *testing.T) {
	// Use a concrete non-nil tx value so the type assertion succeeds.
	// A nil interface value stored in context cannot be type-asserted back.
	tx := &concreteTx{}

	ctx := rls.WithTxInContext(context.Background(), tx)

	retrieved, ok := rls.TxFromContext(ctx)
	if !ok {
		t.Fatal("TxFromContext: expected ok=true, got false")
	}
	if retrieved != pgx.Tx(tx) {
		t.Errorf("TxFromContext: expected the same tx pointer, got different value")
	}
}

func TestTxFromContext_ReturnsFalseWhenAbsent(t *testing.T) {
	tx, ok := rls.TxFromContext(context.Background())
	if ok {
		t.Errorf("TxFromContext: expected ok=false for empty context, got true")
	}
	if tx != nil {
		t.Errorf("TxFromContext: expected nil tx for empty context, got %v", tx)
	}
}

func TestTxFromContext_ReturnsFalseForUnrelatedContextValue(t *testing.T) {
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "some-value")
	_, ok := rls.TxFromContext(ctx)
	if ok {
		t.Error("TxFromContext: should not retrieve unrelated context values")
	}
}

func TestWithTxInContext_ChildContextInherits(t *testing.T) {
	tx := &concreteTx{}
	parent := rls.WithTxInContext(context.Background(), tx)

	child, cancel := context.WithCancel(parent)
	defer cancel()

	retrieved, ok := rls.TxFromContext(child)
	if !ok {
		t.Error("TxFromContext: child context should inherit transaction from parent")
	}
	if retrieved != pgx.Tx(tx) {
		t.Error("TxFromContext: child context should return the same tx as parent")
	}
}

// ─── RLS Context ──────────────────────────────────────────────────────────────

func TestContext_IsAdmin_SystemAdmin(t *testing.T) {
	ctx := rls.Context{UserRole: "SYSTEM_ADMIN"}
	if !ctx.IsAdmin() {
		t.Error("IsAdmin: SYSTEM_ADMIN should return true")
	}
}

func TestContext_IsAdmin_NationalRegulator(t *testing.T) {
	ctx := rls.Context{UserRole: "NATIONAL_REGULATOR"}
	if !ctx.IsAdmin() {
		t.Error("IsAdmin: NATIONAL_REGULATOR should return true")
	}
}

func TestContext_IsAdmin_FieldOfficer(t *testing.T) {
	ctx := rls.Context{UserRole: "FIELD_OFFICER"}
	if ctx.IsAdmin() {
		t.Error("IsAdmin: FIELD_OFFICER should return false")
	}
}

func TestContext_IsAdmin_GWLManager(t *testing.T) {
	ctx := rls.Context{UserRole: "GWL_MANAGER"}
	if ctx.IsAdmin() {
		t.Error("IsAdmin: GWL_MANAGER should return false")
	}
}

func TestContext_IsAdmin_EmptyRole(t *testing.T) {
	ctx := rls.Context{}
	if ctx.IsAdmin() {
		t.Error("IsAdmin: empty role should return false")
	}
}

func TestContext_ZeroValue(t *testing.T) {
	var ctx rls.Context
	if ctx.DistrictID != "" || ctx.UserRole != "" || ctx.UserID != "" {
		t.Error("zero-value Context should have empty strings")
	}
}

// ─── FromFiber ────────────────────────────────────────────────────────────────

func TestFromFiber_ExtractsLocals(t *testing.T) {
	app := fiber.New()
	var captured rls.Context

	app.Get("/test", func(c *fiber.Ctx) error {
		c.Locals("rls_district_id", "district-uuid-123")
		c.Locals("rls_user_role", "FIELD_OFFICER")
		c.Locals("rls_user_id", "user-uuid-456")
		captured = rls.FromFiber(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if captured.DistrictID != "district-uuid-123" {
		t.Errorf("FromFiber: DistrictID = %q, want %q", captured.DistrictID, "district-uuid-123")
	}
	if captured.UserRole != "FIELD_OFFICER" {
		t.Errorf("FromFiber: UserRole = %q, want %q", captured.UserRole, "FIELD_OFFICER")
	}
	if captured.UserID != "user-uuid-456" {
		t.Errorf("FromFiber: UserID = %q, want %q", captured.UserID, "user-uuid-456")
	}
}

func TestFromFiber_ReturnsEmptyWhenLocalsAbsent(t *testing.T) {
	app := fiber.New()
	var captured rls.Context

	app.Get("/test", func(c *fiber.Ctx) error {
		captured = rls.FromFiber(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if captured.DistrictID != "" || captured.UserRole != "" || captured.UserID != "" {
		t.Errorf("FromFiber: expected empty Context when locals absent, got %+v", captured)
	}
}

func TestFromFiber_HandlesWrongLocalType(t *testing.T) {
	app := fiber.New()
	var captured rls.Context

	app.Get("/test", func(c *fiber.Ctx) error {
		// Set locals with wrong type — type assertion should fail gracefully
		c.Locals("rls_district_id", 12345)
		c.Locals("rls_user_role", true)
		captured = rls.FromFiber(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if captured.DistrictID != "" || captured.UserRole != "" {
		t.Errorf("FromFiber: expected empty strings for wrong-type locals, got %+v", captured)
	}
}

func TestFromFiber_AllRolesExtractCorrectly(t *testing.T) {
	roles := []string{
		"SYSTEM_ADMIN", "NATIONAL_REGULATOR", "FIELD_OFFICER",
		"FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_ANALYST", "GWL_EXECUTIVE",
	}

	for _, role := range roles {
		role := role
		t.Run(role, func(t *testing.T) {
			app := fiber.New()
			var captured rls.Context

			app.Get("/test", func(c *fiber.Ctx) error {
				c.Locals("rls_user_role", role)
				captured = rls.FromFiber(c)
				return c.SendStatus(200)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if _, err := app.Test(req); err != nil {
				t.Fatalf("app.Test: %v", err)
			}

			if captured.UserRole != role {
				t.Errorf("FromFiber: UserRole = %q, want %q", captured.UserRole, role)
			}
		})
	}
}

// ─── Middleware ───────────────────────────────────────────────────────────────

func TestMiddleware_FallsThroughWhenPoolIsNil(t *testing.T) {
	// With a nil pool, BeginTx will fail. The middleware should log a warning
	// and call c.Next() without injecting a transaction.
	app := fiber.New()
	var txFoundInHandler bool
	handlerCalled := false

	logger, _ := zap.NewDevelopment()
	app.Use(rls.Middleware(nil, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		handlerCalled = true
		_, txFoundInHandler = rls.TxFromContext(c.Context())
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if !handlerCalled {
		t.Error("handler should be called even when RLS transaction fails")
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if txFoundInHandler {
		t.Error("no tx should be in context when pool is nil")
	}
}

func TestMiddleware_HandlerRunsFor4xxResponse(t *testing.T) {
	// Verify that 4xx responses still run the handler (middleware doesn't block)
	logger, _ := zap.NewDevelopment()
	app := fiber.New()
	handlerCalled := false

	app.Use(rls.Middleware(nil, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		handlerCalled = true
		return c.SendStatus(400)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if !handlerCalled {
		t.Error("handler should be called for 4xx responses")
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMiddleware_GETRequestFallsThrough(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	app := fiber.New()

	app.Use(rls.Middleware(nil, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("GET: expected 200, got %d", resp.StatusCode)
	}
}

func TestMiddleware_POSTRequestFallsThrough(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	app := fiber.New()

	app.Use(rls.Middleware(nil, logger))
	app.Post("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(201)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 201 {
		t.Errorf("POST: expected 201, got %d", resp.StatusCode)
	}
}

func TestMiddleware_PATCHRequestFallsThrough(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	app := fiber.New()

	app.Use(rls.Middleware(nil, logger))
	app.Patch("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("PATCH", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("PATCH: expected 200, got %d", resp.StatusCode)
	}
}

func TestMiddleware_DELETERequestFallsThrough(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	app := fiber.New()

	app.Use(rls.Middleware(nil, logger))
	app.Delete("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	req := httptest.NewRequest("DELETE", "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Errorf("DELETE: expected 204, got %d", resp.StatusCode)
	}
}

// ─── Context isolation ────────────────────────────────────────────────────────

func TestWithTxInContext_DoesNotLeakBetweenRequests(t *testing.T) {
	// Verify that transactions stored in one request context do not leak
	// into another request's context.
	var tx1Found, tx2Found bool

	app := fiber.New()
	logger, _ := zap.NewDevelopment()

	app.Use(rls.Middleware(nil, logger))
	app.Get("/test", func(c *fiber.Ctx) error {
		_, found := rls.TxFromContext(c.Context())
		if c.Query("req") == "1" {
			tx1Found = found
		} else {
			tx2Found = found
		}
		return c.SendStatus(200)
	})

	req1 := httptest.NewRequest("GET", "/test?req=1", nil)
	req2 := httptest.NewRequest("GET", "/test?req=2", nil)

	if _, err := app.Test(req1); err != nil {
		t.Fatalf("req1: %v", err)
	}
	if _, err := app.Test(req2); err != nil {
		t.Fatalf("req2: %v", err)
	}

	// Both requests should have the same result (no tx with nil pool)
	if tx1Found != tx2Found {
		t.Errorf("transaction state leaked between requests: req1=%v, req2=%v", tx1Found, tx2Found)
	}
}

// ─── concreteTx ───────────────────────────────────────────────────────────────

// concreteTx is a minimal concrete pgx.Tx implementation for testing context
// storage. It satisfies the pgx.Tx interface without a real DB connection.
type concreteTx struct{}

func (t *concreteTx) Begin(ctx context.Context) (pgx.Tx, error) { return t, nil }
func (t *concreteTx) Commit(ctx context.Context) error           { return nil }
func (t *concreteTx) Rollback(ctx context.Context) error         { return nil }
func (t *concreteTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *concreteTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (t *concreteTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (t *concreteTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *concreteTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (t *concreteTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (t *concreteTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row { return nil }
func (t *concreteTx) Conn() *pgx.Conn                                               { return nil }

// ─── setLocals / sanitize tests ───────────────────────────────────────────────
// These tests verify the SQL-injection-safe input validation in setLocals.
// We test the exported sanitize helpers indirectly via FromFiber + Middleware,
// and directly via the exported SanitizeUUID / SanitizeRole helpers.

func TestSanitizeUUID_ValidLowercase(t *testing.T) {
	input := "d0000001-0000-0000-0000-000000000001"
	got := rls.SanitizeUUID(input, "fallback")
	if got != input {
		t.Errorf("SanitizeUUID(%q) = %q, want %q", input, got, input)
	}
}

func TestSanitizeUUID_ValidUppercase_Normalised(t *testing.T) {
	input := "D0000001-0000-0000-0000-000000000001"
	expected := "d0000001-0000-0000-0000-000000000001"
	got := rls.SanitizeUUID(input, "fallback")
	if got != expected {
		t.Errorf("SanitizeUUID(%q) = %q, want %q (lowercase)", input, got, expected)
	}
}

func TestSanitizeUUID_Empty_ReturnsFallback(t *testing.T) {
	got := rls.SanitizeUUID("", "00000000-0000-0000-0000-000000000000")
	if got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("SanitizeUUID(\"\") = %q, want fallback UUID", got)
	}
}

func TestSanitizeUUID_InvalidFormat_ReturnsFallback(t *testing.T) {
	cases := []string{
		"not-a-uuid",
		"'; DROP TABLE users; --",
		"00000000-0000-0000-0000-00000000000",  // too short
		"00000000-0000-0000-0000-0000000000000", // too long
		"gggggggg-0000-0000-0000-000000000000",  // invalid hex
		"",
	}
	fallback := "00000000-0000-0000-0000-000000000000"
	for _, c := range cases {
		got := rls.SanitizeUUID(c, fallback)
		if got != fallback {
			t.Errorf("SanitizeUUID(%q) = %q, want fallback %q", c, got, fallback)
		}
	}
}

func TestSanitizeUUID_SQLInjectionAttempt_ReturnsFallback(t *testing.T) {
	// Simulate an attacker who has somehow injected a malicious district_id
	// into the JWT claims. The sanitizer must reject it.
	malicious := "'; SET LOCAL rls.user_role = 'SYSTEM_ADMIN'; --"
	got := rls.SanitizeUUID(malicious, "00000000-0000-0000-0000-000000000000")
	if got == malicious {
		t.Error("SanitizeUUID: SQL injection attempt should be rejected")
	}
}

func TestSanitizeRole_KnownRoles(t *testing.T) {
	knownRoles := []string{
		"SYSTEM_ADMIN", "NATIONAL_REGULATOR", "FIELD_OFFICER",
		"FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_ANALYST", "GWL_EXECUTIVE",
	}
	for _, role := range knownRoles {
		got := rls.SanitizeRole(role)
		if got != role {
			t.Errorf("SanitizeRole(%q) = %q, want %q", role, got, role)
		}
	}
}

func TestSanitizeRole_UnknownRole_ReturnsAnonymous(t *testing.T) {
	cases := []string{
		"SUPER_USER",
		"'; DROP TABLE users; --",
		"SYSTEM_ADMIN' OR '1'='1",
		"",
		"admin",
		"root",
	}
	for _, c := range cases {
		got := rls.SanitizeRole(c)
		if got != "ANONYMOUS" {
			t.Errorf("SanitizeRole(%q) = %q, want ANONYMOUS", c, got)
		}
	}
}

func TestSanitizeRole_SQLInjectionAttempt_ReturnsAnonymous(t *testing.T) {
	malicious := "FIELD_OFFICER'; SET LOCAL rls.user_role = 'SYSTEM_ADMIN'; --"
	got := rls.SanitizeRole(malicious)
	if got != "ANONYMOUS" {
		t.Errorf("SanitizeRole: SQL injection attempt should return ANONYMOUS, got %q", got)
	}
}

func TestSanitizeRole_Anonymous_IsAllowed(t *testing.T) {
	// ANONYMOUS is in the allowlist (used as the safe default)
	got := rls.SanitizeRole("ANONYMOUS")
	if got != "ANONYMOUS" {
		t.Errorf("SanitizeRole(ANONYMOUS) = %q, want ANONYMOUS", got)
	}
}
