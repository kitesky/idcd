package response

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kite365/idcd/apps/api/internal/errcode"
	shared "github.com/kite365/idcd/lib/shared/i18n"
)

// stubTranslator records the last call and returns a canned message keyed
// on locale + key so tests can assert ErrorCode picked the right locale.
type stubTranslator struct {
	calls []stubCall
	out   string
}

type stubCall struct {
	locale string
	key    string
	params map[string]any
}

func (s *stubTranslator) T(locale, key string, params map[string]any) string {
	s.calls = append(s.calls, stubCall{locale: locale, key: key, params: params})
	return s.out
}

func TestErrorCodeBasic(t *testing.T) {
	stub := &stubTranslator{out: "请先登录"}
	restore := SetCatalogForTesting(stub)
	defer restore()

	req := httptest.NewRequest("GET", "/x", nil)
	ctx := context.WithValue(req.Context(), "request_id", "req_test1") //nolint:staticcheck
	ctx = shared.WithLocale(ctx, "cn")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	ErrorCode(ctx, rr, req, errcode.AuthRequired, nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d want 401", rr.Code)
	}

	if len(stub.calls) != 1 || stub.calls[0].locale != "cn" || stub.calls[0].key != "errors.AUTH_REQUIRED" {
		t.Errorf("stub calls = %+v", stub.calls)
	}

	var body CodedErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Error.Code != "AUTH_REQUIRED" {
		t.Errorf("body code = %q", body.Error.Code)
	}
	if body.Error.Message != "请先登录" {
		t.Errorf("body message = %q", body.Error.Message)
	}
	if body.Error.RequestID != "req_test1" || body.RequestID != "req_test1" {
		t.Errorf("request_id = %q / %q", body.Error.RequestID, body.RequestID)
	}
}

func TestErrorCodeUsesEnglishWhenCtxSaysSo(t *testing.T) {
	stub := &stubTranslator{out: "Please sign in to continue"}
	restore := SetCatalogForTesting(stub)
	defer restore()

	req := httptest.NewRequest("GET", "/x", nil)
	ctx := shared.WithLocale(req.Context(), "en")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	ErrorCode(ctx, rr, req, errcode.AuthRequired, nil)

	if stub.calls[0].locale != "en" {
		t.Errorf("translator locale = %q want en", stub.calls[0].locale)
	}
}

func TestErrorCodeForwardsParams(t *testing.T) {
	stub := &stubTranslator{out: "monitor xyz not found"}
	restore := SetCatalogForTesting(stub)
	defer restore()

	req := httptest.NewRequest("GET", "/x", nil)
	ctx := shared.WithLocale(req.Context(), "en")
	params := map[string]any{"id": "xyz"}

	rr := httptest.NewRecorder()
	ErrorCode(ctx, rr, req, errcode.MonitorNotFound, params)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d want 404", rr.Code)
	}
	if stub.calls[0].params["id"] != "xyz" {
		t.Errorf("params not forwarded: %+v", stub.calls[0].params)
	}
	var body CodedErrorResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body.Error.Params["id"] != "xyz" {
		t.Errorf("body params = %+v", body.Error.Params)
	}
}

func TestErrorCodeStatusForUnknownCodeDefaults500(t *testing.T) {
	stub := &stubTranslator{out: "boom"}
	restore := SetCatalogForTesting(stub)
	defer restore()

	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	ErrorCode(req.Context(), rr, req, errcode.Code("NOT_REGISTERED_CODE"), nil)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("unmapped code status = %d want 500", rr.Code)
	}
}
