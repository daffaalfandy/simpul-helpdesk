package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// SQLSTATE codes services map to API errors.
const (
	SQLStateRestrictViolation   = "23001" // ON DELETE RESTRICT
	SQLStateForeignKeyViolation = "23503"
	SQLStateUniqueViolation     = "23505"
	SQLStateCheckViolation      = "23514"
)

// Violation returns the SQLSTATE and constraint name of a Postgres error.
// Deferred constraint triggers report theirs when the transaction commits.
func Violation(err error) (sqlState, constraint string, ok bool) {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) {
		return "", "", false
	}
	return pe.Code, pe.ConstraintName, true
}

// IsViolation reports whether err is a Postgres error with this SQLSTATE and constraint.
func IsViolation(err error, sqlState, constraint string) bool {
	code, name, ok := Violation(err)
	return ok && code == sqlState && name == constraint
}
