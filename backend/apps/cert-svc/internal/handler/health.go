package handler

import (
	"context"
	"net/http"
	"time"
)

// healthz is a liveness probe — returns 200 as long as the HTTP server
// can accept and answer a request. No downstream dependency checks here.
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz returns 200 only when every configured downstream Pinger
// responds within a short budget. A nil Pinger is treated as "not
// configured" and skipped, so unit tests can construct a Deps{} without
// real DB/Redis instances.
func readyz(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		checks := map[string]error{}
		if deps.DB != nil {
			checks["db"] = deps.DB.Ping(ctx)
		}
		if deps.Redis != nil {
			checks["redis"] = deps.Redis.Ping(ctx)
		}

		failed := map[string]string{}
		for name, err := range checks {
			if err != nil {
				failed[name] = err.Error()
			}
		}

		if len(failed) > 0 {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status":  "unavailable",
				"failed":  failed,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
