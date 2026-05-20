// Package handler provides HTTP request handlers for the API Gateway.
package handler

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
	"github.com/miekg/dns"

	"github.com/kite365/idcd/apps/api/internal/denylist"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/db/gen/idcdmain"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/netfilter"
	"github.com/kite365/idcd/lib/shared/netutil"
)

// ICPQuerier 是 InfoHandler 需要的 sqlc 子集。nil 时 ICP 接口直接走 fallback
// (只返回 inquiry_url),避免单测必须配 DB。
type ICPQuerier interface {
	GetICPRecordByDomain(ctx context.Context, domain string) (idcdmain.IcpRecord, error)
}

// rejectIfBlockedHost validates a user-supplied hostname against the
// SSRF / metadata netfilter and writes a 400 response if blocked.
// Returns true when the caller should stop processing the request.
//
// Used by the public DNS-class endpoints (rdns / spf / dmarc / dns / mx / dkim)
// to prevent attackers from coercing the API into resolving names that point
// at internal infrastructure or cloud-metadata IPs.
func rejectIfBlockedHost(w http.ResponseWriter, r *http.Request, host string) bool {
	if blocked, reason := netfilter.IsBlocked(host); blocked {
		response.Error(w, r, apperr.Validation("Target host not allowed", reason))
		return true
	}
	return false
}

// normalizeDomain 把用户粘来的 URL/路径剥到 host:
// "https://example.com/path?x=1" → "example.com"。
// DKIM/SPF/DMARC/MX 这些 handler 之前不做规范化,用户复制 URL 时查不到。
func normalizeDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.IndexByte(s, '/'); idx != -1 {
		s = s[:idx]
	}
	return strings.ToLower(s)
}

// InfoHandler handles network information query endpoints.
type InfoHandler struct {
	httpClient *http.Client
	icpQ       ICPQuerier // 可选; nil 时 ICP 接口走 fallback。
}

