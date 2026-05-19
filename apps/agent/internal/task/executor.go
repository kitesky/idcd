// Package task provides task execution logic for the agent.
package task

import (
	"fmt"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/probe"
	"github.com/kite365/idcd/apps/agent/internal/watermark"
)

// Executor manages the execution of probe tasks.
type Executor struct {
	httpProbe       *probe.HTTPProbe
	pingProbe       *probe.PingProbe
	tcpProbe        *probe.TCPProbe
	dnsProbe        *probe.DNSProbe
	tracerouteProbe *probe.TracerouteProbe
	smtpProbe       *probe.SMTPProbe
	ntpProbe        *probe.NTPProbe
	mtrProbe        *probe.MTRProbe
	speedtestProbe  *probe.SpeedtestProbe
	secretKey       []byte
}

// NewExecutor creates a new task executor with the given secret key for watermarking.
// geo may be nil — traceroute / MTR hops will simply omit country/city/lat/lng.
func NewExecutor(secretKey []byte, geo probe.GeoLookup) *Executor {
	pingProbe := &probe.PingProbe{}
	// MTR needs an explicit sender. PingProbe.Sender is lazily initialised on
	// first Execute(), so reading it during NewExecutor would always be nil —
	// which silently locked MTR into its single-ping-per-hop fallback path.
	// Inject ICMPPingSender directly: it tries the unprivileged datagram
	// socket first (listenICMP4), so it works on macOS without root and on
	// Linux without CAP_NET_RAW provided ping_group_range covers the agent.
	return &Executor{
		httpProbe:       &probe.HTTPProbe{},
		pingProbe:       pingProbe,
		tcpProbe:        &probe.TCPProbe{},
		dnsProbe:        &probe.DNSProbe{},
		tracerouteProbe: &probe.TracerouteProbe{Geo: geo},
		smtpProbe:       &probe.SMTPProbe{},
		ntpProbe:        &probe.NTPProbe{},
		mtrProbe:        &probe.MTRProbe{Sender: &probe.ICMPPingSender{}, Geo: geo},
		speedtestProbe:  &probe.SpeedtestProbe{},
		secretKey:       secretKey,
	}
}

// Execute runs the given task and returns a signed result.
func (e *Executor) Execute(task Task) *probe.Result {
	// Set default timeout if not specified
	timeout := task.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	var result *probe.Result

	// Route to appropriate probe based on task type
	switch task.Type {
	case TaskHTTP:
		result = e.httpProbe.Execute(task.Target, timeout, task.Options)
	case TaskPing:
		result = e.pingProbe.Execute(task.Target, timeout, task.Options)
	case TaskTCP:
		result = e.tcpProbe.Execute(task.Target, timeout, task.Options)
	case TaskDNS:
		result = e.dnsProbe.Execute(task.Target, timeout, task.Options)
	case TaskTraceroute:
		result = e.tracerouteProbe.Execute(task.Target, timeout, task.Options)
	case TaskSMTP:
		result = e.smtpProbe.Execute(task.Target, timeout, task.Options)
	case TaskNTP:
		result = e.ntpProbe.Execute(task.Target, timeout, task.Options)
	case TaskMTR:
		result = e.mtrProbe.Execute(task.Target, timeout, task.Options)
	case TaskSpeedtest:
		result = e.speedtestProbe.Execute(task.Target, timeout, task.Options)
	default:
		timestamp := time.Now()
		result = &probe.Result{
			TaskID:     task.ID,
			NodeID:     task.NodeID,
			Type:       task.Type,
			Target:     task.Target,
			Success:    false,
			Error:      fmt.Sprintf("unsupported task type: %s", task.Type),
			Data:       map[string]any{},
			Timestamp:  timestamp,
			DurationMs: 0,
		}
		result.Watermark = watermark.Sign(
			task.NodeID,
			task.ID,
			task.Target,
			timestamp,
			e.secretKey,
		)
		return result
	}

	// Fill in common result fields
	result.TaskID = task.ID
	result.NodeID = task.NodeID
	result.Type = task.Type
	result.Target = task.Target

	// Generate watermark
	result.Watermark = watermark.Sign(
		task.NodeID,
		task.ID,
		task.Target,
		result.Timestamp,
		e.secretKey,
	)

	return result
}

// SetPingProbe allows injecting a custom ping probe (for testing).
func (e *Executor) SetPingProbe(p *probe.PingProbe) {
	e.pingProbe = p
}

// SetDNSProbe allows injecting a custom DNS probe (for testing).
func (e *Executor) SetDNSProbe(p *probe.DNSProbe) {
	e.dnsProbe = p
}

// ExecuteBatch executes multiple tasks and returns their results.
func (e *Executor) ExecuteBatch(tasks []Task) []*probe.Result {
	results := make([]*probe.Result, 0, len(tasks))

	for _, task := range tasks {
		result := e.Execute(task)
		results = append(results, result)
	}

	return results
}