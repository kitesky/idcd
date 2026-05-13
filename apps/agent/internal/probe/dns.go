package probe

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

// RealDNSResolver implements DNSResolver using the miekg/dns library.
type RealDNSResolver struct {
	client     *dns.Client
	nameserver string
}

// NewRealDNSResolver creates a new real DNS resolver.
func NewRealDNSResolver() *RealDNSResolver {
	return &RealDNSResolver{
		client:     &dns.Client{},
		nameserver: "8.8.8.8:53", // default to Google DNS
	}
}

// Execute performs a DNS resolution probe.
func (p *DNSProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	if p.Resolver == nil {
		p.Resolver = NewRealDNSResolver()
	}

	// Set timeout for DNS client
	if realResolver, ok := p.Resolver.(*RealDNSResolver); ok {
		realResolver.client.Timeout = timeout
	}

	data := map[string]any{}

	// Perform A record lookup
	aRecords, err := p.Resolver.LookupHost(target)
	if err != nil {
		return &Result{
			Success:    false,
			Error:      fmt.Sprintf("DNS lookup failed: %v", err),
			Data:       data,
			Timestamp:  start,
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	resolveTime := time.Since(start)
	data["resolve_ms"] = resolveTime.Milliseconds()
	data["records"] = map[string]any{
		"A": aRecords,
	}

	// Try additional record types (best effort)
	if mxRecords, err := p.Resolver.LookupMX(target); err == nil {
		data["records"].(map[string]any)["MX"] = mxRecords
	}

	if txtRecords, err := p.Resolver.LookupTXT(target); err == nil {
		data["records"].(map[string]any)["TXT"] = txtRecords
	}

	if cname, err := p.Resolver.LookupCNAME(target); err == nil {
		data["records"].(map[string]any)["CNAME"] = cname
	}

	if nsRecords, err := p.Resolver.LookupNS(target); err == nil {
		data["records"].(map[string]any)["NS"] = nsRecords
	}

	// Add nameserver info
	if realResolver, ok := p.Resolver.(*RealDNSResolver); ok {
		data["nameserver"] = realResolver.nameserver
	}

	return &Result{
		Success:    true,
		Data:       data,
		Timestamp:  start,
		DurationMs: resolveTime.Milliseconds(),
	}
}

// LookupHost resolves A records for the given hostname.
func (r *RealDNSResolver) LookupHost(name string) ([]string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)

	resp, _, err := r.client.Exchange(m, r.nameserver)
	if err != nil {
		return nil, err
	}

	var ips []string
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}

	return ips, nil
}

// LookupMX resolves MX records for the given domain.
func (r *RealDNSResolver) LookupMX(name string) ([]MXRecord, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeMX)

	resp, _, err := r.client.Exchange(m, r.nameserver)
	if err != nil {
		return nil, err
	}

	var records []MXRecord
	for _, ans := range resp.Answer {
		if mx, ok := ans.(*dns.MX); ok {
			records = append(records, MXRecord{
				Host:     mx.Mx,
				Priority: mx.Preference,
			})
		}
	}

	return records, nil
}

// LookupTXT resolves TXT records for the given domain.
func (r *RealDNSResolver) LookupTXT(name string) ([]string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeTXT)

	resp, _, err := r.client.Exchange(m, r.nameserver)
	if err != nil {
		return nil, err
	}

	var records []string
	for _, ans := range resp.Answer {
		if txt, ok := ans.(*dns.TXT); ok {
			records = append(records, txt.Txt...)
		}
	}

	return records, nil
}

// LookupCNAME resolves CNAME record for the given name.
func (r *RealDNSResolver) LookupCNAME(name string) (string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeCNAME)

	resp, _, err := r.client.Exchange(m, r.nameserver)
	if err != nil {
		return "", err
	}

	for _, ans := range resp.Answer {
		if cname, ok := ans.(*dns.CNAME); ok {
			return cname.Target, nil
		}
	}

	return "", fmt.Errorf("no CNAME record found")
}

// LookupNS resolves NS records for the given domain.
func (r *RealDNSResolver) LookupNS(name string) ([]string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeNS)

	resp, _, err := r.client.Exchange(m, r.nameserver)
	if err != nil {
		return nil, err
	}

	var records []string
	for _, ans := range resp.Answer {
		if ns, ok := ans.(*dns.NS); ok {
			records = append(records, ns.Ns)
		}
	}

	return records, nil
}