// NewInfoHandler creates a new info handler.
func NewInfoHandler() *InfoHandler {
	return &InfoHandler{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// WithICPQuerier 装配 ICP 自建库查询接口,返回同一 handler 以便链式调用。
func (h *InfoHandler) WithICPQuerier(q ICPQuerier) *InfoHandler {
	h.icpQ = q
	return h
}

// --- Response types ---

// IPInfoResponse represents IP geolocation information.
type IPInfoResponse struct {
	IP           string `json:"ip"`
	Country      string `json:"country"`
	City         string `json:"city"`
	ASN          string `json:"asn"`
	ISP          string `json:"isp"`
	IsDatacenter bool   `json:"is_datacenter"`
	IsProxy      bool   `json:"is_proxy"`
}

// WhoisResponse represents WHOIS domain information.
type WhoisResponse struct {
	Domain       string   `json:"domain"`
	Registrar    string   `json:"registrar,omitempty"`
	CreationDate string   `json:"creation_date,omitempty"`
	ExpiryDate   string   `json:"expiry_date,omitempty"`
	NameServers  []string `json:"name_servers,omitempty"`
	Note         string   `json:"note,omitempty"`
}

// RDNSResponse represents reverse DNS lookup results.
type RDNSResponse struct {
	IP        string   `json:"ip"`
	Hostnames []string `json:"hostnames"`
}

// SPFResponse represents SPF record query results.
type SPFResponse struct {
	Domain string `json:"domain"`
	Record string `json:"record,omitempty"`
	Found  bool   `json:"found"`
}

// DMARCResponse represents DMARC record query results.
type DMARCResponse struct {
	Domain string `json:"domain"`
	Record string `json:"record,omitempty"`
	Found  bool   `json:"found"`
}

// DNSResponse represents DNS query results.
type DNSResponse struct {
	Domain  string      `json:"domain"`
	Type    string      `json:"type"`
	Records []DNSRecord `json:"records"`
}

// DNSRecord represents a single DNS record.
// Priority 仅 MX 有意义,其它类型置 0 不返回。
type DNSRecord struct {
	Value    string `json:"value"`
	TTL      uint32 `json:"ttl,omitempty"`
	Priority uint16 `json:"priority,omitempty"`
}

// SSLResponse represents SSL certificate information.
//
// Valid==false 时 VerifyError 解释了为什么链验证失败(过期/自签/SAN 不匹配
// 等)；其它字段照样填充,因为前端要展示坏证书的实际内容(过期日期、issuer、
// SAN)以便用户诊断,而不是 503 抛错完事。
type SSLResponse struct {
	Domain          string   `json:"domain"`
	Issuer          string   `json:"issuer"`
	Subject         string   `json:"subject"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	SANDomains      []string `json:"san_domains"`
	Protocol        string   `json:"protocol"`
	DaysUntilExpiry int      `json:"days_until_expiry"`
	Valid           bool     `json:"valid"`
	VerifyError     string   `json:"verify_error,omitempty"`
}

// ICPResponse represents ICP filing information.
//
// 本地库命中:icp_number / company / type / filed_at 完整。
// 未命中: Note 给提示 + InquiryURL 让前端跳转工信部公示页(主域名预填)。
type ICPResponse struct {
	Domain     string `json:"domain"`
	ICPNumber  string `json:"icp_number"`
	Company    string `json:"company,omitempty"`
	Type       string `json:"type,omitempty"`
	FiledAt    string `json:"filed_at,omitempty"`
	Note       string `json:"note,omitempty"`
	InquiryURL string `json:"inquiry_url,omitempty"`
	Source     string `json:"source,omitempty"`
}

// --- IP Query Handler ---

// IP handles GET /v1/info/ip?q=<IP或域名> — IP geolocation query with SSRF protection.
func (h *InfoHandler) IP(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// SSRF protection: resolve hostname and validate all returned IPs.
	resolvedAddr, err := denylist.CheckTarget(query)
	if err != nil {
		response.Error(w, r, apperr.Forbidden(err.Error()))
		return
	}
	// Extract the bare IP from the resolved address (strip port if present).
	ip := resolvedAddr
	if host, _, splitErr := net.SplitHostPort(resolvedAddr); splitErr == nil {
		ip = host
	}

	// If the caller supplied a non-IP hostname, CheckTarget may return the
	// original target verbatim on DNS failure (so the downstream probe can
	// surface a clearer error). For this endpoint, however, an unresolvable
	// hostname means we have nothing meaningful to look up upstream — and
	// ip-api.com will respond with an HTML error page that breaks our
	// JSON-decode path. Reject it as a validation error up front.
	if net.ParseIP(ip) == nil {
		if _, lookupErr := net.DefaultResolver.LookupHost(r.Context(), ip); lookupErr != nil {
			response.Error(w, r, apperr.Validation("invalid or unresolvable domain", "q"))
			return
		}
	}

	// Query ip-api.com
	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,message,country,city,isp,as,hosting,proxy", ip)
	req, err := http.NewRequestWithContext(r.Context(), "GET", apiURL, nil)
	if err != nil {
		response.Error(w, r, apperr.Internal("Failed to create request", err))
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		response.Error(w, r, apperr.Unavailable("Failed to query IP information", err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		response.Error(w, r, apperr.Internal("Failed to read response", err))
		return
	}

	// Defensive: ip-api.com occasionally returns an HTML error page (e.g. when
	// the upstream cannot resolve the target). Treat anything that is not JSON
	// as a validation failure rather than a 500 — the user-facing semantics
	// match "unresolvable / unsupported input".
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "json") {
		response.Error(w, r, apperr.Validation("invalid or unresolvable domain", "q"))
		return
	}

	// Parse ip-api response
	var apiResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Country string `json:"country"`
		City    string `json:"city"`
		ISP     string `json:"isp"`
		AS      string `json:"as"`
		Hosting bool   `json:"hosting"`
		Proxy   bool   `json:"proxy"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		response.Error(w, r, apperr.Validation("invalid or unresolvable domain", "q"))
		return
	}

	if apiResp.Status != "success" {
		response.Error(w, r, apperr.Validation("IP query failed", apiResp.Message))
		return
	}

	result := IPInfoResponse{
		IP:           ip,
		Country:      apiResp.Country,
		City:         apiResp.City,
		ASN:          apiResp.AS,
		ISP:          apiResp.ISP,
		IsDatacenter: apiResp.Hosting,
		IsProxy:      apiResp.Proxy,
	}

	response.JSON(w, r, http.StatusOK, result)
}

// --- WHOIS Query Handler ---

// Whois handles GET /v1/info/whois?q=<domain或IP> — WHOIS query.
//
// 用 likexian/whois 取 raw,whois-parser 做字段解析。比手写 grep 强在:
//  1. 自动跟随 IANA referral(.io / .ai / .app 等先 iana 再 nic.io)
//  2. 覆盖 50+ TLD 的字段命名差异
//  3. CN 域名中文字段(注册商/注册日期/到期日期)在 parser 里有规则
func (h *InfoHandler) Whois(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}
	result := queryWhois(r.Context(), query)
	response.JSON(w, r, http.StatusOK, result)
}

