package response

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/kite365/idcd/apps/api/internal/errcode"
	shared "github.com/kite365/idcd/lib/shared/i18n"
	"github.com/kite365/idcd/lib/shared/logger"
)

// CodedErrorResponse is the new error envelope used by handlers that have
// migrated to the errcode + catalog flow. It carries the wire-stable code,
// the localized message, the rendering params (for client-side overrides)
// and the request id — same shape the existing ErrorResponse uses but with
// an additional Params field.
//
// The legacy ErrorResponse / Error() helper remains untouched; new
// handlers should call ErrorCode and old ones can migrate incrementally.
type CodedErrorResponse struct {
	Error     CodedErrorDetail `json:"error"`
	RequestID string           `json:"request_id"`
}

// CodedErrorDetail is the inner body of CodedErrorResponse.
type CodedErrorDetail struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Params    map[string]any `json:"params,omitempty"`
	RequestID string         `json:"request_id"`
}

// translator is the minimal slice of *i18n.Catalog ErrorCode needs. Defined
// as an interface so tests / callers can inject a stub without depending on
// the singleton.
type translator interface {
	T(locale, key string, params map[string]any) string
}

// catalogProvider is overridable for tests. The default delegates to the
// shared singleton; tests can swap it via SetCatalogForTesting.
var catalogProvider = func() translator { return shared.MustDefaultCatalog() }

// SetCatalogForTesting overrides the catalog used by ErrorCode. Returns a
// restore function that the test should defer-call to undo the swap.
func SetCatalogForTesting(t translator) func() {
	prev := catalogProvider
	catalogProvider = func() translator { return t }
	return func() { catalogProvider = prev }
}

// ErrorCode writes a localized error response built from the given errcode.
// The message is rendered by the i18n catalog using the locale stored on
// ctx (via shared.WithLocale); when no locale is present the catalog falls
// back to the registry default.
//
// Handlers should pass the original request so request_id can be picked up
// from headers / context. Params are forwarded verbatim into the response
// body so clients that prefer to re-render their own translation have the
// inputs available.
func ErrorCode(ctx context.Context, w http.ResponseWriter, r *http.Request, code errcode.Code, params map[string]any) {
	requestID := getRequestID(r)
	locale := shared.FromContextWith(ctx, shared.MustDefault())
	cat := catalogProvider()

	message := cat.T(locale, "errors."+string(code), params)
	status := errcode.HTTPStatus(code)

	resp := CodedErrorResponse{
		Error: CodedErrorDetail{
			Code:      string(code),
			Message:   message,
			Params:    params,
			RequestID: requestID,
		},
		RequestID: requestID,
	}

	log := logger.FromContext(ctx, logger.New("production"))
	log.Error("API coded error response",
		"code", code,
		"status", status,
		"locale", locale,
		"request_id", requestID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error("failed to encode coded error response", "error", err, "request_id", requestID)
		// Best-effort plain fallback; status is already written so we can't
		// reset it. The body fallback at least carries the code.
		_, _ = w.Write([]byte(`{"error":{"code":"` + string(code) + `","message":"","request_id":"` + requestID + `"},"request_id":"` + requestID + `"}`))
	}
}
