// Package contracts is the SSOT for cross-service Redis Streams message payloads.
//
// 设计动机 (ARCHITECTURE-REVIEW-2026-05-21.md P0-4):
//
// 在引入此包之前，跨服务的 Redis Stream payload 全部是 `map[string]any`,
// 生产者 (gateway/scheduler) 和消费者 (aggregator/notifier) 各自靠字符串
// 字面量约定字段名。一次字段名拼写错误（如 "duration_ms" 写成 "duraion_ms"）
// 会导致运行时静默丢数据，没有任何编译期或单测信号。
//
// 本包通过强类型结构体 + ToStreamValues / Parse* 函数对，把"字段名"这层
// 契约从字符串字面量提升为 Go 类型系统的一等公民:
//
//   - 生产者: stream.Client.AddProbeResultTyped(ctx, contracts.ProbeResult{...})
//   - 消费者: contracts.ParseProbeResult(streamMsg.Values)
//
// 任何拼错字段名的代码都将无法通过编译。
//
// # Schema 演进规则
//
// 每个 payload 类型携带 `SchemaVer` 字段，版本号语义:
//
//   - 新增"可选"字段 (指针 / `omitempty`) → SchemaVer 不变
//   - 删除字段 / 改字段语义 / 改字段类型 → SchemaVer +1, 消费端必须分支处理
//   - SchemaVer == 0 视为旧消息 (在本包出现前的写入)，按当前 V1 schema 兜底解析
//   - SchemaVer 大于消费端已知最大版本 → 返回 ErrUnknownSchemaVer,
//     调用方决定 drop / DLQ / panic, 不在解析层强行兜底
//
// # 字段命名规则
//
// 每个字段的 stream tag 值必须与现有 map[string]any 写入时使用的 key 完全一致,
// 以保证已在 Redis 中的存量消息能被新消费者继续解析。这是有意的"做减法":
// 我们只是把字段名集中托管到一个 Go 文件，不引入新的 wire format。
//
// JSON tag 用于应用层日志 / HTTP debug 输出, 与 stream tag 可以不同
// (例如 SchemaVer 在 JSON 里是 schema_ver, 在 stream 里也是 schema_ver)。
//
// # 迁移兼容性注意
//
// MonitorEvent 把现有 `AddMonitorEvent(extra map[string]any)` 的扁平 extra
// 改成单字段 `extra` (JSON-encoded bytes) — 这是一个**有意的破坏性变更**,
// 目的是消除"什么字段都能塞进 stream"的隐患。AddMonitorEvent 原 API 保留
// 兼容 (deprecated), 现有调用方可继续使用; 新代码必须走 AddMonitorEventTyped
// + contracts.MonitorEvent.
package contracts
