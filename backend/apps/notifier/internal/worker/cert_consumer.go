// Package worker — cert_consumer.go wires the S2 W8 Redis Stream consumer
// for the `cert:notifications` stream produced by apps/cert-svc.
//
// Producer side (apps/cert-svc/internal/service/notifications.go) XADDs one
// entry per cert event using lib/shared/contracts.CertNotificationEvent
// (P0-4 W2). The wire layout is flat — all fields live at the top level of
// the stream values map (sans is JSON-encoded as a single field because
// Redis stream values must be scalar):
//
//	schema_ver       — int, current = 1
//	event            — "cert.issued" | "cert.failed" | "cert.expiring" | "cert.renewal_failed" | "cert.revoked"
//	account_id       — string (UUID / numeric — producer-defined)
//	cert_id          — string-encoded int64 (may be 0 when not yet persisted)
//	order_id         — string-encoded int64
//	sans             — JSON-encoded []string (omitted when empty)
//	ca               — string (omitted when empty)
//	days_to_expire   — string-encoded int (omitted when zero)
//	error_message    — string (omitted when empty)
//	not_after        — RFC3339 (omitted when zero)
//	subject          — pre-rendered email subject (optional)
//	body             — pre-rendered plain-text body (optional)
//	emitted_at       — RFC3339 timestamp the producer chose
//
// Old wire layout (pre P0-4 W2) wrapped everything except event/ids/emitted_at
// in a `payload` JSON string. That format is no longer supported; both
// producer and consumer migrated atomically because the stream's in-flight
// volume is low enough that loss of unprocessed legacy messages is acceptable.
//
// This consumer:
//  1. Creates the consumer group on demand (BUSYGROUP is ignored — idempotent).
//  2. Runs an XREADGROUP loop with a short block, parses each entry, looks up
//     the recipient email + locale via EmailLookup, renders the matching HTML
//     template, and Sends via email.Sender.
//  3. ACKs (XACK) once the email is sent OR once an entry is permanently
//     poisoned (so a malformed event doesn't block the group forever).
//  4. Retries transient errors up to `maxAttempts` (default 3) in-process; on
//     final failure forwards the entry to a dead-letter stream
//     (`<stream>:dead`) along with the error and original ID, then ACKs.
//
// Concurrency: a single consumer goroutine is enough at S2 scale — cert
// notifications are sparse compared to asynq email tasks. Multiple notifier
// replicas share the same group so Redis distributes entries 1:1.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kite365/idcd/apps/notifier/internal/email"
	"github.com/kite365/idcd/apps/notifier/internal/template"
	"github.com/kite365/idcd/lib/shared/contracts"
	"github.com/kite365/idcd/lib/shared/i18n"
)

// Cert event-type constants. Mirrored from apps/cert-svc — keeping the
// strings hard-coded here (rather than importing cert-svc) keeps notifier
// free of a cross-app dependency.
const (
	EventCertIssued        = "cert.issued"
	EventCertFailed        = "cert.failed"
	EventCertExpiring      = "cert.expiring"
	EventCertRenewalFailed = "cert.renewal_failed"
	EventCertRevoked       = "cert.revoked" // accepted but currently no email template
)

// DeadStreamSuffix is appended to CertConsumer.stream to form the dead-letter
// stream name (e.g. "cert:notifications:dead").
const DeadStreamSuffix = ":dead"

// defaultDashboardBase is the user-facing site used when building deep links
// for the email CTA buttons. Overridable via NewCertConsumer's option list.
const defaultDashboardBase = "https://idcd.com"

// CertConsumerDefaults captures the retry / block / batch knobs in one place
// so tests can override them without reaching into unexported fields.
const (
	defaultMaxAttempts   = 3
	defaultBlockTimeout  = 2 * time.Second
	defaultBatchSize     = 16
	defaultRetryBaseWait = 50 * time.Millisecond
)

// EmailLookup resolves an account_id to a recipient email + preferred locale.
// Returning ("", "", nil) is a soft-miss: the consumer ACKs the entry with a
// warn log instead of retrying (the user may have been deleted / opted out).
// Returning a non-nil error triggers the retry path.
type EmailLookup func(ctx context.Context, accountID int64) (recipient, locale string, err error)

