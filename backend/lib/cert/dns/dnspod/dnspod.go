// Package dnspod 实现 dns.Provider 接口，针对腾讯云 DNSPod。
//
// 走 DNSPod Cloud API v3（https://dnspod.tencentcloudapi.com/）直连：
// 跟 cloudflare provider 一样自己封一层薄 client，原因：
//   - lego 的 tencentcloud provider 同样把 fqdn 计算包死在
//     (domain, token, keyAuth) 接口里，我们的 ca.DnsSolver 拿到的是
//     已经 hash 完的 (fqdn, value)，无法反推 keyAuth；
//   - tencentcloud-sdk-go 全套体量大、传递依赖多，我们只需 4 个 action
//     （DescribeDomainList / DescribeRecordList / CreateRecord /
//     DeleteRecord），自封后 httptest.Server mock 完整流程零依赖；
//   - TC3-HMAC-SHA256 签名 ≈ 60 行 crypto 代码，可控。
//
// credential：
//
//	{
//	  "secret_id":  "<腾讯云 SecretId  AKID...>",
//	  "secret_key": "<腾讯云 SecretKey>",
//	}
//
// 不接老的 DNSPod LoginToken（已被官方标记为 legacy）。
package dnspod

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	defaultBaseURL = "https://dnspod.tencentcloudapi.com"
	// DNSPod 通常在国内 DNS 节点 1-3 分钟内传播；3 分钟给 CA validate
	// 一个安全余量。
	defaultPropagationTimeout = 3 * time.Minute
	defaultPollingInterval    = 5 * time.Second
	defaultTTL                = 600

	apiService = "dnspod"
	apiVersion = "2021-03-23"
	apiRegion  = "" // DNSPod is a global service; region is optional.
)

// Config 是 provider 构造选项。零值 Config 走 production 默认。
type Config struct {
	// BaseURL 覆盖默认 dnspod API 地址；仅测试使用。
	BaseURL string
	// HTTPClient 覆盖默认 http.Client；nil 时用 30s 超时的内部默认。
	HTTPClient *http.Client
	// PropagationTimeout 是 solver.Timeout() 返回值；零值用 3min 默认。
	PropagationTimeout time.Duration
	// PollingInterval 给 caller 做参考（lego 通过 challenge.ProviderTimeout
	// 读取）；本包未直接使用，仅保留对外语义。零值用 5s。
	PollingInterval time.Duration
	// TTL 是 CreateRecord 写入的 TTL（秒）；零值用 600。
	TTL int
}

// New 返回一个 DNSPod provider 实例。
func New(cfg Config) dns.Provider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	pt := cfg.PropagationTimeout
	if pt <= 0 {
		pt = defaultPropagationTimeout
	}
	pi := cfg.PollingInterval
	if pi <= 0 {
		pi = defaultPollingInterval
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &dpProvider{
		baseURL:            strings.TrimRight(baseURL, "/"),
		hc:                 hc,
		propagationTimeout: pt,
		pollingInterval:    pi,
		ttl:                ttl,
	}
}

type dpProvider struct {
	baseURL            string
	hc                 *http.Client
	propagationTimeout time.Duration
	pollingInterval    time.Duration
	ttl                int
}

func (p *dpProvider) Kind() dns.ProviderKind { return dns.KindDNSPod }

// ValidateCredential 字段层校验：secret_id / secret_key 非空且长度 >=10。
func (p *dpProvider) ValidateCredential(credential map[string]string) error {
	id, ok := credential["secret_id"]
	if !ok || id == "" {
		return fmt.Errorf("%w: missing secret_id", dns.ErrInvalidCredential)
	}
	if len(id) < 10 {
		return fmt.Errorf("%w: secret_id too short", dns.ErrInvalidCredential)
	}
	key, ok := credential["secret_key"]
	if !ok || key == "" {
		return fmt.Errorf("%w: missing secret_key", dns.ErrInvalidCredential)
	}
	if len(key) < 10 {
		return fmt.Errorf("%w: secret_key too short", dns.ErrInvalidCredential)
	}
	return nil
}

