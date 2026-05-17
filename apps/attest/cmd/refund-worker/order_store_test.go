package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/attest/internal/refund"
	"github.com/kite365/idcd/apps/attest/internal/repo"
)

// newStore builds a repoOrderStore plumbed to a fresh pgxmock pool.
func newStore(t *testing.T) (*repoOrderStore, pgxmock.PgxPoolIface) {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	repos := repo.New(pool)
	return &repoOrderStore{
		orders:  repos.Orders,
		reports: repos.Reports,
		pool:    pool,
		now:     time.Now,
	}, pool
}

// expectUserEmailQuery wires the cross-schema "owner_id → user.email"
// lookup that loadOrder now performs after fetching the verdict_order.
// Tests that don't care about the email can call this with the
// canonical owner_id ("u_1") + email ("u_1@example.com") shipped by
// sampleOrderRow.
func expectUserEmailQuery(pool pgxmock.PgxPoolIface, ownerID, email string) {
	pool.ExpectQuery(`SELECT email::text FROM idcd_main\."user"\s+WHERE id`).
		WithArgs(ownerID).
		WillReturnRows(pgxmock.NewRows([]string{"email"}).AddRow(email))
}

func orderCols() []string {
	return []string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end", "status",
		"price_cny", "price_paid_cny", "ext_order_id",
		"refund_reason", "refund_attempt_count", "refund_last_error",
		"refund_apology_sent_at",
		"created_at", "paid_at", "delivered_at", "failed_at", "refunded_at",
	}
}

func sampleOrderRow(id string) []any {
	now := time.Now().UTC()
	paymenthub := "pdle_" + id
	return []any{
		id, "u_1", "sla", "example.com",
		now, now.Add(24 * time.Hour), "failed",
		float64(199), (*float64)(nil), &paymenthub,
		(*string)(nil), 1, (*string)(nil),
		(*time.Time)(nil),
		now, (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil),
	}
}

func reportCols() []string {
	return []string{
		"id", "order_id", "pdf_url", "pdf_size_bytes",
		"content_hash", "signature", "signature_key_id", "signature_key_version",
		"tsa_provider", "tsa_response_blob", "tsa_time",
		"blockchain_anchor", "nodes_used", "node_consistency_pct",
		"llm_used", "llm_model", "llm_prompt_version",
		"self_verify_status", "self_verify_at",
		"confidence_label", "report_type", "archived_url",
		"created_at",
	}
}

func sampleReportRow(reportID, orderID string) []any {
	now := time.Now().UTC()
	return []any{
		reportID, orderID, "s3://bkt/r.pdf", (*int64)(nil),
		"h", []byte("sig"), "k", 1,
		"digicert", []byte{}, now,
		[]byte("{}"), []byte("{}"), (*float64)(nil),
		false, (*string)(nil), (*string)(nil),
		(*string)(nil), (*time.Time)(nil),
		(*string)(nil), "verdict", (*string)(nil),
		now,
	}
}

func TestRepoOrderStore_GetByReportID_Success(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE id`).
		WithArgs("vr_1").
		WillReturnRows(pgxmock.NewRows(reportCols()).AddRow(sampleReportRow("vr_1", "v_1")...))
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnRows(pgxmock.NewRows(orderCols()).AddRow(sampleOrderRow("v_1")...))
	expectUserEmailQuery(pool, "u_1", "u_1@example.com")

	o, err := s.GetByReportID(context.Background(), "vr_1")
	require.NoError(t, err)
	assert.Equal(t, "v_1", o.ID)
	assert.Equal(t, "u_1", o.OwnerID)
	assert.Equal(t, "u_1@example.com", o.UserEmail)
	assert.Equal(t, "pdle_v_1", o.ExtOrderID)
	assert.Equal(t, "CNY", o.Currency)
	assert.InDelta(t, 199.0, o.PriceCNYYuan, 0.001)
}

func TestRepoOrderStore_GetByReportID_ReportMissing(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE id`).
		WithArgs("vr_x").
		WillReturnError(pgx.ErrNoRows)

	_, err := s.GetByReportID(context.Background(), "vr_x")
	assert.ErrorIs(t, err, refund.ErrOrderNotFound)
}

