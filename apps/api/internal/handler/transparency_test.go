package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTransparencyHandler_Get(t *testing.T) {
	h := NewTransparencyHandler()

	req := httptest.NewRequest(http.MethodGet, "/v1/transparency", nil)
	ctx := context.WithValue(req.Context(), "request_id", "test-transparency-001")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var envelope struct {
		Data struct {
			OverallStatus string `json:"overall_status"`
			Nodes         struct {
				Total int `json:"total"`
			} `json:"nodes"`
			KMS struct {
				Status string `json:"status"`
			} `json:"kms"`
			TSA struct {
				Providers []struct {
					Name string `json:"name"`
				} `json:"providers"`
			} `json:"tsa"`
		} `json:"data"`
	}

	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to parse response: %v — body: %s", err, rr.Body.String())
	}

	if envelope.Data.OverallStatus != "operational" {
		t.Errorf("expected overall_status=operational, got %q", envelope.Data.OverallStatus)
	}

	if envelope.Data.Nodes.Total == 0 {
		t.Error("expected nodes.total > 0")
	}

	if envelope.Data.KMS.Status != "operational" {
		t.Errorf("expected kms.status=operational, got %q", envelope.Data.KMS.Status)
	}

	if len(envelope.Data.TSA.Providers) == 0 {
		t.Error("expected at least one TSA provider")
	}
}