// HealthCheck 通过 DescribeDomainList（Limit=1）验证凭据有效；
// 鉴权错误（AuthFailure*）→ ErrInvalidCredential，5xx → ErrUpstreamUnavailable。
func (p *dpProvider) HealthCheck(ctx context.Context, credential map[string]string) error {
	if err := p.ValidateCredential(credential); err != nil {
		return err
	}
	c := p.newClient(credential["secret_id"], credential["secret_key"])
	req := describeDomainListReq{Limit: 1}
	var resp describeDomainListResp
	if err := c.call(ctx, "DescribeDomainList", req, &resp); err != nil {
		return err
	}
	return nil
}

// BuildSolver 返回一个绑定了凭据的 ca.DnsSolver。
func (p *dpProvider) BuildSolver(_ context.Context, credential map[string]string, _ []string) (ca.DnsSolver, error) {
	if err := p.ValidateCredential(credential); err != nil {
		return nil, err
	}
	return &dpSolver{
		client:  p.newClient(credential["secret_id"], credential["secret_key"]),
		timeout: p.propagationTimeout,
		ttl:     p.ttl,
	}, nil
}

func (p *dpProvider) newClient(id, key string) *dpClient {
	return &dpClient{
		baseURL:   p.baseURL,
		secretID:  id,
		secretKey: key,
		hc:        p.hc,
		now:       time.Now,
	}
}

// ---- solver -----------------------------------------------------------------

type dpSolver struct {
	client  *dpClient
	timeout time.Duration
	ttl     int
}

func (s *dpSolver) Timeout() time.Duration { return s.timeout }

func (s *dpSolver) Present(ctx context.Context, fqdn, value string) error {
	zone, err := s.client.findZone(ctx, fqdn)
	if err != nil {
		return err
	}
	sub, err := extractSubDomain(fqdn, zone)
	if err != nil {
		return err
	}
	return s.client.createTXT(ctx, zone, sub, value, s.ttl)
}

func (s *dpSolver) CleanUp(ctx context.Context, fqdn, value string) error {
	zone, err := s.client.findZone(ctx, fqdn)
	if err != nil {
		return err
	}
	sub, err := extractSubDomain(fqdn, zone)
	if err != nil {
		return err
	}
	ids, err := s.client.listMatchingTXT(ctx, zone, sub, value)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.client.deleteRecord(ctx, zone, id); err != nil {
			return err
		}
	}
	return nil
}

// extractSubDomain 从 fqdn 反推 zone 下的子域，如
// (_acme-challenge.www.example.com., example.com) → _acme-challenge.www。
// 若 fqdn 就等于 zone 本身，返回 "@"（DNSPod 约定）。
func extractSubDomain(fqdn, zone string) (string, error) {
	name := strings.TrimSuffix(fqdn, ".")
	zone = strings.TrimSuffix(zone, ".")
	if name == zone {
		return "@", nil
	}
	suffix := "." + zone
	if !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("%w: fqdn %q not in zone %q", dns.ErrZoneNotFound, fqdn, zone)
	}
	return strings.TrimSuffix(name, suffix), nil
}

// ---- http client ------------------------------------------------------------

type dpClient struct {
	baseURL   string
	secretID  string
	secretKey string
	hc        *http.Client
	now       func() time.Time
}