// CertConsumer subscribes to a Redis Stream of cert notifications and emits
// localised HTML emails via email.Sender. Construct with NewCertConsumer.
type CertConsumer struct {
	rdb           redis.UniversalClient
	sender        email.Sender
	templates     *template.Templates
	lookup        EmailLookup
	logger        *slog.Logger
	stream        string
	deadStream    string
	group         string
	consumerName  string
	dashboardBase string
	registry      *i18n.Registry

	// Tunables (defaults captured in the const block above).
	maxAttempts   int
	blockTimeout  time.Duration
	batchSize     int64
	retryBaseWait time.Duration

	// clock is test-only; replaces time.Now for retry wait calculation.
	clock func() time.Time
}

// CertConsumerOption tunes a CertConsumer at construction time.
type CertConsumerOption func(*CertConsumer)

// WithCertConsumerName overrides the per-replica consumer name. Defaults to
// "<group>-<hostname>" so log lines disambiguate replicas.
func WithCertConsumerName(name string) CertConsumerOption {
	return func(c *CertConsumer) {
		if name != "" {
			c.consumerName = name
		}
	}
}

// WithCertDashboardBase overrides the deep-link base for CTA buttons. Useful
// for staging and local dev where the dashboard isn't on idcd.com.
func WithCertDashboardBase(base string) CertConsumerOption {
	return func(c *CertConsumer) {
		if base != "" {
			c.dashboardBase = strings.TrimRight(base, "/")
		}
	}
}

// WithCertMaxAttempts overrides the in-process retry cap. Defaults to 3.
func WithCertMaxAttempts(n int) CertConsumerOption {
	return func(c *CertConsumer) {
		if n > 0 {
			c.maxAttempts = n
		}
	}
}

// WithCertBlockTimeout overrides the XREADGROUP BLOCK duration. A short
// timeout lets the goroutine notice ctx cancellation quickly; a longer one
// reduces Redis round-trips on idle clusters.
func WithCertBlockTimeout(d time.Duration) CertConsumerOption {
	return func(c *CertConsumer) {
		if d > 0 {
			c.blockTimeout = d
		}
	}
}

// WithCertBatchSize overrides the XREADGROUP COUNT. Defaults to 16.
func WithCertBatchSize(n int64) CertConsumerOption {
	return func(c *CertConsumer) {
		if n > 0 {
			c.batchSize = n
		}
	}
}

// WithCertRetryBaseWait sets the base delay between in-process retries
// (linearly scaled by attempt number). Defaults to 50ms.
func WithCertRetryBaseWait(d time.Duration) CertConsumerOption {
	return func(c *CertConsumer) {
		if d >= 0 {
			c.retryBaseWait = d
		}
	}
}

// WithCertRegistry overrides the i18n registry used to validate the locale
// returned from EmailLookup. Tests use this to inject a hermetic registry.
func WithCertRegistry(reg *i18n.Registry) CertConsumerOption {
	return func(c *CertConsumer) {
		if reg != nil {
			c.registry = reg
		}
	}
}

// withCertClock is test-only.
func withCertClock(f func() time.Time) CertConsumerOption {
	return func(c *CertConsumer) {
		if f != nil {
			c.clock = f
		}
	}
}

