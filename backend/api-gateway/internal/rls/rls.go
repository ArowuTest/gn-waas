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
// # Usage
//
// In a Fiber handler, extract the RLS context and wrap repository calls:
//
//	ctx, err := rls.BeginTx(c.Context(), pool, rls.FromFiber(c))
//	if err != nil { ... }
//	defer ctx.Rollback(c.Context())
//
//	results, err := repo.ListWithTx(ctx.Tx, ...)
//	ctx.Commit(c.Context())
//
// Or use the convenience wrapper:
//
//	err = rls.WithTx(c.Context(), pool, rls.FromFiber(c), func(tx pgx.Tx) error {
//	    results, err = repo.ListWithTx(tx, ...)
//	    return err
//	})
package rls

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Context holds the RLS values extracted from the authenticated request.
type Context struct {
	DistrictID string // UUID string; "00000000-0000-0000-0000-000000000000" for admins
	UserRole   string // e.g. "SYSTEM_ADMIN", "DISTRICT_OFFICER"
	UserID     string // UUID string of the authenticated user
}

// IsAdmin returns true if the user has a system-wide admin role that bypasses RLS.
func (c Context) IsAdmin() bool {
	return c.UserRole == "SYSTEM_ADMIN" || c.UserRole == "NATIONAL_REGULATOR"
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

// setLocals executes the three SET LOCAL statements that activate RLS policies.
// These are transaction-scoped: they expire when the transaction ends.
func setLocals(ctx context.Context, tx pgx.Tx, rlsCtx Context) error {
	districtID := rlsCtx.DistrictID
	if districtID == "" {
		districtID = "00000000-0000-0000-0000-000000000000"
	}
	userRole := rlsCtx.UserRole
	if userRole == "" {
		userRole = "ANONYMOUS"
	}
	userID := rlsCtx.UserID
	if userID == "" {
		userID = "00000000-0000-0000-0000-000000000000"
	}

	_, err := tx.Exec(ctx, fmt.Sprintf(
		`SET LOCAL rls.district_id = '%s'; SET LOCAL rls.user_role = '%s'; SET LOCAL rls.user_id = '%s'`,
		districtID, userRole, userID,
	))
	if err != nil {
		return fmt.Errorf("rls.setLocals: failed to set RLS session variables: %w", err)
	}
	return nil
}
