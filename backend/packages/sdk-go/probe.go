package idcd

import (
	"context"
	"net/http"
)

// ProbeHTTP sends an HTTP probe request.
func (c *Client) ProbeHTTP(ctx context.Context, req ProbeHTTPRequest) (*ProbeResult, error) {
	var out ProbeResult
	if err := c.do(ctx, http.MethodPost, "/v1/probe/http", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbePing sends a Ping probe request.
func (c *Client) ProbePing(ctx context.Context, req ProbePingRequest) (*ProbeResult, error) {
	var out ProbeResult
	if err := c.do(ctx, http.MethodPost, "/v1/probe/ping", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeDNS sends a DNS probe request.
func (c *Client) ProbeDNS(ctx context.Context, req ProbeDNSRequest) (*ProbeResult, error) {
	var out ProbeResult
	if err := c.do(ctx, http.MethodPost, "/v1/probe/dns", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeTCP sends a TCP probe request.
func (c *Client) ProbeTCP(ctx context.Context, req ProbeTCPRequest) (*ProbeResult, error) {
	var out ProbeResult
	if err := c.do(ctx, http.MethodPost, "/v1/probe/tcp", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProbeTraceroute sends a Traceroute probe request.
func (c *Client) ProbeTraceroute(ctx context.Context, req ProbeTracerouteRequest) (*ProbeResult, error) {
	var out ProbeResult
	if err := c.do(ctx, http.MethodPost, "/v1/probe/traceroute", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Diagnose runs a full diagnosis (http+ping+dns+tcp+traceroute) on a target.
func (c *Client) Diagnose(ctx context.Context, target string) (*DiagnoseResult, error) {
	req := ProbeRequest{Target: target}
	var out DiagnoseResult
	if err := c.do(ctx, http.MethodPost, "/v1/diagnose", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
