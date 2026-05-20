package processor

import "github.com/jackc/pgx/v5"

// pgxErrNoRows returns the pgx no-rows sentinel for use in pgxmock expectations.
func pgxErrNoRows() error {
	return pgx.ErrNoRows
}
