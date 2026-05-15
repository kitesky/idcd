package probe

import (
	"context"
	"net"
	"time"
)

// MTRProbe executes MTR (traceroute + per-hop ping) probes.
type MTRProbe struct {
	Sender PingSender
}

// MTRHop represents a single MTR hop with ping statistics.
type MTRHop struct {
	Hop      int     `json:"hop"`
	IP       string  `json:"ip"`
	Hostname string  `json:"hostname,omitempty"`
	SentPkts int     `json:"sent_pkts"`
	RecvPkts int     `json:"recv_pkts"`
	Loss     float64 `json:"loss_pct"`
	AvgRTTMs float64 `json:"avg_rtt_ms"`
	MinRTTMs float64 `json:"min_rtt_ms"`
	MaxRTTMs float64 `json:"max_rtt_ms"`
	Timeout  bool    `json:"timeout,omitempty"`
}

// Execute runs MTR: traceroute first to discover hops, then pings each hop 3 times.
func (p *MTRProbe) Execute(target string, timeout time.Duration, options map[string]any) *Result {
	start := time.Now()

	tr := &TracerouteProbe{}
	trResult := tr.Execute(target, timeout, options)

	hops, _ := trResult.Data["hops"].([]TracerouteHop)
	if len(hops) == 0 {
		return &Result{
			Type:       TaskMTR,
			Target:     target,
			Success:    false,
			Error:      "traceroute returned no hops",
			Data:       map[string]any{"hops": []MTRHop{}},
			DurationMs: time.Since(start).Milliseconds(),
			Timestamp:  time.Now(),
		}
	}

	const pingsPerHop = 3
	const rdnsTimeout = 1 * time.Second
	pingTimeout := 2 * time.Second

	mtrHops := make([]MTRHop, 0, len(hops))

	for _, h := range hops {
		mh := MTRHop{
			Hop:     h.Hop,
			IP:      h.IP,
			Timeout: h.Timeout,
		}

		if h.Timeout || h.IP == "" || h.IP == "*" {
			mh.SentPkts = pingsPerHop
			mh.Loss = 100.0
			mtrHops = append(mtrHops, mh)
			continue
		}

		if h.Hostname != "" {
			mh.Hostname = h.Hostname
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), rdnsTimeout)
			if names, err := net.DefaultResolver.LookupAddr(ctx, h.IP); err == nil && len(names) > 0 {
				mh.Hostname = names[0]
			}
			cancel()
		}

		mh.SentPkts = pingsPerHop

		if p.Sender != nil {
			stats, err := p.Sender.SendPing(h.IP, pingTimeout, pingsPerHop)
			if err == nil {
				mh.RecvPkts = stats.PacketsReceived
				mh.Loss = stats.PacketLoss
				mh.AvgRTTMs = msFloat(stats.AvgRTT)
				mh.MinRTTMs = msFloat(stats.MinRTT)
				mh.MaxRTTMs = msFloat(stats.MaxRTT)
			} else {
				mh.Loss = 100.0
			}
		} else {
			mh.RecvPkts = 1
			rttMs := msFloat(h.RTT)
			mh.AvgRTTMs = rttMs
			mh.MinRTTMs = rttMs
			mh.MaxRTTMs = rttMs
		}

		mtrHops = append(mtrHops, mh)
	}

	return &Result{
		Type:    TaskMTR,
		Target:  target,
		Success: true,
		Data: map[string]any{
			"hops":           mtrHops,
			"total_hops":     len(mtrHops),
			"target_reached": trResult.Success,
		},
		DurationMs: time.Since(start).Milliseconds(),
		Timestamp:  time.Now(),
	}
}

// msFloat converts a time.Duration to milliseconds as a float64.
func msFloat(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
