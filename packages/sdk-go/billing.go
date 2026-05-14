package idcd

import (
	"context"
	"net/http"
)

// Subscribe initiates a subscription to a plan and returns the payment URL.
func (c *Client) Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResult, error) {
	var out SubscribeResult
	if err := c.do(ctx, http.MethodPost, "/v1/billing/subscribe", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelSubscription cancels the user's active subscription.
func (c *Client) CancelSubscription(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/v1/billing/cancel", nil, nil)
}

// GetSubscription returns the user's current subscription.
func (c *Client) GetSubscription(ctx context.Context) (*Subscription, error) {
	var out Subscription
	if err := c.do(ctx, http.MethodGet, "/v1/billing/subscription", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListInvoices returns paginated invoices for the authenticated user.
// Pass page=0 and pageSize=0 to use server defaults (page 1, 20 items).
func (c *Client) ListInvoices(ctx context.Context, page, pageSize int) (*InvoiceList, error) {
	path := "/v1/billing/invoices"
	sep := "?"
	if page > 0 {
		path += sep + "page=" + itoa(page)
		sep = "&"
	}
	if pageSize > 0 {
		path += sep + "page_size=" + itoa(pageSize)
	}
	var out InvoiceList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// itoa converts an int to a string without importing strconv at the package level.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}
