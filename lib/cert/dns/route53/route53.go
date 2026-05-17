// Package route53 实现 dns.Provider 接口，针对 AWS Route 53 DNS。
//
// 直连 AWS SDK Go v2 (service/route53)，不经过 lego 中间层，原因与
// cloudflare 包同理：lego 的 Present(domain, token, keyAuth) 会内部
// 重新计算 fqdn / value（SHA-256 不可逆），与本接口 Present(ctx, fqdn,
// value) 的契约对不上。Config 字段与 lego route53.Config 保持对齐，
// 方便后续切换或文档参照。
//
// credential payload（明文 JSON，由 vault 解密后传入）：
//
//	{
//	  "access_key_id":     "AKIA...",
//	  "secret_access_key": "...",
//	  "region":            "us-east-1",       // 可选
//	  "hosted_zone_id":    "Z123ABC"          // 可选，留空走自动 zone 检测
//	}
//
// 错误 → sentinel 映射:
//   - AccessDenied / InvalidClientTokenId / UnrecognizedClient / SignatureDoesNotMatch
//     → dns.ErrInvalidCredential
//   - NoSuchHostedZone / HostedZoneNotFound / 找不到 zone → dns.ErrZoneNotFound
//   - 5xx / Throttling / 网络错误 → dns.ErrUpstreamUnavailable
package route53

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awsroute53 "github.com/aws/aws-sdk-go-v2/service/route53"
	awstypes "github.com/aws/aws-sdk-go-v2/service/route53/types"
	smithy "github.com/aws/smithy-go"

	"github.com/kite365/idcd/lib/cert/ca"
	"github.com/kite365/idcd/lib/cert/dns"
)

const (
	// Route 53 propagation 通常 30-60s，但偶尔会到分钟级；3 分钟是
	// AWS 官方建议给 CA validate 的安全窗。
	defaultPropagationTimeout = 3 * time.Minute
	// Route 53 GetChange 轮询周期；10s 与官方 CLI 默认一致。
	defaultPollingInterval = 10 * time.Second
	// TXT TTL；60s 与 lego 默认对齐。
	defaultTTL = 60
	// 默认 region；Route 53 是全局服务，但 SDK 仍需要一个签名 region。
	defaultRegion = "us-east-1"
)

// Config 配置 Route 53 provider。零值 Config 走默认值。
type Config struct {
	// PropagationTimeout 决定 Present 调用最长等多久 GetChange 返回 INSYNC。
	PropagationTimeout time.Duration
	// PollingInterval 是 GetChange 轮询周期。
	PollingInterval time.Duration
	// TTL 是写入的 TXT 记录 TTL。
	TTL int

	// BaseEndpoint 覆盖 Route 53 API 地址；仅测试使用。生产留空，由
	// SDK 解析为 https://route53.amazonaws.com。
	BaseEndpoint string
	// HTTPClient 覆盖底层 http.Client；nil 时由 SDK 提供默认。
	HTTPClient awsroute53.HTTPClient
	// MaxRetries 决定 SDK 自身的重试次数；0 走 SDK 默认 (3)。
	MaxRetries int
}

// New 返回一个 Route 53 provider 实例。
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
	return &r53Provider{cfg: cfg}
}

type r53Provider struct {
	cfg Config
}

func (p *r53Provider) Kind() dns.ProviderKind { return dns.KindRoute53 }

// ValidateCredential 字段层校验：access_key_id 16-128 字节，secret >=20。
func (p *r53Provider) ValidateCredential(credential map[string]string) error {
	akid, ok := credential["access_key_id"]
	if !ok || akid == "" {
		return fmt.Errorf("%w: missing access_key_id", dns.ErrInvalidCredential)
	}
	if l := len(akid); l < 16 || l > 128 {
		return fmt.Errorf("%w: access_key_id length %d out of range [16,128]", dns.ErrInvalidCredential, l)
	}
	secret, ok := credential["secret_access_key"]
	if !ok || secret == "" {
		return fmt.Errorf("%w: missing secret_access_key", dns.ErrInvalidCredential)
	}
	if len(secret) < 20 {
		return fmt.Errorf("%w: secret_access_key too short", dns.ErrInvalidCredential)
	}
	return nil
}

