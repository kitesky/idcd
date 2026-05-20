package idcd

import "encoding/json"

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Code       string `json:"code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return e.Code + ": " + e.Message + " (" + e.Detail + ")"
	}
	return e.Code + ": " + e.Message
}

// ProbeRequest is the common request for all probe endpoints.
type ProbeRequest struct {
	Target string                 `json:"target"`
	Nodes  []string               `json:"nodes,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// ProbeHTTPRequest is the request for HTTP probe.
type ProbeHTTPRequest = ProbeRequest

// ProbePingRequest is the request for Ping probe.
type ProbePingRequest = ProbeRequest

// ProbeDNSRequest is the request for DNS probe.
type ProbeDNSRequest = ProbeRequest

// ProbeTCPRequest is the request for TCP probe.
type ProbeTCPRequest = ProbeRequest

// ProbeTracerouteRequest is the request for Traceroute probe.
type ProbeTracerouteRequest = ProbeRequest

// ProbeResult is the response from a probe endpoint.
type ProbeResult struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
}

// DiagnoseResult is the response from the diagnose endpoint.
type DiagnoseResult struct {
	DiagnosisID string   `json:"diagnosis_id"`
	TaskIDs     []string `json:"task_ids"`
	Status      string   `json:"status"`
}

// IPInfo represents IP geolocation information.
type IPInfo struct {
	IP           string `json:"ip"`
	Country      string `json:"country"`
	City         string `json:"city"`
	ASN          string `json:"asn"`
	ISP          string `json:"isp"`
	IsDatacenter bool   `json:"is_datacenter"`
	IsProxy      bool   `json:"is_proxy"`
}

// DNSRecord represents a single DNS record.
type DNSRecord struct {
	Value string `json:"value"`
	TTL   uint32 `json:"ttl,omitempty"`
}

// DNSResult represents DNS query results.
type DNSResult struct {
	Domain  string      `json:"domain"`
	Type    string      `json:"type"`
	Records []DNSRecord `json:"records"`
}

// SSLResult represents SSL certificate information.
type SSLResult struct {
	Domain          string   `json:"domain"`
	Issuer          string   `json:"issuer"`
	Subject         string   `json:"subject"`
	NotBefore       string   `json:"not_before"`
	NotAfter        string   `json:"not_after"`
	SANDomains      []string `json:"san_domains"`
	Protocol        string   `json:"protocol"`
	DaysUntilExpiry int      `json:"days_until_expiry"`
}

// WHOISResult represents WHOIS domain information.
type WHOISResult struct {
	Domain       string   `json:"domain"`
	Registrar    string   `json:"registrar"`
	CreationDate string   `json:"creation_date"`
	ExpiryDate   string   `json:"expiry_date"`
	NameServers  []string `json:"name_servers"`
}

// ICPResult represents ICP filing information.
type ICPResult struct {
	Domain    string `json:"domain"`
	ICPNumber string `json:"icp_number"`
	Company   string `json:"company,omitempty"`
	Type      string `json:"type,omitempty"`
	FiledAt   string `json:"filed_at,omitempty"`
	Note      string `json:"note,omitempty"`
}

// CreateMonitorRequest is the body for creating a monitor.
type CreateMonitorRequest struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Target    string          `json:"target"`
	Config    json.RawMessage `json:"config,omitempty"`
	IntervalS int32           `json:"interval_s,omitempty"`
	NodeCount int32           `json:"node_count,omitempty"`
}

// UpdateMonitorRequest is the body for updating a monitor.
type UpdateMonitorRequest struct {
	Name      *string          `json:"name,omitempty"`
	Config    *json.RawMessage `json:"config,omitempty"`
	IntervalS *int32           `json:"interval_s,omitempty"`
	Status    *string          `json:"status,omitempty"`
}

