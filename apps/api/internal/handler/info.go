// Package handler provides HTTP request handlers for the API Gateway.
package handler

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kite365/idcd/apps/api/internal/denylist"
	"github.com/kite365/idcd/apps/api/internal/response"
	"github.com/kite365/idcd/lib/shared/apperr"
	"github.com/kite365/idcd/lib/shared/netutil"
)

// InfoHandler handles network information query endpoints.
type InfoHandler struct {
	httpClient *http.Client
}

// NewInfoHandler creates a new info handler.
func NewInfoHandler() *InfoHandler {
	return &InfoHandler{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
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
type DNSRecord struct {
	Value string `json:"value"`
	TTL   uint32 `json:"ttl,omitempty"`
}

// SSLResponse represents SSL certificate information.
type SSLResponse struct {
	Domain          string   `json:"domain"`
	Issuer          string   `json:"issuer"`
	Subject         string   `json:"subject"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	SANDomains      []string `json:"san_domains"`
	Protocol        string   `json:"protocol"`
	DaysUntilExpiry int      `json:"days_until_expiry"`
}

// ICPResponse represents ICP filing information.
type ICPResponse struct {
	Domain    string `json:"domain"`
	ICPNumber string `json:"icp_number"`
	Company   string `json:"company,omitempty"`
	Type      string `json:"type,omitempty"`
	FiledAt   string `json:"filed_at,omitempty"`
	Note      string `json:"note,omitempty"`
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
		response.Error(w, r, apperr.Internal("Failed to parse IP API response", err))
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
func (h *InfoHandler) Whois(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// Normalize: remove protocol prefix and path.
	query = strings.TrimPrefix(query, "https://")
	query = strings.TrimPrefix(query, "http://")
	if idx := strings.IndexByte(query, '/'); idx != -1 {
		query = query[:idx]
	}
	query = strings.ToLower(strings.TrimSpace(query))

	server := whoisServer(query)
	result := queryWhois(r.Context(), query, server)
	response.JSON(w, r, http.StatusOK, result)
}

// whoisServer returns the appropriate WHOIS server for the given domain.
func whoisServer(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return "whois.iana.org"
	}
	tld := parts[len(parts)-1]
	switch tld {
	case "com", "net":
		return "whois.verisign-grs.com"
	case "org":
		return "whois.pir.org"
	case "cn":
		return "whois.cnnic.cn"
	case "io":
		return "whois.iana.org"
	default:
		return "whois.iana.org"
	}
}

// queryWhois performs a TCP WHOIS query against the given server.
func queryWhois(ctx context.Context, domain, server string) WhoisResponse {
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", server+":43")
	if err != nil {
		return WhoisResponse{Domain: domain, Note: "WHOIS query unavailable: " + err.Error()}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	fmt.Fprintf(conn, "%s\r\n", domain)

	lr := io.LimitReader(conn, 65536)
	raw, err := io.ReadAll(lr)
	if err != nil && len(raw) == 0 {
		return WhoisResponse{Domain: domain, Note: "WHOIS read error: " + err.Error()}
	}

	return parseWhoisResponse(domain, string(raw))
}

// parseWhoisResponse parses a raw WHOIS response text into a WhoisResponse.
func parseWhoisResponse(domain, raw string) WhoisResponse {
	result := WhoisResponse{Domain: domain}
	var nameServers []string

	for line := range strings.SplitSeq(raw, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "registrar:") && result.Registrar == "" {
			result.Registrar = extractWhoisField(line)
		}
		if (strings.Contains(lower, "creation date:") || strings.Contains(lower, "registered:")) && result.CreationDate == "" {
			result.CreationDate = extractWhoisField(line)
		}
		if (strings.Contains(lower, "registry expiry date:") || strings.Contains(lower, "expiry date:") || strings.Contains(lower, "expires:")) && result.ExpiryDate == "" {
			result.ExpiryDate = extractWhoisField(line)
		}
		if strings.Contains(lower, "name server:") {
			if ns := extractWhoisField(line); ns != "" {
				nameServers = append(nameServers, strings.ToLower(ns))
			}
		}
	}
	result.NameServers = nameServers
	return result
}

// extractWhoisField extracts the value after the first colon in a WHOIS line.
func extractWhoisField(line string) string {
	_, after, found := strings.Cut(line, ":")
	if !found {
		return ""
	}
	return strings.TrimSpace(after)
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
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
func (h *InfoHandler) DNS(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	recordType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if recordType == "" {
		recordType = "A"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var records []DNSRecord
	var err error

	switch recordType {
	case "A":
		records, err = h.queryARecords(ctx, query)
	case "AAAA":
		records, err = h.queryAAAARecords(ctx, query)
	case "MX":
		records, err = h.queryMXRecords(ctx, query)
	case "TXT":
		records, err = h.queryTXTRecords(ctx, query)
	case "CNAME":
		records, err = h.queryCNAMERecord(ctx, query)
	case "NS":
		records, err = h.queryNSRecords(ctx, query)
	case "CAA":
		// CAA records require external library (github.com/miekg/dns)
		// For S1, return empty
		records = []DNSRecord{}
	default:
		response.Error(w, r, apperr.Validation("Invalid DNS type", "Supported types: A, AAAA, MX, TXT, CNAME, NS, CAA"))
		return
	}

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

func (h *InfoHandler) queryARecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domain)
	if err != nil {
		return nil, err
	}
	records := make([]DNSRecord, 0, len(ips))
	for _, ip := range ips {
		records = append(records, DNSRecord{Value: ip.String()})
	}
	return records, nil
}

func (h *InfoHandler) queryAAAARecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip6", domain)
	if err != nil {
		return nil, err
	}
	records := make([]DNSRecord, 0, len(ips))
	for _, ip := range ips {
		records = append(records, DNSRecord{Value: ip.String()})
	}
	return records, nil
}

func (h *InfoHandler) queryMXRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	mxs, err := net.DefaultResolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, err
	}
	records := make([]DNSRecord, 0, len(mxs))
	for _, mx := range mxs {
		records = append(records, DNSRecord{Value: mx.Host})
	}
	return records, nil
}

func (h *InfoHandler) queryTXTRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	txts, err := net.DefaultResolver.LookupTXT(ctx, domain)
	if err != nil {
		return nil, err
	}
	records := make([]DNSRecord, 0, len(txts))
	for _, txt := range txts {
		records = append(records, DNSRecord{Value: txt})
	}
	return records, nil
}

func (h *InfoHandler) queryCNAMERecord(ctx context.Context, domain string) ([]DNSRecord, error) {
	cname, err := net.DefaultResolver.LookupCNAME(ctx, domain)
	if err != nil {
		return nil, err
	}
	return []DNSRecord{{Value: cname}}, nil
}

func (h *InfoHandler) queryNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	nss, err := net.DefaultResolver.LookupNS(ctx, domain)
	if err != nil {
		return nil, err
	}
	records := make([]DNSRecord, 0, len(nss))
	for _, ns := range nss {
		records = append(records, DNSRecord{Value: ns.Host})
	}
	return records, nil
}

// --- SSL Query Handler ---

// SSL handles GET /v1/info/ssl?q=<domain> — SSL certificate query.
func (h *InfoHandler) SSL(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// Normalise: strip protocol prefix and path.
	query = strings.TrimPrefix(query, "https://")
	query = strings.TrimPrefix(query, "http://")
	if idx := strings.IndexByte(query, '/'); idx != -1 {
		query = query[:idx]
	}

	// SSRF protection: resolve and validate before dialing.
	// CheckTarget returns the pre-resolved IP to prevent DNS rebinding.
	resolvedAddr, ssrfErr := denylist.CheckTarget(query + ":443")
	if ssrfErr != nil {
		response.Error(w, r, apperr.Forbidden(ssrfErr.Error()))
		return
	}

	// Connect using the pre-resolved IP; preserve original hostname as SNI.
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", resolvedAddr, &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         query,
	})
	if err != nil {
		response.Error(w, r, apperr.Unavailable("Failed to connect to SSL endpoint", err))
		return
	}
	defer conn.Close()

	// Get certificate
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		response.Error(w, r, apperr.NotFound("No certificates found"))
		return
	}

	cert := certs[0]
	daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

	result := SSLResponse{
		Domain:          query,
		Issuer:          cert.Issuer.CommonName,
		Subject:         cert.Subject.CommonName,
		NotBefore:       cert.NotBefore.Format(time.RFC3339),
		NotAfter:        cert.NotAfter.Format(time.RFC3339),
		SANDomains:      cert.DNSNames,
		Protocol:        conn.ConnectionState().NegotiatedProtocol,
		DaysUntilExpiry: daysUntilExpiry,
	}

	response.JSON(w, r, http.StatusOK, result)
}

// --- ICP Query Handler ---

// ICP handles GET /v1/info/icp?q=<domain> — ICP filing query.
func (h *InfoHandler) ICP(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// Normalize: remove protocol prefix and path.
	query = strings.TrimPrefix(query, "https://")
	query = strings.TrimPrefix(query, "http://")
	if idx := strings.IndexByte(query, '/'); idx != -1 {
		query = query[:idx]
	}
	query = strings.ToLower(strings.TrimSpace(query))

	result := h.queryICP(r.Context(), query)
	response.JSON(w, r, http.StatusOK, result)
}

// queryICP attempts to query ICP filing information for the given domain.
// Falls back gracefully if the external service is unavailable.
func (h *InfoHandler) queryICP(ctx context.Context, domain string) ICPResponse {
	apiURL := "https://icplishi.com/api/?domain=" + domain
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return ICPResponse{Domain: domain, Note: "ICP 备案查询需接入工信部官方 API，当前为演示模式。请访问 https://beian.miit.gov.cn/ 手动查询。"}
	}
	req.Header.Set("User-Agent", "idcd-icp-tool/1.0")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return ICPResponse{Domain: domain, Note: "ICP 备案查询服务暂时不可用，请访问 https://beian.miit.gov.cn/ 手动查询。"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ICPResponse{Domain: domain, Note: "ICP 备案查询服务返回错误，请访问 https://beian.miit.gov.cn/ 手动查询。"}
	}

	var apiResp struct {
		Code int `json:"code"`
		Data struct {
			ICPNo  string `json:"icpNo"`
			Name   string `json:"name"`
			Type   string `json:"type"`
			Domain string `json:"domain"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return ICPResponse{Domain: domain, Note: "ICP 备案数据解析失败，请访问 https://beian.miit.gov.cn/ 手动查询。"}
	}

	if apiResp.Code != 0 || apiResp.Data.ICPNo == "" {
		return ICPResponse{Domain: domain, Note: "未查询到 ICP 备案记录"}
	}

	return ICPResponse{
		Domain:    domain,
		ICPNumber: apiResp.Data.ICPNo,
		Company:   apiResp.Data.Name,
		Type:      apiResp.Data.Type,
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
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
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	selector := strings.TrimSpace(r.URL.Query().Get("selector"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", ""))
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

	var apiResp struct {
		Status      string `json:"status"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		ISP         string `json:"isp"`
		AS          string `json:"as"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		response.Error(w, r, apperr.Internal("Failed to parse response", err))
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