// NewCertConsumer constructs a CertConsumer with defaults filled in. All
// required dependencies (rdb / sender / templates / lookup / logger / stream
// / group) must be non-nil / non-empty; missing values surface as an error
// at construction so wiring bugs fail fast.
func NewCertConsumer(
	rdb redis.UniversalClient,
	sender email.Sender,
	templates *template.Templates,
	lookup EmailLookup,
	logger *slog.Logger,
	stream, group string,
	opts ...CertConsumerOption,
) (*CertConsumer, error) {
	if rdb == nil {
		return nil, errors.New("cert consumer: redis client required")
	}
	if sender == nil {
		return nil, errors.New("cert consumer: email sender required")
	}
	if templates == nil {
		return nil, errors.New("cert consumer: templates required")
	}
	if lookup == nil {
		return nil, errors.New("cert consumer: email lookup required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if stream == "" {
		return nil, errors.New("cert consumer: stream name required")
	}
	if group == "" {
		return nil, errors.New("cert consumer: consumer group required")
	}

	c := &CertConsumer{
		rdb:           rdb,
		sender:        sender,
		templates:     templates,
		lookup:        lookup,
		logger:        logger,
		stream:        stream,
		deadStream:    stream + DeadStreamSuffix,
		group:         group,
		consumerName:  defaultConsumerName(group),
		dashboardBase: defaultDashboardBase,
		registry:      i18n.MustDefault(),
		maxAttempts:   defaultMaxAttempts,
		blockTimeout:  defaultBlockTimeout,
		batchSize:     defaultBatchSize,
		retryBaseWait: defaultRetryBaseWait,
		clock:         func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// defaultConsumerName derives a consumer name from the hostname so concurrent
// notifier replicas land in different XREADGROUP slots. Falls back to a
// deterministic "<group>-local" when os.Hostname fails (Docker without
// hostname injection).
func defaultConsumerName(group string) string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "local"
	}
	return group + "-" + host
}

// Stream returns the source stream name (without the dead-letter suffix).
func (c *CertConsumer) Stream() string { return c.stream }

// DeadStream returns the dead-letter stream name.
func (c *CertConsumer) DeadStream() string { return c.deadStream }

// ConsumerName returns the resolved consumer name (useful for tests / logs).
func (c *CertConsumer) ConsumerName() string { return c.consumerName }

// Run starts the consumer loop. Blocks until ctx is cancelled.
//
// The loop is intentionally simple: on each iteration we (1) ensure the
// consumer group exists, (2) XREADGROUP with a short block, (3) process each
// returned entry. Errors at the read or group-create layer are logged and
// re-tried after a short backoff — we never panic the goroutine.
func (c *CertConsumer) Run(ctx context.Context) error {
	if err := c.EnsureGroup(ctx); err != nil {
		// A failure here is recoverable (Redis flapping at startup); log and
		// keep going — the loop retries every iteration.
		c.logger.Warn("cert consumer: ensure group failed at startup (will retry)",
			"stream", c.stream, "group", c.group, "err", err)
	}
	c.logger.Info("cert consumer started",
		"stream", c.stream, "group", c.group, "consumer", c.consumerName)
	defer c.logger.Info("cert consumer stopped",
		"stream", c.stream, "group", c.group, "consumer", c.consumerName)

	for {
		if ctx.Err() != nil {
			return nil
		}

		// Best-effort re-create — XGROUP CREATE MKSTREAM is idempotent thanks
		// to BUSYGROUP handling in EnsureGroup, so calling it on every read
		// loop iteration is cheap and self-healing (e.g. someone DELs the
		// stream during a debug session).
		if err := c.EnsureGroup(ctx); err != nil {
			c.logger.Warn("cert consumer: ensure group failed (will retry)",
				"stream", c.stream, "err", err)
			if !sleepOrDone(ctx, time.Second) {
				return nil
			}
			continue
		}

		msgs, err := c.readGroup(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Warn("cert consumer: XREADGROUP failed (will retry)",
				"stream", c.stream, "err", err)
			if !sleepOrDone(ctx, time.Second) {
				return nil
			}
			continue
		}

		for _, msg := range msgs {
			c.handleEntry(ctx, msg)
		}
	}
}

// EnsureGroup is exported so the wiring layer (main.go) can probe the Redis
// connection at startup. Returns nil on first creation or BUSYGROUP.
func (c *CertConsumer) EnsureGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, "$").Err()
	if err == nil {
		return nil
	}
	// Idempotent path: BUSYGROUP means "the group already exists" — exactly
	// what we want. go-redis surfaces this as a string error so we string-
	// match (cf. consumer.go in apps/aggregator which uses the same pattern).
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

// readGroup performs a single XREADGROUP call.
func (c *CertConsumer) readGroup(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumerName,
		Streams:  []string{c.stream, ">"},
		Count:    c.batchSize,
		Block:    c.blockTimeout,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // BLOCK timed out with no messages — normal.
	}
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

