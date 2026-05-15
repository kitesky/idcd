// Package probe implements various network probing capabilities.
package probe

import (
	"time"
)

// TaskType represents the type of probe to execute.
type TaskType string

const (
	TaskHTTP       TaskType = "http"
	TaskPing       TaskType = "ping"
	TaskTCP        TaskType = "tcp"
	TaskDNS        TaskType = "dns"
	TaskTraceroute TaskType = "traceroute"
	TaskSMTP       TaskType = "smtp"
	TaskNTP        TaskType = "ntp"
	TaskMTR        TaskType = "mtr"
)

// Result represents the outcome of a probe execution.
type Result struct {
	TaskID     string            `json:"task_id"`
	NodeID     string            `json:"node_id"`
	Type       TaskType          `json:"type"`
	Target     string            `json:"target"`
	Success    bool              `json:"success"`
	Data       map[string]any    `json:"data"`        // probe-specific results
	Error      string            `json:"error,omitempty"`
	Watermark  string            `json:"watermark"`   // HMAC-SHA256 signature
	Timestamp  time.Time         `json:"timestamp"`
	DurationMs int64             `json:"duration_ms"`
}

// Probe defines the interface for executing network probes.
type Probe interface {
	Execute(target string, timeout time.Duration, options map[string]any) *Result
}

// HTTPProbe executes HTTP/HTTPS probes.
type HTTPProbe struct{}

// PingProbe executes ICMP ping probes.
type PingProbe struct {
	Sender PingSender
}

// TCPProbe executes TCP connection probes.
type TCPProbe struct{}

// DNSProbe executes DNS resolution probes.
type DNSProbe struct {
	Resolver DNSResolver
}

// TracerouteProbe executes traceroute probes.
type TracerouteProbe struct{}

// SMTPProbe executes SMTP banner/EHLO connection probes.
type SMTPProbe struct{}

// NTPProbe executes NTP server time queries.
type NTPProbe struct{}

// PingSender interface allows mocking ICMP operations in tests.
type PingSender interface {
	SendPing(target string, timeout time.Duration, count int) (PingStats, error)
}

// PingStats holds ping statistics.
type PingStats struct {
	PacketsSent     int
	PacketsReceived int
	PacketLoss      float64 // percentage
	MinRTT          time.Duration
	AvgRTT          time.Duration
	MaxRTT          time.Duration
	StdDevRTT       time.Duration
}

// DNSResolver interface allows mocking DNS operations in tests.
type DNSResolver interface {
	LookupHost(name string) ([]string, error)
	LookupMX(name string) ([]MXRecord, error)
	LookupTXT(name string) ([]string, error)
	LookupCNAME(name string) (string, error)
	LookupNS(name string) ([]string, error)
}

// MXRecord represents a DNS MX record.
type MXRecord struct {
	Host     string
	Priority uint16
}

// TracerouteHop represents a single hop in a traceroute.
type TracerouteHop struct {
	Hop      int           `json:"hop"`
	IP       string        `json:"ip"`
	Hostname string        `json:"hostname,omitempty"`
	RTT      time.Duration `json:"rtt_ms"`
	Timeout  bool          `json:"timeout"`
}