// attest-server is the Evidence/Attestation HTTP API entry point.
//
// S2 MVP wires only the public /healthz endpoint; /verify lands when
// the verify handler agent ships. Generator / Self-Verify workers run
// as separate binaries (cmd/generator, cmd/verifier).
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/kite365/idcd/apps/attest/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "attest-server: load config: %v\n", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	log.Info("attest-server starting",
		"port", cfg.Port,
		"env", cfg.Env,
		"sign_backend", cfg.SignBackend,
		"tsa_providers", cfg.TSAProviders,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: cfg.Addr(), Handler: mux}
	log.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}
