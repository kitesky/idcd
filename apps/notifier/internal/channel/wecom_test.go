package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWecomChannel_Type(t *testing.T) {
	ch := NewWecom(WecomConfig{WebhookURL: "http://example.com"})
	if ch.Type() != "wecom" {
		t.Errorf("expected type 'wecom', got %q", ch.Type())
	}
}

func TestWecomChannel_Send_Success(t *testing.T) {
	var received wecomRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWecom(WecomConfig{WebhookURL: srv.URL})
	p := Payload{Title: "Alert", Body: "Something broke", URL: "https://dash.idcd.com", Level: "critical"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if received.MsgType != "markdown" {
		t.Errorf("expected msgtype 'markdown', got %q", received.MsgType)
	}
	if !strings.Contains(received.Markdown.Content, p.Title) {
		t.Errorf("expected content to contain title %q", p.Title)
	}
	if !strings.Contains(received.Markdown.Content, p.Body) {
		t.Errorf("expected content to contain body %q", p.Body)
	}
	if !strings.Contains(received.Markdown.Content, p.URL) {
		t.Errorf("expected content to contain URL %q", p.URL)
	}
	// Critical level should use red icon
	if !strings.Contains(received.Markdown.Content, "🔴") {
		t.Errorf("expected red icon for critical level")
	}
}

func TestWecomChannel_Send_WarningLevel(t *testing.T) {
	var received wecomRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWecom(WecomConfig{WebhookURL: srv.URL})
	p := Payload{Title: "Warn", Body: "latency high", URL: "https://dash.idcd.com", Level: "warning"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(received.Markdown.Content, "🟡") {
		t.Errorf("expected yellow icon for warning level")
	}
}

func TestWecomChannel_Send_InfoLevel(t *testing.T) {
	var received wecomRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWecom(WecomConfig{WebhookURL: srv.URL})
	p := Payload{Title: "Info", Body: "recovered", URL: "https://dash.idcd.com", Level: "info"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(received.Markdown.Content, "🔵") {
		t.Errorf("expected blue icon for info level")
	}
}

func TestWecomChannel_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := NewWecom(WecomConfig{WebhookURL: srv.URL})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "info"}

	if err := ch.Send(context.Background(), p); err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestLevelIcon(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"critical", "🔴"},
		{"warning", "🟡"},
		{"info", "🔵"},
		{"unknown", "🔵"},
	}
	for _, tc := range tests {
		got := levelIcon(tc.level)
		if got != tc.want {
			t.Errorf("levelIcon(%q) = %q, want %q", tc.level, got, tc.want)
		}
	}
}
