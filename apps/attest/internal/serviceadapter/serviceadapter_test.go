package serviceadapter

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

	"github.com/kite365/idcd/apps/attest/internal/repo"
	"github.com/kite365/idcd/apps/attest/internal/service"
)

// newMockPool returns a fresh pgxmock pool with QueryMatcherEqual so we
// can pin to the SQL strings declared in the repo package.
func newMockPool(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// orderRowColumns mirrors the projection in repo.verdictOrderColumns —
// keep in sync with verdict_orders.go.
func orderRowColumns() []string {
	return []string{
		"id", "owner_id", "template", "target",
		"time_window_start", "time_window_end", "status",
		"price_cny", "price_paid_cny", "paddle_order_id",
		"refund_reason", "refund_attempt_count", "refund_last_error",
		"refund_apology_sent_at",
		"created_at", "paid_at", "delivered_at", "failed_at", "refunded_at",
	}
}

func sampleOrderRow(id string) []any {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	return []any{
		id, "u_owner1", "sla", "example.com",
		now, now.Add(24 * time.Hour), "paid",
		float64(199), (*float64)(nil), (*string)(nil),
		(*string)(nil), 0, (*string)(nil),
		(*time.Time)(nil),
		now, (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil), (*time.Time)(nil),
	}
}

// reportRowColumns mirrors repo.verdictReportColumns.
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

func sampleReportRow(id, orderID string) []any {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	pct := float64(100)
	archived := "s3://bucket/" + id + ".pdf"
	return []any{
		id, orderID, "file:///tmp/" + id + ".pdf", (*int64)(nil),
		"deadbeef", []byte("sig"), "kms-key", 1,
		"digicert", []byte{}, now,
		[]byte("{}"), []byte("[]"), &pct,
		false, (*string)(nil), (*string)(nil),
		(*string)(nil), (*time.Time)(nil),
		(*string)(nil), "observation_only", &archived, now,
	}
}

// anyArgs returns a slice of n pgxmock.AnyArg matchers.
func anyArgs(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = pgxmock.AnyArg()
	}
	return out
}

// --- WrapOrders / Wrappers panic on nil --------------------------------

func TestWrap_PanicsOnNilRepo(t *testing.T) {
	assert.Panics(t, func() { _ = WrapOrders(nil) })
	assert.Panics(t, func() { _ = WrapReports(nil) })
}

// --- ordersAdapter.GetByID --------------------------------------------

func TestOrdersAdapter_GetByID_Happy(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_abc").
		WillReturnRows(pgxmock.NewRows(orderRowColumns()).AddRow(sampleOrderRow("v_abc")...))

	o, err := adapter.GetByID(context.Background(), "v_abc")
	require.NoError(t, err)
	require.NotNil(t, o)
	assert.Equal(t, "v_abc", o.ID)
	assert.Equal(t, "u_owner1", o.OwnerID)
	assert.Equal(t, "sla", o.Template)
	assert.Equal(t, "example.com", o.Target)
	assert.Equal(t, "paid", o.Status)
	assert.InDelta(t, 199.0, o.PriceCNY, 0.001)

	require.NoError(t, pool.ExpectationsWereMet())
}

func TestOrdersAdapter_GetByID_NotFound(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_missing").
		WillReturnError(pgx.ErrNoRows)

	o, err := adapter.GetByID(context.Background(), "v_missing")
	assert.Nil(t, o)
	assert.ErrorIs(t, err, repo.ErrNotFound)
}

