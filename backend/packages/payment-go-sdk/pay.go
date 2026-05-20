package payment

import "context"

// CreateOrder creates a new payment order.
func (c *Client) CreateOrder(ctx context.Context, req *CreateOrderReq) (*CreateOrderResp, error) {
	var resp CreateOrderResp
	if err := c.post(ctx, "/api/v1/pay/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// VerifyReceipt verifies a receipt from Apple IAP or Google Play.
func (c *Client) VerifyReceipt(ctx context.Context, req *VerifyReceiptReq) (*VerifyReceiptResp, error) {
	var resp VerifyReceiptResp
	if err := c.post(ctx, "/api/v1/pay/verify", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryOrder queries an order by order number or app order ID.
func (c *Client) QueryOrder(ctx context.Context, req *QueryOrderReq) (*QueryOrderResp, error) {
	var resp QueryOrderResp
	if err := c.get(ctx, "/api/v1/pay/query", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CloseOrder closes a pending order by order number or app order ID.
func (c *Client) CloseOrder(ctx context.Context, req *CloseOrderReq) error {
	return c.post(ctx, "/api/v1/pay/close", req, nil)
}
