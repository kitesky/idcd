package payment

import (
	"net/http"
	"time"
)

// Option configures the Client.
type Option func(*Client)

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithAPISecret sets the API secret for signing requests.
func WithAPISecret(secret string) Option {
	return func(c *Client) {
		c.apiSecret = []byte(secret)
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient sets a custom HTTP client.
// If hc.Timeout is zero, it is set to the timeout previously configured via
// WithTimeout. This modifies hc in place; callers that share hc elsewhere
// will observe the updated timeout.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc.Timeout == 0 {
			hc.Timeout = c.httpClient.Timeout
		}
		c.httpClient = hc
	}
}

// WithRetry configures the client to retry failed idempotent requests.
// maxRetries is the maximum number of retries (0 means no retry).
// baseDelay is the initial delay between retries (exponential backoff).
func WithRetry(maxRetries int, baseDelay time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryDelay = baseDelay
	}
}
