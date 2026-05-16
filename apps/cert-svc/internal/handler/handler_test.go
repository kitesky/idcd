package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouter_HealthzAndReadyz_PublicNoAuth(t *testing.T) {
	r := New(Deps{})
	for _, p := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 (body=%s)", p, rec.Code, rec.Body.String())
		}
	}
}

func TestRouter_BusinessRoutes_NilAuth_Returns401(t *testing.T) {
	// With no AuthnMiddleware wired, every /v1/cert route must reject.
	r := New(Deps{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/cert/orders"},
		{http.MethodPost, "/v1/cert/orders"},
		{http.MethodGet, "/v1/cert/orders/42"},
		{http.MethodPost, "/v1/cert/dns-credentials"},
		{http.MethodGet, "/v1/cert/dns-credentials"},
		{http.MethodDelete, "/v1/cert/dns-credentials/9"},
		{http.MethodGet, "/v1/cert/certs"},
		{http.MethodGet, "/v1/cert/certs/9"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401 (body=%s)", rec.Code, rec.Body.String())
			}
			var body errResp
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body not JSON errResp: %v (raw=%s)", err, rec.Body.String())
			}
			if body.Code != codeUnauthorized {
				t.Errorf("code = %q, want %s", body.Code, codeUnauthorized)
			}
		})
	}
}

func TestRouter_UnknownOutsideV1Returns404(t *testing.T) {
	// Routes outside /v1/cert/* aren't behind auth — chi returns 404.
	r := New(Deps{})
	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown route status = %d, want 404", rec.Code)
	}
}

func TestWriteErr_SetsCanonicalShape(t *testing.T) {
	rec := httptest.NewRecorder()
	writeErr(rec, http.StatusBadRequest, codeDomainInvalid, "bad", map[string]string{"sans[0]": "bad"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var body errResp
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid body: %v", err)
	}
	if body.Code != codeDomainInvalid || body.Fields["sans[0]"] != "bad" {
		t.Errorf("body = %+v", body)
	}
}
