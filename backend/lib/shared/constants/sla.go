package constants

import "time"

// Verdict / 客服 SLA 阶梯 (对应 D12 决策).
//
// D12 决策摘要: 3 档 SLA, Verdict 失败纯自动 / 1h 仅 P0 (KMS / 节点失窃) /
// 24h 常规客服. Pre-4 接受 SLA 偶尔滑至 24h+ 现实风险.
const (
	// VerdictAutoTimeout 是 Verdict 自动流程超时阈值 (0 = 不超时, 走完即完).
	// 对应 D12 "Verdict 失败纯自动" 档.
	VerdictAutoTimeout = time.Duration(0)

	// VerdictCriticalP0SLA 是 P0 事件 (KMS 失效 / 节点失窃 / Verdict 系统宕机)
	// 必须响应的最长时间. 对应 D12 "1h 仅 P0" 档.
	VerdictCriticalP0SLA = 1 * time.Hour

	// VerdictRoutineSLA 是常规客服工单的响应上限. 对应 D12 "24h 常规客服" 档.
	// Pre-4 允许 SLA 偶尔滑至 24h+ (Backup HSM 推迟 S4 的代价).
	VerdictRoutineSLA = 24 * time.Hour
)
