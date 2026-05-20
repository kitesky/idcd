package idcd

import (
	"context"
	"net/http"
)

// CreateAlertChannel creates a new alert notification channel.
func (c *Client) CreateAlertChannel(ctx context.Context, req CreateAlertChannelRequest) (*AlertChannel, error) {
	var out AlertChannel
	if err := c.do(ctx, http.MethodPost, "/v1/alert-channels", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAlertChannels returns all alert channels for the authenticated user.
func (c *Client) ListAlertChannels(ctx context.Context) ([]AlertChannel, error) {
	var out AlertChannelList
	if err := c.do(ctx, http.MethodGet, "/v1/alert-channels", nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// DeleteAlertChannel deletes an alert channel by ID.
func (c *Client) DeleteAlertChannel(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/alert-channels/"+id, nil, nil)
}

// TestAlertChannel sends a test notification through a channel.
func (c *Client) TestAlertChannel(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/alert-channels/"+id+"/test", nil, nil)
}

// CreateAlertPolicy creates a new alert policy.
func (c *Client) CreateAlertPolicy(ctx context.Context, req CreateAlertPolicyRequest) (*AlertPolicy, error) {
	var out AlertPolicy
	if err := c.do(ctx, http.MethodPost, "/v1/alert-policies", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAlertPolicies returns all alert policies for the authenticated user.
// Pass monitorID to filter by monitor; pass an empty string to list all.
func (c *Client) ListAlertPolicies(ctx context.Context, monitorID string) ([]AlertPolicy, error) {
	path := "/v1/alert-policies"
	if monitorID != "" {
		path += "?monitor_id=" + monitorID
	}
	var out AlertPolicyList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// UpdateAlertPolicy partially updates an alert policy.
func (c *Client) UpdateAlertPolicy(ctx context.Context, id string, req UpdateAlertPolicyRequest) (*AlertPolicy, error) {
	var out AlertPolicy
	if err := c.do(ctx, http.MethodPatch, "/v1/alert-policies/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteAlertPolicy deletes an alert policy by ID.
func (c *Client) DeleteAlertPolicy(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/alert-policies/"+id, nil, nil)
}

// ListAlertEvents returns alert events for the authenticated user.
// Pass monitorID or status to filter; pass empty strings to list all.
func (c *Client) ListAlertEvents(ctx context.Context, monitorID, status string) ([]AlertEvent, error) {
	path := "/v1/alert-events"
	sep := "?"
	if monitorID != "" {
		path += sep + "monitor_id=" + monitorID
		sep = "&"
	}
	if status != "" {
		path += sep + "status=" + status
	}
	var out AlertEventList
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// AcknowledgeAlertEvent acknowledges an alert event.
func (c *Client) AcknowledgeAlertEvent(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/alert-events/"+id+"/ack", nil, nil)
}
