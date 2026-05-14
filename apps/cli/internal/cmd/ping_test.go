package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kite365/idcd/apps/cli/internal/api"
)

func TestPingCmd_success(t *testing.T) {
	resp := api.PingResponse{
		Target: "google.com",
		Results: []api.PingResult{
			{Node: "jp-tok-ntt", Location: "Tokyo, JP", LatencyMs: 32, Status: "ok"},
			{Node: "us-lax-cf", Location: "Los Angeles", LatencyMs: 89, Status: "ok"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	os.Setenv("IDCD_API_URL", srv.URL)
	defer os.Unsetenv("IDCD_API_URL")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"ping", "google.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ping command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "google.com") {
		t.Errorf("expected target in output, got: %s", output)
	}
	if !strings.Contains(output, "jp-tok-ntt") {
		t.Errorf("expected node name in output, got: %s", output)
	}
	if !strings.Contains(output, "32ms") {
		t.Errorf("expected latency in output, got: %s", output)
	}
}

func TestPingCmd_stubFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	os.Setenv("IDCD_API_URL", srv.URL)
	defer os.Unsetenv("IDCD_API_URL")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"ping", "example.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ping command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "example.com") {
		t.Errorf("expected fallback stub output with target, got: %s", output)
	}
}

func TestPingCmd_jsonFormat(t *testing.T) {
	resp := api.PingResponse{
		Target: "test.com",
		Results: []api.PingResult{
			{Node: "jp-tok-ntt", Location: "Tokyo, JP", LatencyMs: 10, Status: "ok"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	os.Setenv("IDCD_API_URL", srv.URL)
	defer os.Unsetenv("IDCD_API_URL")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--format", "json", "ping", "test.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ping json command failed: %v", err)
	}

	var out api.PingResponse
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if out.Target != "test.com" {
		t.Errorf("unexpected target: %s", out.Target)
	}
}
