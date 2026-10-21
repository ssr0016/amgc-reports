package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgForeignKeyViolation is the SQLSTATE Postgres reports for a foreign key
// violation.
const pgForeignKeyViolation = "23503"

// pgUniqueViolation is the SQLSTATE Postgres reports for a unique violation.
const pgUniqueViolation = "23505"

// ErrNotFound is returned (wrapped) when a row the caller asked to change does
// not exist. Test for it with errors.Is.
var ErrNotFound = errors.New("not found")

// ErrForeignKeyViolation is returned (wrapped) when a statement is refused
// because it would break a foreign key, e.g. deleting a role users still hold.
// Test for it with errors.Is.
var ErrForeignKeyViolation = errors.New("foreign key violation")

// ErrUniqueViolation is returned (wrapped) when a write is refused because it
// would duplicate a unique value, e.g. a second role with the same name. Test
// for it with errors.Is.
var ErrUniqueViolation = errors.New("unique violation")

// IsUniqueViolation reports whether err is, or wraps, a Postgres unique
// violation.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// IsForeignKeyViolation reports whether err is, or wraps, a Postgres foreign
// key violation.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation
}
