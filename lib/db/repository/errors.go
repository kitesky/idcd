package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by all repositories.
var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate entry")
)

// isNoRows reports whether err is a pgx no-rows error.
func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// isDuplicate reports whether err is a PostgreSQL unique_violation (23505).
func isDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
