package idcd

import (
	"context"
	"fmt"
	"net/http"
)

// ListMonitors returns all monitors for the authenticated user.
func (c *Client) ListMonitors(ctx context.Context) ([]Monitor, error) {
	var out MonitorList
	if err := c.do(ctx, http.MethodGet, "/v1/monitors", nil, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

// CreateMonitor creates a new monitor.
func (c *Client) CreateMonitor(ctx context.Context, req CreateMonitorRequest) (*Monitor, error) {
	var out Monitor
	if err := c.do(ctx, http.MethodPost, "/v1/monitors", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMonitor retrieves a monitor by ID.
func (c *Client) GetMonitor(ctx context.Context, id string) (*Monitor, error) {
	var out Monitor
	if err := c.do(ctx, http.MethodGet, "/v1/monitors/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateMonitor partially updates a monitor.
func (c *Client) UpdateMonitor(ctx context.Context, id string, req UpdateMonitorRequest) (*Monitor, error) {
	var out Monitor
	if err := c.do(ctx, http.MethodPatch, "/v1/monitors/"+id, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMonitor deletes (archives) a monitor.
func (c *Client) DeleteMonitor(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/monitors/"+id, nil, nil)
}

// PauseMonitor pauses a monitor.
func (c *Client) PauseMonitor(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/monitors/"+id+"/pause", nil, nil)
}

// ResumeMonitor resumes a paused monitor.
func (c *Client) ResumeMonitor(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/v1/monitors/"+id+"/resume", nil, nil)
}

// BulkMonitorAction performs a bulk action (pause/resume/delete) on multiple monitors.
func (c *Client) BulkMonitorAction(ctx context.Context, ids []string, action string) (*BulkResult, error) {
	body := struct {
		IDs    []string `json:"ids"`
		Action string   `json:"action"`
	}{IDs: ids, Action: action}
	var out BulkResult
	if err := c.do(ctx, http.MethodPost, "/v1/monitors/bulk", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMonitorChecks returns the check history for a monitor.
// hours controls the time window (max 168); pass 0 to use the server default (24h).
func (c *Client) GetMonitorChecks(ctx context.Context, id string, hours int) (*MonitorChecks, error) {
	path := "/v1/monitors/" + id + "/checks"
	if hours > 0 {
		path += fmt.Sprintf("?hours=%d", hours)
	}
	var out MonitorChecks
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMonitorBaseline returns the anchor baseline for a monitor.
func (c *Client) GetMonitorBaseline(ctx context.Context, id string) (*MonitorBaseline, error) {
	var out MonitorBaseline
	if err := c.do(ctx, http.MethodGet, "/v1/monitors/"+id+"/baseline", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
