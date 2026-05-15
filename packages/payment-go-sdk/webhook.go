package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	maxWebhookBodySize = 1 << 20 // 1 MB
	webhookTolerance   = 300      // 5 minutes in seconds
)

// Sentinel errors returned by WebhookHandler.Parse; callers can use errors.Is().
var (
	ErrWebhookTimestampExpired = errors.New("payment: webhook: timestamp expired")
	ErrWebhookInvalidSignature = errors.New("payment: webhook: invalid signature")
)

// WebhookHandler verifies and parses incoming webhook notifications.
//
// Example with Gin:
//
//	wh := payment.NewWebhookHandler("your-callback-secret")
//
//	r.POST("/webhook/payment", func(c *gin.Context) {
//	    event, err := wh.Parse(c.Request)
//	    if err != nil {
//	        c.JSON(401, gin.H{"error": err.Error()})
//	        return
//	    }
//	    switch event.EventType {
//	    case payment.EventOrderPaid:
//	        order, _ := event.OrderData()
//	        // handle paid order...
//	    }
//	    c.JSON(200, gin.H{"status": "ok"})
//	})
type WebhookHandler struct {
	secret []byte
}

// NewWebhookHandler creates a webhook handler with the given callback secret.
func NewWebhookHandler(callbackSecret string) *WebhookHandler {
	return &WebhookHandler{secret: []byte(callbackSecret)}
}

// Parse reads the request, verifies the signature, and returns the event.
func (wh *WebhookHandler) Parse(r *http.Request) (*WebhookEvent, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		return nil, fmt.Errorf("payment: webhook: read body: %w", err)
	}

	timestamp := r.Header.Get("X-Webhook-Timestamp")
	signature := r.Header.Get("X-Webhook-Signature")
	if err := wh.verify(timestamp, body, signature); err != nil {
		return nil, err
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("payment: webhook: invalid timestamp: %w", err)
	}
	now := time.Now().Unix()
	if now < ts-webhookTolerance || now > ts+webhookTolerance {
		return nil, ErrWebhookTimestampExpired
	}

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("payment: webhook: parse event: %w", err)
	}
	event.RawBody = body
	return &event, nil
}

func (wh *WebhookHandler) verify(timestamp string, body []byte, signature string) error {
	mac := hmac.New(sha256.New, wh.secret)
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return ErrWebhookInvalidSignature
	}
	return nil
}
