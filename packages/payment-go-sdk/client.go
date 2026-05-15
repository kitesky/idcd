package payment

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// randReader is used for nonce generation; replaceable in tests.
var randReader = rand.Reader

// ClientInterface defines the public API of the payment client.
// Use this interface to mock the client in tests.
type ClientInterface interface {
	CreateOrder(ctx context.Context, req *CreateOrderReq) (*CreateOrderResp, error)
	VerifyReceipt(ctx context.Context, req *VerifyReceiptReq) (*VerifyReceiptResp, error)
	QueryOrder(ctx context.Context, req *QueryOrderReq) (*QueryOrderResp, error)
	CloseOrder(ctx context.Context, req *CloseOrderReq) error
	CreateRefund(ctx context.Context, req *RefundReq) (*RefundResp, error)
	QueryRefund(ctx context.Context, req *QueryRefundReq) (*RefundResp, error)
	QuerySubscription(ctx context.Context, req *QuerySubscriptionReq) (*SubscriptionResp, error)
}

// Ensure Client implements ClientInterface.
var _ ClientInterface = (*Client)(nil)

// Client is the payment platform API client.
// A Client is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  []byte
	httpClient *http.Client
	maxRetries int
	retryDelay time.Duration
}

// New creates a new payment API client.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// apiResponse is the standard API envelope.
type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) nonce() (string, error) {
	b := make([]byte, 16)
	if _, err := io.ReadFull(randReader, b); err != nil {
		return "", fmt.Errorf("payment: generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	if c.apiKey == "" || len(c.apiSecret) == 0 {
		return nil, errors.New("payment: apiKey and apiSecret must be set")
	}

	u := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("payment: create request: %w", err)
	}

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce, err := c.nonce()
	if err != nil {
		return nil, err
	}
	sig := sign(method, path, ts, nonce, body, c.apiSecret)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-Signature", sig)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("payment: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("payment: read response: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("payment: decode response: %w", err)
	}

	if apiResp.Code != 0 {
		return nil, &APIError{Code: apiResp.Code, Message: apiResp.Message}
	}

	return apiResp.Data, nil
}

// isRetryable reports whether an error from do() warrants a retry.
// Only transient network errors and specific API error codes are retried.
// Context cancellation and deterministic client-side errors are not retried.
func isRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == ErrInternalError || apiErr.Code == ErrChannelTimeout
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	data, err := c.do(ctx, method, path, body)
	if err == nil || c.maxRetries <= 0 {
		return data, err
	}

	if !isRetryable(err) {
		return nil, err
	}

	// Retry with exponential backoff.
	delay := c.retryDelay
	if delay <= 0 {
		delay = 500 * time.Millisecond
	}
	for i := 0; i < c.maxRetries; i++ {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		data, err = c.do(ctx, method, path, body)
		if err == nil {
			return data, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		delay *= 2
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
	}
	return nil, err
}

func (c *Client) post(ctx context.Context, path string, reqBody any, respPtr any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("payment: marshal request: %w", err)
	}

	data, err := c.doWithRetry(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}

	return decodeData(data, respPtr)
}

func (c *Client) get(ctx context.Context, path string, params any, respPtr any) error {
	if params != nil {
		q := structToQuery(params)
		if encoded := q.Encode(); encoded != "" {
			path = path + "?" + encoded
		}
	}

	data, err := c.doWithRetry(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}

	return decodeData(data, respPtr)
}

// decodeData unmarshals API response data into respPtr when appropriate.
func decodeData(data []byte, respPtr any) error {
	if respPtr != nil && len(data) > 0 {
		if err := json.Unmarshal(data, respPtr); err != nil {
			return fmt.Errorf("payment: decode data: %w", err)
		}
	}
	return nil
}

// structToQuery converts a struct to url.Values using json tags.
func structToQuery(v any) url.Values {
	q := url.Values{}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return q
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)

		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		tagParts := strings.Split(tag, ",")
		name := tagParts[0]

		if len(tagParts) > 1 && tagParts[1] == "omitempty" && fv.IsZero() {
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			q.Set(name, fv.String())
		case reflect.Int, reflect.Int64:
			q.Set(name, strconv.FormatInt(fv.Int(), 10))
		case reflect.Bool:
			q.Set(name, strconv.FormatBool(fv.Bool()))
		}
	}
	return q
}
