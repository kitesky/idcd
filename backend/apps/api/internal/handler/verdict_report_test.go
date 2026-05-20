package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newVerdictReportTestHandler(t *testing.T) (*VerdictReportHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewVerdictReportHandler(mockPool), mockPool
}

func verdictReportColumns() []string {
	return []string{
		"id", "order_id", "pdf_url", "content_hash",
		"signature_key_id", "tsa_provider", "tsa_time",
		"nodes_used", "node_consistency_pct",
		"self_verify_status", "report_type",
		"archived_url", "created_at",
		"owner_id",
	}
}

func reqWithReportID(t *testing.T, userID, id string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/reports/"+id, nil)
	req = prepReq(req, userID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

func TestVerdictReportHandler_Get_Owner(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	pct := 97.5
	selfVerify := "pass"
	archived := "s3://bucket/worm/r_001.pdf"
	nodesJSON := []byte(`["node_a","node_b","node_c"]`)

	rows := pgxmock.NewRows(verdictReportColumns()).AddRow(
		"r_001", "v_001", "s3://bucket/r_001.pdf", "sha256:abcdef",
		"key_v1", "digicert", now,
		nodesJSON, &pct,
		&selfVerify, "observation_only",
		&archived, now,
		"u_test_user",
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report .+ JOIN idcd_attest\.verdict_order`).
		WithArgs("r_001").
		WillReturnRows(rows)

	req := reqWithReportID(t, "u_test_user", "r_001")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	var resp struct {
		Data GetVerdictReportResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "r_001", resp.Data.ID)
	assert.Equal(t, "v_001", resp.Data.OrderID)
	assert.Equal(t, "s3://bucket/r_001.pdf", resp.Data.PDFURL)
	assert.Equal(t, "sha256:abcdef", resp.Data.ContentHash)
	assert.Equal(t, "key_v1", resp.Data.SignatureKeyID)
	assert.Equal(t, "digicert", resp.Data.TSAProvider)
	assert.Equal(t, []string{"node_a", "node_b", "node_c"}, resp.Data.NodesUsed)
	require.NotNil(t, resp.Data.NodeConsistencyPct)
	assert.InDelta(t, 97.5, *resp.Data.NodeConsistencyPct, 0.001)
	require.NotNil(t, resp.Data.SelfVerifyStatus)
	assert.Equal(t, "pass", *resp.Data.SelfVerifyStatus)
	assert.Equal(t, "observation_only", resp.Data.ReportType)
	require.NotNil(t, resp.Data.ArchivedURL)
	assert.Equal(t, archived, *resp.Data.ArchivedURL)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictReportHandler_Get_NullableColumns_DefaultsEmptyNodes(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)

	rows := pgxmock.NewRows(verdictReportColumns()).AddRow(
		"r_002", "v_002", "s3://bucket/r_002.pdf", "sha256:def",
		"key_v1", "globalsign", now,
		[]byte(nil), (*float64)(nil),
		(*string)(nil), "observation_only",
		(*string)(nil), now,
		"u_test_user",
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report`).
		WithArgs("r_002").
		WillReturnRows(rows)

	req := reqWithReportID(t, "u_test_user", "r_002")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, "body=%s", rr.Body.String())

	var resp struct {
		Data GetVerdictReportResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "r_002", resp.Data.ID)
	assert.Equal(t, []string{}, resp.Data.NodesUsed)
	assert.Nil(t, resp.Data.NodeConsistencyPct)
	assert.Nil(t, resp.Data.SelfVerifyStatus)
	assert.Nil(t, resp.Data.ArchivedURL)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictReportHandler_Get_NotOwner(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows(verdictReportColumns()).AddRow(
		"r_003", "v_003", "s3://bucket/r_003.pdf", "sha256:abc",
		"key_v1", "digicert", now,
		[]byte(`["node_a"]`), (*float64)(nil),
		(*string)(nil), "observation_only",
		(*string)(nil), now,
		"u_other_user",
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report`).
		WithArgs("r_003").
		WillReturnRows(rows)

	req := reqWithReportID(t, "u_test_user", "r_003")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictReportHandler_Get_NotFound(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows(verdictReportColumns())
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report`).
		WithArgs("r_missing").
		WillReturnRows(rows)

	req := reqWithReportID(t, "u_test_user", "r_missing")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictReportHandler_Get_NoAuth(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/reports/r_001", nil)
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestVerdictReportHandler_Get_MissingID(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/verdict/reports/", nil)
	req = prepReq(req, "u_test_user")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestVerdictReportHandler_Get_QueryError(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report`).
		WithArgs("r_001").
		WillReturnError(assertAnError())

	req := reqWithReportID(t, "u_test_user", "r_001")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestVerdictReportHandler_Get_BadNodesJSON(t *testing.T) {
	h, mockPool := newVerdictReportTestHandler(t)
	defer mockPool.Close()

	now := time.Now().UTC().Truncate(time.Second)
	rows := pgxmock.NewRows(verdictReportColumns()).AddRow(
		"r_004", "v_004", "s3://bucket/r_004.pdf", "sha256:xyz",
		"key_v1", "ntsc", now,
		[]byte(`not-json`), (*float64)(nil),
		(*string)(nil), "observation_only",
		(*string)(nil), now,
		"u_test_user",
	)
	mockPool.ExpectQuery(`SELECT .+ FROM idcd_attest\.verdict_report`).
		WithArgs("r_004").
		WillReturnRows(rows)

	req := reqWithReportID(t, "u_test_user", "r_004")
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestNewVerdictReportHandler_NotNil(t *testing.T) {
	assert.NotNil(t, NewVerdictReportHandler(nil))
}

// assertAnError returns a non-nil error suitable for pgxmock WillReturnError.
func assertAnError() error {
	return errSentinel
}

var errSentinel = &handlerTestErr{msg: "synthetic query error"}

type handlerTestErr struct{ msg string }

func (e *handlerTestErr) Error() string { return e.msg }
