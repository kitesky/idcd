// Package service — NotificationWatcher polls cert.* tables for events
// that downstream notifier services should fan out (webhook / WeCom /
// DingTalk / Feishu / email). It is intentionally independent of the ACME
// orchestrator: a separate goroutine, a separate pgx pool reference, and
// a separate Redis Stream (`cert:notifications`) so a notifier outage can
// never block issuance.
//
// The watcher does NOT call notifier-specific transports — it only writes
// structured events. The downstream notifier service (apps/notifier, S2)
// will subscribe to `cert:notifications` and dispatch per-channel.
//
// Sources monitored:
//
//	cert.order_events  — `cert_persisted` (cert.issued) /
//	                     `acme_request_failed` (cert.failed) /
//	                     `revoke_completed` (cert.revoked)
//	cert.certs         — issued certs nearing expiry (30/14/7/1 day buckets)
//	cert.renewal_jobs  — failed jobs with attempt_count >= renewalFailThreshold
//
// Idempotency:
//
//	order_events  — Redis cursor key `cert:notifications:cursor` stores
//	                the last `cert.order_events.id` we emitted from.
//	expiring      — `cert:expiring:notified:<cert_id>:<bucket>` SETNX flag
//	                with a 7-day TTL prevents the same bucket re-firing.
//	renewal_jobs  — `cert:renewal_failed:notified:<job_id>` SETNX flag
//	                with a 30-day TTL prevents repeat alerts.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/cert-svc/internal/repo"
	"github.com/kite365/idcd/lib/shared/contracts"
	"github.com/kite365/idcd/lib/shared/rediskey"
	sharedstream "github.com/kite365/idcd/lib/shared/stream"
)

// Notification Redis stream + event-type constants.
//
// 命名约定：DefaultNotificationStream 是 cert-svc 服务内的入口符号，但具体
// 字符串值来自 lib/shared/stream（与 notifier/cert_consumer 等下游服务共享真值）。
const (
	// DefaultNotificationStream is the Redis Stream the watcher writes to.
	// 真值: stream.CertNotifications = "cert:notifications"。
	DefaultNotificationStream = sharedstream.CertNotifications
	// DefaultNotificationCursorKey holds the last processed
	// cert.order_events.id so we never re-emit on restart.
	DefaultNotificationCursorKey = sharedstream.CertNotificationsCursor
	// DefaultNotificationPollInterval is the tick between watcher passes.
	DefaultNotificationPollInterval = 60 * time.Second

	// EventCertIssued is emitted when a cert_persisted WAL entry appears.
	EventCertIssued = "cert.issued"
	// EventCertFailed is emitted on acme_request_failed.
	EventCertFailed = "cert.failed"
	// EventCertExpiring is emitted when an issued cert enters a
	// configured bucket (30/14/7/1 days to NotAfter).
	EventCertExpiring = "cert.expiring"
	// EventCertRenewalFailed is emitted when a renewal_jobs row reaches
	// status='failed' with attempt_count >= renewalFailThreshold.
	EventCertRenewalFailed = "cert.renewal_failed"
	// EventCertRevoked is emitted when a revoke_completed WAL entry appears.
	EventCertRevoked = "cert.revoked"

	// renewalFailThreshold is the attempt-count at which we consider a
	// renewal exhausted enough to alert the user.
	renewalFailThreshold = 3
	// notificationBatchLimit caps a single SQL pull.
	notificationBatchLimit = 100
	// expiringTTL is how long a "we already notified bucket X" marker
	// lives in Redis. 7 days covers the gap between adjacent buckets.
	expiringTTL = 7 * 24 * time.Hour
	// renewalFailedTTL bounds the renewal-failed dedupe marker.
	renewalFailedTTL = 30 * 24 * time.Hour
)

// NotificationPool is the minimal pgx surface NotificationWatcher needs
// for the raw SQL it issues. *pgxpool.Pool and pgxmock.PgxPoolIface both
// satisfy it, so unit tests need not stand up Postgres.
type NotificationPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NotificationWatcher polls cert.* tables and emits structured events to
// a Redis Stream for the downstream notifier service. It is independent
// of the ACME orchestrator (separate goroutine, separate cursor).
type NotificationWatcher struct {
	repos        *repo.Repos
	rdb          *redis.Client
	streamClient *sharedstream.Client // lazy-init from rdb in xaddNotification
	pool         NotificationPool
	stream       string
	cursorKey    string
	pollInterval time.Duration
	expiringDays []int
	logger       *slog.Logger
	clock        func() time.Time
}

