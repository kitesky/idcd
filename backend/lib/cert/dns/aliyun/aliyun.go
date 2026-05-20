// Package aliyun 实现 dns.Provider 接口，针对阿里云 DNS（alidns）。
//
// 桥接策略：
//   - 凭据校验复用 lego 的 alidns.NewDNSProviderConfig（参数完整性检查走
//     lego，避免重复实现 AccessKey/SecretKey/RegionID 三者的组合规则）；
//   - 写记录（Present/CleanUp）和 HealthCheck 实际调用直接走阿里云
//     alidns-20150109/v4 SDK——这与 lego 内部用的 SDK 一致，但绕开 lego
//     的 challenge.Provider 接口（其 Present(domain, token, keyAuth) 把
//     value 强行从 sha256(keyAuth) 计算，无法用我们已有的 (fqdn, value)
//     直接调用）。
//
// 凭据字段：
//
//	{
//	  "access_key_id":     "<AK ID, 必填>",
//	  "access_key_secret": "<AK Secret, 必填>",
//	  "region_id":         "cn-hangzhou (默认) 或其它"
//	}
package aliyun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/dara"
	teatea "github.com/alibabacloud-go/tea/tea"
	alidnssdk "github.com/go-acme/alidns-20150109/v4/client"
	legoalidns "github.com/go-acme/lego/v4/providers/dns/alidns"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	defaultRegionID           = "cn-hangzhou"
	defaultPropagationTimeout = 3 * time.Minute
	defaultPollingInterval    = 5 * time.Second
	// alidns 免费版最低 600s；用户付费版可降。S2 用默认 600s。
	defaultTTL = 600
	// alidns SDK 调用使用的 HTTP 读超时。
	defaultHTTPTimeout = 10 * time.Second

	minKeyLength = 10
)

// Config 是 provider 构造选项。零值 Config 走 production 默认。
type Config struct {
	// PropagationTimeout caps the DNS-01 wait. Defaults to 3 minutes.
	PropagationTimeout time.Duration
	// PollingInterval is how often to re-check propagation. Defaults to 5s.
	PollingInterval time.Duration
	// TTL for created TXT records. Defaults to 600s (Aliyun minimum for free tier).
	TTL int

	// newClient 注入 client 工厂；nil 时用 production realClient。测试用。
	newClient func(cred map[string]string) (txtClient, error)
}

// New 返回一个 Aliyun DNS provider 实例。
func New(cfg Config) dns.Provider {
	if cfg.PropagationTimeout <= 0 {
		cfg.PropagationTimeout = defaultPropagationTimeout
	}
	if cfg.PollingInterval <= 0 {
		cfg.PollingInterval = defaultPollingInterval
	}
	if cfg.TTL <= 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.newClient == nil {
		cfg.newClient = realClientFactory
	}
	return &aliProvider{cfg: cfg}
}

type aliProvider struct {
	cfg Config
}

func (p *aliProvider) Kind() dns.ProviderKind { return dns.KindAliyun }

// ValidateCredential 字段层校验：access_key_id / access_key_secret 必填，长度 >= 10。
func (p *aliProvider) ValidateCredential(cred map[string]string) error {
	ak, ok := cred["access_key_id"]
	if !ok || strings.TrimSpace(ak) == "" {
		return fmt.Errorf("%w: missing access_key_id", dns.ErrInvalidCredential)
	}
	if len(ak) < minKeyLength {
		return fmt.Errorf("%w: access_key_id too short (<%d)", dns.ErrInvalidCredential, minKeyLength)
	}
	sk, ok := cred["access_key_secret"]
	if !ok || strings.TrimSpace(sk) == "" {
		return fmt.Errorf("%w: missing access_key_secret", dns.ErrInvalidCredential)
	}
	if len(sk) < minKeyLength {
		return fmt.Errorf("%w: access_key_secret too short (<%d)", dns.ErrInvalidCredential, minKeyLength)
	}
	return nil
}

// HealthCheck 调 DescribeDomains 列表 API 验证凭据有效（只读，无副作用）。
func (p *aliProvider) HealthCheck(ctx context.Context, cred map[string]string) error {
	if err := p.ValidateCredential(cred); err != nil {
		return err
	}
	c, err := p.cfg.newClient(cred)
	if err != nil {
		return err
	}
	return c.describeDomains(ctx)
}

