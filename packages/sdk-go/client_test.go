package idcd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer(t *testing.T, handler http.Handler) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := New("test_key", WithBaseURL(srv.URL))
	return srv, client
}

func TestProbeHTTP_success(t *testing.T) {
	want := ProbeResult{TaskID: "task_abc123", Status: "queued"}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/probe/http" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test_key" {
			t.Errorf("missing or wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))

	got, err := client.ProbeHTTP(context.Background(), ProbeHTTPRequest{Target: "https://example.com"})
	if err != nil {
		t.Fatalf("ProbeHTTP error: %v", err)
	}
	if got.TaskID != want.TaskID || got.Status != want.Status {
		t.Errorf("ProbeHTTP = %+v, want %+v", got, want)
	}
}

func TestProbeHTTP_apiError(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "validation_error",
			"message": "invalid target",
		})
	}))

	_, err := client.ProbeHTTP(context.Background(), ProbeHTTPRequest{Target: ""})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
	if apiErr.Code != "validation_error" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "validation_error")
	}
}

func TestListMonitors_success(t *testing.T) {
	want := MonitorList{
		Items: []Monitor{
			{ID: "mon_1", Name: "Homepage", Type: "http", Target: "https://example.com", Status: "active"},
		},
		Total: 1, Page: 1, Limit: 20,
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/monitors" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))

	monitors, err := client.ListMonitors(context.Background())
	if err != nil {
		t.Fatalf("ListMonitors error: %v", err)
	}
	if len(monitors) != 1 || monitors[0].ID != "mon_1" {
		t.Errorf("ListMonitors = %+v, want 1 monitor with ID mon_1", monitors)
	}
}

func TestCreateMonitor_success(t *testing.T) {
	want := Monitor{
		ID: "mon_new", Name: "API Check", Type: "http",
		Target: "https://api.example.com", Status: "active", IntervalS: 300,
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/monitors" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req CreateMonitorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Name != "API Check" {
			t.Errorf("Name = %q, want %q", req.Name, "API Check")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(want)
	}))

	got, err := client.CreateMonitor(context.Background(), CreateMonitorRequest{
		Name:      "API Check",
		Type:      "http",
		Target:    "https://api.example.com",
		IntervalS: 300,
	})
	if err != nil {
		t.Fatalf("CreateMonitor error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

func TestGetIPInfo_success(t *testing.T) {
	want := IPInfo{
		IP: "1.2.3.4", Country: "US", City: "San Francisco",
		ASN: "AS15169 Google", ISP: "Google LLC",
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/info/ip" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("q") != "1.2.3.4" {
			t.Errorf("q param = %q, want %q", r.URL.Query().Get("q"), "1.2.3.4")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))

	got, err := client.GetIPInfo(context.Background(), "1.2.3.4")
	if err != nil {
		t.Fatalf("GetIPInfo error: %v", err)
	}
	if got.IP != want.IP || got.Country != want.Country {
		t.Errorf("GetIPInfo = %+v, want %+v", got, want)
	}
}

func TestClient_withOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ProbeResult{TaskID: "t1", Status: "queued"})
	}))
	defer srv.Close()

	customHTTPClient := &http.Client{Timeout: 5 * time.Second}
	client := New("my_key",
		WithBaseURL(srv.URL),
		WithHTTPClient(customHTTPClient),
	)

	if client.baseURL != srv.URL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, srv.URL)
	}
	if client.apiKey != "my_key" {
		t.Errorf("apiKey = %q, want %q", client.apiKey, "my_key")
	}
	if client.httpClient != customHTTPClient {
		t.Error("httpClient was not replaced by WithHTTPClient")
	}

	got, err := client.ProbeHTTP(context.Background(), ProbeHTTPRequest{Target: "example.com"})
	if err != nil {
		t.Fatalf("ProbeHTTP error: %v", err)
	}
	if got.TaskID != "t1" {
		t.Errorf("TaskID = %q, want %q", got.TaskID, "t1")
	}
}

func TestGetSLAReport_success(t *testing.T) {
	want := SLAReport{
		Period: SLAPeriod{From: "2026-03-01", To: "2026-05-31"},
		Monitors: []SLAMonitorEntry{
			{ID: "mon_1", Name: "Homepage", Type: "http", AvgUptimePct: 99.95},
		},
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/reports/sla" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))

	got, err := client.GetSLAReport(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetSLAReport error: %v", err)
	}
	if got.Period.From != want.Period.From {
		t.Errorf("Period.From = %q, want %q", got.Period.From, want.Period.From)
	}
	if len(got.Monitors) != 1 || got.Monitors[0].ID != "mon_1" {
		t.Errorf("Monitors = %+v, want 1 monitor", got.Monitors)
	}
}

func TestDeleteMonitor_noContent(t *testing.T) {
	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/monitors/mon_xyz" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	if err := client.DeleteMonitor(context.Background(), "mon_xyz"); err != nil {
		t.Fatalf("DeleteMonitor error: %v", err)
	}
}

func TestBulkMonitorAction_success(t *testing.T) {
	want := BulkResult{
		Succeeded: []string{"mon_1", "mon_2"},
		Failed:    []string{},
		Total:     2,
	}

	_, client := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/monitors/bulk" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))

	got, err := client.BulkMonitorAction(context.Background(), []string{"mon_1", "mon_2"}, "pause")
	if err != nil {
		t.Fatalf("BulkMonitorAction error: %v", err)
	}
	if len(got.Succeeded) != 2 || got.Total != 2 {
		t.Errorf("BulkMonitorAction = %+v, want %+v", got, want)
	}
}
