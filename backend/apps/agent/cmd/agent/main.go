// Package main implements the idcd agent node binary.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/buffer"
	"github.com/kite365/idcd/apps/agent/internal/config"
	"github.com/kite365/idcd/apps/agent/internal/fingerprint"
	agentws "github.com/kite365/idcd/apps/agent/internal/ws"
	"github.com/kite365/idcd/apps/agent/internal/probe"
	"github.com/kite365/idcd/apps/agent/internal/task"
	"github.com/kite365/idcd/apps/agent/internal/watermark"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/netfilter"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// Agent represents the main agent process.
type Agent struct {
	cfg      atomic.Pointer[config.Config]
	cfgPath  string
	logger   *slog.Logger
	executor *task.Executor
	buffer   *buffer.Buffer
	client   *http.Client
	ws       *agentws.Client
	// taskCh decouples ws readLoop from probe execution. handleTask only
	// pushes here and returns immediately; a dedicated worker drains it.
	// Otherwise a 30+s traceroute blocks readLoop, gateway's ping never gets
	// a pong, the 60s pongTimeout fires, and the connection churns every
	// ~40-67s — exactly the "ws 1006 abnormal closure" loop we were seeing.
	// Buffered so a transient executor stall doesn't backpressure the ws
	// read; size matches BatchSize ceiling so realistic bursts don't block.
	taskCh   chan task.Task
	shutdown chan struct{}
	wg       sync.WaitGroup
	// cancel cancels the context all worker goroutines listen on.
	// Set by Run, invoked by Stop so wg.Wait() doesn't deadlock waiting
	// for goroutines that only exit on ctx.Done().
	cancel   context.CancelFunc
	// geoCloser holds the open mmdb file handle (non-nil only when
	// GeoIPDBPath loaded successfully). Closed in Stop so the process
	// doesn't leak the fd until exit.
	geoCloser io.Closer
}

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(version())
		os.Exit(0)
	}

	cfgPath := config.DefaultPath()
	cfg := config.MustLoad(cfgPath)

	log := logger.New("production")
	log.Info("starting idcd agent", "node_id", cfg.NodeID, "version", version())

	// Surface raw ICMP availability up-front so a missing CAP_NET_RAW shows
	// in startup logs instead of being silently discovered when the first
	// traceroute task arrives and times out. See M8/M9 in
	// docs/MCP-TEST-REPORT-2026-05-20.md.
	if probe.SupportsRawICMP() {
		log.Info("icmp socket mode: raw (CAP_NET_RAW effective; traceroute + ping full functionality)")
	} else {
		log.Warn("icmp socket mode: dgram fallback (no CAP_NET_RAW) — traceroute will degrade to single-hop TCP reachability; ping still works")
	}

	telCfg := telemetry.Config{
		ServiceName:    "idcd-agent",
		ServiceVersion: version(),
		OTLPEndpoint:   cfg.Observability.Telemetry.OTLPEndpoint,
		SamplingRate:   cfg.Observability.Telemetry.SamplingRate,
		Enabled:        cfg.Observability.Telemetry.Enabled,
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		log.Error("failed to init telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	agent, err := NewAgent(cfg, cfgPath, log)
	if err != nil {
		log.Error("failed to create agent", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go agent.Run(ctx)
	go startHealthServer(cfg.NodeID, agent.ws)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("received shutdown signal, stopping agent")
	agent.Stop()
	log.Info("agent stopped")
}

// NewAgent creates and wires all agent components.
func NewAgent(cfg *config.Config, cfgPath string, log *slog.Logger) (*Agent, error) {
	// GeoIP is optional — if the mmdb file is missing or unreadable we keep
	// running with hops that lack country/city/lat/lng. Missing geo is a
	// degraded experience (no map path), not a probe failure.
	var geo probe.GeoLookup
	var geoCloser io.Closer
	if cfg.GeoIPDBPath != "" {
		g, err := probe.OpenMMDB(cfg.GeoIPDBPath)
		if err != nil {
			log.Warn("geoip mmdb load failed; traceroute hops will have no geo",
				"path", cfg.GeoIPDBPath, "err", err)
		} else {
			geo = g
			geoCloser = g
			log.Info("geoip mmdb loaded", "path", cfg.GeoIPDBPath)
		}
	}

	executor := task.NewExecutor([]byte(cfg.SecretKey), geo)

	buf, err := buffer.New(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("create buffer: %w", err)
	}
	buf.SetLogger(log)

	a := &Agent{
		cfgPath:   cfgPath,
		logger:    log,
		executor:  executor,
		buffer:    buf,
		client:    &http.Client{Timeout: 30 * time.Second},
		taskCh:    make(chan task.Task, 128),
		shutdown:  make(chan struct{}),
		geoCloser: geoCloser,
	}
	a.cfg.Store(cfg)

	// Collect initial fingerprint
	fp, err := fingerprint.Collect()
	if err != nil {
		log.Warn("failed to collect fingerprint", "err", err)
	}

	wsClient := agentws.New(cfg.GatewayURL, cfg.SecretKey, cfg.NodeID, log)
	if fp != nil {
		wsClient.UpdateFingerprint(fp)
	}

	// Register control message handlers
	wsClient.Handle("upgrade",       a.handleUpgrade)
	wsClient.Handle("reload_config", a.handleReloadConfig)
	wsClient.Handle("task",          a.handleTask)
	wsClient.Handle("ack",           a.handleAck)

	a.ws = wsClient
	return a, nil
}

// Run starts all agent goroutines. Derives an internal context from ctx
// so Stop() can cancel all workers without depending on the caller's defer.
func (a *Agent) Run(ctx context.Context) {
	ctx, a.cancel = context.WithCancel(ctx)
	a.wg.Add(3)

	// WebSocket connection loop (includes heartbeat)
	go func() {
		defer a.wg.Done()
		a.ws.Run(ctx)
	}()

	// Periodic result upload + buffer cleanup
	go func() {
		defer a.wg.Done()
		a.runMainLoop(ctx)
	}()

	// Task worker: drains taskCh serially so probe execution never blocks
	// the ws read loop. Single worker keeps ICMP socket / fd pressure
	// bounded; tasks remain ordered which matches the previous (broken)
	// synchronous semantics.
	go func() {
		defer a.wg.Done()
		a.runTaskWorker(ctx)
	}()
}

// Stop gracefully shuts down the agent.
// Order: cancel ctx first so ws.Run and runTaskWorker exit, then close shutdown
// so runMainLoop exits, then wait. Without the cancel, wg.Wait() blocks forever
// because two of the three workers only listen on ctx.Done().
func (a *Agent) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	close(a.shutdown)
	a.wg.Wait()
	if a.geoCloser != nil {
		if err := a.geoCloser.Close(); err != nil {
			a.logger.Warn("geoip close", "err", err)
		}
	}
	if a.buffer != nil {
		a.buffer.Cleanup(24 * time.Hour)
		a.buffer.Close()
	}
}

func (a *Agent) runMainLoop(ctx context.Context) {
	cfg := a.cfg.Load()
	pollInterval, err := time.ParseDuration(cfg.PollInterval)
	if err != nil {
		pollInterval = 30 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.shutdown:
			return
		case <-ticker.C:
			a.uploadPendingResults()
			// Reset ticker if poll interval was changed via hot-reload
			newCfg := a.cfg.Load()
			if newD, _ := time.ParseDuration(newCfg.PollInterval); newD != pollInterval {
				pollInterval = newD
				ticker.Reset(pollInterval)
			}
		}
	}
}

// ── control message handlers ─────────────────────────────────────────────────

type upgradeCmd struct {
	CmdID       string `json:"cmd_id"`       // echoed back in the ack for precise gateway-side update
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"` // "sha256:<hex>"
}

// upgradeAckFlushDelay is how long the agent waits after sending the OTA ack
// before triggering SIGTERM — enough time for the WebSocket frame to be flushed.
const upgradeAckFlushDelay = 500 * time.Millisecond

// upgradeAllowedHosts is the set of hostnames permitted as OTA download sources.
var upgradeAllowedHosts = map[string]bool{
	"releases.idcd.com": true,
	"github.com":        true,
	"objects.githubusercontent.com": true,
}

func (a *Agent) handleUpgrade(payload json.RawMessage) error {
	var cmd upgradeCmd
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("upgrade: parse payload: %w", err)
	}
	if cmd.DownloadURL == "" {
		return fmt.Errorf("upgrade: download_url is required")
	}
	if cmd.Checksum == "" {
		return fmt.Errorf("upgrade: checksum is required — refusing unsigned upgrade")
	}

	// Validate download URL against allowed hosts to prevent SSRF/supply-chain attacks.
	parsedURL, err := url.Parse(cmd.DownloadURL)
	if err != nil {
		return fmt.Errorf("upgrade: invalid download_url: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("upgrade: download_url must use https, got %q", parsedURL.Scheme)
	}
	if !upgradeAllowedHosts[parsedURL.Hostname()] {
		return fmt.Errorf("upgrade: download host %q is not in the allowed list", parsedURL.Hostname())
	}

	a.logger.Info("OTA upgrade initiated", "version", cmd.Version, "url", cmd.DownloadURL)

	// 1. Download to temp file alongside current binary
	selfPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		selfPath = os.Args[0]
	}
	selfPath, _ = filepath.Abs(selfPath)
	tmpPath := selfPath + ".new"

	if err := downloadFile(a.client, cmd.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("upgrade: download: %w", err)
	}
	defer os.Remove(tmpPath) // clean up if we abort

	// 2. Verify SHA-256 checksum (mandatory).
	if err := verifySHA256(tmpPath, cmd.Checksum); err != nil {
		return fmt.Errorf("upgrade: checksum mismatch: %w", err)
	}
	a.logger.Info("upgrade: checksum verified")

	// 3. Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("upgrade: chmod: %w", err)
	}

	// 4. Atomic rename over current binary
	if err := os.Rename(tmpPath, selfPath); err != nil {
		return fmt.Errorf("upgrade: rename: %w", err)
	}

	a.logger.Info("upgrade: binary replaced, sending ack then restarting",
		"version", cmd.Version, "path", selfPath)

	// 5. Ack before exit so gateway marks command as acked.
	// cmd_id allows the gateway to update the exact command row, not just the latest of its type.
	_ = a.ws.Send("cmd_ack", map[string]string{"command": "upgrade", "cmd_id": cmd.CmdID, "version": cmd.Version})

	// Give the ack time to be flushed, then SIGTERM ourselves.
	// systemd's Restart=always will start the new binary.
	time.AfterFunc(upgradeAckFlushDelay, func() {
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGTERM)
	})
	return nil
}

