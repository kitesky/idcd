// Package gcloud 实现 dns.Provider 接口，针对 Google Cloud DNS。
//
// 桥接 go-acme/lego v4 的 gcloud provider：
//   - 复用 lego 的 `gcloud.NewDNSProviderConfig` 来校验 / 实例化底层 provider
//     （含 PropagationTimeout / PollingInterval / TTL 默认值与字段约束）；
//   - HTTPClient 走 `golang.org/x/oauth2/google.JWTConfigFromJSON` 由 service
//     account JSON 构造，注入到 lego config.HTTPClient；
//   - Present / CleanUp 由本包用 `google.golang.org/api/dns/v1` 直接实现：
//     lego 的 Present 签名是 (domain, token, keyAuth)，会自己算 fqdn / value，
//     而 idcd 的 `ca.DnsSolver.Present(ctx, fqdn, value)` 已传 challenge 计算
//     好的 fqdn 与 value，无法反推 keyAuth；故仅借 lego 做底层 client 配置，
//     业务逻辑自写。
//
// credential：
//
//	{
//	  "service_account_json": "<GCP service account JSON key 全文>",
//	  "project_id": "<可选；不填则从 service_account_json.project_id 解析>"
//	}
package gcloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	legogcloud "github.com/go-acme/lego/v4/providers/dns/gcloud"
	"golang.org/x/oauth2/google"
	gdns "google.golang.org/api/dns/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	defaultPropagationTimeout = 3 * time.Minute
	defaultPollingInterval    = 5 * time.Second
	defaultTTL                = 120

	// cloudDNSScope 与 lego 内部用的一致；只读列 zone 也用同一个 scope。
	cloudDNSScope = gdns.NdevClouddnsReadwriteScope
)

// Config 是 provider 构造选项。零值 Config 走默认（3min / 5s / TTL 120）。
type Config struct {
	PropagationTimeout time.Duration
	PollingInterval    time.Duration
	TTL                int

	// Endpoint 覆盖默认 Cloud DNS endpoint；仅测试使用（httptest）。
	Endpoint string

	// HTTPClient 覆盖默认 http.Client，仅 HealthCheck / BuildSolver 在没有
	// service_account_json 的极端测试场景用；正常代码路径下由
	// service_account_json 派生的 oauth2 client 取代它。
	HTTPClient *http.Client
}

// New 返回一个 GCloud provider 实例。
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
	return &gcloudProvider{cfg: cfg}
}

type gcloudProvider struct {
	cfg Config
}

func (p *gcloudProvider) Kind() dns.ProviderKind { return dns.KindGCloud }

// serviceAccountKey 是我们从 credential["service_account_json"] 解析出来的
// 字段结构；不必覆盖 GCP 全部字段，只校验业务必填的三项。
type serviceAccountKey struct {
	Type         string `json:"type"`
	ProjectID    string `json:"project_id"`
	ClientEmail  string `json:"client_email"`
	PrivateKey   string `json:"private_key"`
	PrivateKeyID string `json:"private_key_id"`
	TokenURI     string `json:"token_uri"`
}

// ValidateCredential 字段层校验：service_account_json 必须是合法 JSON、
// 含 client_email / private_key / project_id 三项；如果调用方显式提供
// project_id 字段也不允许为空。
func (p *gcloudProvider) ValidateCredential(credential map[string]string) error {
	saJSON, ok := credential["service_account_json"]
	if !ok || strings.TrimSpace(saJSON) == "" {
		return fmt.Errorf("%w: missing service_account_json", dns.ErrInvalidCredential)
	}
	var sa serviceAccountKey
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return fmt.Errorf("%w: service_account_json not valid json: %v", dns.ErrInvalidCredential, err)
	}
	if sa.ClientEmail == "" {
		return fmt.Errorf("%w: service_account_json missing client_email", dns.ErrInvalidCredential)
	}
	if sa.PrivateKey == "" {
		return fmt.Errorf("%w: service_account_json missing private_key", dns.ErrInvalidCredential)
	}
	if sa.ProjectID == "" {
		return fmt.Errorf("%w: service_account_json missing project_id", dns.ErrInvalidCredential)
	}
	if pid, ok := credential["project_id"]; ok && strings.TrimSpace(pid) == "" {
		return fmt.Errorf("%w: project_id provided but empty", dns.ErrInvalidCredential)
	}
	return nil
}

// resolveProject 决定要用哪个 GCP project：credential 显式提供优先，
// 否则取 service_account_json.project_id。调用前请先 ValidateCredential。
func resolveProject(cred map[string]string, sa serviceAccountKey) string {
	if pid, ok := cred["project_id"]; ok && strings.TrimSpace(pid) != "" {
		return strings.TrimSpace(pid)
	}
	return sa.ProjectID
}

