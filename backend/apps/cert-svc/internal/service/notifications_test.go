package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
)

// notifTestEnv bundles the moving parts a watcher test needs.
type notifTestEnv struct {
	pool pgxmock.PgxPoolIface
	rdb  *redis.Client
	mr   *miniredis.Miniredis
	w    *NotificationWatcher
	now  time.Time
}

func newNotifEnv(t *testing.T, opts ...NotificationOption) *notifTestEnv {
	t.Helper()
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	now := time.Date(2026, 5, 17, 12, 0, 0, 0, time.UTC)
	full := append([]NotificationOption{
		WithNotificationPool(pool),
		withClock(func() time.Time { return now }),
	}, opts...)

	w := NewNotificationWatcher(repo.NewWithPool(pool), rdb, full...)
	return &notifTestEnv{pool: pool, rdb: rdb, mr: mr, w: w, now: now}
}

// readStream returns all entries on the notification stream as a slice of
// (event, payload) tuples for easy assertion.
func (e *notifTestEnv) readStream(t *testing.T) []map[string]string {
	t.Helper()
	entries, err := e.mr.Stream(DefaultNotificationStream)
	require.NoError(t, err)
	out := make([]map[string]string, 0, len(entries))
	for _, en := range entries {
		m := map[string]string{}
		for i := 0; i+1 < len(en.Values); i += 2 {
			m[en.Values[i]] = en.Values[i+1]
		}
		out = append(out, m)
	}
	return out
}

func TestProcessOrderEvents_EmitsIssuedEvent(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	// order_events row: cert_persisted with {"cert_id": 77}
	payload, _ := json.Marshal(map[string]int64{"cert_id": 77})
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(0), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}).AddRow(
			int64(10), int64(100), 5, actionCertPersisted, payload, env.now,
			"42", []string{"example.com"}, "lets-encrypt", ptrInt64(77),
		))

	// emitOrderEvent loads the cert to populate NotAfter — provide that.
	notAfter := env.now.Add(90 * 24 * time.Hour)
	env.pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(77)).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "account_id", "sans", "issuer", "serial_hex",
			"fingerprint_sha256", "leaf_pem", "chain_pem", "key_kms_handle",
			"not_before", "not_after", "status", "revoked_at", "revoke_reason", "created_at",
		}).AddRow(
			int64(77), int64(100), "42", []string{"example.com"}, "lets-encrypt", "deadbeef",
			"fp", "leaf", "chain", "kms-handle",
			env.now, notAfter, "issued", nil, nil, env.now,
		))

	require.NoError(t, env.w.processOrderEvents(ctx))
	require.NoError(t, env.pool.ExpectationsWereMet())

	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertIssued, entries[0]["event"])
	assert.Equal(t, "42", entries[0]["account_id"])
	assert.Equal(t, "77", entries[0]["cert_id"])
	assert.Equal(t, "100", entries[0]["order_id"])
	// P0-4 W2: wire layout 改为 flat — 字段直接在 stream values 顶层, 不再有 payload JSON.
	assert.Contains(t, entries[0]["subject"], "签发成功")
	assert.Equal(t, "lets-encrypt", entries[0]["ca"])
	assert.Equal(t, "1", entries[0]["schema_ver"])

	// Cursor advanced to last event id.
	v, err := env.rdb.Get(ctx, DefaultNotificationCursorKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "10", v)
}

func TestProcessOrderEvents_EmitsFailedEvent(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	errText := []byte("dns01 challenge: nxdomain")
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(0), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}).AddRow(
			int64(20), int64(200), 4, actionACMERequestFailed, errText, env.now,
			int64(43), []string{"fail.test"}, "lets-encrypt", nil,
		))

	require.NoError(t, env.w.processOrderEvents(ctx))
	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertFailed, entries[0]["event"])
	assert.Equal(t, "200", entries[0]["order_id"])
	assert.Equal(t, "dns01 challenge: nxdomain", entries[0]["error_message"])
	assert.Contains(t, entries[0]["subject"], "签发失败")
}

