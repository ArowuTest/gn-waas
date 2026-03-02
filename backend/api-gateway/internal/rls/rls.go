// Package rls provides helpers for activating PostgreSQL Row-Level Security
// within the GN-WAAS API Gateway.
//
// # Why this is needed
//
// The database migrations (012_row_level_security.sql) define RLS policies that
// enforce district-level data isolation. However, RLS policies only take effect
// when the session variables they depend on are set:
//
//	SET LOCAL rls.district_id = '<uuid>';
//	SET LOCAL rls.user_role   = '<role>';
//	SET LOCAL rls.user_id     = '<uuid>';
//
// These must be set inside a transaction (SET LOCAL is transaction-scoped).
// Without this, every query runs as the superuser and RLS is bypassed entirely.
//
// # Usage — Middleware (recommended, covers all endpoints automatically)
//
// Register the middleware on all authenticated route groups in app.go:
//
//	api := app.Group("/api/v1", authMiddleware, rls.Middleware(db))
//
// The middleware begins an RLS-activated transaction, stores it in the request
// context, and commits/rolls back automatically. Repositories retrieve the
// transaction via rls.TxFromContext and use it for all queries.
//
// # Usage — Manual (for fine-grained control)
//
//	handle, err := rls.BeginTx(c.Context(), pool, rls.FromFiber(c))
//	if err != nil { ... }
//	defer handle.Rollback(c.Context())
//	results, err := repo.ListWithTx(handle.Tx, ...)
//	return handle.Commit(c.Context())
package rls

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ─── Context key ─────────────────────────────────────────────────────────────

// txContextKey is the unexported key used to store a pgx.Tx in a context.
type txContextKey struct{}

// WithTxInContext returns a new context carrying the given transaction.
// Repositories call TxFromContext to retrieve it.
func WithTxInContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// TxFromContext retrieves the RLS-activated transaction stored by the middleware.
// Returns (tx, true) if a transaction is present, (nil, false) otherwise.
func TxFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx, ok
}

// ─── RLS Context ─────────────────────────────────────────────────────────────

// Context holds the RLS values extracted from the authenticated request.
type Context struct {
	DistrictID string // UUID string; "00000000-0000-0000-0000-000000000000" for admins
	UserRole   string // e.g. "SYSTEM_ADMIN", "DISTRICT_OFFICER"
	UserID     string // UUID string of the authenticated user
}

// IsAdmin returns true if the user has a system-wide admin role that bypasses RLS.
func (c Context) IsAdmin() bool {
	return c.UserRole == "SYSTEM_ADMIN" || c.UserRole == "MOF_AUDITOR"
}

// FromFiber extracts the RLS context from Fiber locals set by SetRLSContext middleware.
// Returns a zero-value Context (empty strings) if the locals are not set.
func FromFiber(c *fiber.Ctx) Context {
	districtID, _ := c.Locals("rls_district_id").(string)
	userRole, _ := c.Locals("rls_user_role").(string)
	userID, _ := c.Locals("rls_user_id").(string)
	return Context{
		DistrictID: districtID,
		UserRole:   userRole,
		UserID:     userID,
	}
}

// ─── Fiber Middleware ─────────────────────────────────────────────────────────

