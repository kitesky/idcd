package channel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DingtalkConfig holds configuration for the DingTalk robot channel.
type DingtalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret"` // signing secret for DingTalk signature
}

// DingtalkChannel sends alert notifications via a DingTalk robot webhook
// with HMAC-SHA256 signature authentication.
type DingtalkChannel struct {
	cfg    DingtalkConfig
	client *http.Client
}

// NewDingtalk creates a DingtalkChannel.
func NewDingtalk(cfg DingtalkConfig) *DingtalkChannel {
	return &DingtalkChannel{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Type implements Channel.
func (d *DingtalkChannel) Type() string { return "dingtalk" }

// dingtalkRequest is the JSON body for the DingTalk robot API.
type dingtalkRequest struct {
	MsgType  string            `json:"msgtype"`
	Markdown dingtalkMarkdown  `json:"markdown"`
}

type dingtalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// Send implements Channel.
func (d *DingtalkChannel) Send(ctx context.Context, p Payload) error {
	icon := levelIcon(p.Level)
	text := fmt.Sprintf("### %s %s\n\n%s\n\n[查看详情](%s)", icon, p.Title, p.Body, p.URL)

	req := dingtalkRequest{
		MsgType: "markdown",
		Markdown: dingtalkMarkdown{
			Title: p.Title,
			Text:  text,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("dingtalk: marshal payload: %w", err)
	}

	signedURL, err := d.signURL()
	if err != nil {
		return fmt.Errorf("dingtalk: sign url: %w", err)
	}

	return d.post(ctx, signedURL, data)
}

// signURL appends timestamp + sign query params to the webhook URL.
func (d *DingtalkChannel) signURL() (string, error) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	stringToSign := ts + "\n" + d.cfg.Secret

	mac := hmac.New(sha256.New, []byte(d.cfg.Secret))
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	base := d.cfg.WebhookURL
	separator := "?"
	if len(base) > 0 {
		for _, ch := range base {
			if ch == '?' {
				separator = "&"
				break
			}
		}
	}
	return base + separator + "timestamp=" + ts + "&sign=" + url.QueryEscape(sig), nil
}

func (d *DingtalkChannel) post(ctx context.Context, targetURL string, data []byte) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("dingtalk: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("dingtalk: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dingtalk: unexpected status: %d", resp.StatusCode)
	}
	return nil
}
