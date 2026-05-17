// Package rediskey provides typed constructors for Redis keys whose name
// schema is shared across idcd services.
//
// 背景
//
// 像 `cert:expiring:notified:<cert_id>:<bucket>` 这种 key 之前是 cert-svc
// 用 fmt.Sprintf 拼出来的字面量；notifier / 测试 / 监控脚本要读同一把 key
// 时只能 copy 同一段格式串，schema 一旦变动需要在多处同步改。
//
// 现在所有读写共享 key 的代码统一调用本包的构造器，schema 变更只影响这里
// 一处；同时构造器签名上的参数类型让"key 应该用 cert_id 还是 order_id"
// 这种细节问题在编译期就能暴露。
//
// 命名约定
//
// 函数名一律按 "实体 + 行为 + 限定词" 命名：
//
//	CertExpiringNotifiedKey(certID, bucket) — "对 cert 的 expiring 通知已发"
//	CertRenewalFailedNotifiedKey(jobID)     — "对 renewal_job 的失败通知已发"
//
// schema 规则：":" 作为命名空间分隔符 (与 cert:notifications 等 stream 一致)，
// 实体 ID 直接写十进制，时间相关参数（如 expiring 的 bucket，单位"距过期的天数"）
// 紧跟实体 ID 后。
package rediskey

import "strconv"

// CertExpiringNotifiedKey 是 "cert <certID> 在 <bucket> 天窗口的过期通知
// 已发出" 的 dedupe SETNX key。
//
//	cert:expiring:notified:<cert_id>:<bucket>
//
// bucket 取自 NotificationWatcher.expiringDays（默认 30/14/7/1）；
// TTL 由 cert-svc service.expiringTTL 控制（7 天）。
//
// 同 bucket 重复进入窗口时 SetNX 第二次必失败，从而保证一个 bucket 一封邮件。
func CertExpiringNotifiedKey(certID int64, bucket int) string {
	return "cert:expiring:notified:" + strconv.FormatInt(certID, 10) +
		":" + strconv.Itoa(bucket)
}

// CertRenewalFailedNotifiedKey 是 "对 renewal_job <jobID> 的 'renewal_failed'
// 通知已发出" 的 dedupe SETNX key。
//
//	cert:renewal_failed:notified:<job_id>
//
// TTL 由 cert-svc service.renewalFailedTTL 控制（30 天）—— 比 expiring 长是
// 因为 renewal job 失败后等待 30 天人工干预的窗口足够。
func CertRenewalFailedNotifiedKey(jobID int64) string {
	return "cert:renewal_failed:notified:" + strconv.FormatInt(jobID, 10)
}