// buildHTTPClient 用 service account JSON 构造 oauth2 http.Client。
func (p *gcloudProvider) buildHTTPClient(ctx context.Context, saJSON []byte) (*http.Client, error) {
	conf, err := google.JWTConfigFromJSON(saJSON, cloudDNSScope)
	if err != nil {
		return nil, fmt.Errorf("%w: parse service account key: %v", dns.ErrInvalidCredential, err)
	}
	return conf.Client(ctx), nil
}

// newDNSService 构造 *gdns.Service；测试可通过 cfg.Endpoint 把流量指向 httptest。
func (p *gcloudProvider) newDNSService(ctx context.Context, hc *http.Client) (*gdns.Service, error) {
	opts := []option.ClientOption{option.WithHTTPClient(hc)}
	if p.cfg.Endpoint != "" {
		opts = append(opts, option.WithEndpoint(p.cfg.Endpoint))
	}
	svc, err := gdns.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: build cloud dns service: %v", dns.ErrUpstreamUnavailable, err)
	}
	return svc, nil
}

// BuildSolver 根据 credential 返回绑定到具体 GCP project 的 solver。
//
// 借助 lego 的 NewDNSProviderConfig 来标准化字段约束（TTL / Timeout /
// PollingInterval）并 fail-fast 配置错误；实际 Present / CleanUp 由本包自写。
func (p *gcloudProvider) BuildSolver(ctx context.Context, credential map[string]string, _ []string) (ca.DnsSolver, error) {
	if err := p.ValidateCredential(credential); err != nil {
		return nil, err
	}
	saJSON := []byte(credential["service_account_json"])
	var sa serviceAccountKey
	_ = json.Unmarshal(saJSON, &sa) // already validated above

	project := resolveProject(credential, sa)

	hc, err := p.buildHTTPClient(ctx, saJSON)
	if err != nil {
		return nil, err
	}

	// 通过 lego 走一遍配置约束（验证 HTTPClient 非空 / Project 非空等）。
	legoCfg := legogcloud.NewDefaultConfig()
	legoCfg.Project = project
	legoCfg.PropagationTimeout = p.cfg.PropagationTimeout
	legoCfg.PollingInterval = p.cfg.PollingInterval
	legoCfg.TTL = p.cfg.TTL
	legoCfg.HTTPClient = hc
	if _, err := legogcloud.NewDNSProviderConfig(legoCfg); err != nil {
		return nil, fmt.Errorf("%w: lego gcloud config: %v", dns.ErrInvalidCredential, err)
	}

	svc, err := p.newDNSService(ctx, hc)
	if err != nil {
		return nil, err
	}

	return &gcloudSolver{
		svc:     svc,
		project: project,
		ttl:     int64(p.cfg.TTL),
		timeout: p.cfg.PropagationTimeout,
	}, nil
}

// HealthCheck 调 ManagedZones.List 验证凭据有效。
func (p *gcloudProvider) HealthCheck(ctx context.Context, credential map[string]string) error {
	if err := p.ValidateCredential(credential); err != nil {
		return err
	}
	saJSON := []byte(credential["service_account_json"])
	var sa serviceAccountKey
	_ = json.Unmarshal(saJSON, &sa)

	project := resolveProject(credential, sa)

	hc, err := p.buildHTTPClient(ctx, saJSON)
	if err != nil {
		return err
	}
	svc, err := p.newDNSService(ctx, hc)
	if err != nil {
		return err
	}
	if _, err := svc.ManagedZones.List(project).MaxResults(1).Context(ctx).Do(); err != nil {
		return mapGCloudErr(err)
	}
	return nil
}

// mapGCloudErr 把 Cloud DNS API 错误映射到 dns sentinel。
func mapGCloudErr(err error) error {
	if err == nil {
		return nil
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		switch {
		case gerr.Code == http.StatusUnauthorized, gerr.Code == http.StatusForbidden:
			return fmt.Errorf("%w: http %d: %s", dns.ErrInvalidCredential, gerr.Code, gerr.Message)
		case gerr.Code == http.StatusNotFound:
			return fmt.Errorf("%w: http %d: %s", dns.ErrZoneNotFound, gerr.Code, gerr.Message)
		case gerr.Code >= 500:
			return fmt.Errorf("%w: http %d: %s", dns.ErrUpstreamUnavailable, gerr.Code, gerr.Message)
		default:
			return fmt.Errorf("%w: http %d: %s", dns.ErrUpstreamUnavailable, gerr.Code, gerr.Message)
		}
	}
	// 非 googleapi.Error：网络 / oauth2 / 解析失败一律归到 upstream。
	return fmt.Errorf("%w: %v", dns.ErrUpstreamUnavailable, err)
}

