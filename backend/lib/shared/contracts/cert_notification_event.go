package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// CertNotificationEvent is the standard payload of the `cert:notifications`
// Redis stream (钱相关 / 合规相关 — 证书签发 / 续期 / 失败 / 即将到期通知).
//
// 生产者: cert-svc NotificationWatcher (apps/cert-svc/internal/service/notifications.go)
// 消费者: notifier cert consumer (apps/notifier/internal/worker/cert_consumer.go)
//
// 字段对应关系 (与历史 XAdd 写入的兼容性):
//
//   旧 wire layout (P0-4 W2 之前):
//     - event / account_id / cert_id / order_id / emitted_at      顶层
//     - payload                                                    JSON 二层, 内含
//       account_id / cert_id / order_id / sans / ca /
//       days_to_expire / error_message / not_after / subject / body
//
//   新 wire layout (P0-4 W2 后, 由 ToStreamValues 写出):
//     - 所有字段平铺成 stream values 顶层, 不再有 "payload" 二层 JSON
//     - sans 用 JSON-encode 单 stream key (因为 stream value 必须是 scalar)
//     - 缺省字段一律 omit (而非写空字符串), 与 ProbeResult / MonitorEvent 一致
//
// 重要: cert:notifications 这次没有灰度兼容期 — 必须 producer 与 consumer 同时
// 升级。理由: 在线流量极小 (S2 期, cert-svc 一天写几十条), 旧 in-flight 消息可
// 接受被丢弃; 保留旧 "payload JSON 二层" 解析路径只会让 contracts 包变成
// "什么字段都能塞" 的反模式, 失去 P0-4 编译期检查字段名拼写错误的目标。
type CertNotificationEvent struct {
	SchemaVer int    `json:"schema_ver"               stream:"schema_ver"`
	EventType string `json:"event"                    stream:"event"`

	// AccountID 是 string 而非 int64, 因为生产者侧 NotificationData.AccountID
	// 历史上就是 string (UUID / 数字串都可能). 消费者侧若需要 int64, 自行 strconv.
	AccountID string `json:"account_id"               stream:"account_id"`
	CertID    int64  `json:"cert_id"                  stream:"cert_id"`
	OrderID   int64  `json:"order_id"                 stream:"order_id"`

	// SANs 在 stream 中以 JSON-encoded 数组字符串存储 (Redis stream value 是
	// scalar). 空 / nil 一律 omit.
	SANs []string `json:"sans,omitempty"             stream:"sans,omitempty"`

	CA           string `json:"ca,omitempty"             stream:"ca,omitempty"`
	DaysToExpire int    `json:"days_to_expire,omitempty" stream:"days_to_expire,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"  stream:"error_message,omitempty"`

	// NotAfter zero value (time.Time{}.IsZero() == true) 视为 "不适用",
	// 与 NotificationData.NotAfter == nil 等价. ToStreamValues 在 zero 时 omit.
	NotAfter time.Time `json:"not_after,omitempty"       stream:"not_after,omitempty"`

	Subject string `json:"subject,omitempty"           stream:"subject,omitempty"`
	Body    string `json:"body,omitempty"              stream:"body,omitempty"`

	// EmittedAt 是生产者标注的事件时间 (与历史 emitted_at 字段一致). zero
	// 时 ToStreamValues 兜底为 time.Now().UTC(), 行为与旧 watcher 一致。
	EmittedAt time.Time `json:"emitted_at"               stream:"emitted_at"`
}

// CertNotificationEventSchemaV1 is the current wire format version.
// Increment only on breaking field changes (rename / remove / retype),
// not for additive optional fields.
const CertNotificationEventSchemaV1 = 1

// nowUTC is the timestamp source for EmittedAt fallback, overridable in tests.
var nowUTC = func() time.Time { return time.Now().UTC() }

