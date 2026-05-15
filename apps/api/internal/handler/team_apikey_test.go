package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"errors"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

func newTeamAPIKeyHandler(t *testing.T) (*TeamAPIKeyHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	return NewTeamAPIKeyHandler(mockPool), mockPool
}

func withTeamID(r *http.Request, teamID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", teamID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}


func prepTeamReq(r *http.Request, userID, teamID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDContextKey(), userID)
	r = r.WithContext(ctx)
	r = withTeamID(r, teamID)
	r = withRequestID(r, "test-req-id")
	return r
}

func TestTeamAPIKey_Create_NonMember_403(t *testing.T) {
	h, mockPool := newTeamAPIKeyHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery("SELECT role FROM team_memberships").
		WithArgs("team_001", "u_stranger").
		WillReturnError(errors.New("no rows in result set"))

	body, _ := json.Marshal(map[string]string{"name": "CI Key", "type": "production"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepTeamReq(req, "u_stranger", "team_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestTeamAPIKey_Create_Member_Role_403(t *testing.T) {
	h, mockPool := newTeamAPIKeyHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery("SELECT role FROM team_memberships").
		WithArgs("team_001", "u_member").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	body, _ := json.Marshal(map[string]string{"name": "CI Key", "type": "production"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepTeamReq(req, "u_member", "team_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestTeamAPIKey_Create_Admin_201(t *testing.T) {
	h, mockPool := newTeamAPIKeyHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery("SELECT role FROM team_memberships").
		WithArgs("team_001", "u_admin").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	mockPool.ExpectQuery("INSERT INTO api_key").
		WithArgs(
			pgxmock.AnyArg(), // id
			"team_001",       // owner_id / team_id
			"CI Key",         // name
			pgxmock.AnyArg(), // prefix
			pgxmock.AnyArg(), // secret_hash
			pgxmock.AnyArg(), // scopes
			"u_admin",        // created_by
			"production",     // key_type
		).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "prefix", "scopes", "status", "key_type", "created_at"}).
			AddRow("key_abc", "CI Key", "deadbeef", []string{"read", "write"}, "active", "production", now))

	body, _ := json.Marshal(map[string]string{"name": "CI Key", "type": "production"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = prepTeamReq(req, "u_admin", "team_001")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var resp struct {
		Data struct {
			Prefix string `json:"prefix"`
			Key    string `json:"key"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.True(t, strings.HasPrefix(resp.Data.Key, apiKeyLivePrefix), "key should start with sk_live_")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamAPIKey_List_200(t *testing.T) {
	h, mockPool := newTeamAPIKeyHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery("SELECT EXISTS").
		WithArgs("team_001", "u_admin").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	mockPool.ExpectQuery("SELECT id, name, prefix").
		WithArgs("team_001").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "prefix", "scopes", "status", "key_type", "created_at"}).
			AddRow("key_abc", "CI Key", "deadbeef", []string{"read", "write"}, "active", "production", now))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = prepTeamReq(req, "u_admin", "team_001")
	rr := httptest.NewRecorder()

	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data []teamAPIKeyResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Len(t, resp.Data, 1)
	assert.Equal(t, "key_abc", resp.Data[0].ID)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamAPIKey_Delete_204(t *testing.T) {
	h, mockPool := newTeamAPIKeyHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery("SELECT role FROM team_memberships").
		WithArgs("team_001", "u_admin").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	mockPool.ExpectQuery("SELECT owner_id FROM api_key").
		WithArgs("key_abc").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow("team_001"))

	mockPool.ExpectExec("UPDATE api_key SET status").
		WithArgs("key_abc").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDContextKey(), "u_admin")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "team_001")
	rctx.URLParams.Add("key_id", "key_abc")
	req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, rctx))
	req = withRequestID(req, "test-req-id")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}