// ---- solver -----------------------------------------------------------------

type gcloudSolver struct {
	svc     *gdns.Service
	project string
	ttl     int64
	timeout time.Duration
}

func (s *gcloudSolver) Timeout() time.Duration { return s.timeout }

// Present 在匹配的 managed zone 内创建/合并 TXT 记录。
func (s *gcloudSolver) Present(ctx context.Context, fqdn, value string) error {
	zone, err := s.findManagedZone(ctx, fqdn)
	if err != nil {
		return err
	}
	name := dnsFqdn(fqdn)

	// Cloud DNS 的 TXT rrdata 元素需要被双引号包裹（API 约定）。
	quoted := quoteTXT(value)

	rec := &gdns.ResourceRecordSet{
		Name:    name,
		Type:    "TXT",
		Ttl:     s.ttl,
		Rrdatas: []string{quoted},
	}

	// 已经存在同名 TXT：把现有 rrdatas 合并进新 rrset，再走 delete+add 替换。
	existing, err := s.listTXT(ctx, zone.Name, name)
	if err != nil {
		return err
	}
	change := &gdns.Change{Additions: []*gdns.ResourceRecordSet{rec}}
	if len(existing) > 0 {
		for _, ex := range existing {
			for _, d := range ex.Rrdatas {
				if d == quoted {
					continue
				}
				rec.Rrdatas = append(rec.Rrdatas, d)
			}
		}
		change.Deletions = existing
	}
	if _, err := s.svc.Changes.Create(s.project, zone.Name, change).Context(ctx).Do(); err != nil {
		return mapGCloudErr(err)
	}
	return nil
}

// CleanUp 把 fqdn 上对应 value 的 TXT 删掉（其它 rrdata 保留）。
func (s *gcloudSolver) CleanUp(ctx context.Context, fqdn, value string) error {
	zone, err := s.findManagedZone(ctx, fqdn)
	if err != nil {
		return err
	}
	name := dnsFqdn(fqdn)
	quoted := quoteTXT(value)

	existing, err := s.listTXT(ctx, zone.Name, name)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return nil
	}

	change := &gdns.Change{Deletions: existing}
	var remain []string
	for _, ex := range existing {
		for _, d := range ex.Rrdatas {
			if d == quoted {
				continue
			}
			remain = append(remain, d)
		}
	}
	if len(remain) > 0 {
		change.Additions = []*gdns.ResourceRecordSet{{
			Name:    name,
			Type:    "TXT",
			Ttl:     s.ttl,
			Rrdatas: remain,
		}}
	}
	if _, err := s.svc.Changes.Create(s.project, zone.Name, change).Context(ctx).Do(); err != nil {
		return mapGCloudErr(err)
	}
	return nil
}

// findManagedZone 反向查 zone：从最具体的 suffix 向上找已托管的 zone。
func (s *gcloudSolver) findManagedZone(ctx context.Context, fqdn string) (*gdns.ManagedZone, error) {
	name := strings.TrimSuffix(fqdn, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return nil, fmt.Errorf("%w: fqdn %q too short", dns.ErrZoneNotFound, fqdn)
	}
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".") + "."
		resp, err := s.svc.ManagedZones.List(s.project).DnsName(candidate).Context(ctx).Do()
		if err != nil {
			return nil, mapGCloudErr(err)
		}
		for _, z := range resp.ManagedZones {
			if z.Visibility == "public" || z.Visibility == "" {
				return z, nil
			}
		}
	}
	return nil, fmt.Errorf("%w: no managed zone matches %q", dns.ErrZoneNotFound, fqdn)
}

func (s *gcloudSolver) listTXT(ctx context.Context, zone, fqdn string) ([]*gdns.ResourceRecordSet, error) {
	resp, err := s.svc.ResourceRecordSets.List(s.project, zone).Name(fqdn).Type("TXT").Context(ctx).Do()
	if err != nil {
		return nil, mapGCloudErr(err)
	}
	return resp.Rrsets, nil
}

// dnsFqdn 确保末尾带点，符合 Cloud DNS 的 FQDN 要求。
func dnsFqdn(s string) string {
	if strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}

// quoteTXT 把 TXT 值用双引号包裹（Cloud DNS 协议要求）。
func quoteTXT(v string) string {
	if strings.HasPrefix(v, `"`) && strings.HasSuffix(v, `"`) && len(v) >= 2 {
		return v
	}
	return `"` + v + `"`
}

// compile-time interface check.
var _ dns.Provider = (*gcloudProvider)(nil)
var _ ca.DnsSolver = (*gcloudSolver)(nil)