// BuildSolver 构造一个绑定凭据的 ca.DnsSolver。
//
// 同时跑一遍 lego 的 NewDNSProviderConfig 做参数完整性校验（lego 内部对
// AccessKey/SecretKey 组合的检查覆盖比我们 ValidateCredential 更严）；
// 校验失败则 wrap 为 ErrInvalidCredential。
func (p *aliProvider) BuildSolver(_ context.Context, cred map[string]string, _ []string) (ca.DnsSolver, error) {
	if err := p.ValidateCredential(cred); err != nil {
		return nil, err
	}
	if err := legoValidateConfig(cred); err != nil {
		return nil, fmt.Errorf("%w: %v", dns.ErrInvalidCredential, err)
	}
	c, err := p.cfg.newClient(cred)
	if err != nil {
		return nil, err
	}
	return &aliSolver{
		client:  c,
		ttl:     p.cfg.TTL,
		timeout: p.cfg.PropagationTimeout,
	}, nil
}

// legoValidateConfig 跑一遍 lego 的 NewDNSProviderConfig；该函数会检查
// APIKey/SecretKey/RegionID 组合是否完整。我们丢掉返回的 DNSProvider
// （它的 Present 接口与我们的 ca.DnsSolver 不兼容），仅借它做校验。
func legoValidateConfig(cred map[string]string) error {
	cfg := legoalidns.NewDefaultConfig()
	cfg.APIKey = cred["access_key_id"]
	cfg.SecretKey = cred["access_key_secret"]
	cfg.RegionID = regionOrDefault(cred)
	_, err := legoalidns.NewDNSProviderConfig(cfg)
	return err
}

func regionOrDefault(cred map[string]string) string {
	if r := strings.TrimSpace(cred["region_id"]); r != "" {
		return r
	}
	return defaultRegionID
}

// ---- solver -----------------------------------------------------------------

type aliSolver struct {
	client  txtClient
	ttl     int
	timeout time.Duration
}

func (s *aliSolver) Timeout() time.Duration { return s.timeout }

func (s *aliSolver) Present(ctx context.Context, fqdn, value string) error {
	zone, rr, err := s.client.resolveZone(ctx, fqdn)
	if err != nil {
		return err
	}
	return s.client.addTXT(ctx, zone, rr, value, s.ttl)
}