// call 执行一次 TC3-HMAC-SHA256 签名的 POST，到 dnspod.tencentcloudapi.com/。
// payload 是 action 对应的 request body；out 接收 Response.Response 字段。
func (c *dpClient) call(ctx context.Context, action string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: marshal: %v", dns.ErrInvalidCredential, err)
	}
	host := hostOnly(c.baseURL)
	now := c.now().UTC()
	tsStr := fmt.Sprintf("%d", now.Unix())
	date := now.Format("2006-01-02")

	auth := tc3Sign(tc3SignParams{
		secretID:  c.secretID,
		secretKey: c.secretKey,
		service:   apiService,
		host:      host,
		action:    action,
		body:      body,
		date:      date,
		timestamp: tsStr,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: build req: %v", dns.ErrUpstreamUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", tsStr)
	req.Header.Set("X-TC-Version", apiVersion)
	if apiRegion != "" {
		req.Header.Set("X-TC-Region", apiRegion)
	}
	req.Header.Set("Authorization", auth)

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", dns.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read body: %v", dns.ErrUpstreamUnavailable, err)
	}
	if err := mapHTTPStatus(resp.StatusCode, raw); err != nil {
		return err
	}
	// 业务层错误以 Response.Error 字段返回（HTTP 200 + JSON Error{Code,Message}）。
	var envelope struct {
		Response json.RawMessage `json:"Response"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("%w: decode envelope: %v", dns.ErrUpstreamUnavailable, err)
	}
	var errProbe struct {
		Error *apiError `json:"Error"`
	}
	if err := json.Unmarshal(envelope.Response, &errProbe); err != nil {
		return fmt.Errorf("%w: decode error probe: %v", dns.ErrUpstreamUnavailable, err)
	}
	if errProbe.Error != nil {
		return mapAPIError(errProbe.Error)
	}
	if out != nil {
		if err := json.Unmarshal(envelope.Response, out); err != nil {
			return fmt.Errorf("%w: decode response: %v", dns.ErrUpstreamUnavailable, err)
		}
	}
	return nil
}

type apiError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

// mapHTTPStatus 把 HTTP 状态映射到 dns sentinel。
func mapHTTPStatus(code int, raw []byte) error {
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
		return fmt.Errorf("%w: http %d: %s", dns.ErrUpstreamUnavailable, code, snippet(raw))
	}
}

// mapAPIError 把 Tencent Cloud API 业务错误映射到 dns sentinel。
// AuthFailure.* / UnauthorizedOperation → ErrInvalidCredential
// ResourceNotFound.* / InvalidParameter.DomainNotExists → ErrZoneNotFound
// 其它（含 InternalError）→ ErrUpstreamUnavailable
func mapAPIError(e *apiError) error {
	c := e.Code
	switch {
	case strings.HasPrefix(c, "AuthFailure"), c == "UnauthorizedOperation":
		return fmt.Errorf("%w: %s: %s", dns.ErrInvalidCredential, c, e.Message)
	case strings.HasPrefix(c, "ResourceNotFound"),
		c == "InvalidParameter.DomainNotExists",
		c == "InvalidParameterValue.DomainNotExists":
		return fmt.Errorf("%w: %s: %s", dns.ErrZoneNotFound, c, e.Message)
	default:
		return fmt.Errorf("%w: %s: %s", dns.ErrUpstreamUnavailable, c, e.Message)
	}
}

func snippet(b []byte) string {
	const max = 200
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}

func hostOnly(baseURL string) string {
	s := strings.TrimPrefix(baseURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

// findZone 用 fqdn 反向查 DNSPod 已托管的 zone：从最长 suffix 向上试 apex。
func (c *dpClient) findZone(ctx context.Context, fqdn string) (string, error) {
	name := strings.TrimSuffix(fqdn, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: fqdn %q too short", dns.ErrZoneNotFound, fqdn)
	}
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		req := describeDomainListReq{Limit: 1, Keyword: candidate}
		var resp describeDomainListResp
		if err := c.call(ctx, "DescribeDomainList", req, &resp); err != nil {
			// 401/403 / AuthFailure 直接出错（更短 suffix 同 token 同结果）。
			if errors.Is(err, dns.ErrInvalidCredential) {
				return "", err
			}
			return "", err
		}
		for _, d := range resp.DomainList {
			if d.Name == candidate || d.Punycode == candidate {
				return d.Name, nil
			}
		}
	}
	return "", fmt.Errorf("%w: no zone matches %q", dns.ErrZoneNotFound, fqdn)
}

func (c *dpClient) createTXT(ctx context.Context, zone, sub, value string, ttl int) error {
	req := createRecordReq{
		Domain:     zone,
		SubDomain:  sub,
		RecordType: "TXT",
		RecordLine: "默认",
		Value:      value,
		TTL:        uint64(ttl),
	}
	return c.call(ctx, "CreateRecord", req, nil)
}

func (c *dpClient) listMatchingTXT(ctx context.Context, zone, sub, value string) ([]uint64, error) {
	req := describeRecordListReq{
		Domain:     zone,
		Subdomain:  sub,
		RecordType: "TXT",
		RecordLine: "默认",
	}
	var resp describeRecordListResp
	if err := c.call(ctx, "DescribeRecordList", req, &resp); err != nil {
		// ResourceNotFound.NoDataOfRecord → 视为空列表，不是错误。
		if errors.Is(err, dns.ErrZoneNotFound) && strings.Contains(err.Error(), "NoDataOfRecord") {
			return nil, nil
		}
		return nil, err
	}
	out := make([]uint64, 0, len(resp.RecordList))
	for _, r := range resp.RecordList {
		// DNSPod 不在 value 上加引号，但 cleanup 时谨慎匹配 quoted 形式。
		if r.Value == value || r.Value == "\""+value+"\"" {
			out = append(out, r.RecordId)
		}
	}
	return out, nil
}

func (c *dpClient) deleteRecord(ctx context.Context, zone string, recordID uint64) error {
	req := deleteRecordReq{Domain: zone, RecordId: recordID}
	return c.call(ctx, "DeleteRecord", req, nil)
}

// ---- TC3-HMAC-SHA256 signing ------------------------------------------------
// 文档：https://cloud.tencent.com/document/api/1427/56189

type tc3SignParams struct {
	secretID  string
	secretKey string
	service   string
	host      string
	action    string
	body      []byte
	date      string // YYYY-MM-DD UTC
	timestamp string // unix seconds
}

// tc3Sign 计算 Authorization 头值。
func tc3Sign(p tc3SignParams) string {
	// 1. canonical request
	httpMethod := "POST"
	canonicalURI := "/"
	canonicalQuery := ""
	canonicalHeaders := "content-type:application/json; charset=utf-8\n" +
		"host:" + p.host + "\n" +
		"x-tc-action:" + strings.ToLower(p.action) + "\n"
	signedHeaders := "content-type;host;x-tc-action"
	hashedBody := hashSHA256Hex(p.body)
	canonicalReq := strings.Join([]string{
		httpMethod,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		hashedBody,
	}, "\n")

	// 2. string to sign
	credentialScope := p.date + "/" + p.service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		p.timestamp,
		credentialScope,
		hashSHA256Hex([]byte(canonicalReq)),
	}, "\n")

	// 3. derive signing key
	secretDate := hmacSHA256([]byte("TC3"+p.secretKey), p.date)
	secretService := hmacSHA256(secretDate, p.service)
	secretSigning := hmacSHA256(secretService, "tc3_request")

	// 4. signature
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	return "TC3-HMAC-SHA256 " +
		"Credential=" + p.secretID + "/" + credentialScope + ", " +
		"SignedHeaders=" + signedHeaders + ", " +
		"Signature=" + signature
}

func hashSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, s string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(s))
	return h.Sum(nil)
}

// ---- DNSPod wire types ------------------------------------------------------
//
// 字段命名与 Tencent Cloud DNSPod API v3 一致（PascalCase JSON）。
// 仅列出我们用到的字段。

type describeDomainListReq struct {
	Limit   uint64 `json:"Limit,omitempty"`
	Offset  uint64 `json:"Offset,omitempty"`
	Keyword string `json:"Keyword,omitempty"`
}

type describeDomainListResp struct {
	DomainList []domainListItem `json:"DomainList"`
	RequestId  string           `json:"RequestId"`
}

type domainListItem struct {
	DomainId uint64 `json:"DomainId"`
	Name     string `json:"Name"`
	Punycode string `json:"Punycode"`
}

type createRecordReq struct {
	Domain     string `json:"Domain"`
	SubDomain  string `json:"SubDomain"`
	RecordType string `json:"RecordType"`
	RecordLine string `json:"RecordLine"`
	Value      string `json:"Value"`
	TTL        uint64 `json:"TTL,omitempty"`
}

type describeRecordListReq struct {
	Domain     string `json:"Domain"`
	Subdomain  string `json:"Subdomain,omitempty"`
	RecordType string `json:"RecordType,omitempty"`
	RecordLine string `json:"RecordLine,omitempty"`
}

type describeRecordListResp struct {
	RecordList []recordListItem `json:"RecordList"`
	RequestId  string           `json:"RequestId"`
}

type recordListItem struct {
	RecordId uint64 `json:"RecordId"`
	Value    string `json:"Value"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
}

type deleteRecordReq struct {
	Domain   string `json:"Domain"`
	RecordId uint64 `json:"RecordId"`
}

// compile-time interface checks.
var _ dns.Provider = (*dpProvider)(nil)
var _ ca.DnsSolver = (*dpSolver)(nil)