// queryWhois 走 likexian 客户端 + parser,失败 fallback 到 raw + 手写解析。
func queryWhois(ctx context.Context, domain string) WhoisResponse {
	// likexian/whois 客户端的 Whois 调用本身是阻塞同步的,不接 ctx。
	// 用客户端的 SetTimeout 把上限锁在 10s,且通过 ctx.Done() 让请求生命周期
	// 提前结束时不再等满 — 即使后台 goroutine 仍在阻塞,我们也不再持有它。
	// 这样不会泄漏调用方协程,后台 goroutine 最多被 SetTimeout 兜底回收。
	type whoisOut struct {
		raw string
		err error
	}
	ch := make(chan whoisOut, 1)
	go func() {
		raw, err := whois.NewClient().SetTimeout(10 * time.Second).Whois(domain)
		ch <- whoisOut{raw, err}
	}()
	var raw string
	select {
	case out := <-ch:
		if out.err != nil {
			return WhoisResponse{Domain: domain, Note: "WHOIS query unavailable: " + out.err.Error()}
		}
		raw = out.raw
	case <-ctx.Done():
		return WhoisResponse{Domain: domain, Note: "WHOIS query timed out"}
	}

	parsed, perr := whoisparser.Parse(raw)
	if perr != nil {
		// parser 不支持的 TLD 走旧 fallback,至少能填 registrar/NS
		return fallbackParseWhois(domain, raw)
	}

	result := WhoisResponse{Domain: domain}
	if parsed.Registrar != nil {
		result.Registrar = parsed.Registrar.Name
	}
	if parsed.Domain != nil {
		result.CreationDate = parsed.Domain.CreatedDate
		result.ExpiryDate = parsed.Domain.ExpirationDate
		if len(parsed.Domain.NameServers) > 0 {
			ns := make([]string, 0, len(parsed.Domain.NameServers))
			for _, n := range parsed.Domain.NameServers {
				ns = append(ns, strings.ToLower(n))
			}
			result.NameServers = ns
		}
	}
	// parser 解析出空时回退手写 grep(覆盖中文 WHOIS / 非主流 TLD)
	if result.Registrar == "" && result.CreationDate == "" && len(result.NameServers) == 0 {
		return fallbackParseWhois(domain, raw)
	}
	return result
}

// fallbackParseWhois 是旧的手写解析,保留作 parser 不识别时的兜底。
// 覆盖常见英文字段 + CN 域名常见中文字段。
func fallbackParseWhois(domain, raw string) WhoisResponse {
	result := WhoisResponse{Domain: domain}
	var nameServers []string

	// 中文 WHOIS 字段名可能跟半角或全角冒号,统一把行 normalize 成半角后再 grep,
	// 避免 4 个字段各维护两套字符串(之前 注册商 有全角,日期字段只覆盖半角)。
	for line := range strings.SplitSeq(raw, "\n") {
		nline := strings.ReplaceAll(line, "：", ":")
		lower := strings.ToLower(nline)
		switch {
		case (strings.Contains(lower, "registrar:") || strings.Contains(nline, "注册商:")) && result.Registrar == "":
			result.Registrar = extractWhoisField(nline)
		case (strings.Contains(lower, "creation date:") || strings.Contains(lower, "registered:") ||
			strings.Contains(nline, "注册日期:") || strings.Contains(nline, "注册时间:")) && result.CreationDate == "":
			result.CreationDate = extractWhoisField(nline)
		case (strings.Contains(lower, "registry expiry date:") || strings.Contains(lower, "expiry date:") ||
			strings.Contains(lower, "expires:") || strings.Contains(nline, "到期日期:") ||
			strings.Contains(nline, "到期时间:")) && result.ExpiryDate == "":
			result.ExpiryDate = extractWhoisField(nline)
		case strings.Contains(lower, "name server:") || strings.Contains(nline, "DNS Serve") || strings.Contains(nline, "域名服务器:"):
			if ns := extractWhoisField(nline); ns != "" {
				nameServers = append(nameServers, strings.ToLower(ns))
			}
		}
	}
	result.NameServers = nameServers
	return result
}

