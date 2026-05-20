// Package freetsa is the RFC3161 TSA adapter for freetsa.org's free
// public timestamp service (https://freetsa.org/tsr). Like digicert /
// globalsign it requires no authentication and speaks plain RFC3161
// over HTTPS POST.
//
// **dev / pre-prod only**：freetsa.org 不保证 SLA，且回签 cert chain
// 不在常见 Adobe / EU LOTL 信任锚里，签出来的 PDF 公开 PAdES 验证大概率
// 不通过。仅供 attest-generator 在没有 commercial TSA 配额的环境下跑通
// pipeline 用 — 生产请使用 digicert / globalsign。
//
// Implementation note: 协议逻辑沿用 tsa/internal/rfc3161client；本文件
// 只承担"配置 + 命名"。
package freetsa

import (
	"context"
	"crypto"
	"net/http"
	"time"

	"github.com/kite365/idcd/lib/attest/tsa"
	"github.com/kite365/idcd/lib/attest/tsa/internal/rfc3161client"
)

const (
	// DefaultEndpoint 是 freetsa.org 公开的 TSA URL。
	DefaultEndpoint = "https://freetsa.org/tsr"

	providerName = "freetsa"
)

// Config 镜像 digicert.Config / globalsign.Config 字段语义。
type Config struct {
	Endpoint   string
	HTTPClient *http.Client
}

// New 返回向 freetsa POST RFC3161 timestamp query 的 tsa.Provider。
// 无状态，并发安全。
func New(cfg Config) tsa.Provider {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &provider{endpoint: endpoint, client: cfg.HTTPClient}
}

type provider struct {
	endpoint string
	client   *http.Client
}

func (p *provider) Name() string { return providerName }

func (p *provider) Stamp(ctx context.Context, hashAlg crypto.Hash, digest []byte) ([]byte, time.Time, error) {
	return rfc3161client.Stamp(ctx, rfc3161client.Config{
		Endpoint:     p.endpoint,
		HTTPClient:   p.client,
		ProviderName: providerName,
	}, hashAlg, digest)
}

var _ tsa.Provider = (*provider)(nil)