type reloadConfigCmd struct {
	CmdID string `json:"cmd_id"`
}

func (a *Agent) handleReloadConfig(payload json.RawMessage) error {
	var cmd reloadConfigCmd
	_ = json.Unmarshal(payload, &cmd)

	a.logger.Info("config hot-reload requested")

	newCfg, err := config.Load(a.cfgPath)
	if err != nil {
		return fmt.Errorf("reload_config: load: %w", err)
	}

	a.cfg.Store(newCfg)

	// Re-collect fingerprint in case hostname/interfaces changed
	if fp, err := fingerprint.Collect(); err == nil {
		a.ws.UpdateFingerprint(fp)
	}

	a.logger.Info("config reloaded",
		"poll_interval", newCfg.PollInterval,
		"batch_size", newCfg.BatchSize)

	_ = a.ws.Send("cmd_ack", map[string]string{"command": "reload_config", "cmd_id": cmd.CmdID})
	return nil
}

func (a *Agent) handleTask(payload json.RawMessage) error {
	var t task.Task
	if err := json.Unmarshal(payload, &t); err != nil {
		return fmt.Errorf("task: parse: %w", err)
	}

	// SSRF / metadata-IP guard: a malicious gateway or compromised admin could
	// otherwise have every agent simultaneously dial 169.254.169.254 (cloud
	// metadata service) and exfiltrate IAM credentials. Refuse the task
	// before any DNS / TCP / HTTP / SMTP I/O happens against task.Target.
	if blocked, reason := checkTaskTarget(t); blocked {
		a.logger.Warn("task: target blocked by netfilter",
			"task_id", t.ID, "type", t.Type, "target", t.Target, "reason", reason)
		result := a.failedTaskResult(t, "target blocked by netfilter: "+reason)
		a.deliverResult(result)
		return nil
	}

	// Hand off to the worker — must NOT call Execute inline, that would
	// block the ws readLoop and starve gateway's ping/pong (60s timeout)
	// for the duration of the probe (traceroute hits 30+s easily).
	select {
	case a.taskCh <- t:
	default:
		// Queue full: surface as a fast failure so the user sees something
		// instead of the task silently disappearing. The worker is healthy
		// but we're being asked faster than we can probe.
		a.logger.Warn("task: queue full, dropping",
			"task_id", t.ID, "type", t.Type, "queue_len", len(a.taskCh))
		a.deliverResult(a.failedTaskResult(t, "agent overloaded: task queue full"))
	}
	return nil
}

