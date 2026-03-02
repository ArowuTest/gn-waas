package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Querier is a common interface satisfied by both *pgxpool.Pool and pgx.Tx.
// Using this interface allows repository methods to work with either a
// connection pool (normal operation) or a transaction (RLS-activated queries).
//
// Both pgxpool.Pool and pgx.Tx implement Query, QueryRow, and Exec with
// identical signatures, so this interface works without any adapter.
type Querier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}