// Middleware returns a Fiber handler that transparently activates PostgreSQL RLS
// for every request in the group it is applied to.
//
// For each request it:
//  1. Reads the RLS context from Fiber locals (set by the JWT auth middleware)
//  2. Begins a pgx transaction with the appropriate access mode:
//     - GET/HEAD → ReadOnly transaction (prevents accidental writes)
//     - POST/PATCH/PUT/DELETE → ReadWrite transaction
//  3. Executes SET LOCAL for the three RLS session variables
//  4. Stores the transaction in the Go request context via WithTxInContext
//  5. Calls c.Next() to run the handler
//  6. Commits on success (HTTP < 400), rolls back on error or HTTP >= 400
//
// Repositories retrieve the transaction via TxFromContext and use it instead
// of the pool, so RLS is enforced for every query without any handler changes.
//
// If the transaction cannot be started (e.g. pool exhausted), the middleware
// logs a warning and allows the request to proceed WITHOUT RLS — this is a
// deliberate degradation-over-denial choice. Ops should alert on these logs.
func Middleware(pool *pgxpool.Pool, logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Guard: if pool is nil (e.g. in unit tests), skip RLS and proceed.
		if pool == nil {
			logger.Warn("RLS middleware: pool is nil — proceeding WITHOUT RLS enforcement",
				zap.String("path", c.Path()),
			)
			return c.Next()
		}

		rlsCtx := FromFiber(c)

		// Choose access mode based on HTTP method
		isReadOnly := c.Method() == http.MethodGet || c.Method() == http.MethodHead

		var handle *TxHandle
		var txErr error
		if isReadOnly {
			handle, txErr = BeginReadOnlyTx(c.Context(), pool, rlsCtx)
		} else {
			handle, txErr = BeginTx(c.Context(), pool, rlsCtx)
		}

		if txErr != nil {
			// Non-fatal: log and proceed without RLS rather than blocking the request.
			// This should trigger an alert in production monitoring.
			logger.Warn("RLS middleware: failed to begin RLS transaction — proceeding WITHOUT RLS enforcement",
				zap.Error(txErr),
				zap.String("user_id", rlsCtx.UserID),
				zap.String("user_role", rlsCtx.UserRole),
				zap.String("district_id", rlsCtx.DistrictID),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
			)
			return c.Next()
		}

		// Store the RLS-activated transaction in the Go request context.
		// Repositories will retrieve it via TxFromContext.
		goCtx := WithTxInContext(c.Context(), handle.Tx)
		c.SetUserContext(goCtx)

		// Run the handler
		handlerErr := c.Next()

		// Commit on success, rollback on any error or non-2xx/3xx response
		statusCode := c.Response().StatusCode()
		if handlerErr == nil && statusCode < 400 {
			if commitErr := handle.Commit(goCtx); commitErr != nil {
				logger.Error("RLS middleware: failed to commit transaction",
					zap.Error(commitErr),
					zap.String("path", c.Path()),
				)
				// Rollback is deferred below
			}
		} else {
			handle.Rollback(goCtx)
		}

		return handlerErr
	}
}

// ─── TxHandle ────────────────────────────────────────────────────────────────

// TxHandle wraps a pgx.Tx with a convenience Commit/Rollback API.
type TxHandle struct {
	Tx pgx.Tx
}

// Commit commits the transaction.
func (h *TxHandle) Commit(ctx context.Context) error {
	return h.Tx.Commit(ctx)
}

// Rollback rolls back the transaction. Safe to call after Commit (no-op).
func (h *TxHandle) Rollback(ctx context.Context) {
	_ = h.Tx.Rollback(ctx)
}

// ─── Transaction helpers ──────────────────────────────────────────────────────

// BeginTx starts a read-committed transaction and immediately activates RLS
// by setting the three session-local variables that the DB policies depend on.
//
// The caller MUST call Rollback (or Commit) when done:
//
//	handle, err := rls.BeginTx(ctx, pool, rlsCtx)
//	if err != nil { return err }
//	defer handle.Rollback(ctx)
//	// ... use handle.Tx ...
//	return handle.Commit(ctx)
func BeginTx(ctx context.Context, pool *pgxpool.Pool, rlsCtx Context) (*TxHandle, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	})
	if err != nil {
		return nil, fmt.Errorf("rls.BeginTx: begin transaction: %w", err)
	}

	if err := setLocals(ctx, tx, rlsCtx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	return &TxHandle{Tx: tx}, nil
}

// BeginReadOnlyTx starts a read-only transaction with RLS activated.
// Use this for SELECT-only operations to prevent accidental writes.
func BeginReadOnlyTx(ctx context.Context, pool *pgxpool.Pool, rlsCtx Context) (*TxHandle, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("rls.BeginReadOnlyTx: begin transaction: %w", err)
	}

	if err := setLocals(ctx, tx, rlsCtx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}

	return &TxHandle{Tx: tx}, nil
}

// WithTx is a convenience wrapper that begins an RLS transaction, calls fn,
// and commits on success or rolls back on error.
//
//	err = rls.WithTx(ctx, pool, rlsCtx, func(tx pgx.Tx) error {
//	    _, err := tx.Exec(ctx, "SELECT ...")
//	    return err
//	})
func WithTx(ctx context.Context, pool *pgxpool.Pool, rlsCtx Context, fn func(pgx.Tx) error) error {
	handle, err := BeginTx(ctx, pool, rlsCtx)
	if err != nil {
		return err
	}
	defer handle.Rollback(ctx)

	if err := fn(handle.Tx); err != nil {
		return err
	}

	return handle.Commit(ctx)
}