func (s *aliSolver) CleanUp(ctx context.Context, fqdn, value string) error {
	zone, rr, err := s.client.resolveZone(ctx, fqdn)
	if err != nil {
		return err
	}
	ids, err := s.client.listMatchingTXT(ctx, zone, rr, value)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.client.deleteRecord(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ---- txtClient interface ----------------------------------------------------

// txtClient 是 solver 与 alidns API 之间的最小接口；
// 生产由 realClient 实现，测试可以注入 fake。
type txtClient interface {
	// describeDomains 调用 DescribeDomains 列表 API 做 HealthCheck。
	describeDomains(ctx context.Context) error
	// resolveZone 把 fqdn 解析为 (zone domainName, rr 主机记录名)；rr
	// 不含 zone 部分。例如 fqdn=_acme-challenge.foo.example.com.，
	// zone=example.com，rr=_acme-challenge.foo。
	resolveZone(ctx context.Context, fqdn string) (zone, rr string, err error)
	// addTXT 创建一个 TXT 记录。
	addTXT(ctx context.Context, zone, rr, value string, ttl int) error
	// listMatchingTXT 列出 (zone, rr) 下 value 匹配的 TXT 记录 ID 列表。
	listMatchingTXT(ctx context.Context, zone, rr, value string) ([]string, error)
	// deleteRecord 按 recordID 删除一条记录。
	deleteRecord(ctx context.Context, recordID string) error
}

// ---- realClient (alidns SDK 实现) -------------------------------------------

// realClientFactory 在 production 路径上构造 alidns SDK client。
func realClientFactory(cred map[string]string) (txtClient, error) {
	cfg := new(openapi.Config).
		SetAccessKeyId(cred["access_key_id"]).
		SetAccessKeySecret(cred["access_key_secret"]).
		SetRegionId(regionOrDefault(cred)).
		SetReadTimeout(int(defaultHTTPTimeout.Milliseconds()))
	c, err := alidnssdk.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: aliyun: new client: %v", dns.ErrUpstreamUnavailable, err)
	}
	return &realClient{c: c}, nil
}

type realClient struct {
	c *alidnssdk.Client
}

func (r *realClient) describeDomains(ctx context.Context) error {
	req := new(alidnssdk.DescribeDomainsRequest).
		SetPageNumber(1).
		SetPageSize(1)
	_, err := alidnssdk.DescribeDomainsWithContext(ctx, r.c, req, new(dara.RuntimeOptions))
	return mapSDKError(err)
}

// resolveZone 拿到 fqdn 后调 DescribeDomains 反查 zone。
// 阿里云 DNS 的"主域名"是 example.com，子记录 RR 是 sub.host 这种。
// 我们从 fqdn 向上找最匹配的 zone。
func (r *realClient) resolveZone(ctx context.Context, fqdn string) (string, string, error) {
	name := strings.TrimSuffix(fqdn, ".")
	if name == "" {
		return "", "", fmt.Errorf("%w: empty fqdn", dns.ErrZoneNotFound)
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", "", fmt.Errorf("%w: fqdn %q too short", dns.ErrZoneNotFound, fqdn)
	}

	// 阿里云 DescribeDomains 接受 KeyWord 模糊查询；用最具体的二级域名
	// 作 keyword，再逐级匹配返回列表里的 DomainName。
	// 从 labels[0..] 起每个 suffix 都试一遍（最长 suffix 最先匹配 apex）。
	knownZones, err := r.listAllDomainNames(ctx)
	if err != nil {
		return "", "", err
	}
	if len(knownZones) == 0 {
		return "", "", fmt.Errorf("%w: no domains for credential", dns.ErrZoneNotFound)
	}

	// 对每个候选 suffix（从最具体向上），看是否在已注册 zone 列表里。
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		for _, z := range knownZones {
			if strings.EqualFold(candidate, z) {
				rr := strings.TrimSuffix(strings.Join(labels[:i], "."), ".")
				if rr == "" {
					rr = "@"
				}
				return z, rr, nil
			}
		}
	}
	return "", "", fmt.Errorf("%w: no zone matches %q", dns.ErrZoneNotFound, fqdn)
}

// listAllDomainNames 拉一次 DescribeDomains 分页列表，返回所有主域名。
// 用户名下域名数一般有限（<100），一次拉到 200 条够用。
func (r *realClient) listAllDomainNames(ctx context.Context) ([]string, error) {
	var out []string
	var page int64 = 1
	const pageSize int64 = 100
	for {
		req := new(alidnssdk.DescribeDomainsRequest).
			SetPageNumber(page).
			SetPageSize(pageSize)
		resp, err := alidnssdk.DescribeDomainsWithContext(ctx, r.c, req, new(dara.RuntimeOptions))
		if err != nil {
			return nil, mapSDKError(err)
		}
		if resp == nil || resp.Body == nil || resp.Body.Domains == nil {
			break
		}
		for _, d := range resp.Body.Domains.Domain {
			if d != nil && d.DomainName != nil {
				out = append(out, *d.DomainName)
			}
		}
		total := int64(0)
		if resp.Body.TotalCount != nil {
			total = *resp.Body.TotalCount
		}
		if page*pageSize >= total {
			break
		}
		page++
	}
	return out, nil
}

func (r *realClient) addTXT(ctx context.Context, zone, rr, value string, ttl int) error {
	req := new(alidnssdk.AddDomainRecordRequest).
		SetDomainName(zone).
		SetRR(rr).
		SetType("TXT").
		SetValue(value).
		SetTTL(int64(ttl))
	_, err := alidnssdk.AddDomainRecordWithContext(ctx, r.c, req, new(dara.RuntimeOptions))
	return mapSDKError(err)
}

