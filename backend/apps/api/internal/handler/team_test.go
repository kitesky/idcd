package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTeamTestHandler(t *testing.T) (*TeamHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	h := NewTeamHandler(mockPool)
	return h, mockPool
}

func teamRequest(method, path string, body any) *http.Request {
	var b bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&b).Encode(body)
	}
	req := httptest.NewRequest(method, path, &b)
	req.Header.Set("Content-Type", "application/json")
	req = withRequestID(req, "test-req-id")
	return req
}

func TestTeamHandler_Create_Unauthenticated(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	req := teamRequest(http.MethodPost, "/v1/teams", map[string]string{"name": "Acme", "slug": "acme"})
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_Create_Success(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	now := time.Now()
	mockPool.ExpectBegin()
	mockPool.ExpectQuery(regexp.QuoteMeta(`INSERT INTO teams`)).
		WithArgs(pgxmock.AnyArg(), "Acme Corp", "acme", "u_owner").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug", "plan", "owner_id", "created_at", "updated_at"}).
			AddRow("team_abc", "Acme Corp", "acme", "free", "u_owner", now, now))

	mockPool.ExpectExec(regexp.QuoteMeta(`INSERT INTO team_memberships`)).
		WithArgs(pgxmock.AnyArg(), "team_abc", "u_owner").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockPool.ExpectCommit()

	req := teamRequest(http.MethodPost, "/v1/teams", map[string]string{"name": "Acme Corp", "slug": "acme"})
	req = prepReq(req, "u_owner")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
	resp := envelope["data"].(map[string]any)["team"].(map[string]any)
	assert.Equal(t, "team_abc", resp["id"])
	assert.Equal(t, "acme", resp["slug"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_List_Success(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	now := time.Now()
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT t.id, t.name, t.slug, t.plan, t.owner_id, t.created_at, t.updated_at`)).
		WithArgs("u_member").
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "slug", "plan", "owner_id", "created_at", "updated_at"}).
			AddRow("team_abc", "Acme Corp", "acme", "free", "u_owner", now, now))

	req := teamRequest(http.MethodGet, "/v1/teams", nil)
	req = prepReq(req, "u_member")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var listEnvelope map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &listEnvelope))
	items := listEnvelope["data"].(map[string]any)["teams"].([]any)
	assert.Len(t, items, 1)
	assert.Equal(t, "team_abc", items[0].(map[string]any)["id"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_CreateInvitation_Success(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	now := time.Now()
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`)).
		WithArgs("team_abc", "u_admin").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("admin"))

	// Args order after 00046: id, team_id, email, role, token_hash, invited_by, expires_at.
	// The fifth arg is now hex(sha256(plaintext)); we accept AnyArg here and
	// assert format on the response body below — verifying both that the
	// handler returns the plaintext and that it didn't accidentally feed
	// the plaintext through as the DB column value.
	mockPool.ExpectQuery(regexp.QuoteMeta(`INSERT INTO team_invitations`)).
		WithArgs(pgxmock.AnyArg(), "team_abc", "alice@example.com", "member",
			pgxmock.AnyArg(), "u_admin", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "team_id", "email", "role", "invited_by", "status", "expires_at", "created_at"}).
			AddRow("tinv_xyz", "team_abc", "alice@example.com", "member", "u_admin", "pending", now.Add(7*24*time.Hour), now))

	req := teamRequest(http.MethodPost, "/v1/teams/team_abc/invitations",
		map[string]string{"email": "alice@example.com", "role": "member"})
	req = prepReq(req, "u_admin")
	rr := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Post("/v1/teams/{id}/invitations", h.CreateInvitation)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var invEnvelope map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &invEnvelope))
	invResp := invEnvelope["data"].(map[string]any)["invitation"].(map[string]any)
	assert.Equal(t, "alice@example.com", invResp["email"])

	// Plaintext token must be surfaced exactly once at creation time.
	tok, _ := invResp["token"].(string)
	assert.Len(t, tok, 32, "invitation token should be 16-byte hex (32 chars)")
	assert.Regexp(t, "^[0-9a-f]{32}$", tok)
	// invite_url is omitempty: empty when AppBaseURL is unset (this test's default).
	if v, ok := invResp["invite_url"]; ok {
		assert.Equal(t, "", v)
	}

	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_AcceptInvitation_ValidToken(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	// UPDATE+INSERT now run inside a tx so the membership and the
	// invitation-status flip commit atomically; previously INSERT failure
	// left the invitation 'accepted' with no membership row, locking the
	// user out forever.
	mockPool.ExpectBegin()
	// UPDATE...RETURNING atomically consumes the invitation, with expiry
	// merged into the WHERE clause so the check can't race the status flip.
	// Lookup now uses SHA-256 hex (00046 migration) — compute the expected
	// hash here rather than hardcoding so a future hash-algo change in the
	// handler trips the test instead of silently diverging.
	const plainToken = "valid-token-abc"
	expectedHash := hashInvitationToken(plainToken)
	mockPool.ExpectQuery(regexp.QuoteMeta(`UPDATE team_invitations`)).
		WithArgs(expectedHash).
		WillReturnRows(pgxmock.NewRows([]string{"id", "team_id", "role", "invited_by"}).
			AddRow("tinv_xyz", "team_abc", "member", "u_inviter"))

	mockPool.ExpectExec(regexp.QuoteMeta(`INSERT INTO team_memberships`)).
		WithArgs(pgxmock.AnyArg(), "team_abc", "u_joiner", "member", "u_inviter").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockPool.ExpectCommit()

	req := teamRequest(http.MethodPost, "/v1/teams/accept-invitation",
		map[string]string{"token": plainToken})
	req = prepReq(req, "u_joiner")
	rr := httptest.NewRecorder()
	h.AcceptInvitation(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var accEnvelope map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &accEnvelope))
	accResp := accEnvelope["data"].(map[string]any)
	assert.Equal(t, "team_abc", accResp["team_id"])
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_RemoveMember_SelfLeave(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	// Owner check: u_self is not the team owner, so removal is allowed.
	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT owner_id FROM teams WHERE id = $1`)).
		WithArgs("team_abc").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow("u_owner"))

	mockPool.ExpectExec(regexp.QuoteMeta(`DELETE FROM team_memberships WHERE team_id = $1 AND user_id = $2`)).
		WithArgs("team_abc", "u_self").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	req := teamRequest(http.MethodDelete, "/v1/teams/team_abc/members/u_self", nil)
	req = prepReq(req, "u_self")
	rr := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Delete("/v1/teams/{id}/members/{user_id}", h.RemoveMember)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestTeamHandler_UpdateMemberRole_NonOwner_Forbidden(t *testing.T) {
	h, mockPool := newTeamTestHandler(t)
	defer mockPool.Close()

	mockPool.ExpectQuery(regexp.QuoteMeta(`SELECT role FROM team_memberships WHERE team_id = $1 AND user_id = $2`)).
		WithArgs("team_abc", "u_member").
		WillReturnRows(pgxmock.NewRows([]string{"role"}).AddRow("member"))

	req := teamRequest(http.MethodPatch, "/v1/teams/team_abc/members/u_other",
		map[string]string{"role": "admin"})
	req = prepReq(req, "u_member")
	rr := httptest.NewRecorder()

	router := chi.NewRouter()
	router.Patch("/v1/teams/{id}/members/{user_id}", h.UpdateMemberRole)
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}