// WithReadOnlyTx is like WithTx but uses a read-only transaction.
func WithReadOnlyTx(ctx context.Context, pool *pgxpool.Pool, rlsCtx Context, fn func(pgx.Tx) error) error {
	handle, err := BeginReadOnlyTx(ctx, pool, rlsCtx)
	if err != nil {
		return err
	}
	defer handle.Rollback(ctx)

	if err := fn(handle.Tx); err != nil {
		return err
	}

	return handle.Commit(ctx)
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

// validUUIDPattern matches a canonical UUID (8-4-4-4-12 hex digits).
// Used to validate district_id and user_id before embedding in SQL.
const validUUIDPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`

// knownRoles is the exhaustive set of roles defined in the SQL enum user_role.
// Any value not in this set is rejected and replaced with "ANONYMOUS".
var knownRoles = map[string]struct{}{
	"SYSTEM_ADMIN":       {},
	"MOF_AUDITOR": {},
	"FIELD_OFFICER":      {},
	"FIELD_SUPERVISOR":   {},
	"GWL_MANAGER":        {},
	"GWL_ANALYST":        {},
	"GWL_EXECUTIVE":      {},
	"ANONYMOUS":          {},
}

// setLocals activates PostgreSQL Row-Level Security for the current transaction
// by setting three session-local configuration parameters that the RLS policies
// read via current_setting().
//
// Security design:
//   - PostgreSQL's SET LOCAL does not support parameterized queries ($1 syntax).
//     Using fmt.Sprintf to build the SQL string would be a SQL injection risk.
//   - Instead, we use SELECT set_config($1, $2, true) which IS fully parameterized.
//     The third argument (true) makes the setting transaction-local, equivalent
//     to SET LOCAL.
//   - All three inputs are validated before use:
//     • district_id and user_id must match the canonical UUID format.
//     • user_role must be a member of the known roles allowlist.
//     Any value that fails validation is replaced with a safe default.
//
// This eliminates the SQL injection risk while preserving the RLS semantics.
func setLocals(ctx context.Context, tx pgx.Tx, rlsCtx Context) error {
	districtID := sanitizeUUID(rlsCtx.DistrictID, "00000000-0000-0000-0000-000000000000")
	userRole   := sanitizeRole(rlsCtx.UserRole)
	userID     := sanitizeUUID(rlsCtx.UserID, "00000000-0000-0000-0000-000000000000")

	// set_config(setting_name, new_value, is_local)
	// is_local=true → equivalent to SET LOCAL (transaction-scoped)
	// Each call is a separate parameterized query — no string interpolation.
	for _, kv := range []struct{ key, val string }{
		{"rls.district_id", districtID},
		{"rls.user_role", userRole},
		{"rls.user_id", userID},
	} {
		if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", kv.key, kv.val); err != nil {
			return fmt.Errorf("rls.setLocals: set_config(%s): %w", kv.key, err)
		}
	}
	return nil
}

// sanitizeUUID validates that s is a canonical lowercase UUID.
// Returns fallback if s is empty or does not match the pattern.
// This prevents any SQL injection via the district_id or user_id fields.
func sanitizeUUID(s, fallback string) string {
	if s == "" {
		return fallback
	}
	// Normalise to lowercase for consistent matching
	s = strings.ToLower(s)
	if matched, _ := regexp.MatchString(validUUIDPattern, s); !matched {
		return fallback
	}
	return s
}

// sanitizeRole validates that s is a known user role.
// Returns "ANONYMOUS" if s is empty or not in the allowlist.
// This prevents any SQL injection via the user_role field.
func sanitizeRole(s string) string {
	if _, ok := knownRoles[s]; ok {
		return s
	}
	return "ANONYMOUS"
}

// ─── Exported test helpers ────────────────────────────────────────────────────
// These exported wrappers allow unit tests to verify the input sanitization
// logic without requiring a real database connection.

// SanitizeUUID is the exported version of sanitizeUUID for testing.
// In production code, use setLocals directly — it calls sanitizeUUID internally.
func SanitizeUUID(s, fallback string) string {
	return sanitizeUUID(s, fallback)
}

// SanitizeRole is the exported version of sanitizeRole for testing.
// In production code, use setLocals directly — it calls sanitizeRole internally.
func SanitizeRole(s string) string {
	return sanitizeRole(s)
}