// Monitor represents a monitor resource.
type Monitor struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Target      string          `json:"target"`
	Config      json.RawMessage `json:"config"`
	IntervalS   int32           `json:"interval_s"`
	NodeCount   int32           `json:"node_count"`
	Status      string          `json:"status"`
	LastCheckAt *string         `json:"last_check_at,omitempty"`
	NextCheckAt *string         `json:"next_check_at,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

// MonitorList is the paginated list of monitors.
type MonitorList struct {
	Items []Monitor `json:"items"`
	Total int       `json:"total"`
	Page  int       `json:"page"`
	Limit int       `json:"limit"`
}

// BulkResult is the response from a bulk monitor action.
type BulkResult struct {
	Succeeded []string `json:"succeeded"`
	Failed    []string `json:"failed"`
	Total     int      `json:"total"`
}

// CheckBucket represents one time-bucket in the checks history.
type CheckBucket struct {
	BucketStart  string  `json:"bucket_start"`
	Total        int64   `json:"total"`
	Success      int64   `json:"success"`
	Failure      int64   `json:"failure"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Status       string  `json:"status"`
}

// MonitorChecks is the checks history for a monitor.
type MonitorChecks struct {
	MonitorID         string        `json:"monitor_id"`
	Hours             int           `json:"hours"`
	ResolutionMinutes int           `json:"resolution_minutes"`
	Buckets           []CheckBucket `json:"buckets"`
}

// MonitorBaseline represents the anchor baseline for a monitor.
type MonitorBaseline struct {
	ID          string   `json:"id"`
	MonitorID   string   `json:"monitor_id"`
	P50Latency  *float64 `json:"p50_latency_ms"`
	P95Latency  *float64 `json:"p95_latency_ms"`
	P99Latency  *float64 `json:"p99_latency_ms"`
	SuccessRate *float64 `json:"success_rate"`
	SampleCount int      `json:"sample_count"`
	ComputedAt  string   `json:"computed_at"`
	WindowHours int      `json:"window_hours"`
}

// CreateAlertChannelRequest is the body for creating an alert channel.
type CreateAlertChannelRequest struct {
	Name   string          `json:"name"`
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config,omitempty"`
}

// AlertChannel represents an alert notification channel.
type AlertChannel struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    json.RawMessage `json:"config"`
	Verified  bool            `json:"verified"`
	CreatedAt string          `json:"created_at"`
}

// AlertChannelList is the list of alert channels.
type AlertChannelList struct {
	Items []AlertChannel `json:"items"`
}

// CreateAlertPolicyRequest is the body for creating an alert policy.
type CreateAlertPolicyRequest struct {
	Name       string   `json:"name"`
	MonitorID  string   `json:"monitor_id"`
	ChannelIDs []string `json:"channel_ids,omitempty"`
	DelayS     *int     `json:"delay_s,omitempty"`
	RecoveryN  *int     `json:"recovery_n,omitempty"`
	MuteStart  *string  `json:"mute_start,omitempty"`
	MuteEnd    *string  `json:"mute_end,omitempty"`
}

// UpdateAlertPolicyRequest is the body for updating an alert policy.
type UpdateAlertPolicyRequest struct {
	Name       *string   `json:"name,omitempty"`
	ChannelIDs *[]string `json:"channel_ids,omitempty"`
	DelayS     *int      `json:"delay_s,omitempty"`
	RecoveryN  *int      `json:"recovery_n,omitempty"`
	MuteStart  *string   `json:"mute_start,omitempty"`
	MuteEnd    *string   `json:"mute_end,omitempty"`
	Enabled    *bool     `json:"enabled,omitempty"`
}

// AlertPolicy represents an alert policy resource.
type AlertPolicy struct {
	ID         string   `json:"id"`
	UserID     string   `json:"user_id"`
	MonitorID  string   `json:"monitor_id"`
	ChannelIDs []string `json:"channel_ids"`
	Name       string   `json:"name"`
	DelayS     int      `json:"delay_s"`
	RecoveryN  int      `json:"recovery_n"`
	MuteStart  *string  `json:"mute_start,omitempty"`
	MuteEnd    *string  `json:"mute_end,omitempty"`
	Enabled    bool     `json:"enabled"`
	CreatedAt  string   `json:"created_at"`
}

