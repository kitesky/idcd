package paymenthub

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeLookup struct {
	mu       sync.Mutex
	orderID  string
	status   string
	err      error
	called   int
	gotInput string
}

func (f *fakeLookup) LookupByExtOrderID(_ context.Context, extOrderID string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	f.gotInput = extOrderID
	return f.orderID, f.status, f.err
}

type fakeOrders struct {
	mu       sync.Mutex
	err      error
	called   int
	gotID    string
	gotFrom  string
	gotTo    string
	gotErrReason *string
}

func (f *fakeOrders) UpdateStatus(_ context.Context, id, from, to string, errReason *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	f.gotID = id
	f.gotFrom = from
	f.gotTo = to
	f.gotErrReason = errReason
	return f.err
}

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

const testSecret = "whsec_test"

func sign(t *testing.T, secret, timestamp string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func newHandler(t *testing.T, lookup OrderLookup, orders OrderStatusUpdater, rdb RetryEnqueuer, now time.Time) *Handler {
	t.Helper()
	return &Handler{
		Secret: []byte(testSecret),
		Lookup: lookup,
		Orders: orders,
		Redis:  rdb,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return now },
	}
}

func doRequest(t *testing.T, h *Handler, method string, body []byte, hdr http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/webhooks/paymenthub", bytes.NewReader(body))
	for k, vv := range hdr {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func signedHeaders(t *testing.T, body []byte, ts time.Time) http.Header {
	t.Helper()
	tsStr := strconv.FormatInt(ts.Unix(), 10)
	sig := sign(t, testSecret, tsStr, body)
	return http.Header{
		"X-Webhook-Timestamp": []string{tsStr},
		"X-Webhook-Signature": []string{sig},
	}
}

func refundBody(eventType, eventID, extOrderID string) []byte {
	return []byte(fmt.Sprintf(
		`{"event_id":%q,"event_type":%q,"data":{"ext_order_id":%q}}`,
		eventID, eventType, extOrderID,
	))
}

func TestServeHTTP_HappyPath(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
	}{
		{"transaction.payment_refunded", EventTransactionPaymentRefunded},
		{"transaction.refunded", EventTransactionRefunded},
		{"subscription.payment_refunded", EventSubscriptionPaymentRefunded},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_700_000_000, 0).UTC()
			lookup := &fakeLookup{orderID: "v_abc", status: "paid"}
			orders := &fakeOrders{}
			mr, rdb := newRedis(t)
			h := newHandler(t, lookup, orders, rdb, now)

			body := refundBody(tc.eventType, "evt_1", "pad_123")
			rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d want 200", rec.Code)
			}
			if lookup.called != 1 || lookup.gotInput != "pad_123" {
				t.Fatalf("lookup called=%d input=%q", lookup.called, lookup.gotInput)
			}
			if orders.called != 1 || orders.gotID != "v_abc" || orders.gotFrom != "paid" || orders.gotTo != "refunded" {
				t.Fatalf("orders update: %+v", orders)
			}
			if mr.Exists(RefundRetryStream) {
				t.Fatalf("refund_retry_queue should be empty on happy path")
			}
		})
	}
}

func TestServeHTTP_DeliveredAlsoTransitions(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{orderID: "v_abc", status: "delivered"}
	orders := &fakeOrders{}
	_, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := refundBody(EventTransactionRefunded, "evt_d", "pad_d")
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if orders.gotFrom != "delivered" || orders.gotTo != "refunded" {
		t.Fatalf("expected delivered→refunded, got %s→%s", orders.gotFrom, orders.gotTo)
	}
}

