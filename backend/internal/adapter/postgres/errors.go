// Package postgres contains the Postgres implementations of the domain
// repository interfaces. It is the only package that imports pgx; the domain and
// service layers depend solely on the interfaces they define. SQL is written by
// hand and always parameterized to prevent injection.
package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres SQLSTATE codes we translate to domain errors.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

// isNoRows reports whether err is pgx's no-rows sentinel.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isUniqueViolation reports whether err is a unique-constraint violation and, if
// so, returns the constraint name so callers can produce a specific message.
func isUniqueViolation(err error) (constraint string, ok bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// isForeignKeyViolation reports whether err is a foreign-key violation.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}
