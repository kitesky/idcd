package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func setupNodeCmdHandler(t *testing.T) (*NodeCommandHandler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	h := &NodeCommandHandler{pool: mock, adminToken: enrollTestAdminToken}
	return h, mock
}

func withNodeID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("node_id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestQueueUpgrade_Success(t *testing.T) {
	h, mock := setupNodeCmdHandler(t)
	defer mock.Close()

	// nodeExists check
	mock.ExpectQuery(`SELECT 1 FROM enrolled_nodes`).
		WithArgs("nd_test").
		WillReturnRows(pgxmock.NewRows([]string{"?column?"}).AddRow(1))

	// insert command
	mock.ExpectExec(`INSERT INTO node_commands`).
		WithArgs(pgxmock.AnyArg(), "nd_test", "upgrade", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	body, _ := json.Marshal(upgradeRequest{
		Version:     "v1.2.3",
		DownloadURL: "https://cdn.idcd.com/agent/v1.2.3/idcd-agent-linux-amd64",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/admin/nodes/nd_test/upgrade", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	req = withNodeID(req, "nd_test")
	rr := httptest.NewRecorder()

	h.QueueUpgrade(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet: %v", err)
	}
}

func TestQueueUpgrade_MissingDownloadURL(t *testing.T) {
	h, mock := setupNodeCmdHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT 1 FROM enrolled_nodes`).
		WithArgs("nd_test").
		WillReturnRows(pgxmock.NewRows([]string{"?column?"}).AddRow(1))

	body, _ := json.Marshal(upgradeRequest{Version: "v1.0.0"}) // no download_url
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	req = withNodeID(req, "nd_test")
	rr := httptest.NewRecorder()

	h.QueueUpgrade(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestQueueUpgrade_Unauthorized(t *testing.T) {
	h, mock := setupNodeCmdHandler(t)
	defer mock.Close()

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Admin-Token", "wrong")
	rr := httptest.NewRecorder()

	h.QueueUpgrade(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestQueueReloadConfig_Success(t *testing.T) {
	h, mock := setupNodeCmdHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT 1 FROM enrolled_nodes`).
		WithArgs("nd_test").
		WillReturnRows(pgxmock.NewRows([]string{"?column?"}).AddRow(1))

	mock.ExpectExec(`INSERT INTO node_commands`).
		WithArgs(pgxmock.AnyArg(), "nd_test", "reload_config", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	req = withNodeID(req, "nd_test")
	rr := httptest.NewRecorder()

	h.QueueReloadConfig(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestListNodes_Success(t *testing.T) {
	h, mock := setupNodeCmdHandler(t)
	defer mock.Close()

	mock.ExpectQuery(`SELECT COALESCE`).
		WillReturnRows(pgxmock.NewRows([]string{"json_agg"}).AddRow(`[]`))

	req := httptest.NewRequest(http.MethodGet, "/internal/admin/nodes", nil)
	req.Header.Set("X-Admin-Token", enrollTestAdminToken)
	rr := httptest.NewRecorder()

	h.ListNodes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
