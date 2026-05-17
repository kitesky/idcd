// Package wstimeouts centralises the WebSocket / SSE timeout & heartbeat
// constants shared by the gateway server (apps/gateway) and the agent client
// (apps/agent).
//
// 背景
//
// gateway 和 agent 是 WebSocket 协议的两端：gateway 服务端 SetReadDeadline /
// PingHandler，agent 客户端 SetPongHandler / WriteMessage(Ping)。两端的超时
// 必须配套，否则一边在重连而另一边还在等数据，或者一边发心跳频率太低被另
// 一边判定为掉线。
//
// 之前 apps/gateway/internal/handler/ws.go 和 apps/agent/internal/ws/client.go
// 各写了一份几乎相同的 const 块（pingInterval=54s, writeTimeout=10s ...），
// 调整时容易只改一边导致 silent breakage。
//
// 现在两端都从本包读，确保一致；调整这里时需同时评估服务端 + 客户端的影响。
//
// 调参原则
//
//   - PongTimeout 必须 > PingInterval（约 1.1x 缓冲）；否则上行心跳还没到，
//     下行 pong 已超时。
//   - PingInterval = 0.9 * PongTimeout 是 gorilla/websocket 官方示例的取值。
//   - WriteTimeout 是单帧的写超时，与上面三者无强关系，只影响"网络突然掉线
//     时阻塞多久才被检测出来"。值越小检测越快但偶发抖动可能误杀连接。
//   - HeartbeatInterval 是 agent 主动发送 application-level heartbeat 的频率
//     （独立于 WebSocket ping/pong frame），用于业务侧统计节点活性。
//   - BackoffMin / BackoffMax 限定 agent 重连的指数退避区间。
package wstimeouts

import "time"

// 协议级 ping/pong & I/O 超时（gateway 服务端 + agent 客户端共用）。
const (
	// PingInterval 是 agent 端 WriteMessage(websocket.PingMessage, ...) 的频率。
	// 应满足 PingInterval < PongTimeout，留出网络抖动的余量。
	PingInterval = 54 * time.Second

	// PongTimeout 既是 gateway 服务端 SetReadDeadline 的值，
	// 也是 agent 客户端 SetReadDeadline 的值。读取超过此时长仍无数据
	// 即认定连接异常并触发重连。
	PongTimeout = 60 * time.Second

	// WriteTimeout 是单次 WriteMessage 的最长允许时长。
	// 触达即认为下行链路异常并断开连接。
	WriteTimeout = 10 * time.Second

	// MaxMessageBytes 是单帧入站消息的硬上限。
	// agent 上报的 heartbeat / result / cmd_ack 都远小于此值；
	// 显式 SetReadLimit 防止恶意 agent 发巨型 frame 打爆 gateway 内存。
	MaxMessageBytes = 64 * 1024
)

// 业务级 heartbeat / 重连退避（agent 端使用，gateway 端不感知）。
const (
	// HeartbeatInterval 是 agent 发送 application-level "heartbeat" 业务消息的频率。
	// 与 WebSocket 协议的 ping frame 不同：ping 是握手保活，heartbeat 是给业务侧
	// 上报节点状态（连接活跃但不一定健康）。
	HeartbeatInterval = 30 * time.Second

	// BackoffMin / BackoffMax 限定 agent 在 WebSocket 断开后指数退避重连的区间。
	// 第一次失败 BackoffMin 后重试，后续每次失败 *2，直到 BackoffMax。
	// 调小 BackoffMax 可让节点恢复更快，但会在 gateway 大面积故障时放大请求风暴。
	BackoffMin = 1 * time.Second
	BackoffMax = 60 * time.Second
)
