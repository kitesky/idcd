// Package dns 是 idcd cert 平台的 DNS provider 适配层。
//
// Provider 接口：把用户登记的 DNS 凭据 → 构造出 ACME challenge 用的
// ca.DnsSolver。每个 provider（cloudflare / aliyun / route53 / manual）
// 一个子包实现；本包只提供 ProviderKind 枚举、Provider 接口、Registry
// 以及统一的 sentinel error，子包返回的错误必须能 errors.Is 到其中之一。
package dns

import (
	"context"
	"errors"

	"github.com/kite365/idcd/lib/cert/ca"
)

// ProviderKind 用作 cert.dns_credentials.provider 字段值。
// 新增 provider 时务必同时在 docs/prd/20-free-cert.md 注册。
type ProviderKind string

const (
	KindCloudflare ProviderKind = "cloudflare"
	KindManual     ProviderKind = "manual"
	KindAliyun     ProviderKind = "aliyun"
	KindDNSPod     ProviderKind = "dnspod"
	KindRoute53    ProviderKind = "route53"
	KindGCloud     ProviderKind = "gcloud"
)

// Provider 是 DNS 适配的工厂接口。每个实现接受 vault 解密后的 credential
// payload（已反序列化为 map），并构造一个 ca.DnsSolver 给签发 worker。
type Provider interface {
	Kind() ProviderKind

	// BuildSolver 根据 credential 构造一个 solver。credential 由
	// cert-worker 从 vault 解密得来；domains 是订单 SAN 列表中的根域
	// （用于校验 credential 是否有权写该 zone；可选实现）。
	BuildSolver(ctx context.Context, credential map[string]string, domains []string) (ca.DnsSolver, error)

	// ValidateCredential 在用户提交凭据时调用，只做字段层面校验
	// （非空 / 长度 / 格式），不访问外网。
	ValidateCredential(credential map[string]string) error

	// HealthCheck 调用 provider 只读 API 验证凭据有效。
	// 给 /v1/cert/dns-credentials/{id}/health-check 端点用。
	HealthCheck(ctx context.Context, credential map[string]string) error
}

// Registry 按 kind 查 Provider。Registry 本身非并发安全：调用方应在进程
// 启动阶段完成全部 Register，运行期只读。
type Registry struct {
	providers map[ProviderKind]Provider
}

// NewRegistry 返回一个空 Registry。
func NewRegistry() *Registry {
	return &Registry{providers: map[ProviderKind]Provider{}}
}

// Register 注册一个 Provider；重复注册同 kind 返回 ErrProviderAlreadyRegistered。
func (r *Registry) Register(p Provider) error {
	if p == nil {
		return ErrInvalidCredential
	}
	kind := p.Kind()
	if kind == "" {
		return ErrInvalidCredential
	}
	if _, exists := r.providers[kind]; exists {
		return ErrProviderAlreadyRegistered
	}
	r.providers[kind] = p
	return nil
}

// Get 按 kind 取 Provider；未注册返回 ErrProviderNotRegistered。
func (r *Registry) Get(kind ProviderKind) (Provider, error) {
	p, ok := r.providers[kind]
	if !ok {
		return nil, ErrProviderNotRegistered
	}
	return p, nil
}

// Kinds 列出当前注册的所有 kind（供 admin 列表 API 用）。返回顺序未定义。
func (r *Registry) Kinds() []ProviderKind {
	out := make([]ProviderKind, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	return out
}

// 统一 sentinel error。子包错误必须 wrap 其中之一，worker 据此做重试 /
// 降级 / 上报决策。
var (
	ErrProviderNotRegistered     = errors.New("dns: provider not registered")
	ErrProviderAlreadyRegistered = errors.New("dns: provider already registered")
	ErrInvalidCredential         = errors.New("dns: invalid credential payload")
	ErrZoneNotFound              = errors.New("dns: zone not found / no permission")
	ErrUpstreamUnavailable       = errors.New("dns: upstream api unavailable")
	ErrPropagationTimeout        = errors.New("dns: txt propagation timeout")
)