// handleEntry is the per-message pipeline: parse → process (with retries) →
// ACK / dead-letter. Errors are logged with structured context but never
// returned — the parent loop must keep draining the stream.
func (c *CertConsumer) handleEntry(ctx context.Context, msg redis.XMessage) {
	evt, parseErr := parseCertEvent(msg.Values)
	if parseErr != nil {
		c.logger.Error("cert consumer: malformed event, dead-lettering",
			"stream", c.stream, "msg_id", msg.ID, "err", parseErr)
		c.deadLetter(ctx, msg, fmt.Sprintf("parse: %v", parseErr))
		c.ack(ctx, msg.ID)
		return
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		err := c.processEvent(ctx, evt)
		if err == nil {
			c.ack(ctx, msg.ID)
			return
		}
		lastErr = err
		// Skip / soft-fail: nothing more to do, ACK and move on.
		if errors.Is(err, errSkipEvent) {
			c.logger.Info("cert consumer: event skipped",
				"stream", c.stream, "msg_id", msg.ID,
				"event", evt.EventType, "reason", err)
			c.ack(ctx, msg.ID)
			return
		}
		c.logger.Warn("cert consumer: process attempt failed",
			"stream", c.stream, "msg_id", msg.ID,
			"event", evt.EventType, "attempt", attempt,
			"max_attempts", c.maxAttempts, "err", err)
		if attempt < c.maxAttempts {
			wait := time.Duration(attempt) * c.retryBaseWait
			if !sleepOrDone(ctx, wait) {
				return
			}
		}
	}

	c.logger.Error("cert consumer: max retries exhausted, dead-lettering",
		"stream", c.stream, "msg_id", msg.ID,
		"event", evt.EventType, "err", lastErr)
	c.deadLetter(ctx, msg, fmt.Sprintf("max_retries: %v", lastErr))
	c.ack(ctx, msg.ID)
}

// errSkipEvent is returned by processEvent when there is nothing the consumer
// can usefully do (e.g. EmailLookup returned no recipient — user opted out or
// has no email on file). The caller ACKs and logs INFO, NOT WARN.
var errSkipEvent = errors.New("cert consumer: skip event")

// processEvent dispatches a parsed event to the template renderer and sends
// the email. Returns errSkipEvent for soft-misses, a wrapped error for
// transient failures the retry loop should re-try.
func (c *CertConsumer) processEvent(ctx context.Context, evt certEvent) error {
	recipient, locale, err := c.lookup(ctx, evt.AccountID)
	if err != nil {
		return fmt.Errorf("email lookup: %w", err)
	}
	if recipient == "" {
		return fmt.Errorf("%w: no recipient for account_id=%d", errSkipEvent, evt.AccountID)
	}
	if !c.registry.IsSupported(locale) {
		locale = c.registry.DefaultCode()
	}

	html, subject, err := c.renderEvent(evt, locale)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if html == "" {
		// Unknown event type → soft-skip (e.g. cert.revoked has no template
		// today; the producer still XADDs it for audit purposes).
		return fmt.Errorf("%w: no template for event=%q", errSkipEvent, evt.EventType)
	}

	if err := c.sender.Send(ctx, email.Message{
		To:      recipient,
		Subject: subject,
		HTML:    html,
	}); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	c.logger.Info("cert notification email sent",
		"event", evt.EventType,
		"account_id", evt.AccountID,
		"cert_id", evt.CertID,
		"to", recipient,
		"locale", locale,
	)
	return nil
}