// ToStreamValues converts the struct into a Redis XAdd values map.
//
// Required fields (event_type / account_id / cert_id / order_id / emitted_at)
// are always written. Optional fields are omitted when zero-valued so the
// wire format stays compact.
//
// SchemaVer auto-defaults to CertNotificationEventSchemaV1 when zero.
// EmittedAt auto-defaults to time.Now().UTC() when zero.
func (e CertNotificationEvent) ToStreamValues() map[string]any {
	schemaVer := e.SchemaVer
	if schemaVer == 0 {
		schemaVer = CertNotificationEventSchemaV1
	}
	emitted := e.EmittedAt
	if emitted.IsZero() {
		emitted = nowUTC()
	}

	vals := map[string]any{
		"schema_ver": strconv.Itoa(schemaVer),
		"event":      e.EventType,
		"account_id": e.AccountID,
		"cert_id":    strconv.FormatInt(e.CertID, 10),
		"order_id":   strconv.FormatInt(e.OrderID, 10),
		"emitted_at": emitted.UTC().Format(time.RFC3339),
	}
	if len(e.SANs) > 0 {
		// json.Marshal on a []string never fails — drop the error.
		raw, _ := json.Marshal(e.SANs)
		vals["sans"] = string(raw)
	}
	if e.CA != "" {
		vals["ca"] = e.CA
	}
	if e.DaysToExpire != 0 {
		vals["days_to_expire"] = strconv.Itoa(e.DaysToExpire)
	}
	if e.ErrorMessage != "" {
		vals["error_message"] = e.ErrorMessage
	}
	if !e.NotAfter.IsZero() {
		vals["not_after"] = e.NotAfter.UTC().Format(time.RFC3339)
	}
	if e.Subject != "" {
		vals["subject"] = e.Subject
	}
	if e.Body != "" {
		vals["body"] = e.Body
	}
	return vals
}

// ParseCertNotificationEvent decodes a Redis stream values map into a
// CertNotificationEvent.
//
// Strict requirements (return error):
//   - event must be non-empty
//   - schema_ver, if present and > CertNotificationEventSchemaV1, returns
//     ErrUnknownSchemaVer
//   - sans, if present, must be a JSON-encoded array (or be a string field)
//
// Lenient (best-effort decode, no error):
//   - missing optional fields → zero value
//   - cert_id / order_id / days_to_expire accept int64 / float64 / string
//   - emitted_at / not_after missing or malformed → zero time.Time
//   - account_id missing → empty string (caller decides whether that's fatal)
//
// Note: account_id is intentionally NOT required at parse time because not
// all callers depend on it (and the producer always writes it). Treating it
// as required here would only catch producer bugs we already catch via tests.
func ParseCertNotificationEvent(vals map[string]any) (CertNotificationEvent, error) {
	e := CertNotificationEvent{}

	// schema_ver
	switch v := vals["schema_ver"].(type) {
	case nil:
		e.SchemaVer = 0 // legacy message
	case int:
		e.SchemaVer = v
	case int64:
		e.SchemaVer = int(v)
	case float64:
		e.SchemaVer = int(v)
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: schema_ver=%q: %w", v, err)
		}
		e.SchemaVer = n
	default:
		return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: schema_ver has unexpected type %T", v)
	}
	if e.SchemaVer > CertNotificationEventSchemaV1 {
		return CertNotificationEvent{}, fmt.Errorf("%w: cert_notification_event schema_ver=%d (known max=%d)",
			ErrUnknownSchemaVer, e.SchemaVer, CertNotificationEventSchemaV1)
	}

	event, _ := vals["event"].(string)
	if event == "" {
		return CertNotificationEvent{}, errors.New("contracts.ParseCertNotificationEvent: event is required")
	}
	e.EventType = event

	e.AccountID, _ = vals["account_id"].(string)
	e.CA, _ = vals["ca"].(string)
	e.ErrorMessage, _ = vals["error_message"].(string)
	e.Subject, _ = vals["subject"].(string)
	e.Body, _ = vals["body"].(string)

	if v, ok := vals["cert_id"]; ok {
		n, err := parseInt64(v)
		if err != nil {
			return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: cert_id: %w", err)
		}
		e.CertID = n
	}
	if v, ok := vals["order_id"]; ok {
		n, err := parseInt64(v)
		if err != nil {
			return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: order_id: %w", err)
		}
		e.OrderID = n
	}
	if v, ok := vals["days_to_expire"]; ok {
		n, err := parseInt64(v)
		if err != nil {
			return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: days_to_expire: %w", err)
		}
		e.DaysToExpire = int(n)
	}

	if v, ok := vals["sans"]; ok && v != nil {
		var raw string
		switch x := v.(type) {
		case string:
			raw = x
		case []byte:
			raw = string(x)
		default:
			return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: sans has unexpected type %T", v)
		}
		if raw != "" {
			var sans []string
			if err := json.Unmarshal([]byte(raw), &sans); err != nil {
				return CertNotificationEvent{}, fmt.Errorf("contracts.ParseCertNotificationEvent: sans: %w", err)
			}
			e.SANs = sans
		}
	}

	if s, ok := vals["emitted_at"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			e.EmittedAt = t
		}
	}
	if s, ok := vals["not_after"].(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			e.NotAfter = t
		}
	}

	return e, nil
}
