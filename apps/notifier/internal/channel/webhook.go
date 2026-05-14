package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookConfig holds configuration for the webhook channel.
type WebhookConfig struct {
	URL string `json:"url"`
}

// WebhookChannel sends alert notifications via HTTP POST to a configurable endpoint.
type WebhookChannel struct {
	cfg    WebhookConfig
	client *http.Client
}

// NewWebhook creates a WebhookChannel with a 10-second timeout HTTP client.
func NewWebhook(cfg WebhookConfig) *WebhookChannel {
	return &WebhookChannel{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Type implements Channel.
func (w *WebhookChannel) Type() string { return "webhook" }

// webhookBody is the JSON payload sent to the webhook endpoint.
type webhookBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Level string `json:"level"`
}

// Send implements Channel. It posts the payload as JSON to the configured URL,
// retrying up to 3 times on transient failures.
func (w *WebhookChannel) Send(ctx context.Context, p Payload) error {
	body := webhookBody{
		Title: p.Title,
		Body:  p.Body,
		URL:   p.URL,
		Level: p.Level,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		lastErr = w.doPost(ctx, data)
		if lastErr == nil {
			return nil
		}
	}
	return fmt.Errorf("webhook: all 3 attempts failed: %w", lastErr)
}

func (w *WebhookChannel) doPost(ctx context.Context, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
