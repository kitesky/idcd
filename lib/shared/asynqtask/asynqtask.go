// Package asynqtask centralises asynq queue names, task type names, and the
// D5 refund-retry scheduling policy that producer services (apps/api,
// apps/attest, apps/aggregator) share with the consumer (apps/notifier).
//
// 背景
//
// asynq 的 task type 和 queue name 都是"魔法字符串"——生产者 NewTask 写一遍，
// 消费者 mux.HandleFunc 再注册一遍，但 asynq 本身不强制类型一致。之前这两
// 端各自定义了 `TaskRefundRetry = "payment:refund_retry"`、`QueueBilling =
// "billing"` 等常量，改一边漏改另一边只能等运行时 "task dropped" 报警。
//
// 现在所有 producer / consumer 都从本包读，asynq 协议层契约只在此处变更。
//
// D5 refund retry 调参
//
// CLAUDE.md §M D5 锁定 "Paddle/PaymentHub refund 失败 5min/30min 重试，
// 30min 内强制发用户道歉邮箱"。本包的 RefundRetryFirstDelay /
// RefundRetrySecondDelay / RefundRetryMaxAttempts 直接落地这条决策：
//
//   T+0      退款 API 第一次失败 → enqueue retry，delay = FirstDelay (5min)
//   T+5min   retry 1 失败 → enqueue retry，delay = SecondDelay (25min)
//                          (此时距 T+0 共 5+25=30min — 与 D5 文案一致)
//   T+30min  retry 2 失败 (attempt_count == MaxAttempts) →
//                          停止重试，触发 refund_apology 邮箱给用户兜底
//
// 调整任一值需同步评估：
//   - Paddle / PaymentHub API 限流配额
//   - 道歉邮箱模板里的 "30 分钟内" 措辞 (apps/notifier/internal/template)
//   - admin dashboard "refund_failed" 状态告警阈值 (CLAUDE.md D5)
package asynqtask

import "time"

// Queue names — asynq.Queue(...) options & notifier 端的 queue 优先级表均从此引用.
const (
	// QueueBilling 携带 D5 payment retry / apology 任务。
	// notifier 端优先级最高 (5)，确保配额上 payment 任务总能跑在 cert / alert
	// 之前。
	QueueBilling = "billing"

	// QueueNotifierDefault 是普通通知任务（监控告警 / 系统消息）的默认队列。
	// notifier 端优先级 1。
	QueueNotifierDefault = "notifier:default"

	// QueueNotifierCritical 是紧急通知队列（KMS 异常、节点失窃等 P0 事件）。
	// notifier 端优先级 2，介于 default 与 billing 之间。
	QueueNotifierCritical = "notifier:critical"
)

// Task type names — asynq.NewTask(type, ...) 与 mux.HandleFunc(type, ...) 共用.
const (
	// TaskRefundRetry — D5 退款重试任务。
	// 生产者: apps/api admin_billing (人工 retry)、apps/notifier handlers
	// (自动 retry chain)。消费者: apps/notifier worker.HandleRefundRetry。
	TaskRefundRetry = "payment:refund_retry"

	// TaskRefundApology — D5 退款道歉邮箱兜底任务。
	// 生产者: apps/attest refund-worker (重试链耗尽后兜底)、apps/notifier
	// (retry 链终态)。消费者: apps/notifier worker.HandleRefundApology。
	TaskRefundApology = "payment:refund_apology"

	// TaskAlertNotification — 监控告警通知任务。
	// 生产者: apps/aggregator processor.alert_trigger。消费者: apps/notifier
	// worker 默认 alert handler。
	TaskAlertNotification = "alert:notification"
)

// D5 Refund retry scheduling policy.
const (
	// RefundRetryFirstDelay 是 refund 第一次失败后的等待时长。
	// 之后 enqueue 一个 TaskRefundRetry 在该时长后执行。
	RefundRetryFirstDelay = 5 * time.Minute

	// RefundRetrySecondDelay 是 refund 第二次失败后的等待时长。
	// FirstDelay + SecondDelay = 30min 与 D5 "30min 内发道歉邮箱" 对齐。
	RefundRetrySecondDelay = 25 * time.Minute

	// RefundRetryMaxAttempts 是触发道歉邮箱前的最大自动重试次数。
	// 超过此值后由 refund-worker 切换到 TaskRefundApology 路径。
	RefundRetryMaxAttempts = 2
)
