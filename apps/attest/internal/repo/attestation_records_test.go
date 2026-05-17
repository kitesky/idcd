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

	attestrec "github.com/kite365/idcd/lib/attest/record"
)

// Compile-time guarantee that AttestationRecordsRepo satisfies the
// record.Repository interface — duplicates the production assertion so
// test-only refactors do not silently drop the constraint.
var _ attestrec.Repository = (*AttestationRecordsRepo)(nil)

func newAttestRepo(t *testing.T) (*AttestationRecordsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &AttestationRecordsRepo{pool: pool}, pool
}

func sampleAttestRecord() *attestrec.Record {
	return &attestrec.Record{
		ID:             "att_abc",
		ReportID:       "vr_1",
		Action:         attestrec.ActionSigned,
		Status:         attestrec.StatusPending,
		IdempotencyKey: "idem-1",
		PayloadHash:    "feedface",
		Result:         attestrec.ResultSuccess,
		RetryCount:     0,
	}
}

func attestRecordColumns() []string {
	return []string{
		"id", "report_id", "action", "status",
		"external_id", "idempotency_key", "payload_hash",
		"result", "error_detail", "retry_count",
		"created_at", "completed_at",
	}
}

func sampleAttestRecordRow(id, action, status string, retryCount int) []any {
	now := time.Now().UTC()
	return []any{
		id, "vr_1", action, status,
		(*string)(nil), (*string)(nil), (*string)(nil),
		"success", (*string)(nil), retryCount,
		now, (*time.Time)(nil),
	}
}

