package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WecomConfig holds configuration for the WeCom (企业微信) robot channel.
type WecomConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// WecomChannel sends alert notifications via a WeCom (企业微信) robot webhook.
// It sends a markdown-formatted message.
type WecomChannel struct {
	cfg    WecomConfig
	client *http.Client
}

// NewWecom creates a WecomChannel.
// Returns an error if the configured webhook URL fails SSRF validation.
func NewWecom(cfg WecomConfig) (*WecomChannel, error) {
	if err := validateWebhookURL(cfg.WebhookURL); err != nil {
		return nil, fmt.Errorf("wecom: invalid webhook_url: %w", err)
	}
	return &WecomChannel{
		cfg: cfg,
		client: &http.Client{
			Transport: safeTransport,
			Timeout:   10 * time.Second,
		},
	}, nil
}

// Type implements Channel.
func (w *WecomChannel) Type() string { return "wecom" }

// wecomRequest is the JSON body for the WeCom robot API.
type wecomRequest struct {
	MsgType  string        `json:"msgtype"`
	Markdown wecomMarkdown `json:"markdown"`
}

type wecomMarkdown struct {
	Content string `json:"content"`
}

// Send implements Channel.
func (w *WecomChannel) Send(ctx context.Context, p Payload) error {
	icon := levelIcon(p.Level)
	content := fmt.Sprintf("## %s %s\n\n%s\n\n[查看详情](%s)", icon, p.Title, p.Body, p.URL)

	req := wecomRequest{
		MsgType:  "markdown",
		Markdown: wecomMarkdown{Content: content},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("wecom: marshal payload: %w", err)
	}

	return w.post(ctx, data)
}

func (w *WecomChannel) post(ctx context.Context, data []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("wecom: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(httpReq) //nolint:bodyclose // body closed via drainAndClose helper
	if err != nil {
		return fmt.Errorf("wecom: do request: %w", err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wecom: unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// levelIcon returns an emoji indicator for the alert level.
func levelIcon(level string) string {
	switch level {
	case "critical":
		return "🔴"
	case "warning":
		return "🟡"
	default:
		return "🔵"
	}
}

// newWecomWithClient creates a WecomChannel using the provided HTTP client,
// skipping URL validation. This is intended for tests only.
func newWecomWithClient(cfg WecomConfig, client *http.Client) *WecomChannel {
	return &WecomChannel{cfg: cfg, client: client}
}