func TestRepoOrderStore_GetByReportID_ReportDBError(t *testing.T) {
	s, pool := newStore(t)
	sentinel := errors.New("boom")
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE id`).
		WithArgs("vr_x").
		WillReturnError(sentinel)

	_, err := s.GetByReportID(context.Background(), "vr_x")
	require.Error(t, err)
	assert.NotErrorIs(t, err, refund.ErrOrderNotFound)
}

func TestRepoOrderStore_GetByID_Success_PopulatesEnrichment(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnRows(pgxmock.NewRows(orderCols()).AddRow(sampleOrderRow("v_1")...))
	expectUserEmailQuery(pool, "u_1", "u_1@example.com")

	o, err := s.GetByID(context.Background(), "v_1")
	require.NoError(t, err)
	assert.Equal(t, "v_1", o.ID)
	assert.Equal(t, "u_1", o.OwnerID)
	assert.Equal(t, "u_1@example.com", o.UserEmail)
	assert.Equal(t, "CNY", o.Currency)
}

func TestRepoOrderStore_GetByID_UserMissing_LeavesEmailEmpty(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnRows(pgxmock.NewRows(orderCols()).AddRow(sampleOrderRow("v_1")...))
	pool.ExpectQuery(`SELECT email::text FROM idcd_main\."user"\s+WHERE id`).
		WithArgs("u_1").
		WillReturnError(pgx.ErrNoRows)

	o, err := s.GetByID(context.Background(), "v_1")
	require.NoError(t, err, "user-missing is fail-open per D5")
	assert.Equal(t, "v_1", o.ID)
	assert.Empty(t, o.UserEmail)
}

func TestRepoOrderStore_GetByID_UserLookupDBError_Propagates(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnRows(pgxmock.NewRows(orderCols()).AddRow(sampleOrderRow("v_1")...))
	pool.ExpectQuery(`SELECT email::text FROM idcd_main\."user"\s+WHERE id`).
		WithArgs("u_1").
		WillReturnError(errors.New("db down"))

	_, err := s.GetByID(context.Background(), "v_1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user email lookup")
}

func TestRepoOrderStore_LookupUserEmail_EmptyOwnerID(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.lookupUserEmail(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestRepoOrderStore_LookupUserEmail_NilPoolReturnsEmpty(t *testing.T) {
	s := &repoOrderStore{}
	got, err := s.lookupUserEmail(context.Background(), "u_1")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestRepoOrderStore_GetByID_NotFound(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	_, err := s.GetByID(context.Background(), "missing")
	assert.ErrorIs(t, err, refund.ErrOrderNotFound)
}

func TestRepoOrderStore_GetByID_DBError(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_1").
		WillReturnError(errors.New("boom"))

	_, err := s.GetByID(context.Background(), "v_1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, refund.ErrOrderNotFound)
}

func TestRepoOrderStore_MarkRefunded(t *testing.T) {
	s, pool := newStore(t)
	now := time.Now().UTC()
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET status`).
		WithArgs("refunded", (*string)(nil), "v_1", "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refunded_at`).
		WithArgs(now, "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := s.MarkRefunded(context.Background(), "v_1", "failed", now)
	require.NoError(t, err)
}

func TestRepoOrderStore_MarkRefunded_UpdateStatusFails(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET status`).
		WithArgs("refunded", (*string)(nil), "v_1", "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := s.MarkRefunded(context.Background(), "v_1", "failed", time.Now())
	require.Error(t, err)
}

func TestRepoOrderStore_MarkRefunded_StampFails(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET status`).
		WithArgs("refunded", (*string)(nil), "v_1", "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refunded_at`).
		WithArgs(pgxmock.AnyArg(), "v_1").
		WillReturnError(errors.New("disk full"))

	err := s.MarkRefunded(context.Background(), "v_1", "failed", time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stamp refunded_at")
}

func TestRepoOrderStore_MarkRefundFailed(t *testing.T) {
	s, pool := newStore(t)
	reason := "paymenthub 5xx"
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET status`).
		WithArgs("refund_failed", &reason, "v_1", "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := s.MarkRefundFailed(context.Background(), "v_1", "failed", reason)
	require.NoError(t, err)
}

func TestRepoOrderStore_BumpRefundAttempt(t *testing.T) {
	s, pool := newStore(t)
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_attempt_count`).
		WithArgs("paymenthub err", "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := s.BumpRefundAttempt(context.Background(), "v_1", "paymenthub err", 1)
	require.NoError(t, err)
}

func TestRepoOrderStore_MarkApologySent(t *testing.T) {
	s, pool := newStore(t)
	now := time.Now().UTC()
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order\s+SET refund_apology_sent_at`).
		WithArgs(now, "v_1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := s.MarkApologySent(context.Background(), "v_1", now)
	require.NoError(t, err)
}