func TestProcessOrderEvents_EmitsRevokedEvent(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(0), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}).AddRow(
			int64(30), int64(300), 7, actionRevokeCompleted, nil, env.now,
			int64(44), []string{"r.test"}, "lets-encrypt", ptrInt64(55),
		))

	require.NoError(t, env.w.processOrderEvents(ctx))
	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertRevoked, entries[0]["event"])
	assert.Equal(t, "55", entries[0]["cert_id"])
}

func TestProcessOrderEvents_CursorPersistsAcrossCalls(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	// Seed cursor at 5 — events with id <= 5 should be skipped.
	require.NoError(t, env.rdb.Set(ctx, DefaultNotificationCursorKey, "5", 0).Err())

	payload, _ := json.Marshal(map[string]int64{"cert_id": 1})
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(5), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}).AddRow(
			int64(7), int64(100), 5, actionCertPersisted, payload, env.now,
			"42", []string{"x.test"}, "le", ptrInt64(1),
		))
	// cert lookup for not_after
	env.pool.ExpectQuery(`SELECT .+ FROM cert\.certs\s+WHERE id`).
		WithArgs(int64(1)).
		WillReturnError(redis.Nil) // anything non-nil; we degrade gracefully

	require.NoError(t, env.w.processOrderEvents(ctx))

	v, err := env.rdb.Get(ctx, DefaultNotificationCursorKey).Result()
	require.NoError(t, err)
	assert.Equal(t, "7", v)
}

func TestProcessOrderEvents_EmptyResultLeavesCursor(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()
	require.NoError(t, env.rdb.Set(ctx, DefaultNotificationCursorKey, "9", 0).Err())

	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(9), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}))

	require.NoError(t, env.w.processOrderEvents(ctx))
	v, _ := env.rdb.Get(ctx, DefaultNotificationCursorKey).Result()
	assert.Equal(t, "9", v)
	assert.Empty(t, env.readStream(t))
}

func TestPickBucket_BoundaryConditions(t *testing.T) {
	buckets := []int{30, 14, 7, 1}

	cases := []struct {
		name     string
		dur      time.Duration
		expected int
	}{
		{"31 days — outside 30 bucket", 31 * 24 * time.Hour, 0},
		{"30 days — on 30 bucket", 30 * 24 * time.Hour, 30},
		{"29.5 days — near 30 bucket", 29*24*time.Hour + 12*time.Hour, 30},
		{"22 days — in no bucket", 22 * 24 * time.Hour, 0},
		{"14 days — on 14 bucket", 14 * 24 * time.Hour, 14},
		{"13.2 days — closer to 14", 13*24*time.Hour + 5*time.Hour, 14},
		{"7 days — on 7 bucket", 7 * 24 * time.Hour, 7},
		{"1 day — on 1 bucket", 24 * time.Hour, 1},
		{"0.5 days — close to 1", 12 * time.Hour, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, pickBucket(tc.dur, buckets))
		})
	}
}

func TestProcessExpiringCerts_EmitsAndDedupes(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	// cert with not_after exactly 30 days out → bucket 30.
	notAfter := env.now.Add(30 * 24 * time.Hour)
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.certs c[\s\S]+WHERE c\.status`).
		WithArgs(env.now, env.now.Add(31*24*time.Hour), notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "not_after", "order_id", "ca",
		}).AddRow(
			int64(11), "42", []string{"a.test"}, notAfter, int64(1000), "le",
		))

	require.NoError(t, env.w.processExpiringCerts(ctx))
	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertExpiring, entries[0]["event"])
	// P0-4 W2: days_to_expire 现在直接在顶层 (string-encoded), 而非 payload JSON 内。
	assert.Equal(t, "30", entries[0]["days_to_expire"])

	// SETNX marker should now exist.
	exists, err := env.rdb.Exists(ctx, "cert:expiring:notified:11:30").Result()
	require.NoError(t, err)
	assert.EqualValues(t, 1, exists)

	// Second pass with same cert: SETNX should fail, no new emit.
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.certs c[\s\S]+WHERE c\.status`).
		WithArgs(env.now, env.now.Add(31*24*time.Hour), notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "not_after", "order_id", "ca",
		}).AddRow(
			int64(11), "42", []string{"a.test"}, notAfter, int64(1000), "le",
		))
	require.NoError(t, env.w.processExpiringCerts(ctx))
	assert.Len(t, env.readStream(t), 1, "expected no new entry on dedupe")
}

