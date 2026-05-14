package channel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDingtalkChannel_Type(t *testing.T) {
	ch := NewDingtalk(DingtalkConfig{WebhookURL: "http://example.com", Secret: "s"})
	if ch.Type() != "dingtalk" {
		t.Errorf("expected type 'dingtalk', got %q", ch.Type())
	}
}

func TestDingtalkChannel_Send_Success(t *testing.T) {
	var received dingtalkRequest
	var queryParams url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryParams = r.URL.Query()
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := NewDingtalk(DingtalkConfig{WebhookURL: srv.URL, Secret: "my-secret"})
	p := Payload{Title: "Down", Body: "monitor failed", URL: "https://dash.idcd.com", Level: "critical"}

	if err := ch.Send(context.Background(), p); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if received.MsgType != "markdown" {
		t.Errorf("expected msgtype 'markdown', got %q", received.MsgType)
	}
	if received.Markdown.Title != p.Title {
		t.Errorf("expected title %q, got %q", p.Title, received.Markdown.Title)
	}
	if !strings.Contains(received.Markdown.Text, p.Body) {
		t.Errorf("expected text to contain body %q", p.Body)
	}
	if !strings.Contains(received.Markdown.Text, p.URL) {
		t.Errorf("expected text to contain URL %q", p.URL)
	}

	// Signature query params should be present
	if queryParams.Get("timestamp") == "" {
		t.Error("expected timestamp query param to be set")
	}
	if queryParams.Get("sign") == "" {
		t.Error("expected sign query param to be set")
	}
}

func TestDingtalkChannel_SignURL_ContainsParams(t *testing.T) {
	ch := NewDingtalk(DingtalkConfig{WebhookURL: "https://oapi.dingtalk.com/robot/send?access_token=abc", Secret: "secret"})
	signed, err := ch.signURL()
	if err != nil {
		t.Fatalf("signURL error: %v", err)
	}
	if !strings.Contains(signed, "timestamp=") {
		t.Error("signed URL missing timestamp param")
	}
	if !strings.Contains(signed, "sign=") {
		t.Error("signed URL missing sign param")
	}
	// Should append with & since URL already has ?
	if !strings.Contains(signed, "&timestamp=") {
		t.Error("expected & separator for existing query string")
	}
}

func TestDingtalkChannel_SignURL_NoExistingParams(t *testing.T) {
	ch := NewDingtalk(DingtalkConfig{WebhookURL: "https://oapi.dingtalk.com/robot/send", Secret: "secret"})
	signed, err := ch.signURL()
	if err != nil {
		t.Fatalf("signURL error: %v", err)
	}
	if !strings.Contains(signed, "?timestamp=") {
		t.Error("expected ? separator for clean URL")
	}
}

func TestDingtalkChannel_Send_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	ch := NewDingtalk(DingtalkConfig{WebhookURL: srv.URL, Secret: "s"})
	p := Payload{Title: "T", Body: "B", URL: "U", Level: "warning"}

	if err := ch.Send(context.Background(), p); err == nil {
		t.Fatal("expected error for 400, got nil")
	}
}
