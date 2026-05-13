// Package main implements the idcd agent node binary.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/kite365/idcd/apps/agent/internal/buffer"
	"github.com/kite365/idcd/apps/agent/internal/config"
	"github.com/kite365/idcd/apps/agent/internal/probe"
	"github.com/kite365/idcd/apps/agent/internal/task"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

// Agent represents the main agent process.
type Agent struct {
	cfg      *config.Config
	logger   *slog.Logger
	executor *task.Executor
	buffer   *buffer.Buffer
	client   *http.Client
	shutdown chan struct{}
	wg       sync.WaitGroup
}

func main() {
	// Load configuration
	cfg := config.MustLoad(config.DefaultPath())

	// Initialize logger
	log := logger.New("production") // Agent runs in production mode by default
	log.Info("starting idcd agent", "node_id", cfg.NodeID, "version", "1.0")

	// Initialize OpenTelemetry
	telCfg := telemetry.Config{
		ServiceName:    "idcd-agent",
		ServiceVersion: "v1.0.0",
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

	// Create agent instance
	agent, err := NewAgent(cfg, log)
	if err != nil {
		log.Error("failed to create agent", "error", err)
		os.Exit(1)
	}

	// Start agent
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go agent.Run(ctx)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("received shutdown signal, stopping agent")

	// Graceful shutdown
	agent.Stop()
	log.Info("agent stopped successfully")
}

// NewAgent creates a new agent instance.
func NewAgent(cfg *config.Config, log *slog.Logger) (*Agent, error) {
	// Create task executor
	executor := task.NewExecutor([]byte(cfg.SecretKey))

	// Create result buffer
	buf, err := buffer.New(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("create buffer: %w", err)
	}

	// Create HTTP client for gateway communication
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &Agent{
		cfg:      cfg,
		logger:   log,
		executor: executor,
		buffer:   buf,
		client:   client,
		shutdown: make(chan struct{}),
	}, nil
}

// Run starts the agent main loop.
func (a *Agent) Run(ctx context.Context) {
	defer a.wg.Done()
	a.wg.Add(1)

	// Parse poll interval
	pollInterval, err := time.ParseDuration(a.cfg.PollInterval)
	if err != nil {
		a.logger.Error("invalid poll interval", "interval", a.cfg.PollInterval, "error", err)
		pollInterval = 30 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	a.logger.Info("agent started", "poll_interval", pollInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.shutdown:
			return
		case <-ticker.C:
			a.processCycle()
		}
	}
}

// Stop gracefully stops the agent.
func (a *Agent) Stop() {
	close(a.shutdown)
	a.wg.Wait()

	// Close buffer
	if a.buffer != nil {
		a.buffer.Close()
	}

	// Clean up any pending results older than 24 hours
	if a.buffer != nil {
		a.buffer.Cleanup(24 * time.Hour)
	}
}

// processCycle executes one cycle of the agent's main loop.
func (a *Agent) processCycle() {
	// First, try to send any pending buffered results
	a.uploadPendingResults()

	// Then, fetch and process new tasks
	a.fetchAndProcessTasks()
}

// uploadPendingResults attempts to upload any buffered results to the gateway.
func (a *Agent) uploadPendingResults() {
	pending, err := a.buffer.Pending()
	if err != nil {
		a.logger.Error("failed to get pending results", "error", err)
		return
	}

	if len(pending) == 0 {
		return
	}

	a.logger.Debug("uploading pending results", "count", len(pending))

	// Send in batches
	for i := 0; i < len(pending); i += a.cfg.BatchSize {
		end := i + a.cfg.BatchSize
		if end > len(pending) {
			end = len(pending)
		}

		batch := pending[i:end]
		if a.uploadResultBatch(batch) {
			// Mark as sent only if upload was successful
			for _, result := range batch {
				a.buffer.MarkSent(result.ID)
			}
		}
	}
}

// uploadResultBatch uploads a batch of results to the gateway.
func (a *Agent) uploadResultBatch(results []buffer.PendingResult) bool {
	// Convert to slice of probe.Result
	var payload []probe.Result
	for _, r := range results {
		payload = append(payload, r.Result)
	}

	// Encode as JSON
	data, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("failed to marshal results", "error", err)
		return false
	}

	// TODO: Implement actual HTTP upload to gateway
	// For now, just log that we would upload
	a.logger.Debug("would upload results to gateway",
		"count", len(results),
		"gateway_url", a.cfg.GatewayURL,
		"size_bytes", len(data))

	// Return true to simulate successful upload for now
	return true
}

// fetchAndProcessTasks fetches new tasks from the gateway and executes them.
func (a *Agent) fetchAndProcessTasks() {
	// TODO: Implement actual task fetching from gateway
	// For now, just log that we would fetch tasks
	a.logger.Debug("would fetch tasks from gateway", "gateway_url", a.cfg.GatewayURL)

	// Simulate processing some tasks for demonstration
	tasks := a.generateSampleTasks()
	if len(tasks) > 0 {
		a.logger.Debug("processing tasks", "count", len(tasks))
		results := a.executor.ExecuteBatch(tasks)

		// Store results in buffer
		for _, result := range results {
			if err := a.buffer.Store(*result); err != nil {
				a.logger.Error("failed to store result", "task_id", result.TaskID, "error", err)
			}
		}

		a.logger.Debug("stored task results", "count", len(results))
	}
}

// generateSampleTasks creates some sample tasks for testing.
func (a *Agent) generateSampleTasks() []task.Task {
	// Return empty slice for production - this is just for testing
	return []task.Task{}
}