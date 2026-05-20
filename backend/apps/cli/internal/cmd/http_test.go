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

func TestHTTPCmd_success(t *testing.T) {
	resp := api.HTTPResponse{
		URL: "https://idcd.com",
		Results: []api.HTTPResult{
			{Node: "jp-tok-ntt", Location: "Tokyo, JP", Status: "OK", LatencyMs: 234, HTTPCode: 200, TLS: "TLS 1.3"},
			{Node: "us-lax-cf", Location: "Los Angeles", Status: "OK", LatencyMs: 89, HTTPCode: 200, TLS: "TLS 1.3"},
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
	root.SetArgs([]string{"http", "https://idcd.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("http command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "idcd.com") {
		t.Errorf("expected URL in output, got: %s", output)
	}
	if !strings.Contains(output, "jp-tok-ntt") {
		t.Errorf("expected node in output, got: %s", output)
	}
	if !strings.Contains(output, "200") {
		t.Errorf("expected HTTP code in output, got: %s", output)
	}
	if !strings.Contains(output, "TLS 1.3") {
		t.Errorf("expected TLS info in output, got: %s", output)
	}
}

func TestHTTPCmd_stubFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	os.Setenv("IDCD_API_URL", srv.URL)
	defer os.Unsetenv("IDCD_API_URL")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"http", "https://example.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("http command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "example.com") {
		t.Errorf("expected target in fallback output, got: %s", output)
	}
}
