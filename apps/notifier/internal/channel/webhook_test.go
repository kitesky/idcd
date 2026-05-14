package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookChannel_Type(t *testing.T) {
	ch := NewWebhook(WebhookConfig{URL: "http://example.com"})
	if ch.Type() != "webhook" {
		t.Errorf("expected type 'webhook', got %q", ch.Type())
	}
}

func TestWebhookChannel_Send_Success(t *testing.T) {
	var received webhookBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewWebhook(WebhookConfig{URL: srv.URL})
	p := Payload{Title: "Down", Body: "site is down", URL: "https://example.com", Level: "critical"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if received.Title != p.Title {
		t.Errorf("expected title %q, got %q", p.Title, received.Title)
	}
	if received.Body != p.Body {
		t.Errorf("expected body %q, got %q", p.Body, received.Body)
	}
	if received.Level != p.Level {
		t.Errorf("expected level %q, got %q", p.Level, received.Level)
	}
	if received.URL != p.URL {
		t.Errorf("expected url %q, got %q", p.URL, received.URL)
	}
}

func TestWebhookChannel_Send_ServerError_Retries(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ch := NewWebhook(WebhookConfig{URL: srv.URL})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "warning"}

	err := ch.Send(context.Background(), p)
	if err == nil {
		t.Fatal("expected error on 500 response, got nil")
	}

	// Should have retried 3 times
	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

func TestWebhookChannel_Send_InvalidURL(t *testing.T) {
	ch := NewWebhook(WebhookConfig{URL: "http://127.0.0.1:0/no-such-server"})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "info"}

	err := ch.Send(context.Background(), p)
	if err == nil {
		t.Fatal("expected error for unreachable URL, got nil")
	}
}

func TestWebhookChannel_Send_2xxAccepted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	ch := NewWebhook(WebhookConfig{URL: srv.URL})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "info"}
	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error for 202, got: %v", err)
	}
}