func TestProcessExpiringCerts_BetweenBucketsNoEmit(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	// cert at 22 days out — sits between buckets 30 and 14 with > 1 day
	// distance to either. pickBucket returns 0 → no emit even though the
	// row is fetched (it's still within (now, now+31d]).
	notAfter := env.now.Add(22 * 24 * time.Hour)
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.certs c[\s\S]+WHERE c\.status`).
		WithArgs(env.now, env.now.Add(31*24*time.Hour), notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "not_after", "order_id", "ca",
		}).AddRow(
			int64(12), "42", []string{"b.test"}, notAfter, int64(1001), "le",
		))

	require.NoError(t, env.w.processExpiringCerts(ctx))
	assert.Empty(t, env.readStream(t))
}

func TestProcessRenewalFailures_EmitsAndDedupes(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	lastErr := "rate limit exceeded"
	notAfter := env.now.Add(5 * 24 * time.Hour)
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.renewal_jobs j`).
		WithArgs(renewalFailThreshold, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "attempt_count", "last_error",
			"account_id", "sans", "not_after",
			"order_id", "ca",
		}).AddRow(
			int64(500), int64(77), 3, &lastErr,
			"42", []string{"r.test"}, notAfter,
			int64(900), "le",
		))

	require.NoError(t, env.w.processRenewalFailures(ctx))
	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertRenewalFailed, entries[0]["event"])
	assert.Equal(t, "rate limit exceeded", entries[0]["error_message"])

	exists, err := env.rdb.Exists(ctx, "cert:renewal_failed:notified:500").Result()
	require.NoError(t, err)
	assert.EqualValues(t, 1, exists)

	// Re-run with the same job — SETNX dedupes; no new entry.
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.renewal_jobs j`).
		WithArgs(renewalFailThreshold, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "attempt_count", "last_error",
			"account_id", "sans", "not_after",
			"order_id", "ca",
		}).AddRow(
			int64(500), int64(77), 3, &lastErr,
			"42", []string{"r.test"}, notAfter,
			int64(900), "le",
		))
	require.NoError(t, env.w.processRenewalFailures(ctx))
	assert.Len(t, env.readStream(t), 1)
}

func TestLoadCursor_MissingReturnsZero(t *testing.T) {
	env := newNotifEnv(t)
	id, err := env.w.loadCursor(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), id)
}

func TestLoadCursor_MalformedSelfHeals(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()
	require.NoError(t, env.rdb.Set(ctx, DefaultNotificationCursorKey, "not-a-number", 0).Err())
	id, err := env.w.loadCursor(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), id)
}

func TestTick_RunsAllThreePassesWhenPoolPresent(t *testing.T) {
	env := newNotifEnv(t)
	ctx := context.Background()

	// order_events pass — empty result
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.order_events`).
		WithArgs(int64(0), []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "order_id", "action_seq", "action", "payload_jsonb", "occurred_at",
			"account_id", "sans", "ca", "cert_id",
		}))
	// expiring pass — empty result
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.certs c`).
		WithArgs(env.now, env.now.Add(31*24*time.Hour), notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "account_id", "sans", "not_after", "order_id", "ca",
		}))
	// renewal_failures pass — empty result
	env.pool.ExpectQuery(`SELECT[\s\S]+FROM cert\.renewal_jobs j`).
		WithArgs(renewalFailThreshold, notificationBatchLimit).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "cert_id", "attempt_count", "last_error",
			"account_id", "sans", "not_after",
			"order_id", "ca",
		}))

	env.w.tick(ctx)
	require.NoError(t, env.pool.ExpectationsWereMet())
}

func TestTick_SkipsAllPassesWhenPoolNil(t *testing.T) {
	env := newNotifEnv(t)
	env.w.pool = nil
	// Should not panic, not query, just return.
	env.w.tick(context.Background())
}

func TestNotificationWatcher_RunStopsOnContextCancel(t *testing.T) {
	env := newNotifEnv(t, WithNotificationPollInterval(50*time.Millisecond))
	// no pgx expectations — pool exists but every query is unexpected,
	// which would fail. To avoid that, swap out the pool with a nil one
	// after construction so Run.tick() skips DB passes entirely.
	env.w.pool = nil

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- env.w.Run(ctx) }()

	time.Sleep(150 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop on context cancel")
	}
}

func TestRun_RequiresRedis(t *testing.T) {
	w := NewNotificationWatcher(nil, nil)
	err := w.Run(context.Background())
	require.Error(t, err)
}

func TestNewNotificationWatcher_AppliesOptions(t *testing.T) {
	w := NewNotificationWatcher(nil, nil,
		WithNotificationStream("custom:stream"),
		WithNotificationCursorKey("custom:cursor"),
		WithNotificationPollInterval(123*time.Millisecond),
		WithNotificationExpiringDays([]int{60, -1, 10, 0}),
	)
	assert.Equal(t, "custom:stream", w.stream)
	assert.Equal(t, "custom:cursor", w.cursorKey)
	assert.Equal(t, 123*time.Millisecond, w.pollInterval)
	assert.Equal(t, []int{60, 10}, w.expiringDays)
}

func TestNewNotificationWatcher_EmptyOptionsKeepDefaults(t *testing.T) {
	w := NewNotificationWatcher(nil, nil,
		WithNotificationStream(""),
		WithNotificationCursorKey(""),
		WithNotificationPollInterval(0),
		WithNotificationExpiringDays(nil),
		WithNotificationLogger(nil),
	)
	assert.Equal(t, DefaultNotificationStream, w.stream)
	assert.Equal(t, DefaultNotificationCursorKey, w.cursorKey)
	assert.Equal(t, DefaultNotificationPollInterval, w.pollInterval)
	assert.Equal(t, []int{30, 14, 7, 1}, w.expiringDays)
}

func TestXAddNotification_FormatsCorrectly(t *testing.T) {
	env := newNotifEnv(t)
	notAfter := env.now.Add(30 * 24 * time.Hour)
	data := NotificationData{
		EventType:    EventCertExpiring,
		AccountID:    "1",
		CertID:       2,
		OrderID:      3,
		SANs:         []string{"x.test"},
		CA:           "le",
		NotAfter:     &notAfter,
		DaysToExpire: 30,
	}
	require.NoError(t, env.w.xaddNotification(context.Background(), EventCertExpiring, data, env.now))
	entries := env.readStream(t)
	require.Len(t, entries, 1)
	assert.Equal(t, EventCertExpiring, entries[0]["event"])
	assert.Equal(t, "1", entries[0]["account_id"])
	assert.Equal(t, "2", entries[0]["cert_id"])
	assert.Equal(t, "3", entries[0]["order_id"])
	assert.Equal(t, env.now.Format(time.RFC3339), entries[0]["emitted_at"])
	// P0-4 W2: not_after 现在直接在顶层 (RFC3339 string), 而非 payload JSON 内。
	assert.Equal(t, notAfter.Format(time.RFC3339), entries[0]["not_after"])
	assert.Equal(t, "1", entries[0]["schema_ver"])
}

// ptrInt64 is a tiny helper to take the address of an int64 literal in
// expectation tables.
func ptrInt64(n int64) *int64 { return &n }

// ensure strconv import is used (avoid unused-import lint if test edits
// remove the one place we use it).
var _ = strconv.FormatInt
