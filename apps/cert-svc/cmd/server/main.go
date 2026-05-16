// server is the cert-svc HTTP API entry point.
//
// S1 W1: routes are mounted and return 501. Health + readiness probes
// are real. DB / Redis wiring is deferred to W2 — connecting them here
// would fail-fast in CI before the ACME flow even exists.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/apps/cert-svc/internal/handler"
	"github.com/kite365/idcd/lib/shared/logger"
	"github.com/kite365/idcd/lib/shared/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("cert-svc: load config: %v", err)
	}

	slogLogger := logger.New(cfg.Env)
	slogLogger.Info("cert-svc starting",
		"port", cfg.Port,
		"env", cfg.Env,
		"log_level", cfg.LogLevel,
	)

	// Telemetry is best-effort: a failure here should not block boot,
	// but we do log so an operator can spot it in the journal.
	telCfg := telemetry.Config{
		ServiceName:    "idcd-cert-svc",
		ServiceVersion: "v0.1.0",
		SamplingRate:   0.1,
		Enabled:        false, // flip via config once W2 wires OTLP endpoint
	}
	shutdownTelemetry, err := telemetry.Init(telCfg)
	if err != nil {
		slogLogger.Warn("telemetry init failed; continuing without traces", "error", err)
		shutdownTelemetry = func(context.Context) error { return nil }
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTelemetry(ctx)
	}()

	// Service is initialised lazily by the worker; the server only
	// exposes the orchestrator surface for manual-mode confirmation in
	// W3. For W2 we leave Deps.Service nil — handlers nil-check.
	router := handler.New(handler.Deps{})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slogLogger.Info("cert-svc listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			slogLogger.Error("cert-svc server failed", "error", err)
			os.Exit(1)
		}
	case sig := <-sigChan:
		slogLogger.Info("cert-svc received shutdown signal", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slogLogger.Error("cert-svc graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slogLogger.Info("cert-svc shutdown complete")
}