// HealthCheck 调 ListHostedZones（限制 1 条）验证凭据。
func (p *r53Provider) HealthCheck(ctx context.Context, credential map[string]string) error {
	if err := p.ValidateCredential(credential); err != nil {
		return err
	}
	client, err := p.buildClient(credential)
	if err != nil {
		return err
	}
	one := int32(1)
	_, err = client.ListHostedZones(ctx, &awsroute53.ListHostedZonesInput{MaxItems: &one})
	if err != nil {
		return mapAWSErr(err)
	}
	return nil
}

// BuildSolver 返回一个绑定了凭据的 ca.DnsSolver。
func (p *r53Provider) BuildSolver(_ context.Context, credential map[string]string, _ []string) (ca.DnsSolver, error) {
	if err := p.ValidateCredential(credential); err != nil {
		return nil, err
	}
	client, err := p.buildClient(credential)
	if err != nil {
		return nil, err
	}
	return &r53Solver{
		client:       client,
		hostedZoneID: credential["hosted_zone_id"],
		ttl:          int64(p.cfg.TTL),
		timeout:      p.cfg.PropagationTimeout,
		poll:         p.cfg.PollingInterval,
	}, nil
}

// buildClient 用静态凭据构造 route53 client。
func (p *r53Provider) buildClient(credential map[string]string) (*awsroute53.Client, error) {
	region := credential["region"]
	if region == "" {
		region = defaultRegion
	}
	opts := awsroute53.Options{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			credential["access_key_id"],
			credential["secret_access_key"],
			"",
		),
		RetryMaxAttempts: p.cfg.MaxRetries,
	}
	if p.cfg.BaseEndpoint != "" {
		ep := p.cfg.BaseEndpoint
		opts.BaseEndpoint = &ep
	}
	if p.cfg.HTTPClient != nil {
		opts.HTTPClient = p.cfg.HTTPClient
	}
	return awsroute53.New(opts), nil
}

// ---- solver -----------------------------------------------------------------

type r53Solver struct {
	client       *awsroute53.Client
	hostedZoneID string // 可空：空则走 ListHostedZonesByName 自动检测
	ttl          int64
	timeout      time.Duration
	poll         time.Duration
}

func (s *r53Solver) Timeout() time.Duration { return s.timeout }

func (s *r53Solver) Present(ctx context.Context, fqdn, value string) error {
	zone, err := s.findHostedZoneID(ctx, fqdn)
	if err != nil {
		return err
	}
	if err := s.changeRecord(ctx, zone, fqdn, value, awstypes.ChangeActionUpsert); err != nil {
		return err
	}
	return nil
}

func (s *r53Solver) CleanUp(ctx context.Context, fqdn, value string) error {
	zone, err := s.findHostedZoneID(ctx, fqdn)
	if err != nil {
		return err
	}
	if err := s.changeRecord(ctx, zone, fqdn, value, awstypes.ChangeActionDelete); err != nil {
		// CleanUp 找不到记录不致命，但仍返回 wrap 错误便于排查。
		return err
	}
	return nil
}

// changeRecord 执行 UPSERT / DELETE TXT 记录。
func (s *r53Solver) changeRecord(ctx context.Context, zoneID, fqdn, value string, action awstypes.ChangeAction) error {
	name := fqdn
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}
	// Route 53 TXT value 必须带双引号包裹。
	realValue := `"` + value + `"`

	in := &awsroute53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &awstypes.ChangeBatch{
			Comment: aws.String("Managed by idcd cert-svc"),
			Changes: []awstypes.Change{{
				Action: action,
				ResourceRecordSet: &awstypes.ResourceRecordSet{
					Name: aws.String(name),
					Type: awstypes.RRTypeTxt,
					TTL:  aws.Int64(s.ttl),
					ResourceRecords: []awstypes.ResourceRecord{
						{Value: aws.String(realValue)},
					},
				},
			}},
		},
	}
	if _, err := s.client.ChangeResourceRecordSets(ctx, in); err != nil {
		return mapAWSErr(err)
	}
	return nil
}

