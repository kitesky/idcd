package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDomainsRepo(t *testing.T) (*DomainsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &DomainsRepo{pool: pool}, pool
}

func domainCols() []string {
	return []string{"id", "account_id", "fqdn", "caa_status", "caa_checked_at", "created_at"}
}

func TestDomainsRepo_Upsert_WithCAA(t *testing.T) {
	r, mock := newDomainsRepo(t)
	now := time.Now().UTC()
	st := "ok"

	mock.ExpectQuery(`INSERT INTO cert\.domains.+ON CONFLICT`).
		WithArgs(int64(42), "example.com", &st, pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows(domainCols()).
			AddRow(int64(7), int64(42), "example.com", &st, &now, now))

	d, err := r.Upsert(context.Background(), 42, "example.com", &st)
	require.NoError(t, err)
	assert.Equal(t, int64(7), d.ID)
	assert.Equal(t, "ok", *d.CAAStatus)
}

func TestDomainsRepo_Upsert_NoCAA(t *testing.T) {
	r, mock := newDomainsRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`INSERT INTO cert\.domains.+ON CONFLICT`).
		WithArgs(int64(42), "example.com", (*string)(nil), (*time.Time)(nil)).
		WillReturnRows(pgxmock.NewRows(domainCols()).
			AddRow(int64(8), int64(42), "example.com", (*string)(nil), (*time.Time)(nil), now))

	d, err := r.Upsert(context.Background(), 42, "example.com", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(8), d.ID)
}

func TestDomainsRepo_Upsert_DBError(t *testing.T) {
	r, mock := newDomainsRepo(t)
	mock.ExpectQuery(`INSERT INTO cert\.domains`).
		WithArgs(int64(1), "x", (*string)(nil), (*time.Time)(nil)).
		WillReturnError(errors.New("io"))
	_, err := r.Upsert(context.Background(), 1, "x", nil)
	require.Error(t, err)
}

func TestDomainsRepo_Get_Success(t *testing.T) {
	r, mock := newDomainsRepo(t)
	now := time.Now().UTC()

	mock.ExpectQuery(`SELECT .+ FROM cert\.domains\s+WHERE account_id`).
		WithArgs(int64(42), "example.com").
		WillReturnRows(pgxmock.NewRows(domainCols()).
			AddRow(int64(9), int64(42), "example.com", (*string)(nil), (*time.Time)(nil), now))

	d, err := r.Get(context.Background(), 42, "example.com")
	require.NoError(t, err)
	assert.Equal(t, int64(9), d.ID)
}

func TestDomainsRepo_Get_NotFound(t *testing.T) {
	r, mock := newDomainsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.domains`).
		WithArgs(int64(1), "x").
		WillReturnError(pgx.ErrNoRows)
	_, err := r.Get(context.Background(), 1, "x")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDomainsRepo_Get_DBError(t *testing.T) {
	r, mock := newDomainsRepo(t)
	mock.ExpectQuery(`SELECT .+ FROM cert\.domains`).
		WithArgs(int64(1), "x").
		WillReturnError(errors.New("io"))
	_, err := r.Get(context.Background(), 1, "x")
	require.Error(t, err)
}
