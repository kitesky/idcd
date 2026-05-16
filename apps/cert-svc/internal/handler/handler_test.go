package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// routeCase describes one HTTP route that should currently return 501.
// Keeps the matrix in one place so adding a route is a one-line change.
type routeCase struct {
	method string
	path   string
}

func notImplementedRoutes() []routeCase {
	return []routeCase{
		{http.MethodPost, "/v1/cert/orders"},
		{http.MethodGet, "/v1/cert/orders"},
		{http.MethodGet, "/v1/cert/orders/42"},
		{http.MethodPost, "/v1/cert/orders/42/retry"},

		{http.MethodPost, "/v1/cert/dns-credentials"},
		{http.MethodGet, "/v1/cert/dns-credentials"},
		{http.MethodDelete, "/v1/cert/dns-credentials/7"},
		{http.MethodPost, "/v1/cert/dns-credentials/7/health-check"},

		{http.MethodGet, "/v1/cert/certs"},
		{http.MethodGet, "/v1/cert/certs/9"},
		{http.MethodPost, "/v1/cert/certs/9/download"},
		{http.MethodPost, "/v1/cert/certs/9/revoke"},
	}
}

func TestRoutes_ReturnNotImplemented(t *testing.T) {
	r := New(Deps{})
	for _, tc := range notImplementedRoutes() {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotImplemented {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
			}
			ct := rec.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body not JSON ErrorResponse: %v (raw=%s)", err, rec.Body.String())
			}
			if body.Code != "CERT_NOT_IMPL" {
				t.Errorf("code = %q, want CERT_NOT_IMPL", body.Code)
			}
			if body.Error == "" || body.Message == "" {
				t.Errorf("error/message empty: %+v", body)
			}
		})
	}
}

func TestRoutes_UnknownReturns404(t *testing.T) {
	r := New(Deps{})
	req := httptest.NewRequest(http.MethodGet, "/v1/cert/does-not-exist", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route status = %d, want 404", rec.Code)
	}
}