// runTaskWorker pulls tasks off taskCh and executes them serially.
// Exits when ctx is cancelled — taskCh is drained from upstream (no more
// pushes once ws closes), so a partial drain on shutdown is acceptable.
//
// Each task is executed inside a deferred recover so one probe panicking
// (nil-pointer, oob slice, etc.) doesn't kill the only worker. Without
// the recover, the goroutine dies, taskCh fills to 128, every subsequent
// task fails with "agent overloaded", and the agent stays in that broken
// state until the process is restarted.
func (a *Agent) runTaskWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-a.taskCh:
			a.executeTask(t)
		}
	}
}

func (a *Agent) executeTask(t task.Task) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error("task: probe panicked",
				"task_id", t.ID, "type", t.Type, "target", t.Target, "panic", r)
			a.deliverResult(a.failedTaskResult(t, "internal error: probe panicked"))
		}
	}()
	result := a.executor.Execute(t)
	if result != nil {
		// Echo monitor_id back so aggregator can write monitor_checks + advance
		// schedule. probe.Execute doesn't know about monitors — task layer carries
		// the binding from scheduler through gateway to result stream.
		result.MonitorID = t.MonitorID
		a.deliverResult(*result)
	}
}

// deliverResult tries to ship the result over WebSocket immediately so users
// see tool-page output within seconds of the probe completing. Earlier
// versions only wrote to the SQLite buffer and waited on the 30s upload
// ticker — that one design choice turned every "click ping → see result"
// flow into a 30-60s lag (worst-case 200s+ with re-buffer / network jitter)
// and was the primary cause of users seeing "等待节点返回结果..." forever.
// Falls back to the buffer on send failure so disconnected agents still
// retry on reconnect.
func (a *Agent) deliverResult(r probe.Result) {
	sendErr := a.ws.Send("result", []probe.Result{r})
	if sendErr == nil {
		return
	}
	a.logger.Warn("task: live upload failed, buffering for retry",
		"task_id", r.TaskID, "err", sendErr)
	if err := a.buffer.Store(r); err != nil {
		a.logger.Error("task: buffer store after live-upload failure",
			"task_id", r.TaskID, "err", err)
	}
}