func TestOrdersAdapter_GetByID_DBError(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	sentinel := errors.New("connection lost")
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_order\s+WHERE id`).
		WithArgs("v_bad").
		WillReturnError(sentinel)

	o, err := adapter.GetByID(context.Background(), "v_bad")
	assert.Nil(t, o)
	assert.ErrorIs(t, err, sentinel)
}

// --- ordersAdapter.UpdateStatus / SetDelivered / SetFailed ------------

func TestOrdersAdapter_UpdateStatus_PassesThrough(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs("generating", (*string)(nil), "v_abc", "paid").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := adapter.UpdateStatus(context.Background(), "v_abc", "paid", "generating", nil)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestOrdersAdapter_SetDelivered_RoutesGeneratingToDelivered(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs("delivered", (*string)(nil), "v_abc", "generating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := adapter.SetDelivered(context.Background(), "v_abc", time.Now())
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestOrdersAdapter_SetDelivered_InvalidTransitionPropagates(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs("delivered", (*string)(nil), "v_abc", "generating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	err := adapter.SetDelivered(context.Background(), "v_abc", time.Now())
	assert.ErrorIs(t, err, repo.ErrInvalidStatus)
}

func TestOrdersAdapter_SetFailed_RoutesWithReason(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapOrders(repos.Orders)

	reason := "kms sign: timeout"
	pool.ExpectExec(`UPDATE idcd_attest\.verdict_order`).
		WithArgs("failed", &reason, "v_abc", "generating").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err := adapter.SetFailed(context.Background(), "v_abc", time.Now(), reason)
	require.NoError(t, err)
	require.NoError(t, pool.ExpectationsWereMet())
}

// --- reportsAdapter ---------------------------------------------------

func TestReportsAdapter_Insert_Happy(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapReports(repos.Reports)

	pool.ExpectQuery(`INSERT INTO idcd_attest\.verdict_report`).
		WithArgs(anyArgs(23)...).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("vr_xyz"))

	id, err := adapter.Insert(context.Background(), sampleServiceReport("vr_xyz", "v_abc"))
	require.NoError(t, err)
	assert.Equal(t, "vr_xyz", id)
	require.NoError(t, pool.ExpectationsWereMet())
}

func TestReportsAdapter_Insert_ConflictPropagates(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapReports(repos.Reports)

	pool.ExpectQuery(`INSERT INTO idcd_attest\.verdict_report`).
		WithArgs(anyArgs(23)...).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	id, err := adapter.Insert(context.Background(), sampleServiceReport("vr_xyz", "v_abc"))
	assert.Empty(t, id)
	assert.ErrorIs(t, err, repo.ErrConflict)
}

func TestReportsAdapter_GetByOrderID_NotFoundReturnsNilNil(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapReports(repos.Reports)

	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE order_id`).
		WithArgs("v_missing").
		WillReturnError(pgx.ErrNoRows)

	rep, err := adapter.GetByOrderID(context.Background(), "v_missing")
	assert.Nil(t, rep)
	assert.NoError(t, err, "ErrNotFound must collapse to (nil, nil) per service.ReportRepo contract")
}

func TestReportsAdapter_GetByOrderID_Found(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapReports(repos.Reports)

	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE order_id`).
		WithArgs("v_abc").
		WillReturnRows(pgxmock.NewRows(reportRowColumns()).AddRow(sampleReportRow("vr_xyz", "v_abc")...))

	rep, err := adapter.GetByOrderID(context.Background(), "v_abc")
	require.NoError(t, err)
	require.NotNil(t, rep)
	assert.Equal(t, "vr_xyz", rep.ID)
	assert.Equal(t, "v_abc", rep.OrderID)
	assert.Equal(t, "digicert", rep.TSAProvider)
	assert.Equal(t, "observation_only", rep.ReportType)
	assert.Equal(t, "s3://bucket/vr_xyz.pdf", rep.ArchivedURL)
	assert.InDelta(t, 100.0, rep.NodeConsistencyPct, 0.001)
}

func TestReportsAdapter_GetByOrderID_OtherErrorPropagates(t *testing.T) {
	pool := newMockPool(t)
	repos := repo.New(pool)
	adapter := WrapReports(repos.Reports)

	sentinel := errors.New("conn refused")
	pool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report\s+WHERE order_id`).
		WithArgs("v_abc").
		WillReturnError(sentinel)

	rep, err := adapter.GetByOrderID(context.Background(), "v_abc")
	assert.Nil(t, rep)
	assert.ErrorIs(t, err, sentinel)
}

// sampleServiceReport builds a service.Report with every field
// populated so Insert exercises the full projection path.
func sampleServiceReport(id, orderID string) *service.Report {
	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	return &service.Report{
		ID:                  id,
		OrderID:             orderID,
		PDFURL:              "file:///tmp/" + id + ".pdf",
		ContentHash:         "deadbeef",
		Signature:           []byte("sig"),
		SignatureKeyID:      "kms-key",
		SignatureKeyVersion: 1,
		TSAProvider:         "digicert",
		TSATime:             now,
		NodesUsed:           []byte(`["a","b"]`),
		NodeConsistencyPct:  100,
		ReportType:          "observation_only",
		ArchivedURL:         "s3://bucket/" + id + ".pdf",
		CreatedAt:           now,
	}
}
