// worker is the cert-svc ACME orchestrator.
//
// S1 W1: this is a heartbeat-only placeholder. The real Redis-Stream
// consumer + ACME state machine ship in W2; keeping a runnable binary
// now means deploy manifests and CI builds can be reviewed end-to-end.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kite365/idcd/apps/cert-svc/internal/config"
	"github.com/kite365/idcd/lib/shared/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// We deliberately use the stdlib logger before slog is ready —
		// the config error is the only thing that can fail this early.
		panic("cert-worker: load config: " + err.Error())
	}
	log := logger.New(cfg.Env)
	log.Info("cert-worker started", "env", cfg.Env)

	ctx, cancel := signalContext()
	defer cancel()

	// Tick once a minute purely so an operator can confirm liveness in
	// the journal. W2 will replace this with `redis.XReadGroup`.
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("cert-worker shutting down")
			return
		case <-t.C:
			log.Debug("cert-worker idle tick")
		}
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}