// checkTaskTarget validates task.Target against the netfilter SSRF policy.
// Returns (true, reason) when the task must be rejected. Per-task allow-list
// CIDRs may be supplied via options["allow_cidrs"]:[]string — useful for
// speedtest servers whose operator pins the server set in advance.
func checkTaskTarget(t task.Task) (bool, string) {
	host := extractHost(string(t.Type), t.Target)
	if host == "" {
		return true, "empty target"
	}
	cfg := netfilter.Config{}
	if cidrs := allowCIDRsFromOptions(t.Options); len(cidrs) > 0 {
		nets, err := netfilter.ParseAllowList(cidrs)
		if err != nil {
			return true, "invalid allow_cidrs in task options: " + err.Error()
		}
		cfg.AllowList = nets
	}
	return netfilter.IsBlockedWith(host, cfg)
}

// extractHost strips scheme / path / port from a probe target so netfilter
// sees only the hostname or IP. Mirrors the input shapes the various probes
// accept (DNS gets bare hostname, HTTP gets full URL, TCP/SMTP get host:port,
// etc.).
func extractHost(taskType, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	// URL form: http(s)://host[:port]/path
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if u, err := url.Parse(target); err == nil && u.Host != "" {
			return u.Hostname()
		}
	}
	// Bracketed IPv6 literal, optionally with port: [::1]:80
	if strings.HasPrefix(target, "[") {
		if host, _, err := net.SplitHostPort(target); err == nil {
			return host
		}
		// Bare [::1] without port.
		return strings.Trim(target, "[]")
	}
	// host:port (IPv4 or hostname) — exactly one colon.
	if strings.Count(target, ":") == 1 {
		if host, _, err := net.SplitHostPort(target); err == nil {
			return host
		}
	}
	// Otherwise: bare IP (v4/v6) or bare hostname.
	return target
}

