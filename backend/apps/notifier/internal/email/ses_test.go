package email

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	smithy "github.com/aws/smithy-go"

	"github.com/kite365/idcd/lib/shared/apperr"
)

// --- Fake sesAPI client ------------------------------------------------------

type fakeSESClient struct {
	called    int
	lastInput *sesv2.SendEmailInput
	resp      *sesv2.SendEmailOutput
	err       error
}

func (f *fakeSESClient) SendEmail(_ context.Context, in *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.called++
	f.lastInput = in
	if f.err != nil {
		return nil, f.err
	}
	if f.resp != nil {
		return f.resp, nil
	}
	id := "msg-fake-001"
	return &sesv2.SendEmailOutput{MessageId: &id}, nil
}

func newSender(t *testing.T, fake *fakeSESClient) *SESSender {
	t.Helper()
	return NewSESSender(SESConfig{
		Region:   "us-east-1",
		From:     "noreply@idcd.com",
		FromName: "idcd",
	}, WithSESClient(fake))
}

// --- Success path ------------------------------------------------------------

func TestSESSender_Send_Success(t *testing.T) {
	fake := &fakeSESClient{}
	s := newSender(t, fake)

	msg := Message{
		To:      "user@example.com",
		Subject: "Welcome",
		HTML:    "<h1>Hello</h1>",
	}
	if err := s.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if fake.called != 1 {
		t.Fatalf("SendEmail call count = %d, want 1", fake.called)
	}

	in := fake.lastInput
	if in == nil {
		t.Fatal("lastInput is nil")
	}
	if got := derefString(in.FromEmailAddress); got != "idcd <noreply@idcd.com>" {
		t.Errorf("FromEmailAddress = %q, want 'idcd <noreply@idcd.com>'", got)
	}
	if in.Destination == nil || len(in.Destination.ToAddresses) != 1 || in.Destination.ToAddresses[0] != msg.To {
		t.Errorf("Destination.ToAddresses mismatch: %#v", in.Destination)
	}
	if in.Content == nil || in.Content.Simple == nil ||
		derefString(in.Content.Simple.Subject.Data) != msg.Subject ||
		derefString(in.Content.Simple.Body.Html.Data) != msg.HTML {
		t.Errorf("Content payload mismatch: %#v", in.Content)
	}
	// Charset must be set to UTF-8 to preserve non-ASCII subjects / bodies.
	if got := derefString(in.Content.Simple.Subject.Charset); got != "UTF-8" {
		t.Errorf("Subject.Charset = %q, want UTF-8", got)
	}
	if got := derefString(in.Content.Simple.Body.Html.Charset); got != "UTF-8" {
		t.Errorf("Body.Html.Charset = %q, want UTF-8", got)
	}
}

