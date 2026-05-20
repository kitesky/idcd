package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReferralTestHandler(t *testing.T) (*ReferralHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := NewReferralHandler(mockPool)
	return h, mockPool
}

func TestReferralHandler_GetOrCreateCode_Unauthenticated(t *testing.T) {
	h, mockPool := newReferralTestHandler(t)
	defer mockPool.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/referral/code", nil)
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.GetOrCreateCode(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestReferralHandler_GetOrCreateCode_Creates(t *testing.T) {
	h, mockPool := newReferralTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT id, code, uses_count FROM referral_codes WHERE user_id = $1`)).
		WithArgs("u_test").
		WillReturnError(pgx.ErrNoRows)

	mockPool.ExpectExec(regexp.QuoteMeta(`INSERT INTO referral_codes (id, user_id, code) VALUES ($1, $2, $3)`)).
		WithArgs(pgxmock.AnyArg(), "u_test", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := httptest.NewRequest(http.MethodPost, "/v1/referral/code", nil)
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.GetOrCreateCode(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Contains(t, rr.Body.String(), "IDCD-")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestReferralHandler_GetOrCreateCode_Idempotent(t *testing.T) {
	h, mockPool := newReferralTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"id", "code", "uses_count"}).
		AddRow("ref_abc", "IDCD-ABC123", 3)
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT id, code, uses_count FROM referral_codes WHERE user_id = $1`)).
		WithArgs("u_test").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodPost, "/v1/referral/code", nil)
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.GetOrCreateCode(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "IDCD-ABC123")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestReferralHandler_ListRewards_Success(t *testing.T) {
	h, mockPool := newReferralTestHandler(t)
	defer mockPool.Close()

	rows := pgxmock.NewRows([]string{"id", "referred_id", "code", "status", "reward_amount", "credited_at", "created_at"})
	mockPool.ExpectQuery(`SELECT id, referred_id`).
		WithArgs("u_test").
		WillReturnRows(rows)

	req := httptest.NewRequest(http.MethodGet, "/v1/referral/rewards", nil)
	req = prepReq(req, "u_test")
	rr := httptest.NewRecorder()

	h.ListRewards(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "rewards")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

var (
	_ ReferralPool = (pgxmock.PgxPoolIface)(nil)
	_ = pgx.ErrNoRows
	_ = pgconn.CommandTag{}
)
