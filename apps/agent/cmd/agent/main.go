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
	"github.com/kite365/idcd/lib/shared/logger"
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
	shutdown chan struct{}
	wg       sync.WaitGroup
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
	go startHealthServer(cfg.NodeID)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("received shutdown signal, stopping agent")
	agent.Stop()
	log.Info("agent stopped")
}

// NewAgent creates and wires all agent components.
func NewAgent(cfg *config.Config, cfgPath string, log *slog.Logger) (*Agent, error) {
	executor := task.NewExecutor([]byte(cfg.SecretKey))

	buf, err := buffer.New(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("create buffer: %w", err)
	}

	a := &Agent{
		cfgPath:  cfgPath,
		logger:   log,
		executor: executor,
		buffer:   buf,
		client:   &http.Client{Timeout: 30 * time.Second},
		shutdown: make(chan struct{}),
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

// Run starts all agent goroutines.
func (a *Agent) Run(ctx context.Context) {
	a.wg.Add(2)

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
}

// Stop gracefully shuts down the agent.
func (a *Agent) Stop() {
	close(a.shutdown)
	a.wg.Wait()
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
	result := a.executor.Execute(t)
	if result != nil {
		if err := a.buffer.Store(*result); err != nil {
			a.logger.Error("task: store result", "task_id", t.ID, "err", err)
		}
	}
	return nil
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
// (or AGENT_HEALTH_PORT if set). Used by idcd-agent-ctl and monitoring.
func startHealthServer(nodeID string) {
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
