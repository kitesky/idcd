package handler

import (
	"log/slog"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/middleware"
)

// auditLog emits a structured INFO-level log line for sensitive account /
// team actions (password changes, 2FA toggles, member removal, role changes).
//
// The event name is namespaced "audit.<area>.<action>" — e.g. "audit.password.changed".
// Log aggregation can filter by either the constant "category=audit" attr or
// the event prefix. Pair with extra attrs for the action's payload
// (target_user_id, team_id, etc.).
//
// Intentionally uses slog.Default() so handlers don't need an injected logger;
// the package-level default is configured by main() before any handler runs.
// Failure to log is not a fatal condition — never block the request on audit
// I/O — so this helper has no error return.
func auditLog(r *http.Request, event string, attrs ...any) {
	actorID := middleware.UserIDFromContext(r.Context())
	sessionID := middleware.SessionIDFromContext(r.Context())
	base := []any{
		"category", "audit",
		"event", event,
		"actor_user_id", actorID,
		"session_id", sessionID,
		"ip", middleware.ClientIP(r),
		"user_agent", r.UserAgent(),
	}
	slog.Default().Info("audit", append(base, attrs...)...)
}
