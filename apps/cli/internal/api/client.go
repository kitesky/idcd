package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const defaultBaseURL = "https://api.idcd.com"

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	baseURL := os.Getenv("IDCD_API_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, c.baseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr.Error)
		}
		return fmt.Errorf("API error %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

type PingResult struct {
	Node     string `json:"node"`
	Location string `json:"location"`
	LatencyMs int   `json:"latency_ms"`
	Status   string `json:"status"`
}

type PingResponse struct {
	Target  string       `json:"target"`
	Results []PingResult `json:"results"`
}

func (c *Client) Ping(target string) (*PingResponse, error) {
	payload := map[string]string{"target": target}
	var resp PingResponse
	if err := c.do("POST", "/v1/probe/ping", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type HTTPResult struct {
	Node      string `json:"node"`
	Location  string `json:"location"`
	Status    string `json:"status"`
	LatencyMs int    `json:"latency_ms"`
	HTTPCode  int    `json:"http_code"`
	TLS       string `json:"tls"`
}

type HTTPResponse struct {
	URL     string       `json:"url"`
	Results []HTTPResult `json:"results"`
}

func (c *Client) CheckHTTP(url string) (*HTTPResponse, error) {
	payload := map[string]string{"url": url}
	var resp HTTPResponse
	if err := c.do("POST", "/v1/probe/http", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type DNSRecord struct {
	Name  string `json:"name"`
	TTL   int    `json:"ttl"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

type DNSResponse struct {
	Domain  string      `json:"domain"`
	Type    string      `json:"type"`
	Records []DNSRecord `json:"records"`
}

func (c *Client) LookupDNS(domain, recordType string) (*DNSResponse, error) {
	path := fmt.Sprintf("/v1/probe/dns?domain=%s&type=%s", domain, recordType)
	var resp DNSResponse
	if err := c.do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type DiagnoseCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
}

type DiagnoseResponse struct {
	Target string          `json:"target"`
	Checks []DiagnoseCheck `json:"checks"`
}

func (c *Client) Diagnose(target string) (*DiagnoseResponse, error) {
	payload := map[string]string{"target": target}
	var resp DiagnoseResponse
	if err := c.do("POST", "/v1/probe/diagnose", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

type Monitor struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Uptime string `json:"uptime"`
}

type MonitorsResponse struct {
	Monitors []Monitor `json:"monitors"`
}

func (c *Client) ListMonitors() (*MonitorsResponse, error) {
	var resp MonitorsResponse
	if err := c.do("GET", "/v1/monitors", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) PauseMonitor(id string) error {
	return c.do("POST", "/v1/monitors/"+id+"/pause", nil, nil)
}

func (c *Client) ResumeMonitor(id string) error {
	return c.do("POST", "/v1/monitors/"+id+"/resume", nil, nil)
}

func (c *Client) DeleteMonitor(id string) error {
	return c.do("DELETE", "/v1/monitors/"+id, nil, nil)
}

type CreateMonitorRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Target string `json:"target"`
}

func (c *Client) CreateMonitor(name, monType, target string) (*Monitor, error) {
	payload := CreateMonitorRequest{Name: name, Type: monType, Target: target}
	var resp Monitor
	if err := c.do("POST", "/v1/monitors", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