func TestServeHTTP_UpdateStatusFailsEnqueuesRetry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{orderID: "v_abc", status: "paid"}
	orders := &fakeOrders{err: errors.New("db transient")}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := refundBody(EventTransactionRefunded, "evt_x", "pad_x")
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if !mr.Exists(RefundRetryStream) {
		t.Fatalf("refund_retry_queue should contain one entry after failure")
	}
	entries, err := rdb.XRange(context.Background(), RefundRetryStream, "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count: got %d want 1", len(entries))
	}
	got := entries[0].Values
	if got["order_id"] != "v_abc" {
		t.Errorf("order_id: %v", got["order_id"])
	}
	if got["ext_event_id"] != "evt_x" {
		t.Errorf("ext_event_id: %v", got["ext_event_id"])
	}
	if got["attempt"] != "1" {
		t.Errorf("attempt: %v", got["attempt"])
	}
	if got["scheduled_at"] == nil || got["scheduled_at"] == "" {
		t.Errorf("scheduled_at missing")
	}
}

func TestServeHTTP_LookupFailsEnqueuesRetry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{err: errors.New("db transient")}
	orders := &fakeOrders{}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := refundBody(EventTransactionRefunded, "evt_y", "pad_y")
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if orders.called != 0 {
		t.Fatalf("orders should not be called when lookup errored")
	}
	if !mr.Exists(RefundRetryStream) {
		t.Fatalf("refund_retry_queue should contain one entry after lookup error")
	}
}

func TestServeHTTP_InvalidSignature(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{orderID: "v_abc", status: "paid"}
	orders := &fakeOrders{}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := refundBody(EventTransactionRefunded, "evt_bad", "pad_bad")
	hdr := http.Header{
		"X-Webhook-Timestamp": []string{strconv.FormatInt(now.Unix(), 10)},
		"X-Webhook-Signature": []string{"deadbeef"},
	}
	rec := doRequest(t, h, http.MethodPost, body, hdr)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d want 401", rec.Code)
	}
	if lookup.called != 0 || orders.called != 0 {
		t.Fatalf("nothing should run on bad signature")
	}
	if mr.Exists(RefundRetryStream) {
		t.Fatalf("retry queue must be untouched on bad signature")
	}
}

func TestServeHTTP_MissingSignatureHeaders(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, now)

	body := refundBody(EventTransactionRefunded, "evt", "pad")
	rec := doRequest(t, h, http.MethodPost, body, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing headers should 401, got %d", rec.Code)
	}
}

func TestServeHTTP_ExpiredTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, now)

	body := refundBody(EventTransactionRefunded, "evt", "pad")
	stale := now.Add(-10 * time.Minute)
	hdr := signedHeaders(t, body, stale)
	rec := doRequest(t, h, http.MethodPost, body, hdr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired timestamp should 401, got %d", rec.Code)
	}
}

func TestServeHTTP_FutureTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, now)

	body := refundBody(EventTransactionRefunded, "evt", "pad")
	future := now.Add(10 * time.Minute)
	hdr := signedHeaders(t, body, future)
	rec := doRequest(t, h, http.MethodPost, body, hdr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("future timestamp should 401, got %d", rec.Code)
	}
}

func TestServeHTTP_NonIntegerTimestamp(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, now)

	body := refundBody(EventTransactionRefunded, "evt", "pad")
	hdr := http.Header{
		"X-Webhook-Timestamp": []string{"not-a-number"},
		"X-Webhook-Signature": []string{"x"},
	}
	rec := doRequest(t, h, http.MethodPost, body, hdr)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad timestamp should 401, got %d", rec.Code)
	}
}

func TestServeHTTP_UnknownEventType(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{}
	orders := &fakeOrders{}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := []byte(`{"event_id":"evt","event_type":"transaction.created","data":{"ext_order_id":"pad"}}`)
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("unknown event should 200, got %d", rec.Code)
	}
	if lookup.called != 0 || orders.called != 0 {
		t.Fatalf("unknown event must not touch DB")
	}
	if mr.Exists(RefundRetryStream) {
		t.Fatalf("unknown event must not enqueue")
	}
}

