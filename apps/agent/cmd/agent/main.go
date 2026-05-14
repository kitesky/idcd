// Package main implements the idcd agent node binary.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"` // "sha256:<hex>"
}

func (a *Agent) handleUpgrade(payload json.RawMessage) error {
	var cmd upgradeCmd
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return fmt.Errorf("upgrade: parse payload: %w", err)
	}
	if cmd.DownloadURL == "" {
		return fmt.Errorf("upgrade: download_url is required")
	}

	a.logger.Info("OTA upgrade initiated", "version", cmd.Version, "url", cmd.DownloadURL)

	// 1. Download to temp file alongside current binary
	selfPath, err := exec.LookPath(os.Args[0])
	if err != nil {
		selfPath = os.Args[0]
	}
	selfPath, _ = filepath.Abs(selfPath)
	tmpPath := selfPath + ".new"

	if err := downloadFile(cmd.DownloadURL, tmpPath); err != nil {
		return fmt.Errorf("upgrade: download: %w", err)
	}
	defer os.Remove(tmpPath) // clean up if we abort

	// 2. Verify SHA-256 checksum if provided
	if cmd.Checksum != "" {
		if err := verifySHA256(tmpPath, cmd.Checksum); err != nil {
			return fmt.Errorf("upgrade: checksum mismatch: %w", err)
		}
		a.logger.Info("upgrade: checksum verified")
	}

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

	// 5. Ack before exit so gateway marks command as acked
	_ = a.ws.Send("cmd_ack", map[string]string{"command": "upgrade", "version": cmd.Version})

	// Give the ack time to be flushed, then SIGTERM ourselves.
	// systemd's Restart=always will start the new binary.
	time.AfterFunc(500*time.Millisecond, func() {
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(syscall.SIGTERM)
	})
	return nil
}

func (a *Agent) handleReloadConfig(payload json.RawMessage) error {
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

	_ = a.ws.Send("cmd_ack", map[string]string{"command": "reload_config"})
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
		var results []probe.Result
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

func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec — URL comes from authenticated gateway message
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
	_, err = io.Copy(f, resp.Body)
	return err
}

// verifySHA256 checks that the file at path matches the expected checksum.
// expected format: "sha256:<hex>" or plain hex.
func verifySHA256(path, expected string) error {
	if len(expected) > 7 && expected[:7] == "sha256:" {
		expected = expected[7:]
	}
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