// findHostedZoneID：优先用 config 里固定的 hosted_zone_id；否则 fqdn 反推。
func (s *r53Solver) findHostedZoneID(ctx context.Context, fqdn string) (string, error) {
	if s.hostedZoneID != "" {
		return s.hostedZoneID, nil
	}
	// 从 _acme-challenge.x.y.example.com. 推 apex：去掉 _acme-challenge.
	// 前缀再向上找。
	name := strings.TrimSuffix(fqdn, ".")
	name = strings.TrimPrefix(name, "_acme-challenge.")
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("%w: fqdn %q too short", dns.ErrZoneNotFound, fqdn)
	}
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		zoneID, err := s.lookupZoneByName(ctx, candidate)
		if err != nil {
			return "", err
		}
		if zoneID != "" {
			return zoneID, nil
		}
	}
	return "", fmt.Errorf("%w: no hosted zone matches %q", dns.ErrZoneNotFound, fqdn)
}

func (s *r53Solver) lookupZoneByName(ctx context.Context, apex string) (string, error) {
	// API DNSName 不带 trailing dot；响应里 Name 带 trailing dot。
	in := &awsroute53.ListHostedZonesByNameInput{
		DNSName: aws.String(apex),
	}
	out, err := s.client.ListHostedZonesByName(ctx, in)
	if err != nil {
		return "", mapAWSErr(err)
	}
	want := apex + "."
	for _, hz := range out.HostedZones {
		if hz.Name != nil && *hz.Name == want {
			// PrivateZone 不参与公共 ACME；过滤掉。
			if hz.Config != nil && hz.Config.PrivateZone {
				continue
			}
			id := ""
			if hz.Id != nil {
				id = *hz.Id
			}
			id = strings.TrimPrefix(id, "/hostedzone/")
			if id == "" {
				return "", fmt.Errorf("%w: empty hosted zone id for %s", dns.ErrZoneNotFound, apex)
			}
			return id, nil
		}
	}
	return "", nil
}

// mapAWSErr 把 AWS SDK / smithy error → dns sentinel。
func mapAWSErr(err error) error {
	if err == nil {
		return nil
	}
	// smithy.APIError 包含 ErrorCode + ErrorFault。
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch code {
		case "AccessDenied", "AccessDeniedException",
			"InvalidClientTokenId", "UnrecognizedClientException",
			"SignatureDoesNotMatch", "InvalidSignature",
			"MissingAuthenticationToken":
			return fmt.Errorf("%w: %s: %s", dns.ErrInvalidCredential, code, apiErr.ErrorMessage())
		case "NoSuchHostedZone", "HostedZoneNotFound":
			return fmt.Errorf("%w: %s", dns.ErrZoneNotFound, code)
		case "Throttling", "ThrottlingException",
			"RequestLimitExceeded", "PriorRequestNotComplete",
			"ServiceUnavailable", "InternalError":
			return fmt.Errorf("%w: %s: %s", dns.ErrUpstreamUnavailable, code, apiErr.ErrorMessage())
		}
		// 服务器侧故障一律走 upstream；客户端侧的未识别错误也走 upstream
		// 让 worker 重试（多半是临时网络问题）。
		if apiErr.ErrorFault() == smithy.FaultServer {
			return fmt.Errorf("%w: %s: %s", dns.ErrUpstreamUnavailable, code, apiErr.ErrorMessage())
		}
		return fmt.Errorf("%w: %s: %s", dns.ErrUpstreamUnavailable, code, apiErr.ErrorMessage())
	}
	// 非 smithy 错误：HTTP / context 等。
	// http.StatusCode 没法直接拿到；只能按字符串映射兜底。
	msg := err.Error()
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") {
		return fmt.Errorf("%w: %v", dns.ErrInvalidCredential, err)
	}
	return fmt.Errorf("%w: %v", dns.ErrUpstreamUnavailable, err)
}

// compile-time interface checks.
var _ dns.Provider = (*r53Provider)(nil)
var _ ca.DnsSolver = (*r53Solver)(nil)
