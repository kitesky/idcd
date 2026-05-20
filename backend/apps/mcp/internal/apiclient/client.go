package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Tunables for the shared HTTP transport. The MCP server is a long-lived
// process that repeatedly calls the same idcd API host on behalf of many
// concurrent tool calls; the Go default of MaxIdleConnsPerHost=2 starves
// connection reuse and forces TLS handshakes on most requests. The values
// below are sized for a single-host workload (≤ ~50 concurrent in-flight
// tool calls) and keep an idle pool warm for 90s between bursts.
const (
	transportMaxIdleConns        = 100
	transportMaxIdleConnsPerHost = 20
	transportIdleConnTimeout     = 90 * time.Second
	transportTLSHandshakeTimeout = 10 * time.Second
	transportExpectContinue      = 1 * time.Second
	httpClientTimeout            = 15 * time.Second
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	transport  *http.Transport
}

// newTransport builds the *http.Transport used by Client. Exposed as a
// package-level helper so tests can reuse the exact construction the
// production constructor uses, without depending on private fields.
func newTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          transportMaxIdleConns,
		MaxIdleConnsPerHost:   transportMaxIdleConnsPerHost,
		IdleConnTimeout:       transportIdleConnTimeout,
		TLSHandshakeTimeout:   transportTLSHandshakeTimeout,
		ExpectContinueTimeout: transportExpectContinue,
		ForceAttemptHTTP2:     true,
	}
}

func New(baseURL, apiKey string) *Client {
	tr := newTransport()
	return &Client{
		baseURL:   baseURL,
		apiKey:    apiKey,
		transport: tr,
		httpClient: &http.Client{
			Timeout:   httpClientTimeout,
			Transport: tr,
		},
	}
}

// Transport returns the underlying *http.Transport. Exported primarily so
// tests can assert the connection-pool tunables are wired correctly; callers
// SHOULD NOT mutate the returned transport — its settings are tuned for the
// idcd API workload and racy mutation will desync from the http.Client.
func (c *Client) Transport() *http.Transport {
	return c.transport
}

// Close releases idle HTTP connections held by the underlying transport.
// Safe to call multiple times. Intended for graceful shutdown — after Close
// the Client can still be used, but the next request will pay a fresh
// connection setup cost.
func (c *Client) Close() {
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

func (c *Client) HasAPIKey() bool {
	return c.apiKey != ""
}

func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.do(req, out)
}

func (c *Client) Get(ctx context.Context, path string, params url.Values, out any) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// api 错误信封是 {"error":{"code":"...","message":"...","request_id":"..."},"request_id":"..."}
		// 老结构 {error:string, message:string} 一并兼容,避免上游变更时 mcp 端突然瞎说"unknown"
		var coded struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &coded); jsonErr == nil && coded.Error.Message != "" {
			return fmt.Errorf("API error %d [%s]: %s", resp.StatusCode, coded.Error.Code, coded.Error.Message)
		}
		var flat struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(body, &flat); jsonErr == nil && (flat.Error != "" || flat.Message != "") {
			msg := flat.Error
			if msg == "" {
				msg = flat.Message
			}
			return fmt.Errorf("API error %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("API error %d", resp.StatusCode)
	}

	if out != nil {
		// api 成功信封是 {"data": <payload>, "request_id": "..."}
		// 先剥 data 再二次 unmarshal 进 out;若响应不带 data wrapper(老接口或第三方),
		// 退回直接 unmarshal 整个 body — 兼容性双跑道。
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
			if err := json.Unmarshal(envelope.Data, out); err != nil {
				return fmt.Errorf("unmarshal data: %w", err)
			}
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}

// PollProbeTask 走 api 的 async probe contract:POST /v1/probe/<type> 拿到 task_id,
// 再 GET /v1/probe/tasks/<task_id> 轮询直到 status ∈ {completed, done, failed},
// 把 result 字段 unmarshal 进 out。
//
// 设计上 mcp 工具上层不关心 task_id — 用户视角是"调用 ping 拿结果",所以这层
// 同步语义封装在 apiclient 内,工具调一次就阻塞到结果回来或超时。
//
// 默认每 1s 轮询一次,最长等 30s(同 t1-poll 前端约定)。超时返回 ErrProbeTimeout
// 让工具层渲染"任务超时,拨测节点未及时回报"。
func (c *Client) PollProbeTask(ctx context.Context, postPath string, body any, out any) error {
	var postResp struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	if err := c.Post(ctx, postPath, body, &postResp); err != nil {
		return fmt.Errorf("submit probe: %w", err)
	}
	if postResp.TaskID == "" {
		return fmt.Errorf("submit probe: empty task_id")
	}

	taskPath := "/v1/probe/tasks/" + postResp.TaskID
	deadline := time.Now().Add(probePollMaxWait)
	interval := probePollInterval

	for {
		var taskResp struct {
			TaskID      string          `json:"task_id"`
			Status      string          `json:"status"`
			Result      json.RawMessage `json:"result"`
			CompletedAt *time.Time      `json:"completed_at"`
		}
		if err := c.Get(ctx, taskPath, nil, &taskResp); err != nil {
			return fmt.Errorf("poll task: %w", err)
		}
		switch taskResp.Status {
		case "completed", "done":
			if len(taskResp.Result) == 0 {
				return fmt.Errorf("task %s reported completed but result is empty", postResp.TaskID)
			}
			if out != nil {
				if err := json.Unmarshal(taskResp.Result, out); err != nil {
					return fmt.Errorf("unmarshal result: %w", err)
				}
			}
			return nil
		case "failed":
			return fmt.Errorf("task %s failed", postResp.TaskID)
		}
		// queued / running / dispatched — wait & retry
		if time.Now().After(deadline) {
			return ErrProbeTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// ErrProbeTimeout 用 errors.Is 让工具层识别超时(渲染"节点没及时回报"而非 panic)
var ErrProbeTimeout = errors.New("probe task timed out before completion")

// 轮询节奏 — 30 秒兜底涵盖 ping/http(~5s) + 多跳 traceroute(~15-20s) + agent 排队抖动。
// 调短会让 traceroute 频繁误报超时;调长会让前端用户感觉 hang。
const (
	probePollInterval = 1 * time.Second
	probePollMaxWait  = 30 * time.Second
)
