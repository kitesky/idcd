package contracts_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kite365/idcd/lib/shared/contracts"
)

func TestCertNotificationEvent_RoundTrip(t *testing.T) {
	t.Parallel()
	notAfter := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	emitted := time.Date(2026, 5, 21, 8, 30, 0, 0, time.UTC)
	original := contracts.CertNotificationEvent{
		SchemaVer:    contracts.CertNotificationEventSchemaV1,
		EventType:    "cert.issued",
		AccountID:    "42",
		CertID:       77,
		OrderID:      100,
		SANs:         []string{"a.example.com", "b.example.com"},
		CA:           "lets-encrypt",
		DaysToExpire: 14,
		ErrorMessage: "boom",
		NotAfter:     notAfter,
		Subject:      "[idcd] cert ready",
		Body:         "Hello world",
		EmittedAt:    emitted,
	}
	vals := original.ToStreamValues()
	got, err := contracts.ParseCertNotificationEvent(vals)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	// time.Time fields don't compare cleanly with == when the monotonic clock
	// is involved, but UTC-formatted RFC3339 + parse strips that so direct
	// equality holds here.
	if got.SchemaVer != original.SchemaVer ||
		got.EventType != original.EventType ||
		got.AccountID != original.AccountID ||
		got.CertID != original.CertID ||
		got.OrderID != original.OrderID ||
		got.CA != original.CA ||
		got.DaysToExpire != original.DaysToExpire ||
		got.ErrorMessage != original.ErrorMessage ||
		got.Subject != original.Subject ||
		got.Body != original.Body {
		t.Errorf("scalar round-trip drift:\n  want %+v\n   got %+v", original, got)
	}
	if !got.NotAfter.Equal(original.NotAfter) {
		t.Errorf("NotAfter drift: got %s, want %s", got.NotAfter, original.NotAfter)
	}
	if !got.EmittedAt.Equal(original.EmittedAt) {
		t.Errorf("EmittedAt drift: got %s, want %s", got.EmittedAt, original.EmittedAt)
	}
	if len(got.SANs) != len(original.SANs) {
		t.Fatalf("SANs len drift: got %d, want %d", len(got.SANs), len(original.SANs))
	}
	for i := range got.SANs {
		if got.SANs[i] != original.SANs[i] {
			t.Errorf("SANs[%d] = %q, want %q", i, got.SANs[i], original.SANs[i])
		}
	}
}

func TestCertNotificationEvent_AutoSchemaVer(t *testing.T) {
	t.Parallel()
	e := contracts.CertNotificationEvent{EventType: "cert.issued", AccountID: "1"}
	vals := e.ToStreamValues()
	if got := vals["schema_ver"]; got != "1" {
		t.Errorf("expected schema_ver auto-set to '1', got %v", got)
	}
}

func TestCertNotificationEvent_AutoEmittedAt(t *testing.T) {
	t.Parallel()
	e := contracts.CertNotificationEvent{EventType: "cert.issued", AccountID: "1"}
	vals := e.ToStreamValues()
	s, ok := vals["emitted_at"].(string)
	if !ok || s == "" {
		t.Fatalf("expected emitted_at auto-filled, got %v", vals["emitted_at"])
	}
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Errorf("emitted_at is not RFC3339: %q (%v)", s, err)
	}
}

func TestCertNotificationEvent_OptionalFieldsOmitted(t *testing.T) {
	t.Parallel()
	e := contracts.CertNotificationEvent{
		EventType: "cert.issued",
		AccountID: "1",
		OrderID:   2,
		// CertID 0 → still present as "0" (it's a required field with int default).
		// no SANs, CA, DaysToExpire, ErrorMessage, NotAfter, Subject, Body.
	}
	vals := e.ToStreamValues()
	for _, k := range []string{"sans", "ca", "days_to_expire", "error_message", "not_after", "subject", "body"} {
		if _, present := vals[k]; present {
			t.Errorf("expected key %q to be omitted when zero-valued, but it was present", k)
		}
	}
	// required keys still present
	for _, k := range []string{"schema_ver", "event", "account_id", "cert_id", "order_id", "emitted_at"} {
		if _, present := vals[k]; !present {
			t.Errorf("expected required key %q to be present", k)
		}
	}
	// Reparse — optional fields should be zero values.
	got, err := contracts.ParseCertNotificationEvent(vals)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.SANs != nil {
		t.Errorf("SANs: got %v, want nil", got.SANs)
	}
	if got.CA != "" || got.DaysToExpire != 0 || got.ErrorMessage != "" ||
		got.Subject != "" || got.Body != "" {
		t.Errorf("unexpected non-zero optional: %+v", got)
	}
	if !got.NotAfter.IsZero() {
		t.Errorf("NotAfter: got %s, want zero", got.NotAfter)
	}
}

func TestParseCertNotificationEvent_MissingEvent(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"account_id": "1",
		"cert_id":    "2",
		"order_id":   "3",
	})
	if err == nil {
		t.Fatal("expected error when event is missing")
	}
	if !strings.Contains(err.Error(), "event is required") {
		t.Errorf("err = %q, want substring 'event is required'", err)
	}
}

func TestParseCertNotificationEvent_UnknownSchemaVer(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":      "cert.issued",
		"schema_ver": "999",
	})
	if err == nil {
		t.Fatal("expected error for future schema_ver")
	}
	if !errors.Is(err, contracts.ErrUnknownSchemaVer) {
		t.Errorf("err should wrap ErrUnknownSchemaVer, got: %v", err)
	}
}