func (r *realClient) listMatchingTXT(ctx context.Context, zone, rr, value string) ([]string, error) {
	req := new(alidnssdk.DescribeDomainRecordsRequest).
		SetDomainName(zone).
		SetRRKeyWord(rr).
		SetType("TXT").
		SetPageSize(500)
	resp, err := alidnssdk.DescribeDomainRecordsWithContext(ctx, r.c, req, new(dara.RuntimeOptions))
	if err != nil {
		return nil, mapSDKError(err)
	}
	if resp == nil || resp.Body == nil || resp.Body.DomainRecords == nil {
		return nil, nil
	}
	var ids []string
	for _, rec := range resp.Body.DomainRecords.Record {
		if rec == nil {
			continue
		}
		if rec.RR == nil || rec.Value == nil || rec.RecordId == nil {
			continue
		}
		if !strings.EqualFold(*rec.RR, rr) {
			continue
		}
		if *rec.Value == value {
			ids = append(ids, *rec.RecordId)
		}
	}
	return ids, nil
}

func (r *realClient) deleteRecord(ctx context.Context, recordID string) error {
	req := new(alidnssdk.DeleteDomainRecordRequest).SetRecordId(recordID)
	_, err := alidnssdk.DeleteDomainRecordWithContext(ctx, r.c, req, new(dara.RuntimeOptions))
	return mapSDKError(err)
}

// ---- error mapping ----------------------------------------------------------

// mapSDKError 把 alibaba SDK 错误映射到 dns sentinel。
//   - 4xx auth (401/403, code=InvalidAccessKeyId.* / SignatureDoesNotMatch / Forbidden.*) → ErrInvalidCredential
//   - 404 / code=DomainRecordNotBelongToUser / DomainNotExists → ErrZoneNotFound
//   - 5xx 或网络错误 → ErrUpstreamUnavailable
func mapSDKError(err error) error {
	if err == nil {
		return nil
	}
	// alibaba SDK 走 tea.TeaSDKError() 把所有错误转成 *tea.SDKError。
	if se, ok := err.(*teatea.SDKError); ok {
		status := 0
		if se.StatusCode != nil {
			status = *se.StatusCode
		}
		code := ""
		if se.Code != nil {
			code = *se.Code
		}
		switch {
		case status == 401, status == 403:
			return fmt.Errorf("%w: aliyun status=%d code=%q", dns.ErrInvalidCredential, status, code)
		case status == 404:
			return fmt.Errorf("%w: aliyun status=%d code=%q", dns.ErrZoneNotFound, status, code)
		case status >= 500:
			return fmt.Errorf("%w: aliyun status=%d code=%q", dns.ErrUpstreamUnavailable, status, code)
		}
		// 0 status 或其它：按 code 判。
		switch {
		case isAuthErrorCode(code):
			return fmt.Errorf("%w: aliyun code=%q", dns.ErrInvalidCredential, code)
		case isZoneErrorCode(code):
			return fmt.Errorf("%w: aliyun code=%q", dns.ErrZoneNotFound, code)
		}
		return fmt.Errorf("%w: aliyun code=%q msg=%s", dns.ErrUpstreamUnavailable, code, safeMsg(se))
	}
	// 非 SDK error：可能是 ctx cancel、网络错误等；视为 upstream。
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%w: %v", dns.ErrUpstreamUnavailable, err)
}

func isAuthErrorCode(code string) bool {
	switch code {
	case "InvalidAccessKeyId.NotFound",
		"InvalidAccessKeyId",
		"SignatureDoesNotMatch",
		"Forbidden",
		"Forbidden.RAM",
		"Forbidden.AccessKeyDisabled",
		"NoPermission":
		return true
	}
	return false
}

func isZoneErrorCode(code string) bool {
	switch code {
	case "DomainNotExists",
		"DomainRecordNotBelongToUser",
		"InvalidDomainName.NoExist":
		return true
	}
	return false
}

func safeMsg(se *teatea.SDKError) string {
	if se == nil {
		return ""
	}
	if se.Message == nil {
		return ""
	}
	const max = 200
	m := *se.Message
	if len(m) > max {
		return m[:max] + "..."
	}
	return m
}

// compile-time interface check.
var _ dns.Provider = (*aliProvider)(nil)
var _ ca.DnsSolver = (*aliSolver)(nil)
