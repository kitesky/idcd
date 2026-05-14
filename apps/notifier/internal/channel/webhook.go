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
// Returns an error if the configured URL fails SSRF validation.
func NewWebhook(cfg WebhookConfig) (*WebhookChannel, error) {
	if err := validateWebhookURL(cfg.URL); err != nil {
		return nil, fmt.Errorf("webhook: invalid url: %w", err)
	}
	return &WebhookChannel{
		cfg: cfg,
		client: &http.Client{
			Transport: safeTransport,
			Timeout:   10 * time.Second,
		},
	}, nil
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
// retrying up to 3 times on transient failures. Retries are skipped when the
// context has already been cancelled.
func (w *WebhookChannel) Send(ctx context.Context, p Payload) error {
	body := webhookBody(p) // direct conversion — same field layout

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("webhook: marshal payload: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		// Do not start a new attempt if the caller has already given up.
		if ctx.Err() != nil {
			return fmt.Errorf("webhook: context cancelled before attempt %d: %w", attempt, ctx.Err())
		}
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
	defer drainAndClose(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// newWebhookWithClient creates a WebhookChannel using the provided HTTP client,
// skipping URL validation. This is intended for tests only.
func newWebhookWithClient(cfg WebhookConfig, client *http.Client) *WebhookChannel {
	return &WebhookChannel{cfg: cfg, client: client}
}
