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
	Registrar    string   `json:"registrar"`
	CreationDate string   `json:"creation_date"`
	ExpiryDate   string   `json:"expiry_date"`
	NameServers  []string `json:"name_servers"`
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

	// S1 implementation: return mock data structure
	// Real implementation requires whois library or exec call
	result := WhoisResponse{
		Domain:       query,
		Registrar:    "Mock Registrar (WHOIS will be implemented in S2)",
		CreationDate: "",
		ExpiryDate:   "",
		NameServers:  []string{},
	}

	response.JSON(w, r, http.StatusOK, result)
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
	if idx := strings.Index(query, "/"); idx != -1 {
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

// ICP handles GET /v1/info/icp?q=<domain> — ICP filing query (S1: mock).
func (h *InfoHandler) ICP(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		response.Error(w, r, apperr.Validation("Missing 'q' parameter", "q parameter is required"))
		return
	}

	// S1 mock: return note that real implementation will be in S2
	result := ICPResponse{
		Domain:    query,
		ICPNumber: "",
		Note:      "ICP query will be implemented in S2",
	}

	response.JSON(w, r, http.StatusOK, result)
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