// extractWhoisField 取冒号后的值。调用方已把全角冒号归一为半角(见
// fallbackParseWhois 顶部的 ReplaceAll),所以这里只看半角即可。
func extractWhoisField(line string) string {
	if i := strings.IndexByte(line, ':'); i != -1 {
		return strings.TrimSpace(line[i+1:])
	}
	return ""
}

// --- rDNS Query Handler ---

// RDNS handles GET /v1/info/rdns?q=<IP> — reverse DNS lookup.
func (h *InfoHandler) RDNS(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	if net.ParseIP(query) == nil {
		response.Error(w, r, apperr.Validation("Invalid IP address", ""))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	hostnames, err := net.DefaultResolver.LookupAddr(ctx, query)
	if err != nil {
		// DNS lookup failures (NXDOMAIN, timeout) → empty list, not error
		hostnames = []string{}
	}
	response.JSON(w, r, http.StatusOK, RDNSResponse{IP: query, Hostnames: hostnames})
}

// --- SPF Query Handler ---

// SPF handles GET /v1/info/spf?q=<domain> — SPF record query.
func (h *InfoHandler) SPF(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	txts, err := net.DefaultResolver.LookupTXT(ctx, query)
	if err != nil {
		response.JSON(w, r, http.StatusOK, SPFResponse{Domain: query, Found: false})
		return
	}
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=spf1") {
			response.JSON(w, r, http.StatusOK, SPFResponse{Domain: query, Record: txt, Found: true})
			return
		}
	}
	response.JSON(w, r, http.StatusOK, SPFResponse{Domain: query, Found: false})
}

// --- DMARC Query Handler ---

// DMARC handles GET /v1/info/dmarc?q=<domain> — DMARC record query.
func (h *InfoHandler) DMARC(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dmarcDomain := "_dmarc." + strings.TrimPrefix(query, "_dmarc.")
	txts, err := net.DefaultResolver.LookupTXT(ctx, dmarcDomain)
	if err != nil {
		response.JSON(w, r, http.StatusOK, DMARCResponse{Domain: query, Found: false})
		return
	}
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=DMARC1") {
			response.JSON(w, r, http.StatusOK, DMARCResponse{Domain: query, Record: txt, Found: true})
			return
		}
	}
	response.JSON(w, r, http.StatusOK, DMARCResponse{Domain: query, Found: false})
}

// --- DNS Query Handler ---