// allowCIDRsFromOptions extracts ["allow_cidrs"] from a task's options map.
// Accepts []string or []any (JSON decodes string arrays into []any when the
// surrounding map is map[string]any).
func allowCIDRsFromOptions(opts map[string]any) []string {
	v, ok := opts["allow_cidrs"]
	if !ok {
		return nil
	}
	switch xs := v.(type) {
	case []string:
		return xs
	case []any:
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// failedTaskResult builds a signed Result describing a refused task. It mirrors
// the executor's "unsupported task type" default branch so downstream
// aggregation handles this uniformly.
func (a *Agent) failedTaskResult(t task.Task, errMsg string) probe.Result {
	timestamp := time.Now()
	cfg := a.cfg.Load()
	result := probe.Result{
		TaskID:     t.ID,
		NodeID:     t.NodeID,
		MonitorID:  t.MonitorID,
		Type:       t.Type,
		Target:     t.Target,
		Success:    false,
		Error:      errMsg,
		Data:       map[string]any{},
		Timestamp:  timestamp,
		DurationMs: 0,
	}
	result.Watermark = watermark.Sign(t.NodeID, t.ID, t.Target, timestamp, []byte(cfg.SecretKey))
	return result
}

func (a *Agent) handleAck(payload json.RawMessage) error {
	a.logger.Debug("gateway ack received", "payload", string(payload))
	return nil
}

// ── result upload ─────────────────────────────────────────────────────────────

func (a *Agent) uploadPendingResults() {
	pending, err := a.buffer.Pending()
	if err != nil {
		a.logger.Error("buffer: list pending", "err", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	cfg := a.cfg.Load()
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < len(pending); i += batchSize {
		end := i + batchSize
		if end > len(pending) {
			end = len(pending)
		}
		batch := pending[i:end]
		results := make([]probe.Result, 0, len(batch))
		for _, r := range batch {
			results = append(results, r.Result)
		}
		if err := a.ws.Send("result", results); err != nil {
			a.logger.Warn("upload: send failed", "err", err)
			return
		}
		for _, r := range batch {
			a.buffer.MarkSent(r.ID)
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

const maxBinarySize = 100 << 20 // 100 MB — sanity guard against disk exhaustion

func downloadFile(httpClient *http.Client, url, dest string) error {
	resp, err := httpClient.Get(url) //nolint:gosec — URL comes from authenticated gateway message
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, maxBinarySize))
	return err
}

// verifySHA256 checks that the file at path matches the expected checksum.
// expected format: "sha256:<hex>" or plain hex.
func verifySHA256(path, expected string) error {
	expected, _ = strings.CutPrefix(expected, "sha256:")
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != expected {
		return fmt.Errorf("got %s, want %s", got, expected)
	}
	return nil
}

func version() string {
	// Replaced at build time via -ldflags "-X main.buildVersion=x.y.z"
	if buildVersion != "" {
		return buildVersion
	}
	return "dev"
}

// buildVersion is set via ldflags at build time.
var buildVersion string

// startHealthServer runs a minimal HTTP health endpoint on :9101
// (or AGENT_HEALTH_PORT if set). Used by idcd-agent-ctl and the systemd
// unit's ExecStartPost smoke-check.
//
// Loopback-only bind: the agent is outbound-only, and /health returns
// node_id + version. Container orchestration probes should run via a
// sidecar or unix-socket exporter rather than rebinding 0.0.0.0.
func startHealthServer(nodeID string, ws *agentws.Client) {
	port := os.Getenv("AGENT_HEALTH_PORT")
	if port == "" {
		port = "9101"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","node_id":%q,"version":%q}`, nodeID, version())
	})
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ws.IsConnected() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "websocket not connected")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	srv := &http.Server{
		// Bind to loopback only — the health endpoint is for local tools (idcd-agent-ctl)
		// and should not be reachable from other hosts on the network.
		Addr:        "127.0.0.1:" + port,
		Handler:     mux,
		ReadTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// Non-fatal: health endpoint failure should not crash the agent
		_ = err
	}
}
