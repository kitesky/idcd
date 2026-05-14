package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCommunityNodeTestHandler(t *testing.T) (*CommunityNodeHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := NewCommunityNodeHandler(mockPool)
	return h, mockPool
}

func TestCommunityNode_Apply_Unauthenticated(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	body := `{"hostname":"my.server.com","ip_address":"1.2.3.4","country":"CN"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/nodes/apply", strings.NewReader(body))
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Apply(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_Apply_MissingFields(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	body := `{"hostname":"my.server.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/nodes/apply", strings.NewReader(body))
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.Apply(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_Apply_Success(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(regexp.QuoteMeta(`INSERT INTO node_applications`)).
		WithArgs(
			pgxmock.AnyArg(), // id
			"u_test",         // user_id
			"my.server.com",  // hostname
			"1.2.3.4",        // ip_address
			"CN",             // country
			nil,              // city
			"China Telecom",  // isp
			pgxmock.AnyArg(), // bandwidth_mbps (*int)
			"want to help",   // motivation
			pgxmock.AnyArg(), // now
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body := `{"hostname":"my.server.com","ip_address":"1.2.3.4","country":"CN","isp":"China Telecom","bandwidth_mbps":100,"motivation":"want to help"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/nodes/apply", strings.NewReader(body))
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.Apply(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pending", data["status"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_MyApplications_Success(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"id", "user_id", "hostname", "ip_address", "country", "city", "isp", "status", "created_at", "updated_at"})
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, hostname, ip_address, country, city, isp, status, created_at, updated_at`)).
		WithArgs("u_test").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/nodes/my-applications", nil)
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.MyApplications(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "applications")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_GetPoints_NewUser(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"id", "amount", "balance", "reason", "ref_id", "created_at"})
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT id, amount, balance, reason, ref_id, created_at`)).
		WithArgs("u_test").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/account/points", nil)
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.GetPoints(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(0), data["balance"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_Redeem_InsufficientBalance(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	balanceRow := pgxmock.NewRows([]string{"balance"}).AddRow(100)
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT COALESCE`)).
		WithArgs("u_test").
		WillReturnRows(balanceRow)

	body := `{"reward_type":"api_calls","points":500}`
	req := httptest.NewRequest(http.MethodPost, "/v1/account/points/redeem", strings.NewReader(body))
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.Redeem(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "insufficient")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestCommunityNode_AdminUpdate_Approve(t *testing.T) {
	h, mockPool := newCommunityNodeTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectExec(regexp.QuoteMeta(`UPDATE node_applications`)).
		WithArgs("na_test123", "looks good", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	body := `{"action":"approve","note":"looks good"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/node-applications/na_test123", strings.NewReader(body))
	req = withRequestID(req, "test-req-id")
	req = withChiParam(req, "id", "na_test123")
	rr := httptest.NewRecorder()
	h.AdminUpdate(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "approve", data["action"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

var _ CommunityNodePool = (pgxmock.PgxPoolIface)(nil)