// DNS handles GET /v1/info/dns?q=<domain>&type=<A|AAAA|MX|TXT|CNAME|NS|CAA> — DNS query.
//
// 用 miekg/dns 直接构造 DNS 协议包发上游(默认 1.1.1.1:53),拿到 raw RR 后
// 才能填 TTL、MX priority、CAA 三元组。Go 标准库 net.Resolver 不暴露这些。
func (h *InfoHandler) DNS(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}

	recordType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if recordType == "" {
		recordType = "A"
	}

	qtype, ok := dnsTypeCode(recordType)
	if !ok {
		response.Error(w, r, apperr.Validation("Invalid DNS type", "Supported types: A, AAAA, MX, TXT, CNAME, NS, CAA"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	records, err := queryDNSRR(ctx, query, qtype)
	if err != nil {
		response.Error(w, r, apperr.Internal("DNS query failed", err))
		return
	}

	result := DNSResponse{
		Domain:  query,
		Type:    recordType,
		Records: records,
	}

	response.JSON(w, r, http.StatusOK, result)
}

// dnsTypeCode 把字符串类型映射到 miekg/dns 的常量。
func dnsTypeCode(t string) (uint16, bool) {
	switch t {
	case "A":
		return dns.TypeA, true
	case "AAAA":
		return dns.TypeAAAA, true
	case "MX":
		return dns.TypeMX, true
	case "TXT":
		return dns.TypeTXT, true
	case "CNAME":
		return dns.TypeCNAME, true
	case "NS":
		return dns.TypeNS, true
	case "CAA":
		return dns.TypeCAA, true
	}
	return 0, false
}

// dnsUpstreams 是查询用的上游解析器,顺序尝试直到拿到响应。
//
// 顺序刻意排:
//  1. 223.5.5.5 (AliDNS) — 国内可达性最好,生产首选
//  2. 119.29.29.29 (DNSPod) — 国内备援
//  3. 1.1.1.1 (Cloudflare) — 国际兜底,出海域名 / CAA 准确
//
// 走自己发包(不用 net.Resolver)是因为标准库不暴露 TTL / MX priority / CAA。
var dnsUpstreams = []string{"223.5.5.5:53", "119.29.29.29:53", "1.1.1.1:53"}

// queryDNSRR 用 miekg/dns 发查询并把 RR 转成 DNSRecord 列表。
// 任一上游成功即返回,全部失败才向上抛 error。
func queryDNSRR(ctx context.Context, domain string, qtype uint16) ([]DNSRecord, error) {
	c := &dns.Client{Timeout: 3 * time.Second}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true

	var resp *dns.Msg
	var lastErr error
	for _, ups := range dnsUpstreams {
		r, _, err := c.ExchangeContext(ctx, m, ups)
		if err == nil && r != nil {
			resp = r
			break
		}
		lastErr = err
	}
	if resp == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("all DNS upstreams failed")
		}
		return nil, lastErr
	}
	// NXDOMAIN / NotImp 等 RCODE 转成 Go error,语义清晰且与旧 net.Resolver 一致。
	if resp.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("dns query: rcode %s", dns.RcodeToString[resp.Rcode])
	}
	records := make([]DNSRecord, 0, len(resp.Answer))
	for _, rr := range resp.Answer {
		// 跳过 CNAME 链中的中间结果,只保留请求类型对应的 RR。
		// 例外: 当用户主动查 CNAME 时把 CNAME 链返回。
		if rr.Header().Rrtype != qtype {
			continue
		}
		records = append(records, rrToRecord(rr))
	}
	return records, nil
}

// rrToRecord 把 miekg/dns 的 RR 转成对外 DNSRecord。
func rrToRecord(rr dns.RR) DNSRecord {
	rec := DNSRecord{TTL: rr.Header().Ttl}
	switch v := rr.(type) {
	case *dns.A:
		rec.Value = v.A.String()
	case *dns.AAAA:
		rec.Value = v.AAAA.String()
	case *dns.MX:
		rec.Value = v.Mx
		rec.Priority = v.Preference
	case *dns.TXT:
		rec.Value = strings.Join(v.Txt, "")
	case *dns.CNAME:
		rec.Value = v.Target
	case *dns.NS:
		rec.Value = v.Ns
	case *dns.CAA:
		// CAA 三元组: <flag> <tag> "<value>"
		rec.Value = fmt.Sprintf("%d %s %q", v.Flag, v.Tag, v.Value)
	default:
		// 兜底: 用 miekg/dns 自带 String() 去掉头部
		full := rr.String()
		if i := strings.Index(full, "\t"); i != -1 {
			// 简单截掉前 4 列(name/ttl/class/type),只留 rdata。
			parts := strings.SplitN(full, "\t", 5)
			if len(parts) == 5 {
				rec.Value = parts[4]
			} else {
				rec.Value = full
			}
		} else {
			rec.Value = full
		}
	}
	return rec
}

