package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFeishuChannel_Type(t *testing.T) {
	ch := NewFeishu(FeishuConfig{WebhookURL: "http://example.com"})
	if ch.Type() != "feishu" {
		t.Errorf("expected type 'feishu', got %q", ch.Type())
	}
}

func TestFeishuChannel_Send_Success(t *testing.T) {
	var received feishuRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewFeishu(FeishuConfig{WebhookURL: srv.URL})
	p := Payload{Title: "Down", Body: "site unreachable", URL: "https://dash.idcd.com", Level: "critical"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if received.MsgType != "interactive" {
		t.Errorf("expected msgtype 'interactive', got %q", received.MsgType)
	}
	if received.Card.Header.Template != "red" {
		t.Errorf("expected template 'red' for critical, got %q", received.Card.Header.Template)
	}
	if !strings.Contains(received.Card.Header.Title.Content, p.Title) {
		t.Errorf("expected card header to contain title %q", p.Title)
	}
	if len(received.Card.Elements) == 0 {
		t.Fatal("expected at least one card element")
	}
	if !strings.Contains(received.Card.Elements[0].Text.Content, p.Body) {
		t.Errorf("expected element text to contain body %q", p.Body)
	}
	if !strings.Contains(received.Card.Elements[0].Text.Content, p.URL) {
		t.Errorf("expected element text to contain URL %q", p.URL)
	}
}

func TestFeishuChannel_Send_WarningLevel(t *testing.T) {
	var received feishuRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewFeishu(FeishuConfig{WebhookURL: srv.URL})
	if err := ch.Send(context.Background(), Payload{Level: "warning", Title: "W", Body: "B", URL: "U"}); err != nil {
		t.Fatal(err)
	}
	if received.Card.Header.Template != "yellow" {
		t.Errorf("expected template 'yellow' for warning, got %q", received.Card.Header.Template)
	}
}

func TestFeishuChannel_Send_InfoLevel(t *testing.T) {
	var received feishuRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewFeishu(FeishuConfig{WebhookURL: srv.URL})
	if err := ch.Send(context.Background(), Payload{Level: "info", Title: "I", Body: "B", URL: "U"}); err != nil {
		t.Fatal(err)
	}
	if received.Card.Header.Template != "blue" {
		t.Errorf("expected template 'blue' for info, got %q", received.Card.Header.Template)
	}
}

func TestFeishuChannel_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ch := NewFeishu(FeishuConfig{WebhookURL: srv.URL})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "info"}

	if err := ch.Send(context.Background(), p); err == nil {
		t.Fatal("expected error for 401, got nil")
	}
}

func TestFeishuTemplate(t *testing.T) {
	tests := []struct {
		level string
		want  string
	}{
		{"critical", "red"},
		{"warning", "yellow"},
		{"info", "blue"},
		{"other", "blue"},
	}
	for _, tc := range tests {
		got := feishuTemplate(tc.level)
		if got != tc.want {
			t.Errorf("feishuTemplate(%q) = %q, want %q", tc.level, got, tc.want)
		}
	}
}
