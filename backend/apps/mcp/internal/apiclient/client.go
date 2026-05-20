package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Tunables for the shared HTTP transport. The MCP server is a long-lived
// process that repeatedly calls the same idcd API host on behalf of many
// concurrent tool calls; the Go default of MaxIdleConnsPerHost=2 starves
// connection reuse and forces TLS handshakes on most requests. The values
// below are sized for a single-host workload (≤ ~50 concurrent in-flight
// tool calls) and keep an idle pool warm for 90s between bursts.
const (
	transportMaxIdleConns        = 100
	transportMaxIdleConnsPerHost = 20
	transportIdleConnTimeout     = 90 * time.Second
	transportTLSHandshakeTimeout = 10 * time.Second
	transportExpectContinue      = 1 * time.Second
	httpClientTimeout            = 15 * time.Second
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	transport  *http.Transport
}

// newTransport builds the *http.Transport used by Client. Exposed as a
// package-level helper so tests can reuse the exact construction the
// production constructor uses, without depending on private fields.
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          transportMaxIdleConns,
		MaxIdleConnsPerHost:   transportMaxIdleConnsPerHost,
		IdleConnTimeout:       transportIdleConnTimeout,
		TLSHandshakeTimeout:   transportTLSHandshakeTimeout,
		ExpectContinueTimeout: transportExpectContinue,
		ForceAttemptHTTP2:     true,
	}
}

func New(baseURL, apiKey string) *Client {
	tr := newTransport()
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		transport: tr,
		httpClient: &http.Client{
			Timeout:   httpClientTimeout,
			Transport: tr,
		},
	}
}

// Transport returns the underlying *http.Transport. Exported primarily so
// tests can assert the connection-pool tunables are wired correctly; callers
// SHOULD NOT mutate the returned transport — its settings are tuned for the
// idcd API workload and racy mutation will desync from the http.Client.
func (c *Client) Transport() *http.Transport {
	return c.transport
}

// Close releases idle HTTP connections held by the underlying transport.
// Safe to call multiple times. Intended for graceful shutdown — after Close
// the Client can still be used, but the next request will pay a fresh
// connection setup cost.
func (c *Client) Close() {
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

func (c *Client) HasAPIKey() bool {
	return c.apiKey != ""
}

func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.do(req, out)
}

func (c *Client) Get(ctx context.Context, path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && (apiErr.Error != "" || apiErr.Message != "") {
			msg := apiErr.Error
			if msg == "" {
				msg = apiErr.Message
			}
			return fmt.Errorf("API error %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("API error %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