func TestParseCertNotificationEvent_LegacyMessage(t *testing.T) {
	// schema_ver missing → treated as legacy (SchemaVer=0), must still parse.
	t.Parallel()
	got, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":      "cert.issued",
		"account_id": "42",
		"cert_id":    "77",
		"order_id":   "100",
	})
	if err != nil {
		t.Fatalf("legacy parse failed: %v", err)
	}
	if got.SchemaVer != 0 {
		t.Errorf("legacy SchemaVer: got %d, want 0", got.SchemaVer)
	}
	if got.EventType != "cert.issued" || got.AccountID != "42" || got.CertID != 77 || got.OrderID != 100 {
		t.Errorf("legacy fields drift: %+v", got)
	}
}

func TestParseCertNotificationEvent_SchemaVerTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"string", "1", 1},
		{"int", int(1), 1},
		{"int64", int64(1), 1},
		{"float64", float64(1), 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := contracts.ParseCertNotificationEvent(map[string]any{
				"event":      "cert.issued",
				"schema_ver": tc.in,
			})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got.SchemaVer != tc.want {
				t.Errorf("SchemaVer: got %d, want %d", got.SchemaVer, tc.want)
			}
		})
	}
}

func TestParseCertNotificationEvent_SchemaVerBadString(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":      "cert.issued",
		"schema_ver": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid schema_ver string")
	}
}

func TestParseCertNotificationEvent_SchemaVerUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":      "cert.issued",
		"schema_ver": []byte("1"),
	})
	if err == nil {
		t.Fatal("expected error for []byte schema_ver")
	}
}

func TestParseCertNotificationEvent_BadCertID(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":   "cert.issued",
		"cert_id": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for bad cert_id")
	}
	if !strings.Contains(err.Error(), "cert_id") {
		t.Errorf("err = %q, want substring 'cert_id'", err)
	}
}

func TestParseCertNotificationEvent_BadOrderID(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":    "cert.issued",
		"order_id": []string{"100"},
	})
	if err == nil {
		t.Fatal("expected error for bad order_id type")
	}
}

func TestParseCertNotificationEvent_BadDaysToExpire(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":          "cert.issued",
		"days_to_expire": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for bad days_to_expire")
	}
}

func TestParseCertNotificationEvent_SansAsBytes(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal([]string{"a.example.com"})
	got, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event": "cert.issued",
		"sans":  raw,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got.SANs) != 1 || got.SANs[0] != "a.example.com" {
		t.Errorf("SANs: got %v, want [a.example.com]", got.SANs)
	}
}

func TestParseCertNotificationEvent_SansBadJSON(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event": "cert.issued",
		"sans":  "{not-json",
	})
	if err == nil {
		t.Fatal("expected error for invalid sans JSON")
	}
	if !strings.Contains(err.Error(), "sans") {
		t.Errorf("err = %q, want substring 'sans'", err)
	}
}

func TestParseCertNotificationEvent_SansUnsupportedType(t *testing.T) {
	t.Parallel()
	_, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event": "cert.issued",
		"sans":  []string{"a"},
	})
	if err == nil {
		t.Fatal("expected error for []string sans")
	}
}

func TestParseCertNotificationEvent_MalformedTimestampsIgnored(t *testing.T) {
	// Malformed RFC3339 timestamps should leave fields zero-valued (not error)
	// — best-effort decoding per the doc contract.
	t.Parallel()
	got, err := contracts.ParseCertNotificationEvent(map[string]any{
		"event":      "cert.issued",
		"emitted_at": "not-a-timestamp",
		"not_after":  "also-not-a-timestamp",
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.EmittedAt.IsZero() {
		t.Errorf("EmittedAt: got %s, want zero on malformed input", got.EmittedAt)
	}
	if !got.NotAfter.IsZero() {
		t.Errorf("NotAfter: got %s, want zero on malformed input", got.NotAfter)
	}
}

func TestCertNotificationEvent_SansEncodedAsJSONString(t *testing.T) {
	t.Parallel()
	e := contracts.CertNotificationEvent{
		EventType: "cert.issued",
		SANs:      []string{"a.example.com", "b.example.com"},
	}
	vals := e.ToStreamValues()
	raw, ok := vals["sans"].(string)
	if !ok {
		t.Fatalf("sans encoded as %T, want string", vals["sans"])
	}
	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("sans not valid JSON: %v (raw=%q)", err, raw)
	}
	if len(decoded) != 2 || decoded[0] != "a.example.com" {
		t.Errorf("sans decoded = %v, want [a.example.com b.example.com]", decoded)
	}
}

func TestCertNotificationEvent_DaysToExpireOmittedWhenZero(t *testing.T) {
	t.Parallel()
	e := contracts.CertNotificationEvent{EventType: "cert.issued"}
	vals := e.ToStreamValues()
	if _, present := vals["days_to_expire"]; present {
		t.Error("days_to_expire should be omitted when zero, but was present")
	}
	// With non-zero, it should appear and round-trip.
	e2 := contracts.CertNotificationEvent{EventType: "cert.expiring", DaysToExpire: 7}
	vals2 := e2.ToStreamValues()
	if vals2["days_to_expire"] != "7" {
		t.Errorf("days_to_expire encoded as %v, want '7'", vals2["days_to_expire"])
	}
	got, err := contracts.ParseCertNotificationEvent(vals2)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.DaysToExpire != 7 {
		t.Errorf("DaysToExpire after parse: got %d, want 7", got.DaysToExpire)
	}
}