// NotificationOption tunes a NotificationWatcher at construction.
type NotificationOption func(*NotificationWatcher)

// WithNotificationPool injects the raw pgx pool the watcher uses for the
// SQL queries that have no repo equivalent (`order_events.id > cursor`,
// `certs JOIN orders for CA / SANs`, `renewal_jobs WHERE failed`).
func WithNotificationPool(p NotificationPool) NotificationOption {
	return func(w *NotificationWatcher) { w.pool = p }
}

// WithNotificationPollInterval overrides the default 60s tick.
func WithNotificationPollInterval(d time.Duration) NotificationOption {
	return func(w *NotificationWatcher) {
		if d > 0 {
			w.pollInterval = d
		}
	}
}

// WithNotificationStream overrides the Redis stream key.
func WithNotificationStream(s string) NotificationOption {
	return func(w *NotificationWatcher) {
		if s != "" {
			w.stream = s
		}
	}
}

// WithNotificationCursorKey overrides the Redis cursor key.
func WithNotificationCursorKey(k string) NotificationOption {
	return func(w *NotificationWatcher) {
		if k != "" {
			w.cursorKey = k
		}
	}
}

// WithNotificationExpiringDays overrides the expiry bucket list. Must be
// strictly positive ints; zero / negative entries are dropped.
func WithNotificationExpiringDays(days []int) NotificationOption {
	return func(w *NotificationWatcher) {
		out := make([]int, 0, len(days))
		for _, d := range days {
			if d > 0 {
				out = append(out, d)
			}
		}
		if len(out) > 0 {
			w.expiringDays = out
		}
	}
}

// WithNotificationLogger overrides the default slog logger.
func WithNotificationLogger(l *slog.Logger) NotificationOption {
	return func(w *NotificationWatcher) {
		if l != nil {
			w.logger = l
		}
	}
}

// withClock is test-only; replaces time.Now.
func withClock(f func() time.Time) NotificationOption {
	return func(w *NotificationWatcher) {
		if f != nil {
			w.clock = f
		}
	}
}