func TestAttestationRecordsRepo_Insert_Success(t *testing.T) {
	r, mock := newAttestRepo(t)
	rec := sampleAttestRecord()

	mock.ExpectExec(`INSERT INTO idcd_attest\.attestation_record`).
		WithArgs(anyArgs(12)...).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err := r.Insert(context.Background(), rec)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Critical D4 test: duplicate (report_id, action) must surface as
// attestrec.ErrDuplicateAction — NOT the local ErrConflict — because
// Replayer.Record branches on errors.Is(attestrec.ErrDuplicateAction).
func TestAttestationRecordsRepo_Insert_DuplicateAction(t *testing.T) {
	r, mock := newAttestRepo(t)
	rec := sampleAttestRecord()

	mock.ExpectExec(`INSERT INTO idcd_attest\.attestation_record`).
		WithArgs(anyArgs(12)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	err := r.Insert(context.Background(), rec)
	assert.ErrorIs(t, err, attestrec.ErrDuplicateAction)
	// Must NOT collide with the generic ErrConflict — Replayer relies
	// on the dedicated sentinel.
	assert.NotErrorIs(t, err, ErrConflict)
}

func TestAttestationRecordsRepo_Insert_DBError(t *testing.T) {
	r, mock := newAttestRepo(t)
	rec := sampleAttestRecord()
	sentinel := errors.New("net err")

	mock.ExpectExec(`INSERT INTO idcd_attest\.attestation_record`).
		WithArgs(anyArgs(12)...).
		WillReturnError(sentinel)

	err := r.Insert(context.Background(), rec)
	assert.ErrorIs(t, err, sentinel)
}

func TestAttestationRecordsRepo_Insert_Nil(t *testing.T) {
	r, _ := newAttestRepo(t)
	err := r.Insert(context.Background(), nil)
	require.Error(t, err)
}

func TestAttestationRecordsRepo_Get_Success(t *testing.T) {
	r, mock := newAttestRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record\s+WHERE report_id = \$1 AND action = \$2`).
		WithArgs("vr_1", string(attestrec.ActionSigned)).
		WillReturnRows(pgxmock.NewRows(attestRecordColumns()).
			AddRow(sampleAttestRecordRow("att_1", string(attestrec.ActionSigned), string(attestrec.StatusSuccess), 0)...))

	rec, err := r.Get(context.Background(), "vr_1", attestrec.ActionSigned)
	require.NoError(t, err)
	assert.Equal(t, "att_1", rec.ID)
	assert.Equal(t, attestrec.ActionSigned, rec.Action)
	assert.Equal(t, attestrec.StatusSuccess, rec.Status)
}

// Critical D4 test: missing rows must surface as attestrec.ErrNotFound
// so Replayer.ShouldRun knows the step has never run (vs. a transport
// error which should bubble up).
func TestAttestationRecordsRepo_Get_NotFound(t *testing.T) {
	r, mock := newAttestRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record`).
		WithArgs("vr_missing", string(attestrec.ActionTSAStamped)).
		WillReturnError(pgx.ErrNoRows)

	rec, err := r.Get(context.Background(), "vr_missing", attestrec.ActionTSAStamped)
	assert.Nil(t, rec)
	assert.ErrorIs(t, err, attestrec.ErrNotFound)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestAttestationRecordsRepo_Get_DBError(t *testing.T) {
	r, mock := newAttestRepo(t)
	sentinel := errors.New("conn closed")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record`).
		WithArgs("vr_1", string(attestrec.ActionSigned)).
		WillReturnError(sentinel)

	_, err := r.Get(context.Background(), "vr_1", attestrec.ActionSigned)
	assert.ErrorIs(t, err, sentinel)
}

func TestAttestationRecordsRepo_Update_Success(t *testing.T) {
	r, mock := newAttestRepo(t)
	now := time.Now().UTC()
	rec := &attestrec.Record{
		ID:          "att_1",
		Status:      attestrec.StatusSuccess,
		Result:      attestrec.ResultSuccess,
		ExternalID:  "kms-req-1",
		RetryCount:  1,
		CompletedAt: &now,
	}

	mock.ExpectExec(`UPDATE idcd_attest\.attestation_record`).
		WithArgs(string(attestrec.StatusSuccess), string(attestrec.ResultSuccess),
			"kms-req-1", (any)(nil), 1, &now, "att_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.Update(context.Background(), rec)
	require.NoError(t, err)
}

func TestAttestationRecordsRepo_Update_InvalidTransition(t *testing.T) {
	r, mock := newAttestRepo(t)
	now := time.Now().UTC()
	rec := &attestrec.Record{
		ID:          "att_1",
		Status:      attestrec.StatusFailure,
		Result:      attestrec.ResultFailure,
		ErrorDetail: "kms 5xx",
		RetryCount:  3,
		CompletedAt: &now,
	}

	// SQL guard returns 0 rows when transition is illegal (already
	// terminal, retry_count not strictly greater, etc).
	mock.ExpectExec(`UPDATE idcd_attest\.attestation_record`).
		WithArgs(string(attestrec.StatusFailure), string(attestrec.ResultFailure),
			(any)(nil), "kms 5xx", 3, &now, "att_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.Update(context.Background(), rec)
	assert.ErrorIs(t, err, attestrec.ErrInvalidTransition)
}

func TestAttestationRecordsRepo_Update_DBError(t *testing.T) {
	r, mock := newAttestRepo(t)
	now := time.Now().UTC()
	rec := &attestrec.Record{
		ID:          "att_1",
		Status:      attestrec.StatusSuccess,
		Result:      attestrec.ResultSuccess,
		RetryCount:  1,
		CompletedAt: &now,
	}
	sentinel := errors.New("io")

	mock.ExpectExec(`UPDATE idcd_attest\.attestation_record`).
		WithArgs(anyArgs(7)...).
		WillReturnError(sentinel)

	err := r.Update(context.Background(), rec)
	assert.ErrorIs(t, err, sentinel)
}

func TestAttestationRecordsRepo_Update_Nil(t *testing.T) {
	r, _ := newAttestRepo(t)
	err := r.Update(context.Background(), nil)
	require.Error(t, err)
}

func TestAttestationRecordsRepo_ListByReport_Success(t *testing.T) {
	r, mock := newAttestRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record\s+WHERE report_id = \$1\s+ORDER BY created_at`).
		WithArgs("vr_1").
		WillReturnRows(pgxmock.NewRows(attestRecordColumns()).
			AddRow(sampleAttestRecordRow("att_1", string(attestrec.ActionSigned), string(attestrec.StatusSuccess), 0)...).
			AddRow(sampleAttestRecordRow("att_2", string(attestrec.ActionTSAStamped), string(attestrec.StatusSuccess), 0)...))

	out, err := r.ListByReport(context.Background(), "vr_1")
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, attestrec.ActionSigned, out[0].Action)
	assert.Equal(t, attestrec.ActionTSAStamped, out[1].Action)
}

func TestAttestationRecordsRepo_ListByReport_Empty(t *testing.T) {
	r, mock := newAttestRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record\s+WHERE report_id`).
		WithArgs("vr_none").
		WillReturnRows(pgxmock.NewRows(attestRecordColumns()))

	out, err := r.ListByReport(context.Background(), "vr_none")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestAttestationRecordsRepo_ListByReport_QueryError(t *testing.T) {
	r, mock := newAttestRepo(t)
	sentinel := errors.New("boom")

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.attestation_record`).
		WithArgs("vr_1").
		WillReturnError(sentinel)

	_, err := r.ListByReport(context.Background(), "vr_1")
	assert.ErrorIs(t, err, sentinel)
}
