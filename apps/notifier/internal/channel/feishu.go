package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FeishuConfig holds configuration for the Feishu (飞书) robot channel.
type FeishuConfig struct {
	WebhookURL string `json:"webhook_url"`
}

// FeishuChannel sends alert notifications via a Feishu robot webhook
// using an interactive card message.
type FeishuChannel struct {
	cfg    FeishuConfig
	client *http.Client
}

// NewFeishu creates a FeishuChannel.
// Returns an error if the configured webhook URL fails SSRF validation.
func NewFeishu(cfg FeishuConfig) (*FeishuChannel, error) {
	if err := validateWebhookURL(cfg.WebhookURL); err != nil {
		return nil, fmt.Errorf("feishu: invalid webhook_url: %w", err)
	}
	return &FeishuChannel{
		cfg: cfg,
		client: &http.Client{
			Transport: safeTransport,
			Timeout:   10 * time.Second,
		},
	}, nil
}

// Type implements Channel.
func (f *FeishuChannel) Type() string { return "feishu" }

// feishuRequest is the JSON body for the Feishu robot API with an interactive card.
type feishuRequest struct {
	MsgType string     `json:"msg_type"`
	Card    feishuCard `json:"card"`
}

type feishuCard struct {
	Config   feishuCardConfig    `json:"config"`
	Header   feishuCardHeader    `json:"header"`
	Elements []feishuCardElement `json:"elements"`
}

type feishuCardConfig struct {
	WideScreenMode bool `json:"wide_screen_mode"`
}

type feishuCardHeader struct {
	Title    feishuCardTitle `json:"title"`
	Template string          `json:"template"` // "red", "yellow", "blue"
}

type feishuCardTitle struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

type feishuCardElement struct {
	Tag  string         `json:"tag"`
	Text feishuCardText `json:"text,omitempty"`
	URL  string         `json:"url,omitempty"`
}

type feishuCardText struct {
	Content string `json:"content"`
	Tag     string `json:"tag"`
}

// Send implements Channel.
func (f *FeishuChannel) Send(ctx context.Context, p Payload) error {
	template := feishuTemplate(p.Level)
	icon := levelIcon(p.Level)

	bodyText := fmt.Sprintf("%s\n\n[查看详情](%s)", p.Body, p.URL)

	req := feishuRequest{
		MsgType: "interactive",
		Card: feishuCard{
			Config: feishuCardConfig{WideScreenMode: true},
			Header: feishuCardHeader{
				Title:    feishuCardTitle{Content: icon + " " + p.Title, Tag: "plain_text"},
				Template: template,
			},
			Elements: []feishuCardElement{
				{
					Tag:  "div",
					Text: feishuCardText{Content: bodyText, Tag: "lark_md"},
				},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("feishu: marshal payload: %w", err)
	}

	return f.post(ctx, data)
}

func (f *FeishuChannel) post(ctx context.Context, data []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.cfg.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("feishu: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(httpReq) //nolint:bodyclose // body closed via drainAndClose helper
	if err != nil {
		return fmt.Errorf("feishu: do request: %w", err)
	}
	defer drainAndClose(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu: unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// feishuTemplate maps alert level to Feishu card header color template.
func feishuTemplate(level string) string {
	switch level {
	case "critical":
		return "red"
	case "warning":
		return "yellow"
	default:
		return "blue"
	}
}

// newFeishuWithClient creates a FeishuChannel using the provided HTTP client,
// skipping URL validation. This is intended for tests only.
func newFeishuWithClient(cfg FeishuConfig, client *http.Client) *FeishuChannel {
	return &FeishuChannel{cfg: cfg, client: client}
}