// query{A,AAAA,MX,TXT,CNAME,NS}Records 是 thin wrappers,新代码统一走 queryDNSRR;
// 保留方法形式给老测试用例,后续可以推动改用 typed param 的 queryDNSRR 直接调。
func (h *InfoHandler) queryARecords(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeA)
}
func (h *InfoHandler) queryAAAARecords(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeAAAA)
}
func (h *InfoHandler) queryMXRecords(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeMX)
}
func (h *InfoHandler) queryTXTRecords(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeTXT)
}
func (h *InfoHandler) queryCNAMERecord(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeCNAME)
}
func (h *InfoHandler) queryNSRecords(ctx context.Context, d string) ([]DNSRecord, error) {
	return queryDNSRR(ctx, d, dns.TypeNS)
}

// --- SSL Query Handler ---

// SSL handles GET /v1/info/ssl?q=<domain> — SSL certificate query.
func (h *InfoHandler) SSL(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// SSRF protection: resolve and validate before dialing.
	// CheckTarget returns the pre-resolved IP to prevent DNS rebinding.
	resolvedAddr, ssrfErr := denylist.CheckTarget(query + ":443")
	if ssrfErr != nil {
		response.Error(w, r, apperr.Forbidden(ssrfErr.Error()))
		return
	}

	// SSL 检查工具的核心价值是查 *坏* 证书 — 过期 / 自签 / 域名不匹配 — 所以
	// 先跳过链验证拿到证书,再单独跑 cert.Verify() 把失败原因塞进
	// VerifyError。否则用户最关心的"为什么证书有问题"会被 503 吞掉。
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", resolvedAddr, &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // 见上,故意跳过 + 后续显式验证
		ServerName:         query,
	})
	if err != nil {
		response.Error(w, r, apperr.Unavailable("Failed to connect to SSL endpoint", err))
		return
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		response.Error(w, r, apperr.NotFound("No certificates found"))
		return
	}

	cert := certs[0]
	daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

	// 用 leaf 之外的 PeerCertificates 当 intermediates,跑标准链验证。
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	_, verifyErr := cert.Verify(x509.VerifyOptions{
		DNSName:       query,
		Intermediates: intermediates,
	})
	valid := verifyErr == nil
	verifyMsg := ""
	if !valid {
		verifyMsg = verifyErr.Error()
	}

	result := SSLResponse{
		Domain:          query,
		Issuer:          cert.Issuer.CommonName,
		Subject:         cert.Subject.CommonName,
		NotBefore:       cert.NotBefore.Format(time.RFC3339),
		NotAfter:        cert.NotAfter.Format(time.RFC3339),
		SANDomains:      cert.DNSNames,
		Protocol:        conn.ConnectionState().NegotiatedProtocol,
		DaysUntilExpiry: daysUntilExpiry,
		Valid:           valid,
		VerifyError:     verifyMsg,
	}

	response.JSON(w, r, http.StatusOK, result)
}

// --- ICP Query Handler ---

// ICP handles GET /v1/info/icp?q=<domain> — ICP filing query.
//
// 完全走自建库 icp.records:命中返回完整字段;未命中给前端跳转工信部的
// inquiry_url(不调任何第三方 API,因为公开源都已死或要鉴权)。
func (h *InfoHandler) ICP(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}
	// 去掉 www. 前缀;ICP 备案只看主域名(www.baidu.com → baidu.com)。
	// 其它子域(m.* / api.* / mail.*)不在这里归一 — 不引 publicsuffix 库,
	// 因为公共后缀涉及 co.uk / com.cn 等多段 TLD,粗暴 trim 会出错;
	// 真要支持 m.baidu.com 这种,后续在导入侧把主站和子域都种进库。
	query = strings.TrimPrefix(query, "www.")

	// ICP 不调任何外部接口(纯本地库查询),所以不需要 SSRF guard。
	// 之前误把 gov.cn 等域名挡掉 — 它们恰恰是合法 ICP 主体。

	result := h.queryICP(r.Context(), query)
	response.JSON(w, r, http.StatusOK, result)
}

// miitInquiryURL 是工信部备案公示查询页。前端把这个 URL 渲染成"去工信部查"按钮。
// 不接收 domain 因为工信部 SPA 没有支持 query param 预填的入口,只能跳到主页让
// 用户手填。
func miitInquiryURL() string {
	return "https://beian.miit.gov.cn/#/Integrated/recordQuery"
}

