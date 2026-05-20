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

func newReportsRepo(t *testing.T) (*VerdictReportsRepo, pgxmock.PgxPoolIface) {
	t.Helper()
	pool := newMockPool(t)
	return &VerdictReportsRepo{pool: pool}, pool
}

func sampleReport() *Report {
	return &Report{
		ID:                  "vr_xyz789",
		OrderID:             "v_abc123",
		PDFURL:              "s3://bucket/report.pdf",
		ContentHash:         "deadbeef",
		Signature:           []byte{0x01, 0x02},
		SignatureKeyID:      "alias/attest-2026",
		SignatureKeyVersion: 1,
		TSAProvider:         "digicert",
		TSATime:             time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		NodesUsed:           []byte(`["node1","node2"]`),
		ReportType:          "observation_only",
	}
}

func reportRowColumns() []string {
	return []string{
		"id", "order_id", "pdf_url", "pdf_size_bytes",
		"content_hash", "signature", "signature_key_id", "signature_key_version",
		"tsa_provider", "tsa_response_blob", "tsa_time",
		"blockchain_anchor", "nodes_used", "node_consistency_pct",
		"llm_used", "llm_model", "llm_prompt_version",
		"self_verify_status", "self_verify_at",
		"confidence_label", "report_type", "archived_url", "created_at",
	}
}

func sampleReportRow(id string) []any {
	now := time.Now().UTC()
	return []any{
		id, "v_abc123", "s3://bucket/report.pdf", (*int64)(nil),
		"deadbeef", []byte{0x01}, "alias/k", 1,
		"digicert", []byte(nil), now,
		[]byte(nil), []byte(`["n1"]`), (*float64)(nil),
		false, (*string)(nil), (*string)(nil),
		(*string)(nil), (*time.Time)(nil),
		(*string)(nil), "observation_only", (*string)(nil), now,
	}
}

func TestVerdictReportsRepo_Insert_Success(t *testing.T) {
	r, mock := newReportsRepo(t)
	rep := sampleReport()

	mock.ExpectQuery(`INSERT INTO idcd_attest\.verdict_report`).
		WithArgs(anyArgs(23)...).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("vr_xyz789"))

	id, err := r.Insert(context.Background(), rep)
	require.NoError(t, err)
	assert.Equal(t, "vr_xyz789", id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerdictReportsRepo_Insert_Conflict(t *testing.T) {
	r, mock := newReportsRepo(t)
	rep := sampleReport()

	mock.ExpectQuery(`INSERT INTO idcd_attest\.verdict_report`).
		WithArgs(anyArgs(23)...).
		WillReturnError(&pgconn.PgError{Code: pgUniqueViolation})

	_, err := r.Insert(context.Background(), rep)
	assert.ErrorIs(t, err, ErrConflict)
}

func TestVerdictReportsRepo_Insert_DBError(t *testing.T) {
	r, mock := newReportsRepo(t)
	rep := sampleReport()
	sentinel := errors.New("disk full")

	mock.ExpectQuery(`INSERT INTO idcd_attest\.verdict_report`).
		WithArgs(anyArgs(23)...).
		WillReturnError(sentinel)

	_, err := r.Insert(context.Background(), rep)
	assert.ErrorIs(t, err, sentinel)
}

func TestVerdictReportsRepo_GetByID_Success(t *testing.T) {
	r, mock := newReportsRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE id`).
		WithArgs("vr_xyz789").
		WillReturnRows(pgxmock.NewRows(reportRowColumns()).AddRow(sampleReportRow("vr_xyz789")...))

	rep, err := r.GetByID(context.Background(), "vr_xyz789")
	require.NoError(t, err)
	assert.Equal(t, "vr_xyz789", rep.ID)
	assert.Equal(t, "observation_only", rep.ReportType)
}

func TestVerdictReportsRepo_GetByID_NotFound(t *testing.T) {
	r, mock := newReportsRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE id`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := r.GetByID(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVerdictReportsRepo_GetByOrderID_Success(t *testing.T) {
	r, mock := newReportsRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE order_id`).
		WithArgs("v_abc123").
		WillReturnRows(pgxmock.NewRows(reportRowColumns()).AddRow(sampleReportRow("vr_xyz789")...))

	rep, err := r.GetByOrderID(context.Background(), "v_abc123")
	require.NoError(t, err)
	assert.Equal(t, "v_abc123", rep.OrderID)
}

func TestVerdictReportsRepo_GetByOrderID_NotFound(t *testing.T) {
	r, mock := newReportsRepo(t)

	mock.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE order_id`).
		WithArgs("v_missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := r.GetByOrderID(context.Background(), "v_missing")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestVerdictReportsRepo_UpdateSelfVerify_Success(t *testing.T) {
	r, mock := newReportsRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_report\s+SET self_verify_status`).
		WithArgs("pass", now, "vr_xyz789").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := r.UpdateSelfVerify(context.Background(), "vr_xyz789", "pass", now)
	require.NoError(t, err)
}

func TestVerdictReportsRepo_UpdateSelfVerify_NotFound(t *testing.T) {
	r, mock := newReportsRepo(t)
	now := time.Now().UTC()

	mock.ExpectExec(`UPDATE idcd_attest\.verdict_report\s+SET self_verify_status`).
		WithArgs("fail", now, "missing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := r.UpdateSelfVerify(context.Background(), "missing", "fail", now)
	assert.ErrorIs(t, err, ErrNotFound)
}