// renderEvent picks the template + subject for the event type. Subjects are
// constructed inline (rather than via the shared i18n catalog) because they
// embed dynamic substitutions and the four cert-specific keys would clutter
// the global email subject namespace. Adding catalog keys is a future
// improvement; the inline approach ships today.
func (c *CertConsumer) renderEvent(evt certEvent, locale string) (html, subject string, err error) {
	domain := evt.primaryDomain()
	sans := strings.Join(evt.SANs, ", ")
	notAfter := formatCertTime(evt.NotAfter)

	switch evt.EventType {
	case EventCertIssued:
		subject = certSubjectIssued(locale, domain)
		html, err = c.templates.RenderCertIssued(locale, template.CertIssuedData{
			Domain:       domain,
			SANs:         sans,
			CA:           evt.CA,
			NotAfter:     notAfter,
			CertID:       evt.CertID,
			DashboardURL: c.certDashboardURL(evt.CertID),
		})
	case EventCertFailed:
		subject = certSubjectFailed(locale, domain)
		html, err = c.templates.RenderCertFailed(locale, template.CertFailedData{
			Domain:       domain,
			SANs:         sans,
			CA:           evt.CA,
			OrderID:      evt.OrderID,
			ErrorMessage: evt.ErrorMessage,
			DashboardURL: c.orderDashboardURL(evt.OrderID),
		})
	case EventCertExpiring:
		subject = certSubjectExpiring(locale, domain, evt.DaysToExpire)
		html, err = c.templates.RenderCertExpiring(locale, template.CertExpiringData{
			Domain:       domain,
			SANs:         sans,
			CA:           evt.CA,
			NotAfter:     notAfter,
			CertID:       evt.CertID,
			Days:         evt.DaysToExpire,
			DashboardURL: c.certDashboardURL(evt.CertID),
		})
	case EventCertRenewalFailed:
		subject = certSubjectRenewalFailed(locale, domain)
		html, err = c.templates.RenderCertRenewalFailed(locale, template.CertRenewalFailedData{
			Domain:       domain,
			SANs:         sans,
			CA:           evt.CA,
			NotAfter:     notAfter,
			CertID:       evt.CertID,
			ErrorMessage: evt.ErrorMessage,
			DashboardURL: c.certDashboardURL(evt.CertID),
		})
	case EventCertRevoked:
		// No email template yet — caller's processEvent treats empty html as
		// a soft skip.
		return "", "", nil
	default:
		return "", "", nil
	}
	return html, subject, err
}

// deadLetter writes a copy of msg to <stream>:dead so the entry is durable
// and reviewable by operators. Failures here are logged but never bubble up
// — losing one dead-letter copy is preferable to blocking the main loop.
func (c *CertConsumer) deadLetter(ctx context.Context, msg redis.XMessage, reason string) {
	values := make(map[string]any, len(msg.Values)+3)
	for k, v := range msg.Values {
		values[k] = v
	}
	values["_dead_reason"] = reason
	values["_dead_original_id"] = msg.ID
	values["_dead_at"] = c.clock().Format(time.RFC3339)
	if err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: c.deadStream,
		Values: values,
	}).Err(); err != nil {
		c.logger.Error("cert consumer: XADD to dead stream failed",
			"dead_stream", c.deadStream,
			"original_id", msg.ID,
			"reason", reason,
			"err", err,
		)
	}
}

// ack wraps XACK so failures only warn (an unacked entry resurfaces via
// XAUTOCLAIM in any other notifier replica — non-fatal).
func (c *CertConsumer) ack(ctx context.Context, id string) {
	if err := c.rdb.XAck(ctx, c.stream, c.group, id).Err(); err != nil {
		c.logger.Warn("cert consumer: XACK failed",
			"stream", c.stream, "group", c.group, "msg_id", id, "err", err)
	}
}

// certDashboardURL builds the dashboard deep link for a cert detail page.
// Falls back to the list page when certID is zero.
func (c *CertConsumer) certDashboardURL(certID int64) string {
	if certID <= 0 {
		return c.dashboardBase + "/app/cert"
	}
	return fmt.Sprintf("%s/app/cert/%d", c.dashboardBase, certID)
}

// orderDashboardURL builds the dashboard deep link for an order detail page.
// Falls back to the list page when orderID is zero.
func (c *CertConsumer) orderDashboardURL(orderID int64) string {
	if orderID <= 0 {
		return c.dashboardBase + "/app/cert/orders"
	}
	return fmt.Sprintf("%s/app/cert/orders/%d", c.dashboardBase, orderID)
}

// certEvent is the consumer-side view of a single cert:notifications stream
// entry. Underneath this is a thin adapter around contracts.CertNotificationEvent
// — the same fields, but with AccountID coerced from string to int64 because
// every notifier code path downstream (EmailLookup, log structured fields,
// dashboard deep links) needs the numeric form.
type certEvent struct {
	EventType    string
	AccountID    int64
	CertID       int64
	OrderID      int64
	SANs         []string
	CA           string
	DaysToExpire int
	ErrorMessage string
	NotAfter     time.Time
	Subject      string // optional, preset by producer
	Body         string // optional, preset by producer (plain-text fallback)
	EmittedAt    time.Time
}

