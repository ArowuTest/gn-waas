package database

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error codes
const (
	PGErrUniqueViolation     = "23505"
	PGErrForeignKeyViolation = "23503"
	PGErrNotNullViolation    = "23502"
	PGErrCheckViolation      = "23514"
	PGErrDeadlockDetected    = "40P01"
)

// DBError wraps PostgreSQL errors with domain context
type DBError struct {
	Code    string
	Message string
	Detail  string
	Table   string
	Column  string
	Err     error
}

func (e *DBError) Error() string {
	return fmt.Sprintf("database error [%s]: %s", e.Code, e.Message)
}

func (e *DBError) Unwrap() error {
	return e.Err
}

// IsNotFound returns true if the error is a pgx.ErrNoRows
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation returns true if the error is a unique constraint violation
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PGErrUniqueViolation
	}
	return false
}

// IsForeignKeyViolation returns true if the error is a foreign key violation
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == PGErrForeignKeyViolation
	}
	return false
}

// WrapError wraps a pgx error into a DBError with context
func WrapError(err error, operation string) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return &DBError{
			Code:    "NOT_FOUND",
			Message: fmt.Sprintf("%s: record not found", operation),
			Err:     err,
		}
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &DBError{
			Code:    pgErr.Code,
			Message: pgErr.Message,
			Detail:  pgErr.Detail,
			Table:   pgErr.TableName,
			Column:  pgErr.ColumnName,
			Err:     err,
		}
	}

	return fmt.Errorf("%s: %w", operation, err)
}
