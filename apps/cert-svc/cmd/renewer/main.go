// renewer is the cert-svc renewal scheduler.
//
// S1 W1: hourly tick that logs "scan tick". W2 will replace the body
// with the SELECT in PRD §7.3 and enqueue renewal_jobs.
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

const renewerInterval = time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("cert-renewer: load config: " + err.Error())
	}
	log := logger.New(cfg.Env)
	log.Info("cert-renewer started", "env", cfg.Env, "interval", renewerInterval.String())

	ctx, cancel := signalContext()
	defer cancel()

	t := time.NewTicker(renewerInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("cert-renewer shutting down")
			return
		case <-t.C:
			log.Info("scan tick")
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
