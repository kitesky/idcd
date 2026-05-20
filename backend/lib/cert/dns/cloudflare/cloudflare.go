// Package cloudflare 实现 dns.Provider 接口，针对 Cloudflare DNS。
//
// 走 Cloudflare API v4 (HTTPS REST) 直连：自己封一层薄 client，原因：
//   - lego 的 cloudflare provider 把 fqdn 计算包死在 (domain, token, keyAuth)
//     接口里，与本包反推（fqdn → domain）不可逆（keyAuth 内部再 sha256）；
//   - cloudflare-go SDK 体量大、依赖多，S1 只用三个端点不划算；
//   - 自封后 httptest.Server mock 完整 Present/CleanUp 流程零依赖。
//
// credential：
//
//	{"api_token": "<scoped Zone:DNS:Edit + Zone:Zone:Read 的 token>"}
//
// 不接 Global API Key（已不安全）。
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	defaultBaseURL = "https://api.cloudflare.com/client/v4"
	// Cloudflare DNS 传播极快（边缘节点几乎实时）；2 分钟给 CA validate
	// 一个安全余量。
	defaultPropagationTimeout = 120 * time.Second
)

// Config 是 provider 构造选项。零值 Config 走 production。
type Config struct {
	// BaseURL 覆盖默认 cloudflare API 地址；仅测试使用。
	BaseURL string
	// HTTPClient 覆盖默认 http.Client；nil 时用 30s 超时的内部默认。
	HTTPClient *http.Client
}

// New 返回一个 Cloudflare provider 实例。
func New(cfg Config) dns.Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &cfProvider{baseURL: strings.TrimRight(baseURL, "/"), hc: hc}
}

type cfProvider struct {
	baseURL string
	hc      *http.Client
}

func (p *cfProvider) Kind() dns.ProviderKind { return dns.KindCloudflare }

// ValidateCredential 字段层校验：只看 api_token 长度。
func (p *cfProvider) ValidateCredential(credential map[string]string) error {
	tok, ok := credential["api_token"]
	if !ok || tok == "" {
		return fmt.Errorf("%w: missing api_token", dns.ErrInvalidCredential)
	}
	if len(tok) < 20 {
		return fmt.Errorf("%w: api_token too short", dns.ErrInvalidCredential)
	}
	return nil
}

// HealthCheck 调 /user/tokens/verify 验证 token 有效且未撤销。
func (p *cfProvider) HealthCheck(ctx context.Context, credential map[string]string) error {
	if err := p.ValidateCredential(credential); err != nil {
		return err
	}
	c := p.newClient(credential["api_token"])
	var resp tokenVerifyResp
	if err := c.doJSON(ctx, http.MethodGet, "/user/tokens/verify", nil, &resp); err != nil {
		return err
	}
	if !resp.Success || resp.Result.Status != "active" {
		return fmt.Errorf("%w: token status=%q", dns.ErrInvalidCredential, resp.Result.Status)
	}
	return nil
}

// BuildSolver 返回一个绑定了 token 的 ca.DnsSolver。
func (p *cfProvider) BuildSolver(_ context.Context, credential map[string]string, _ []string) (ca.DnsSolver, error) {
	if err := p.ValidateCredential(credential); err != nil {
		return nil, err
	}
	return &cfSolver{
		client:  p.newClient(credential["api_token"]),
		timeout: defaultPropagationTimeout,
	}, nil
}

func (p *cfProvider) newClient(token string) *cfClient {
	return &cfClient{
		baseURL: p.baseURL,
		token:   token,
		hc:      p.hc,
	}
}

// ---- solver -----------------------------------------------------------------

type cfSolver struct {
	client  *cfClient
	timeout time.Duration
}

func (s *cfSolver) Timeout() time.Duration { return s.timeout }

func (s *cfSolver) Present(ctx context.Context, fqdn, value string) error {
	zoneID, err := s.client.findZoneID(ctx, fqdn)
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(fqdn, ".")
	rec := dnsRecord{
		Type:    "TXT",
		Name:    name,
		Content: value,
		TTL:     60,
	}
	return s.client.createTXT(ctx, zoneID, rec)
}

