package payment

import "context"

// QuerySubscription queries subscriptions by app user ID, original transaction ID,
// product ID, or status.
func (c *Client) QuerySubscription(ctx context.Context, req *QuerySubscriptionReq) (*SubscriptionResp, error) {
	var resp SubscriptionResp
	if err := c.get(ctx, "/api/v1/subscription/query", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