func TestServeHTTP_UnknownExtOrderID(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{err: ErrOrderNotFound}
	orders := &fakeOrders{}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := refundBody(EventTransactionRefunded, "evt", "pad_unknown")
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("unknown order should 200, got %d", rec.Code)
	}
	if orders.called != 0 {
		t.Fatalf("unknown order must not call UpdateStatus")
	}
	if mr.Exists(RefundRetryStream) {
		t.Fatalf("unknown order must not enqueue (it is a no-op, not transient failure)")
	}
}

func TestServeHTTP_MissingExtOrderID(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	lookup := &fakeLookup{}
	orders := &fakeOrders{}
	mr, rdb := newRedis(t)
	h := newHandler(t, lookup, orders, rdb, now)

	body := []byte(`{"event_id":"evt","event_type":"transaction.refunded","data":{}}`)
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if lookup.called != 0 {
		t.Fatalf("must not look up when ext_order_id is empty")
	}
	if mr.Exists(RefundRetryStream) {
		t.Fatalf("must not enqueue when ext_order_id is empty")
	}
}

func TestServeHTTP_MalformedJSON(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, now)

	body := []byte(`{not json`)
	rec := doRequest(t, h, http.MethodPost, body, signedHeaders(t, body, now))
	if rec.Code != http.StatusOK {
		t.Fatalf("malformed JSON should still ack 200, got %d", rec.Code)
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, time.Now())
	rec := doRequest(t, h, http.MethodGet, nil, nil)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET should 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != "POST" {
		t.Fatalf("Allow header missing or wrong: %q", rec.Header().Get("Allow"))
	}
}

func TestServeHTTP_EmptyBody(t *testing.T) {
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, time.Now())
	rec := doRequest(t, h, http.MethodPost, nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body should 400, got %d", rec.Code)
	}
}

func TestServeHTTP_BodyTooLarge(t *testing.T) {
	h := newHandler(t, &fakeLookup{}, &fakeOrders{}, &nopEnqueuer{}, time.Now())
	big := bytes.Repeat([]byte("a"), maxBodyBytes+10)
	rec := doRequest(t, h, http.MethodPost, big, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversize body should 400, got %d", rec.Code)
	}
}

func TestVerifySignature_TableDriven(t *testing.T) {
	secret := []byte("s")
	now := time.Unix(1_700_000_000, 0).UTC()
	body := []byte(`{"a":1}`)
	tsStr := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(tsStr))
	mac.Write([]byte("."))
	mac.Write(body)
	good := hex.EncodeToString(mac.Sum(nil))

	cases := []struct {
		name      string
		secret    []byte
		timestamp string
		body      []byte
		sig       string
		want      bool
	}{
		{"empty secret", nil, tsStr, body, good, false},
		{"empty timestamp", secret, "", body, good, false},
		{"empty signature", secret, tsStr, body, "", false},
		{"valid", secret, tsStr, body, good, true},
		{"bad sig", secret, tsStr, body, "deadbeef", false},
		{"bad timestamp", secret, "not-int", body, good, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := verifySignature(tc.secret, tc.timestamp, tc.body, tc.sig, now)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestHandler_LoggerDefaultsAndNowDefaults(t *testing.T) {
	// Handler with Logger == nil and Now == nil should still serve OK.
	mr, rdb := newRedis(t)
	h := &Handler{
		Secret: []byte(testSecret),
		Lookup: &fakeLookup{orderID: "v_x", status: "paid"},
		Orders: &fakeOrders{},
		Redis:  rdb,
	}
	// Use the handler's own clock so signature verification passes.
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := refundBody(EventTransactionRefunded, "evt", "pad")
	mac := hmac.New(sha256.New, []byte(testSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/webhooks/paymenthub", bytes.NewReader(body))
	req.Header.Set("X-Webhook-Timestamp", ts)
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	_ = mr
}

// nopEnqueuer satisfies RetryEnqueuer without touching a real Redis
// instance. Use only in tests that expect zero enqueue calls.
type nopEnqueuer struct{}

func (n *nopEnqueuer) XAdd(_ context.Context, _ *redis.XAddArgs) *redis.StringCmd {
	return redis.NewStringResult("", nil)
}
