package idcd

import (
	"context"
	"net/http"
	"net/url"
)

// GetIPInfo queries IP geolocation for the given IP address or hostname.
func (c *Client) GetIPInfo(ctx context.Context, ip string) (*IPInfo, error) {
	var out IPInfo
	if err := c.do(ctx, http.MethodGet, "/v1/info/ip?q="+url.QueryEscape(ip), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDNS queries DNS records for the given domain and record type.
func (c *Client) GetDNS(ctx context.Context, domain, recordType string) (*DNSResult, error) {
	path := "/v1/info/dns?q=" + url.QueryEscape(domain)
	if recordType != "" {
		path += "&type=" + url.QueryEscape(recordType)
	}
	var out DNSResult
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSSL queries the SSL certificate for the given host.
func (c *Client) GetSSL(ctx context.Context, host string) (*SSLResult, error) {
	var out SSLResult
	if err := c.do(ctx, http.MethodGet, "/v1/info/ssl?q="+url.QueryEscape(host), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetWHOIS queries WHOIS information for the given domain or IP.
func (c *Client) GetWHOIS(ctx context.Context, query string) (*WHOISResult, error) {
	var out WHOISResult
	if err := c.do(ctx, http.MethodGet, "/v1/info/whois?q="+url.QueryEscape(query), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetICP queries ICP filing information for the given domain.
func (c *Client) GetICP(ctx context.Context, domain string) (*ICPResult, error) {
	var out ICPResult
	if err := c.do(ctx, http.MethodGet, "/v1/info/icp?q="+url.QueryEscape(domain), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
