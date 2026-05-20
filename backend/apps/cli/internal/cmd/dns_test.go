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

func TestDNSCmd_success(t *testing.T) {
	resp := api.DNSResponse{
		Domain: "idcd.com",
		Type:   "A",
		Records: []api.DNSRecord{
			{Name: "idcd.com", TTL: 300, Type: "A", Value: "104.21.0.1"},
			{Name: "idcd.com", TTL: 300, Type: "A", Value: "172.67.0.1"},
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
	root.SetArgs([]string{"dns", "idcd.com", "--type", "A"})
	if err := root.Execute(); err != nil {
		t.Fatalf("dns command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "idcd.com") {
		t.Errorf("expected domain in output, got: %s", output)
	}
	if !strings.Contains(output, "104.21.0.1") {
		t.Errorf("expected IP record in output, got: %s", output)
	}
	if !strings.Contains(output, "300") {
		t.Errorf("expected TTL in output, got: %s", output)
	}
}

func TestDNSCmd_stubFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	os.Setenv("IDCD_API_URL", srv.URL)
	defer os.Unsetenv("IDCD_API_URL")

	root := NewRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"dns", "example.com"})
	if err := root.Execute(); err != nil {
		t.Fatalf("dns command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "example.com") {
		t.Errorf("expected domain in fallback output, got: %s", output)
	}
}
