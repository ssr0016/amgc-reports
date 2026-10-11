package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgForeignKeyViolation is the SQLSTATE Postgres reports for a foreign key
// violation.
const pgForeignKeyViolation = "23503"

// ErrNotFound is returned (wrapped) when a row the caller asked to change does
// not exist. Test for it with errors.Is.
var ErrNotFound = errors.New("not found")

// ErrForeignKeyViolation is returned (wrapped) when a statement is refused
// because it would break a foreign key, e.g. deleting a role users still hold.
// Test for it with errors.Is.
var ErrForeignKeyViolation = errors.New("foreign key violation")

// isForeignKeyViolation reports whether err is, or wraps, a Postgres foreign
// key violation.
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}