// AlertPolicyList is the list of alert policies.
type AlertPolicyList struct {
	Items []AlertPolicy `json:"items"`
}

// AlertEvent represents an alert event.
type AlertEvent struct {
	ID             string          `json:"id"`
	MonitorID      string          `json:"monitor_id"`
	PolicyID       string          `json:"policy_id"`
	Status         string          `json:"status"`
	StartedAt      string          `json:"started_at"`
	ResolvedAt     *string         `json:"resolved_at,omitempty"`
	AcknowledgedBy *string         `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *string         `json:"acknowledged_at,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
}

// AlertEventList is the list of alert events.
type AlertEventList struct {
	Items []AlertEvent `json:"items"`
}

// SubscribeRequest is the body for subscribing to a plan.
type SubscribeRequest struct {
	Plan string `json:"plan"`
}

// Subscription represents a billing subscription.
type Subscription struct {
	ID                 string  `json:"id"`
	Plan               string  `json:"plan"`
	Status             string  `json:"status"`
	Provider           string  `json:"provider"`
	ExtSubID           *string `json:"ext_sub_id,omitempty"`
	CurrentPeriodStart *string `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   *string `json:"current_period_end,omitempty"`
	CancelAt           *string `json:"cancel_at,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

// SubscribeResult is the response from subscribing to a plan.
type SubscribeResult struct {
	SubscriptionID string `json:"subscription_id"`
	PayURL         string `json:"pay_url"`
	ExpiresAt      string `json:"expires_at"`
}

// Invoice represents a billing invoice.
type Invoice struct {
	ID             string  `json:"id"`
	SubscriptionID *string `json:"subscription_id,omitempty"`
	AmountCents    int64   `json:"amount_cents"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	Provider       string  `json:"provider"`
	ExtInvoiceID   *string `json:"ext_invoice_id,omitempty"`
	PaidAt         *string `json:"paid_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// InvoiceList is the paginated list of invoices.
type InvoiceList struct {
	Invoices []Invoice `json:"invoices"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

// MonitorSummary holds per-status counts for monitors.
type MonitorSummary struct {
	Total  int `json:"total"`
	Up     int `json:"up"`
	Down   int `json:"down"`
	Paused int `json:"paused"`
}

// DashboardSummary is the response from the dashboard summary endpoint.
type DashboardSummary struct {
	Monitors      MonitorSummary `json:"monitors"`
	ChecksToday   int            `json:"checks_today"`
	AvgUptime7d   float64        `json:"avg_uptime_7d"`
	IncidentsOpen int            `json:"incidents_open"`
	AlertsFired7d int            `json:"alerts_fired_7d"`
	StatusPages   int            `json:"status_pages"`
}

// DashboardPins is the response from the dashboard pins endpoint.
type DashboardPins struct {
	MonitorIDs []string `json:"monitor_ids"`
}

// UpdatePinsRequest is the body for updating dashboard pins.
type UpdatePinsRequest struct {
	MonitorIDs []string `json:"monitor_ids"`
}

// SLAMonthEntry holds per-month uptime statistics.
type SLAMonthEntry struct {
	Month        string  `json:"month"`
	UptimePct    float64 `json:"uptime_pct"`
	TotalChecks  int64   `json:"total_checks"`
	FailedChecks int64   `json:"failed_checks"`
}

// SLAMonitorEntry holds SLA data for one monitor.
type SLAMonitorEntry struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Months       []SLAMonthEntry `json:"months"`
	AvgUptimePct float64         `json:"avg_uptime_pct"`
}

// SLAPeriod is the date range covered by an SLA report.
type SLAPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SLAReport is the response from the SLA report endpoint.
type SLAReport struct {
	Period   SLAPeriod         `json:"period"`
	Monitors []SLAMonitorEntry `json:"monitors"`
}
