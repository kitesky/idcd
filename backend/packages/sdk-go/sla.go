package idcd

import (
	"context"
	"net/http"
)

// GetSLAReport returns the SLA monthly report for the authenticated user.
// months controls the lookback window (1-12); pass 0 to use the server default (3).
func (c *Client) GetSLAReport(ctx context.Context, months int) (*SLAReport, error) {
	path := "/v1/reports/sla"
	if months > 0 {
		path += "?months=" + itoa(months)
	}
	var out SLAReport
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