func (s *cfSolver) CleanUp(ctx context.Context, fqdn, value string) error {
	zoneID, err := s.client.findZoneID(ctx, fqdn)
	if err != nil {
		// CleanUp 找不到 zone 不应阻塞 worker；但仍包成 dns 错误便于排查。
		return err
	}
	name := strings.TrimSuffix(fqdn, ".")
	ids, err := s.client.listMatchingTXT(ctx, zoneID, name, value)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.client.deleteRecord(ctx, zoneID, id); err != nil {
			return err
		}
	}
	return nil
}

// ---- http client ------------------------------------------------------------

type cfClient struct {
	baseURL string
	token   string
	hc      *http.Client
}

func (c *cfClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%w: marshal: %v", dns.ErrInvalidCredential, err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return fmt.Errorf("%w: build req: %v", dns.ErrUpstreamUnavailable, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", dns.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", dns.ErrUpstreamUnavailable, err)
	}
	if err := mapStatus(resp.StatusCode, raw); err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: decode: %v", dns.ErrUpstreamUnavailable, err)
		}
	}
	return nil
}

// mapStatus 把 HTTP 状态映射到 dns sentinel。
func mapStatus(code int, raw []byte) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized, code == http.StatusForbidden:
		return fmt.Errorf("%w: http %d: %s", dns.ErrInvalidCredential, code, snippet(raw))
	case code == http.StatusNotFound:
		return fmt.Errorf("%w: http %d", dns.ErrZoneNotFound, code)
	case code >= 500:
		return fmt.Errorf("%w: http %d", dns.ErrUpstreamUnavailable, code)
	default:
		// 400 等等：可能是 token 缺权限 / record 重复，归到 upstream 让 worker 重试看。
		return fmt.Errorf("%w: http %d: %s", dns.ErrUpstreamUnavailable, code, snippet(raw))
	}
}

func snippet(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

// findZoneID 用 fqdn 反向查 zone：cloudflare 的 /zones?name= 接受 apex
// domain，所以从最长 suffix 向上试。
func (c *cfClient) findZoneID(ctx context.Context, fqdn string) (string, error) {
	name := strings.TrimSuffix(fqdn, ".")
	// 去掉 _acme-challenge. 前缀（如果有），从余下域名向上试 apex。
	name = strings.TrimPrefix(name, "_acme-challenge.")
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: fqdn %q too short", dns.ErrZoneNotFound, fqdn)
	}
	// 从最具体（去掉头一段）开始向上找：example.com、然后 com（最后会失败）。
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		var resp zonesListResp
		if err := c.doJSON(ctx, http.MethodGet, "/zones?name="+candidate, nil, &resp); err != nil {
			// 401/403 直接出错；其它继续尝试更短 suffix 没有意义（同 token 同结果）。
			if errors.Is(err, dns.ErrInvalidCredential) {
				return "", err
			}
			return "", err
		}
		if resp.Success && len(resp.Result) > 0 {
			return resp.Result[0].ID, nil
		}
	}
	return "", fmt.Errorf("%w: no zone matches %q", dns.ErrZoneNotFound, fqdn)
}

func (c *cfClient) createTXT(ctx context.Context, zoneID string, rec dnsRecord) error {
	var resp dnsRecordResp
	return c.doJSON(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", rec, &resp)
}

func (c *cfClient) listMatchingTXT(ctx context.Context, zoneID, name, content string) ([]string, error) {
	var resp dnsRecordListResp
	path := fmt.Sprintf("/zones/%s/dns_records?type=TXT&name=%s", zoneID, name)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(resp.Result))
	for _, r := range resp.Result {
		// content 在 cloudflare 响应里有可能带双引号包裹；都比一下。
		if r.Content == content || r.Content == "\""+content+"\"" {
			out = append(out, r.ID)
		}
	}
	return out, nil
}

func (c *cfClient) deleteRecord(ctx context.Context, zoneID, recordID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+recordID, nil, nil)
}

// ---- cloudflare wire types ---------------------------------------------------

type dnsRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type dnsRecordResp struct {
	Success bool      `json:"success"`
	Result  dnsRecord `json:"result"`
}

type dnsRecordListItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type dnsRecordListResp struct {
	Success bool                `json:"success"`
	Result  []dnsRecordListItem `json:"result"`
}

type zonesListResp struct {
	Success bool `json:"success"`
	Result  []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"result"`
}

type tokenVerifyResp struct {
	Success bool `json:"success"`
	Result  struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"result"`
}

// compile-time interface check.
var _ dns.Provider = (*cfProvider)(nil)
var _ ca.DnsSolver = (*cfSolver)(nil)