// NewNotificationWatcher constructs a watcher with sensible defaults.
// The Pool is required for processOrderEvents / processExpiringCerts /
// processRenewalFailures; without it those passes are skipped.
func NewNotificationWatcher(repos *repo.Repos, rdb *redis.Client, opts ...NotificationOption) *NotificationWatcher {
	w := &NotificationWatcher{
		repos:        repos,
		rdb:          rdb,
		stream:       DefaultNotificationStream,
		cursorKey:    DefaultNotificationCursorKey,
		pollInterval: DefaultNotificationPollInterval,
		expiringDays: []int{30, 14, 7, 1},
		logger:       slog.Default(),
		clock:        func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// Run starts the watcher loop. Returns nil when ctx is cancelled.
// Individual pass errors are logged and do not stop the loop — a poll
// that fails this tick is retried next tick.
func (w *NotificationWatcher) Run(ctx context.Context) error {
	if w.rdb == nil {
		return errors.New("notification watcher: redis client not configured")
	}
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Kick once immediately so a freshly-started worker drains existing
	// backlog without waiting a full pollInterval.
	w.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *NotificationWatcher) tick(ctx context.Context) {
	if w.pool != nil {
		if err := w.processOrderEvents(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Warn("notification: order_events pass", "err", err)
		}
		if err := w.processExpiringCerts(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Warn("notification: expiring pass", "err", err)
		}
		if err := w.processRenewalFailures(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Warn("notification: renewal_failures pass", "err", err)
		}
	}
}

// loadCursor reads the persisted order_events.id cursor. Missing key
// returns 0 (so first run picks up from the start).
func (w *NotificationWatcher) loadCursor(ctx context.Context) (int64, error) {
	v, err := w.rdb.Get(ctx, w.cursorKey).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load cursor: %w", err)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		// Malformed key — log and treat as zero so we self-heal.
		w.logger.Warn("notification: bad cursor value, resetting", "raw", v, "err", err)
		return 0, nil
	}
	return n, nil
}

// saveCursor persists the last emitted order_events.id.
func (w *NotificationWatcher) saveCursor(ctx context.Context, id int64) error {
	return w.rdb.Set(ctx, w.cursorKey, strconv.FormatInt(id, 10), 0).Err()
}

// orderEventRow is the projection of order_events JOIN orders that the
// watcher needs to emit a notification.
type orderEventRow struct {
	EventID    int64
	OrderID    int64
	ActionSeq  int
	Action     string
	Payload    []byte
	OccurredAt time.Time
	AccountID  string
	SANs       []string
	CA         string
	CertID     *int64
}

const orderEventsSinceSQL = `
	SELECT
		e.id, e.order_id, e.action_seq, e.action, e.payload_jsonb, e.occurred_at,
		o.account_id, o.sans, o.ca, o.cert_id
	FROM cert.order_events e
	JOIN cert.orders o ON o.id = e.order_id
	WHERE e.id > $1
	  AND e.action = ANY($2)
	ORDER BY e.id ASC
	LIMIT $3
`

// processOrderEvents scans cert.order_events.id > cursor for the actions
// we map to notifications, emits one stream entry per row, and bumps
// the cursor monotonically (even on emit failure mid-batch, so we don't
// stall forever; downstream notifier dedupes on its own).
func (w *NotificationWatcher) processOrderEvents(ctx context.Context) error {
	cursor, err := w.loadCursor(ctx)
	if err != nil {
		return err
	}

	actions := []string{actionCertPersisted, actionACMERequestFailed, actionRevokeCompleted}
	rows, err := w.pool.Query(ctx, orderEventsSinceSQL, cursor, actions, notificationBatchLimit)
	if err != nil {
		return fmt.Errorf("query order_events: %w", err)
	}
	defer rows.Close()

	collected := make([]orderEventRow, 0, notificationBatchLimit)
	for rows.Next() {
		var r orderEventRow
		if err := rows.Scan(
			&r.EventID, &r.OrderID, &r.ActionSeq, &r.Action, &r.Payload, &r.OccurredAt,
			&r.AccountID, &r.SANs, &r.CA, &r.CertID,
		); err != nil {
			return fmt.Errorf("scan order_event: %w", err)
		}
		collected = append(collected, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate order_events: %w", err)
	}

	if len(collected) == 0 {
		return nil
	}

	maxID := cursor
	for _, r := range collected {
		if err := w.emitOrderEvent(ctx, r); err != nil {
			w.logger.Warn("notification: emit order_event",
				"event_id", r.EventID, "order_id", r.OrderID, "action", r.Action, "err", err)
			// Don't advance cursor past the failed row — retry next tick.
			break
		}
		if r.EventID > maxID {
			maxID = r.EventID
		}
	}

	if maxID > cursor {
		if err := w.saveCursor(ctx, maxID); err != nil {
			w.logger.Warn("notification: save cursor", "id", maxID, "err", err)
		}
	}
	return nil
}

// emitOrderEvent maps one WAL row to a notification event and XADDs it.
func (w *NotificationWatcher) emitOrderEvent(ctx context.Context, r orderEventRow) error {
	var (
		eventType string
		data      NotificationData
	)

	data = NotificationData{
		AccountID: r.AccountID,
		OrderID:   r.OrderID,
		SANs:      r.SANs,
		CA:        r.CA,
	}
	if r.CertID != nil {
		data.CertID = *r.CertID
	}

	switch r.Action {
	case actionCertPersisted:
		eventType = EventCertIssued
		// payload is `{"cert_id":...}` — pick up cert_id even if orders
		// row hasn't yet had SetCertID propagate.
		var p struct {
			CertID int64 `json:"cert_id"`
		}
		_ = json.Unmarshal(r.Payload, &p)
		if p.CertID > 0 {
			data.CertID = p.CertID
		}
		if data.CertID > 0 {
			if cert, err := w.repos.Certs.GetByID(ctx, data.CertID); err == nil {
				na := cert.NotAfter
				data.NotAfter = &na
			}
		}
	case actionACMERequestFailed:
		eventType = EventCertFailed
		// payload is the plain error string; not JSON.
		data.ErrorMsg = string(r.Payload)
	case actionRevokeCompleted:
		eventType = EventCertRevoked
	default:
		return fmt.Errorf("unmapped action %q", r.Action)
	}

	data.EventType = eventType
	return w.xaddNotification(ctx, eventType, data, r.OccurredAt)
}

// xaddNotification renders the templates and writes one Stream entry via
// the strongly-typed contracts.CertNotificationEvent helper (P0-4 W2).
//
// 历史 wire layout 用 `payload` 顶层字段塞 JSON 二层 (内含 sans/ca/days/...);
// 新 layout 把所有字段平铺为 stream values 顶层 (sans 仍是 JSON 单字段),
// 与 lib/shared/contracts/cert_notification_event.go 定义一致。consumer
// (apps/notifier/internal/worker/cert_consumer.go) 同步升级 — 没有灰度兼容期,
// 该流流量极小, 旧 in-flight 消息可接受被丢弃。
func (w *NotificationWatcher) xaddNotification(ctx context.Context, eventType string, data NotificationData, occurredAt time.Time) error {
	subject, body := RenderNotification(data)

	if occurredAt.IsZero() {
		occurredAt = w.clock()
	}

	evt := contracts.CertNotificationEvent{
		EventType:    eventType,
		AccountID:    data.AccountID,
		CertID:       data.CertID,
		OrderID:      data.OrderID,
		SANs:         data.SANs,
		CA:           data.CA,
		DaysToExpire: data.DaysToExpire,
		ErrorMessage: data.ErrorMsg,
		Subject:      subject,
		Body:         body,
		EmittedAt:    occurredAt.UTC(),
	}
	if data.NotAfter != nil {
		evt.NotAfter = data.NotAfter.UTC()
	}

	// Lazy-init the stream.Client wrapper. We can't do this at construction
	// time without changing every call site of NewNotificationWatcher.
	if w.streamClient == nil {
		w.streamClient = sharedstream.New(w.rdb)
	}
	if _, err := w.streamClient.AddCertNotificationTyped(ctx, evt); err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

// expiringCertRow projects cert.certs JOIN cert.orders for the expiring pass.
type expiringCertRow struct {
	CertID    int64
	AccountID  string
	SANs      []string
	NotAfter  time.Time
	OrderID   int64
	CA        string
}

const expiringCertsScanSQL = `
	SELECT
		c.id, c.account_id, c.sans, c.not_after,
		c.order_id, o.ca
	FROM cert.certs c
	JOIN cert.orders o ON o.id = c.order_id
	WHERE c.status = 'issued'
	  AND c.not_after > $1
	  AND c.not_after <= $2
	ORDER BY c.not_after ASC
	LIMIT $3
`

// processExpiringCerts scans issued certs whose NotAfter falls in
// (now, now + maxBucket+1 day], then matches each to the nearest bucket
// (within +/- 1 day) and SETNX-guards emission.
func (w *NotificationWatcher) processExpiringCerts(ctx context.Context) error {
	if len(w.expiringDays) == 0 {
		return nil
	}
	now := w.clock()
	maxBucket := 0
	for _, d := range w.expiringDays {
		if d > maxBucket {
			maxBucket = d
		}
	}
	upper := now.Add(time.Duration(maxBucket+1) * 24 * time.Hour)

	rows, err := w.pool.Query(ctx, expiringCertsScanSQL, now, upper, notificationBatchLimit)
	if err != nil {
		return fmt.Errorf("query expiring certs: %w", err)
	}
	defer rows.Close()

	type row = expiringCertRow
	collected := make([]row, 0, notificationBatchLimit)
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.CertID, &r.AccountID, &r.SANs, &r.NotAfter, &r.OrderID, &r.CA); err != nil {
			return fmt.Errorf("scan expiring cert: %w", err)
		}
		collected = append(collected, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate expiring certs: %w", err)
	}

	for _, r := range collected {
		bucket := pickBucket(r.NotAfter.Sub(now), w.expiringDays)
		if bucket == 0 {
			continue
		}
		key := rediskey.CertExpiringNotifiedKey(r.CertID, bucket)
		set, err := w.rdb.SetNX(ctx, key, "1", expiringTTL).Result()
		if err != nil {
			w.logger.Warn("notification: setnx expiring", "cert_id", r.CertID, "bucket", bucket, "err", err)
			continue
		}
		if !set {
			continue
		}
		na := r.NotAfter
		data := NotificationData{
			EventType:    EventCertExpiring,
			AccountID:    r.AccountID,
			CertID:       r.CertID,
			OrderID:      r.OrderID,
			SANs:         r.SANs,
			CA:           r.CA,
			NotAfter:     &na,
			DaysToExpire: bucket,
		}
		if err := w.xaddNotification(ctx, EventCertExpiring, data, now); err != nil {
			w.logger.Warn("notification: xadd expiring", "cert_id", r.CertID, "bucket", bucket, "err", err)
			// Roll back the SETNX so next tick can retry.
			_ = w.rdb.Del(ctx, key).Err()
		}
	}
	return nil
}

// pickBucket returns the configured bucket (e.g. 30/14/7/1) closest to
// the cert's remaining lifetime, but only if remaining lies within
// (bucket-1, bucket+1) days. Returns 0 when no bucket matches.
func pickBucket(remaining time.Duration, buckets []int) int {
	days := remaining.Hours() / 24
	best := 0
	bestDiff := math.MaxFloat64
	for _, b := range buckets {
		diff := math.Abs(days - float64(b))
		if diff < 1.0 && diff < bestDiff {
			best = b
			bestDiff = diff
		}
	}
	return best
}

// renewalFailedRow projects renewal_jobs JOIN certs JOIN orders.
type renewalFailedRow struct {
	JobID     int64
	CertID    int64
	Attempts  int
	LastError *string
	AccountID  string
	SANs      []string
	NotAfter  time.Time
	OrderID   int64
	CA        string
}

const renewalFailedSQL = `
	SELECT
		j.id, j.cert_id, j.attempt_count, j.last_error,
		c.account_id, c.sans, c.not_after,
		c.order_id, o.ca
	FROM cert.renewal_jobs j
	JOIN cert.certs c ON c.id = j.cert_id
	JOIN cert.orders o ON o.id = c.order_id
	WHERE j.status = 'failed' AND j.attempt_count >= $1
	ORDER BY j.id ASC
	LIMIT $2
`

// processRenewalFailures emits one cert.renewal_failed event per
// exhausted renewal_jobs row (status='failed', attempts >= threshold),
// SETNX-guarded so each job alerts at most once.
func (w *NotificationWatcher) processRenewalFailures(ctx context.Context) error {
	rows, err := w.pool.Query(ctx, renewalFailedSQL, renewalFailThreshold, notificationBatchLimit)
	if err != nil {
		return fmt.Errorf("query renewal_jobs: %w", err)
	}
	defer rows.Close()

	collected := make([]renewalFailedRow, 0, notificationBatchLimit)
	for rows.Next() {
		var r renewalFailedRow
		if err := rows.Scan(
			&r.JobID, &r.CertID, &r.Attempts, &r.LastError,
			&r.AccountID, &r.SANs, &r.NotAfter,
			&r.OrderID, &r.CA,
		); err != nil {
			return fmt.Errorf("scan renewal_failed: %w", err)
		}
		collected = append(collected, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate renewal_failed: %w", err)
	}

	for _, r := range collected {
		key := rediskey.CertRenewalFailedNotifiedKey(r.JobID)
		set, err := w.rdb.SetNX(ctx, key, "1", renewalFailedTTL).Result()
		if err != nil {
			w.logger.Warn("notification: setnx renewal_failed", "job_id", r.JobID, "err", err)
			continue
		}
		if !set {
			continue
		}
		na := r.NotAfter
		data := NotificationData{
			EventType: EventCertRenewalFailed,
			AccountID: r.AccountID,
			CertID:    r.CertID,
			OrderID:   r.OrderID,
			SANs:      r.SANs,
			CA:        r.CA,
			NotAfter:  &na,
		}
		if r.LastError != nil {
			data.ErrorMsg = *r.LastError
		}
		if err := w.xaddNotification(ctx, EventCertRenewalFailed, data, w.clock()); err != nil {
			w.logger.Warn("notification: xadd renewal_failed", "job_id", r.JobID, "err", err)
			_ = w.rdb.Del(ctx, key).Err()
		}
	}
	return nil
}
