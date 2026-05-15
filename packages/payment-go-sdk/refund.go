package payment

import "context"

// CreateRefund creates a refund for an order.
func (c *Client) CreateRefund(ctx context.Context, req *RefundReq) (*RefundResp, error) {
	var resp RefundResp
	if err := c.post(ctx, "/api/v1/refund/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// QueryRefund queries a refund by refund number or app refund ID.
func (c *Client) QueryRefund(ctx context.Context, req *QueryRefundReq) (*RefundResp, error) {
	var resp RefundResp
	if err := c.get(ctx, "/api/v1/refund/query", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