// queryICP 查本地库;未命中走 fallback。
func (h *InfoHandler) queryICP(ctx context.Context, domain string) ICPResponse {
	if h.icpQ == nil {
		return icpFallback(domain, "ICP database not configured")
	}
	rec, err := h.icpQ.GetICPRecordByDomain(ctx, domain)
	if err != nil {
		// pgx.ErrNoRows 走 fallback;其它 DB 错也降级返回,不抛 5xx,
		// 因为 ICP 是辅助功能不该挡主链路。
		return icpFallback(domain, "ICP record not found in local database")
	}
	resp := ICPResponse{
		Domain:    rec.Domain,
		ICPNumber: rec.IcpNumber,
		Company:   rec.Company,
		Type:      rec.FilingType,
		Source:    rec.Source,
	}
	if rec.FiledAt.Valid {
		resp.FiledAt = rec.FiledAt.Time.Format("2006-01-02")
	}
	if rec.Note != "" {
		resp.Note = rec.Note
	}
	return resp
}

func icpFallback(domain, note string) ICPResponse {
	return ICPResponse{
		Domain:     domain,
		Note:       note + ". Please visit https://beian.miit.gov.cn/ to query manually.",
		InquiryURL: miitInquiryURL(),
	}
}

// --- MX Query Handler ---

// MXResponse represents MX record query results.
type MXResponse struct {
	Domain  string     `json:"domain"`
	Records []MXRecord `json:"records"`
}

// MXRecord represents a single MX record with priority.
type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

// MX handles GET /v1/info/mx?q=<domain> — MX record query.
func (h *InfoHandler) MX(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	mxs, err := net.DefaultResolver.LookupMX(ctx, query)
	if err != nil {
		response.JSON(w, r, http.StatusOK, MXResponse{Domain: query, Records: []MXRecord{}})
		return
	}
	records := make([]MXRecord, 0, len(mxs))
	for _, mx := range mxs {
		records = append(records, MXRecord{Host: mx.Host, Priority: mx.Pref})
	}
	response.JSON(w, r, http.StatusOK, MXResponse{Domain: query, Records: records})
}

// --- DKIM Query Handler ---

// DKIMResponse represents DKIM record query results.
type DKIMResponse struct {
	Domain   string `json:"domain"`
	Selector string `json:"selector"`
	Record   string `json:"record,omitempty"`
	Found    bool   `json:"found"`
}