func (e certEvent) primaryDomain() string {
	if len(e.SANs) > 0 {
		return e.SANs[0]
	}
	return ""
}

// parseCertEvent decodes a single XMessage.Values map into a certEvent by
// delegating to contracts.ParseCertNotificationEvent (the SSOT for the
// cert:notifications wire format, see lib/shared/contracts/cert_notification_event.go).
//
// P0-4 W2 migration: 旧的 `payload` JSON 二层 wire format 已不再支持 —
// producer (cert-svc NotificationWatcher) 同步迁移到 flat layout, 旧 in-flight
// 消息因 missing 顶层字段会被本函数拒绝并进入 dead-letter (deadStream)。
func parseCertEvent(values map[string]any) (certEvent, error) {
	parsed, err := contracts.ParseCertNotificationEvent(values)
	if err != nil {
		// Surface the contract error verbatim so dead-letter `_dead_reason`
		// strings stay informative ("event is required" / "cert_id: ..." / etc).
		return certEvent{}, err
	}
	evt := certEvent{
		EventType:    parsed.EventType,
		CertID:       parsed.CertID,
		OrderID:      parsed.OrderID,
		SANs:         parsed.SANs,
		CA:           parsed.CA,
		DaysToExpire: parsed.DaysToExpire,
		ErrorMessage: parsed.ErrorMessage,
		NotAfter:     parsed.NotAfter,
		Subject:      parsed.Subject,
		Body:         parsed.Body,
		EmittedAt:    parsed.EmittedAt,
	}
	// AccountID is string on the wire (UUID-safe); parse to int64 for the
	// consumer code path. Empty / non-numeric strings degrade to 0, matching
	// the historical int64Field behavior.
	if parsed.AccountID != "" {
		if n, perr := strconv.ParseInt(parsed.AccountID, 10, 64); perr == nil {
			evt.AccountID = n
		}
	}
	return evt, nil
}

// stringField extracts a string value from XMessage.Values. Stream fields
// arrive as `string` from go-redis, but tests may inject any concrete value
// (including []byte) so we accept the common variants.
func stringField(values map[string]any, key string) string {
	raw, ok := values[key]
	if !ok {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

// int64Field parses a stream field as int64, returning 0 on missing / bad.
func int64Field(values map[string]any, key string) int64 {
	s := stringField(values, key)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// sleepOrDone waits d, returning false if ctx cancelled before the wait
// elapses. Used by the retry / re-read backoffs.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// formatCertTime renders an expiry timestamp the same way cert-svc does
// internally — keeping consistency between the email body and the producer's
// own audit log.
func formatCertTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// ---- subject helpers ----
//
// We special-case locale "en" vs everything else (defaulting to Chinese)
// because the registry default is "cn" and any non-English locale falls back
// to Chinese copy.  Adding more locales here is a simple switch extension.

func certSubjectIssued(locale, domain string) string {
	if locale == "en" {
		return fmt.Sprintf("[idcd] Your SSL certificate is ready: %s", domain)
	}
	return fmt.Sprintf("【idcd】您的 SSL 证书已签发：%s", domain)
}

func certSubjectFailed(locale, domain string) string {
	if locale == "en" {
		return fmt.Sprintf("[idcd] Certificate request failed: %s", domain)
	}
	return fmt.Sprintf("【idcd】证书申请失败：%s", domain)
}

func certSubjectExpiring(locale, domain string, days int) string {
	if locale == "en" {
		return fmt.Sprintf("[idcd] Certificate expiring soon: %s (%d days left)", domain, days)
	}
	return fmt.Sprintf("【idcd】证书即将到期提醒：%s（剩余 %d 天）", domain, days)
}

func certSubjectRenewalFailed(locale, domain string) string {
	if locale == "en" {
		return fmt.Sprintf("[idcd] Certificate auto-renewal failed: %s", domain)
	}
	return fmt.Sprintf("【idcd】证书自动续期失败：%s", domain)
}