func TestSESSender_Send_NoDisplayName(t *testing.T) {
	fake := &fakeSESClient{}
	s := NewSESSender(SESConfig{
		Region: "us-east-1",
		From:   "noreply@idcd.com",
		// FromName intentionally blank.
	}, WithSESClient(fake))

	err := s.Send(context.Background(), Message{
		To: "u@example.com", Subject: "s", HTML: "<p>h</p>",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := derefString(fake.lastInput.FromEmailAddress); got != "noreply@idcd.com" {
		t.Errorf("FromEmailAddress = %q, want raw address (no <>)", got)
	}
}

// --- Validation --------------------------------------------------------------

func TestSESSender_Send_Validation(t *testing.T) {
	fake := &fakeSESClient{}
	s := newSender(t, fake)

	tests := []struct {
		name string
		msg  Message
	}{
		{"missing To", Message{Subject: "s", HTML: "<p>h</p>"}},
		{"missing Subject", Message{To: "u@example.com", HTML: "<p>h</p>"}},
		{"missing HTML", Message{To: "u@example.com", Subject: "s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Send(context.Background(), tt.msg)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !apperr.Is(err, apperr.CodeValidation) {
				t.Errorf("error code = %v, want CodeValidation (err=%v)", codeOf(err), err)
			}
			if fake.called != 0 {
				t.Errorf("SendEmail should not be called for invalid input, got %d", fake.called)
			}
		})
	}
}

// --- Error classification ----------------------------------------------------

func TestSESSender_ClassifyError_Throttle(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.TooManyRequestsException{Message: ptr("throttled")},
	}
	s := newSender(t, fake)

	err := s.Send(context.Background(), validMessage())
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("throttle should map to Unavailable, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_SendingPaused(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.SendingPausedException{Message: ptr("paused")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("paused should map to Unavailable, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_InternalServiceError(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.InternalServiceErrorException{Message: ptr("boom")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("internal-svc should map to Unavailable, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_BadRequest(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.BadRequestException{Message: ptr("not a valid email")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("bad request should map to Validation, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_AccountSuspended(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.AccountSuspendedException{Message: ptr("suspended")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeUnauthorized) {
		t.Errorf("suspended should map to Unauthorized, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_MailFromDomain(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.MailFromDomainNotVerifiedException{Message: ptr("verify your domain")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("MailFromDomain should map to Validation, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_LimitExceeded(t *testing.T) {
	fake := &fakeSESClient{
		err: &sestypes.LimitExceededException{Message: ptr("hit hard limit")},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("LimitExceeded should map to Validation (no retry), got %v", codeOf(err))
	}
}

// Generic 5xx server fault falls through named-exception switch and lands
// on the smithy.APIError branch.
func TestSESSender_ClassifyError_GenericServerFault(t *testing.T) {
	fake := &fakeSESClient{
		err: &smithy.GenericAPIError{
			Code:    "ServiceUnavailable",
			Message: "try again later",
			Fault:   smithy.FaultServer,
		},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("generic 5xx should map to Unavailable, got %v", codeOf(err))
	}
}

func TestSESSender_ClassifyError_GenericClientFault(t *testing.T) {
	fake := &fakeSESClient{
		err: &smithy.GenericAPIError{
			Code:    "MalformedQueryString",
			Message: "bad input",
			Fault:   smithy.FaultClient,
		},
	}
	err := newSender(t, fake).Send(context.Background(), validMessage())
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("generic 4xx should map to Validation, got %v", codeOf(err))
	}
}

// Pure transport-layer failures (no smithy.APIError in chain) should be
// classified as Unavailable so asynq retries.
func TestSESSender_ClassifyError_NetworkFailure(t *testing.T) {
	// Stand up an httptest server, capture its addr, close it — calls
	// will then fail with a real connection-refused error from the SDK
	// transport. This exercises the WithHTTPClient code path AND verifies
	// the fallback Unavailable classification.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	hc := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return nil, errors.New("simulated network down")
			},
		},
	}

	s := NewSESSender(SESConfig{
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "SECRET",
		From:      "noreply@idcd.com",
	}, WithHTTPClient(hc))

	err := s.Send(context.Background(), validMessage())
	if err == nil {
		t.Fatal("expected network error")
	}
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("network failure should map to Unavailable, got %v (err=%v)", codeOf(err), err)
	}
}

// --- Wire-level test using httptest.Server -----------------------------------

// SES v2 SendEmail uses POST /v2/email/outbound-emails with a JSON body.
// We point the SDK at httptest.Server and verify success / 4xx / 5xx
// translation end-to-end through the real SDK middleware stack.
func TestSESSender_HTTPClient_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/outbound-emails") {
			t.Errorf("path = %s, want SES outbound-emails", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "user@example.com") {
			t.Errorf("request body missing recipient: %s", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MessageId":"ses-msg-id-123"}`))
	}))
	defer srv.Close()

	s := NewSESSender(SESConfig{
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "SECRET",
		From:      "noreply@idcd.com",
		FromName:  "idcd",
	}, WithHTTPClient(redirectingHTTPClient(srv.URL)))

	if err := s.Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}

func TestSESSender_HTTPClient_4xxBadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Use the smithy JSON-1.1 RPC error shape with __type so the SDK
		// classifies the response as a client-fault APIError.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"BadRequestException","message":"address rejected"}`))
	}))
	defer srv.Close()

	s := NewSESSender(SESConfig{
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "SECRET",
		From:      "noreply@idcd.com",
	}, WithHTTPClient(redirectingHTTPClient(srv.URL)))

	err := s.Send(context.Background(), validMessage())
	if err == nil {
		t.Fatal("expected error")
	}
	// BadRequest may surface as the typed BadRequestException or as a
	// generic client-fault APIError depending on the SDK serializer
	// version. Both map to Validation per classifySESError.
	if !apperr.Is(err, apperr.CodeValidation) {
		t.Errorf("4xx response should map to Validation, got %v (err=%v)", codeOf(err), err)
	}
}

func TestSESSender_HTTPClient_5xxServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"__type":"InternalServiceErrorException","message":"oops"}`))
	}))
	defer srv.Close()

	s := NewSESSender(SESConfig{
		Region:    "us-east-1",
		AccessKey: "AKID",
		SecretKey: "SECRET",
		From:      "noreply@idcd.com",
	}, WithHTTPClient(redirectingHTTPClient(srv.URL)))

	err := s.Send(context.Background(), validMessage())
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.Is(err, apperr.CodeUnavailable) {
		t.Errorf("5xx response should map to Unavailable, got %v (err=%v)", codeOf(err), err)
	}
}

// --- Constructor failure -----------------------------------------------------

func TestNewSESSender_MissingRegion_DefersError(t *testing.T) {
	// Empty region → loadAWSConfig returns an error stored as initErr.
	// The constructor still returns a non-nil sender; Send() surfaces
	// the misconfig as an Internal error so asynq retries until config
	// is fixed and the worker is restarted.
	s := NewSESSender(SESConfig{From: "noreply@idcd.com"})
	if s == nil {
		t.Fatal("NewSESSender returned nil")
	}
	err := s.Send(context.Background(), validMessage())
	if err == nil {
		t.Fatal("expected init error")
	}
	if !apperr.Is(err, apperr.CodeInternal) {
		t.Errorf("missing region should surface as Internal, got %v", codeOf(err))
	}
}

// --- Helpers -----------------------------------------------------------------

func validMessage() Message {
	return Message{
		To:      "user@example.com",
		Subject: "Subject 主题",
		HTML:    "<p>Body 正文</p>",
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptr(s string) *string { return &s }

func codeOf(err error) apperr.Code {
	var e *apperr.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// redirectingHTTPClient returns an *http.Client whose Transport rewrites
// every request URL onto the httptest.Server's base URL. We do this
// instead of overriding the SDK endpoint resolver because the resolver
// API churns across SDK versions while http.Transport stays stable.
func redirectingHTTPClient(baseURL string) *http.Client {
	return &http.Client{Transport: &rewriteTransport{base: baseURL}}
}

type rewriteTransport struct {
	base string
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, err := req.URL.Parse(rt.base)
	if err != nil {
		return nil, err
	}
	// Preserve original path + query — they identify the SES operation.
	u.Path = req.URL.Path
	u.RawQuery = req.URL.RawQuery
	req.URL = u
	req.Host = u.Host
	return http.DefaultTransport.RoundTrip(req)
}
