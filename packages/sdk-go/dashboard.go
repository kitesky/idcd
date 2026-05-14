package idcd

import (
	"context"
	"net/http"
)

// GetDashboardSummary returns the dashboard summary for the authenticated user.
func (c *Client) GetDashboardSummary(ctx context.Context) (*DashboardSummary, error) {
	var out DashboardSummary
	if err := c.do(ctx, http.MethodGet, "/v1/dashboard/summary", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDashboardPins returns the pinned monitor IDs for the authenticated user.
func (c *Client) GetDashboardPins(ctx context.Context) (*DashboardPins, error) {
	var out DashboardPins
	if err := c.do(ctx, http.MethodGet, "/v1/dashboard/pins", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDashboardPins updates the pinned monitors for the authenticated user.
func (c *Client) UpdateDashboardPins(ctx context.Context, req UpdatePinsRequest) (*DashboardPins, error) {
	var out DashboardPins
	if err := c.do(ctx, http.MethodPut, "/v1/dashboard/pins", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
