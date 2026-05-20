package repo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTSARepo(t *testing.T) (*TsaResponsesRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &TsaResponsesRepo{pool: pool}, pool
}

func sampleTSAResponse() *TSAResponse {
	latency := 123
	return &TSAResponse{
		ID:          "tsa_1",
		Provider:    "digicert",
		RequestHash: "deadbeef",
		Status:      "success",
		LatencyMS:   &latency,
	}
}

func tsaRowColumns() []string {
	return []string{
		"id", "provider", "request_hash", "response_blob",
		"serial_number", "issued_at", "valid_until", "status",
		"latency_ms", "used_by_report_id", "created_at",
	}
}

func sampleTSARow(id, provider, status string) []any {
	now := time.Now().UTC()
	return []any{
		id, provider, "hash", []byte(nil),
		(*string)(nil), (*time.Time)(nil), (*time.Time)(nil), status,
		(*int)(nil), (*string)(nil), now,
	}
}

func TestTsaResponsesRepo_Insert_Success(t *testing.T) {
	r, mock := newTSARepo(t)
	rec := sampleTSAResponse()

	mock.ExpectExec(`INSERT INTO idcd_attest\.tsa_response`).
		WithArgs(anyArgs(11)...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := r.Insert(context.Background(), rec)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTsaResponsesRepo_Insert_Conflict(t *testing.T) {
	r, mock := newTSARepo(t)
	rec := sampleTSAResponse()

	mock.ExpectExec(`INSERT INTO idcd_attest\.tsa_response`).
		WithArgs(anyArgs(11)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	err := r.Insert(context.Background(), rec)
	assert.ErrorIs(t, err, ErrConflict)
}

func TestTsaResponsesRepo_Insert_DBError(t *testing.T) {
	r, mock := newTSARepo(t)
	rec := sampleTSAResponse()
	sentinel := errors.New("io")

	mock.ExpectExec(`INSERT INTO idcd_attest\.tsa_response`).
		WithArgs(anyArgs(11)...).
		WillReturnError(sentinel)

	err := r.Insert(context.Background(), rec)
	assert.ErrorIs(t, err, sentinel)
}

func TestTsaResponsesRepo_Insert_Nil(t *testing.T) {
	r, _ := newTSARepo(t)
	err := r.Insert(context.Background(), nil)
	require.Error(t, err)
}

func TestTsaResponsesRepo_GetByID_Success(t *testing.T) {
	r, mock := newTSARepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.tsa_response\s+WHERE id`).
		WithArgs("tsa_1").
		WillReturnRows(pgxmock.NewRows(tsaRowColumns()).AddRow(sampleTSARow("tsa_1", "digicert", "success")...))

	out, err := r.GetByID(context.Background(), "tsa_1")
	require.NoError(t, err)
	assert.Equal(t, "tsa_1", out.ID)
	assert.Equal(t, "digicert", out.Provider)
}

func TestTsaResponsesRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newTSARepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.tsa_response\s+WHERE id`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := r.GetByID(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestTsaResponsesRepo_GetByID_DBError(t *testing.T) {
	r, mock := newTSARepo(t)
	sentinel := errors.New("conn")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.tsa_response\s+WHERE id`).
		WithArgs("tsa_1").
		WillReturnError(sentinel)

	_, err := r.GetByID(context.Background(), "tsa_1")
	assert.ErrorIs(t, err, sentinel)
}

func TestTsaResponsesRepo_ListByProvider_Success(t *testing.T) {
	r, mock := newTSARepo(t)
	since := time.Now().Add(-24 * time.Hour).UTC()

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.tsa_response\s+WHERE provider = \$1 AND created_at`).
		WithArgs("digicert", since, 50).
		WillReturnRows(pgxmock.NewRows(tsaRowColumns()).
			AddRow(sampleTSARow("tsa_1", "digicert", "success")...).
			AddRow(sampleTSARow("tsa_2", "digicert", "timeout")...))

	out, err := r.ListByProvider(context.Background(), "digicert", since, 50)
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "timeout", out[1].Status)
}

func TestTsaResponsesRepo_ListByProvider_QueryError(t *testing.T) {
	r, mock := newTSARepo(t)
	since := time.Now().UTC()
	sentinel := errors.New("net")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.tsa_response`).
		WithArgs("digicert", since, 50).
		WillReturnError(sentinel)

	_, err := r.ListByProvider(context.Background(), "digicert", since, 50)
	assert.ErrorIs(t, err, sentinel)
}