// DKIM handles GET /v1/info/dkim?q=<domain>&selector=<selector> — DKIM record query.
func (h *InfoHandler) DKIM(w http.ResponseWriter, r *http.Request) {
	query := normalizeDomain(r.URL.Query().Get("q"))
	selector := strings.TrimSpace(r.URL.Query().Get("selector"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	if rejectIfBlockedHost(w, r, query) {
		return
	}
	if selector == "" {
		selector = "default"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dkimDomain := selector + "._domainkey." + query
	txts, err := net.DefaultResolver.LookupTXT(ctx, dkimDomain)
	if err != nil {
		response.JSON(w, r, http.StatusOK, DKIMResponse{Domain: query, Selector: selector, Found: false})
		return
	}
	for _, txt := range txts {
		if strings.Contains(txt, "v=DKIM1") || strings.HasPrefix(txt, "p=") {
			response.JSON(w, r, http.StatusOK, DKIMResponse{Domain: query, Selector: selector, Record: txt, Found: true})
			return
		}
	}
	// Return first TXT if no DKIM prefix (some records have split format)
	if len(txts) > 0 {
		response.JSON(w, r, http.StatusOK, DKIMResponse{Domain: query, Selector: selector, Record: strings.Join(txts, ""), Found: true})
		return
	}
	response.JSON(w, r, http.StatusOK, DKIMResponse{Domain: query, Selector: selector, Found: false})
}

// --- ASN Query Handler ---

// ASNResponse represents ASN query results.
type ASNResponse struct {
	Query       string `json:"query"`
	ASN         string `json:"asn"`
	ISP         string `json:"isp"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

// ASN handles GET /v1/info/asn?q=<IP或ASN> — ASN query.
func (h *InfoHandler) ASN(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}

	// If it's an AS number (AS12345), ip-api doesn't support direct ASN lookup;
	// return basic info with the normalised ASN string.
	if strings.HasPrefix(strings.ToUpper(query), "AS") {
		response.JSON(w, r, http.StatusOK, ASNResponse{Query: query, ASN: strings.ToUpper(query)})
		return
	}

	// SSRF / metadata-host guard before forwarding to the third-party service.
	if rejectIfBlockedHost(w, r, query) {
		return
	}

	apiURL := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,message,country,countryCode,isp,as", query)
	req, err := http.NewRequestWithContext(r.Context(), "GET", apiURL, nil)
	if err != nil {
		response.Error(w, r, apperr.Internal("Failed to create request", err))
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		response.Error(w, r, apperr.Unavailable("ASN query failed", err))
		return
	}
	defer resp.Body.Close()

	// ip-api 偶尔返回 HTML 错误页(域名不可解析时),跟 IP() handler 同样防御:
	// 不是 JSON 就当输入合法性失败,而不是 500。
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(strings.ToLower(ct), "json") {
		response.Error(w, r, apperr.Validation("invalid or unresolvable target", "q"))
		return
	}

	var apiResp struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		ISP         string `json:"isp"`
		AS          string `json:"as"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		response.Error(w, r, apperr.Validation("invalid or unresolvable target", "q"))
		return
	}
	if apiResp.Status != "success" {
		response.Error(w, r, apperr.Validation("ASN query failed", apiResp.Message))
		return
	}
	response.JSON(w, r, http.StatusOK, ASNResponse{
		Query:       query,
		ASN:         apiResp.AS,
		ISP:         apiResp.ISP,
		Country:     apiResp.Country,
		CountryCode: apiResp.CountryCode,
	})
}

// --- BGP Query Handler ---

// BGPResponse represents BGP route query results.
type BGPResponse struct {
	IP       string   `json:"ip"`
	Prefixes []string `json:"prefixes"`
	ASNs     []string `json:"asns"`
}

// BGP handles GET /v1/info/bgp?q=<IP> — BGP route query via bgpview.io.
func (h *InfoHandler) BGP(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
		return
	}
	// SSRF / metadata-host guard before forwarding to the third-party service.
	if rejectIfBlockedHost(w, r, query) {
		return
	}

	apiURL := fmt.Sprintf("https://api.bgpview.io/ip/%s", query)
	req, err := http.NewRequestWithContext(r.Context(), "GET", apiURL, nil)
	if err != nil {
		response.Error(w, r, apperr.Internal("Failed to create request", err))
		return
	}
	req.Header.Set("User-Agent", "idcd-bgp-tool/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		response.Error(w, r, apperr.Unavailable("BGP query failed", err))
		return
	}
	defer resp.Body.Close()

	var bgpResp struct {
		Status string `json:"status"`
		Data   struct {
			Prefixes []struct {
				Prefix string `json:"prefix"`
				ASN    struct {
					ASN  int    `json:"asn"`
					Name string `json:"name"`
				} `json:"asn"`
			} `json:"prefixes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bgpResp); err != nil {
		response.Error(w, r, apperr.Internal("Failed to parse BGP response", err))
		return
	}

	prefixes := make([]string, 0)
	asns := make([]string, 0)
	seen := map[string]bool{}
	for _, p := range bgpResp.Data.Prefixes {
		prefixes = append(prefixes, p.Prefix)
		asnStr := fmt.Sprintf("AS%d", p.ASN.ASN)
		if !seen[asnStr] {
			asns = append(asns, asnStr)
			seen[asnStr] = true
		}
	}

	response.JSON(w, r, http.StatusOK, BGPResponse{IP: query, Prefixes: prefixes, ASNs: asns})
}

// --- Helper functions ---

// isIP checks if the string is a valid IP address.
func isIP(s string) bool {
	return net.ParseIP(s) != nil
}

// isPrivateIP checks if an IP address string is private or reserved.
// Delegates to netutil.IsPrivateIP for the canonical full-range check.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return netutil.IsPrivateIP(ip)
}
