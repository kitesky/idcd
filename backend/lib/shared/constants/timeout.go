package constants

import "time"

// 杂项业务 timeout 常量.
const (
	// WebAuthnChallengeTTL 是 WebAuthn 注册 / 登录 challenge 的有效期.
	// 用户必须在此窗口内完成生物认证, 否则 challenge 失效 → 重新触发.
	WebAuthnChallengeTTL = 5 * time.Minute

	// StreamConsumerClaimMinIdle 是 Redis Streams 消费组 XAUTOCLAIM 的
	// min-idle-time 阈值: 消息在 PEL 中超过此时长仍未 ack, 视为消费者
	// 卡死, 其它消费者可接管. 对应 5min 心跳的 dispatcher / aggregator.
	StreamConsumerClaimMinIdle = 5 * time.Minute

	// MonitorFlapThreshold 是 monitor 状态翻转抑制窗口.
	// 同一 monitor 在窗口内 up/down 反复切换会被合并成单条 flap 事件,
	// 避免通知风暴.
	MonitorFlapThreshold = 5 * time.Minute
)
