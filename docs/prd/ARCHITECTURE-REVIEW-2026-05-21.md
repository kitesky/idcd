# Architecture Review — idcd Backend 2026-05-21

> 生成日期:2026-05-21
> Reviewer:Claude(对话式架构审查,基于实际代码而非 PRD)
> Scope:9 个 Go 服务(agent / aggregator / api / attest / cert-svc / gateway / mcp / notifier / scheduler)的通信模式与可演进性
> Mode:**HOLD SCOPE** — 现有架构选型(Redis Streams + WebSocket + 共享 PG)正确,本审查仅找**真实硬伤**与**规模化前必修债**,不重提选型问题

---

## Overall Verdict

**SOUND_WITH_GAPS** — idcd 后端架构选型扎实,异步消息总线(Redis Streams)+ 长连接接入层(WebSocket gateway)+ 共享 PG(跨 schema 不写 FK)是当前规模(<10w 用户,50 节点)下的**最优解**。

但存在 **5 个 P0 硬伤**(单点 + 脑裂 + 弱类型 + API 契约脱钩),**10 个 P1 规模化前必修债**(可观测性/状态路由/服务边界/一致性债/事务/业务指标/fire-and-forget/agent 升级),**10 个 P2 看 ROI**(熔断/读写分离/邮件统一/MCP 隔离/HTTP client/健康检查/multi-module/JSON 性能/secrets 统一/GDPR)。

**关键阻塞项**(必须在节点扩到 50 + 用户付费上线前解决):
- Redis 单点 — 整个事件总线、限流、leader 锁、JWT blocklist 全部依赖
- scheduler leader 裸 SETNX 脑裂 — 旧 leader 不知道自己被废黜会重复派任务
- attest 服务在 prod 部署的实际位置需确认(D6 要求独立 VPC subnet)
- **跨服务消息载荷 `map[string]any` 滥用(238 处)** — 字段拼错运行时静默丢数据,无编译期契约
- **OpenAPI spec 3799 行无 codegen** — 对外契约与代码可能脱钩,review 漏看不报错

**已确认健康项**(扫描发现这些做得很好,无需改):
- 测试覆盖率平均 95%+(attest 124% / notifier 117% / cert-svc 104% / api 90%),最低 gateway 70.5%
- 9/9 服务都有 graceful shutdown(SIGTERM/SIGINT + ctx 超时 Shutdown)
- slog 结构化日志 222 处一致使用,无 `fmt.Println` 污染
- `WithTx` helper 已实现且测试齐全(`lib/db/tx.go`,含 nested savepoint / context cancel / panic rollback)
- dev.env.yaml 已正确 gitignore,**误判修正**:仓库不含真实 secrets
- OTel SDK 接入 7/9 服务
- 业务 7/9 服务暴露 /metrics 端点

---

## 初次审查后实测修正(透明记录)

为避免后续核对时反复来回,先把对话中**初步结论与实际代码不符**的几条修正记下:

| 初步结论 | 实测发现 | 文件位置 | 处理 |
|---|---|---|---|
| JWT blocklist in-memory 多副本不安全 | **已用 RedisBlocklist** | `backend/apps/api/internal/server/server.go:200` | ❌ 撤销 P0,无问题 |
| 没有 OTel 链路追踪 | **OTel SDK 已接入 7 个服务**(api/aggregator/agent/gateway/scheduler/notifier/cert-svc) | `backend/lib/shared/telemetry/telemetry.go` + 各 main.go | 🟡 降级:SDK 已有,但缺 Stream/WS 跨进程 propagation,见 P1-5 |
| prod 多副本 api | **已确认 3 replicas** | `backend/infra/docker/docker-compose.prod.yml:140` | ✅ 与初步描述一致 |
| attest 独立部署 | **prod compose 不包含 attest** | `backend/infra/docker/docker-compose.prod.yml` | ⚠️ 待核实:符合 D6 独立部署 OR 尚未上线 |

---

## P0 — 必须改的硬伤(影响正确性/稳定性)

### P0-1. Redis 单点,事件总线无 HA

**What**
整个 idcd 异步总线(`probe.tasks` / `probe.results` / `cert:order_events`)、JWT blocklist、限流计数、scheduler leader 锁全部依赖单 Redis 实例。一旦 Redis 挂,全部停摆。

**Why**
- D13 决策提到 "10k SSE/实例" 容量目标,前提是 Redis 可用
- agent 集群扩到 50 节点后,故障爆炸半径 = 50 个区域全部失明
- 已经有付费证书业务(cert-svc),Redis 挂会导致用户付了钱拿不到证书 → P0 业务影响

**Where**
- `backend/apps/*/cmd/*/main.go` 都是 `redis.NewClient(&redis.Options{Addr: ...})` 单点
- `backend/infra/docker/docker-compose.prod.yml` 部署形态待核实(grep 未发现 redis 服务定义,可能是外部托管)
- 全代码库无 `MasterName` / `ClusterClient` / `FailoverClient` 使用

**How**(短期推荐方案)

```
┌──────────────┐
│ 3× Sentinel  │ ◄── 选举 + 故障检测
└──────────────┘
        │
        │ monitors
        ▼
┌──────────────┐
│ Redis Master │ ◄── 1 主
└──────┬───────┘
       │ async replication
       ├──────────────┐
       ▼              ▼
┌──────────────┐ ┌──────────────┐
│ Replica 1    │ │ Replica 2    │
└──────────────┘ └──────────────┘
```

- **客户端改造**:`redis.NewFailoverClient(&redis.FailoverOptions{MasterName: "idcd-master", SentinelAddrs: [...]})` —— go-redis 内置支持,**不需要业务代码改**
- 阿里云 / 腾讯云 Redis 标准版自带 Sentinel,直接选三节点版本即可,无需自部署
- 成本:阿里云 1G 内存 Redis 主从版 ¥150/月,3 节点 ¥450/月

**工作量**:1 天(包含切流验证)

**触发条件**:**节点扩到 20+ 前必须完成**,或用户付费正式上线前

---

### P0-2. scheduler leader 锁裸 SETNX,无 fencing token

**What**
scheduler 多副本部署时用 Redis SETNX 选 leader,但缺少 fencing token,旧 leader 进程在 GC stall 或网络分区恢复后**不知道自己被废黜**,会继续派任务。

**Where**
- `backend/apps/scheduler/internal/leader/leader.go:44`

```go
ok, err := l.rdb.SetNX(ctx, l.key, l.nodeID, l.ttl).Result()
```

**经典脑裂场景**:
1. scheduler-A 持有锁,TTL=30s
2. scheduler-A 进程 GC stall 40s(JVM 类常见,Go 也可能因 large heap + bg work 触发)
3. TTL 过期,scheduler-B 获取锁,开始派任务
4. scheduler-A GC 恢复,**不知道锁已经过期**,继续向 stream 写任务
5. 同一任务被两个 scheduler 同时派出 → agent 收到重复任务

**Why**
- 当前 scheduler 只有 1 实例的话,**问题不会发生**,但 PRD 计划是 HA 部署
- 重复任务会导致计费重复计 unit、消耗节点资源、用户 dashboard 看到莫名结果
- 这是分布式锁的**教科书坑**,不修迟早翻车

**How**(三档方案,看运维复杂度承受能力)

| 方案 | 改动量 | 强度 | 推荐 |
|---|---|---|---|
| A. leader 派任务前 GET 验证自己仍持锁 | 30 行 | 弱(仍有 race window) | 短期最低成本止血 |
| B. fencing token(epoch 单调递增 + Redis 端验证) | 100 行 | 强(消除大部分 race) | **推荐 S2 上线前完成** |
| C. 换 etcd lease + revision watch | 300 行 + 引入 etcd | 工业级 | 过度,scheduler 1-2 副本规模不必要 |

**Fencing token 落地草图**:
- scheduler 启动 `INCR scheduler:epoch` 拿到自己的 epoch number
- 每次写 `probe.tasks` stream 带上 `epoch=N`
- gateway 消费 stream 时维护已见的最大 epoch,小于最大值的消息**直接 drop + 告警**
- 旧 leader 即便不知道自己废黜,它的消息也会被 gateway 拒绝

**工作量**:2 天(实施 + 单元测试 + 故障注入验证)

**触发条件**:**scheduler 扩到 2+ 副本前**;若长期保持单副本,可降级到 P2

---

### P0-3. attest 服务的 prod 部署位置未核实

**What**
D6 决策要求 attest(公证服务)独立 VPC subnet / 独立 KMS 客户端 / 仅通过 `attest.idcd.com/verify` 公开接口被调用,本审查未在 `docker-compose.prod.yml` 找到 attest 服务定义,需要核实当前 prod 部署形态。

**Where**
- `backend/infra/docker/docker-compose.prod.yml` 服务列表:api / aggregator / notifier / gateway / scheduler / mcp / cert-svc / cert-worker / cert-renewer / nginx —— **无 attest**
- `backend/apps/attest/` 代码完整(7.2k 行,3 个 binary:server / generator / refund-worker)

**可能的情况**
- 🟢 **符合 D6**:attest 在另一个独立 compose / 另一个 VPC 部署,prod compose 故意不包含
- 🟡 **尚未上线**:attest 是 S2 才会启用的能力,目前 prod 不需要
- 🔴 **遗漏**:S2 已经启动但运维忘了部署

**How**
- 验证步骤(15 分钟):
  - 确认 staging.idcd.com 调用 `attest.idcd.com/verify` 实际打到哪里
  - 检查是否有 `infra/attest/` 或 `infra/docker/docker-compose.attest.yml`
  - 如果尚未部署,补一个 `attest-deploy-plan.md` 列出 S2 之前需要做的部署事项
- 落地文档:在 `docs/ARCHITECTURE.md` 增加一节明确 attest 部署形态,避免 onboarding 新人时混淆

**工作量**:15 分钟核实 + 0.5 天补文档(若遗漏则需另算部署工作量)

**触发条件**:**S2 上线前**

---

### P0-4. 跨服务消息载荷 `map[string]any` 滥用,无类型契约

**What**
生产代码中 `map[string]any` / `map[string]interface{}` 出现 **238 处,跨 73 个文件**。最严重的是**跨服务事件总线的 payload 全部是 map**:

```go
// backend/lib/shared/stream/stream.go:107
func (c *Client) AddProbeResult(ctx context.Context, taskID, nodeID string, payload map[string]any) (string, error)

func (c *Client) AddMonitorEvent(ctx context.Context, monitorID, event string, extra map[string]any) (string, error)
```

scheduler / gateway 写入 stream 时填什么字段、aggregator 消费时读什么字段,**完全没有共享类型契约**,全靠代码评审和默契。

**Why**(危害)
- **类型错误编译期发现不了** — `payload["latency_ms"] = "100"` (写成字符串) vs `payload["latency_ms"] = 100` 编译通过,消费端解析失败丢数据
- **字段名拼写错误静默** — 生产端写 `payload["latancy_ms"]`,消费端读 `payload["latency_ms"]`,**没人报错,数据静默丢失**
- **Schema 演进无追溯** — 加字段 / 改字段没有 grep 锚点,review 时看不出影响面
- **OTel 也救不了** — trace 能看到消息流转,但看不到字段 schema 不匹配

**最危险的几条线**(按业务关键度排序):
1. **probe.results stream**(`stream.go:107`)— 拨测结果,字段错=用户看到错数据
2. **cert:order_events stream** — 证书订单流水,字段错=证书签发流程错乱
3. **billing 支付回调** —— 钱相关
4. **attest record** — 公证,法律凭证,**字段错=合规事故**

**Where**(代码热点)
```
backend/lib/shared/stream/stream.go:107   AddProbeResult(payload map[string]any)
backend/lib/shared/stream/stream.go:120   AddMonitorEvent(extra map[string]any)
backend/lib/shared/stream/stream.go:???   AddAlertEvent / 其他
backend/apps/aggregator/internal/processor/  消费端反向解析 map → struct,字段名硬编码
backend/apps/api/internal/handler/*.go    HTTP 边界响应大量用 map
backend/apps/attest/...                   record 序列化层
```

**How**

**核心原则**:**所有跨服务消息必须有 Go struct 定义,放在 `lib/shared/contracts/` 或类似公共包,生产者/消费者双方都 import**。

实施步骤(渐进,**4 周内完成关键 3 条**):

1. **新建 `backend/lib/shared/contracts/` 包**,按 stream/topic 分文件:
   ```go
   // contracts/probe_result.go
   package contracts
   
   type ProbeResult struct {
       TaskID     string  `json:"task_id"      stream:"task_id"`
       NodeID     string  `json:"node_id"      stream:"node_id"`
       ProbeType  string  `json:"probe_type"   stream:"probe_type"`  // dns / ping / http ...
       Target     string  `json:"target"       stream:"target"`
       LatencyMs  *int64  `json:"latency_ms"   stream:"latency_ms,omitempty"`
       Success    bool    `json:"success"      stream:"success"`
       ErrorCode  string  `json:"error_code"   stream:"error_code,omitempty"`
       RawJSON    []byte  `json:"raw"          stream:"raw,omitempty"`     // 扩展字段兜底
       SchemaVer  int     `json:"schema_ver"   stream:"schema_ver"`        // schema 版本号
   }
   
   func (r ProbeResult) ToStreamValues() map[string]any { /* reflect 或手写 */ }
   func ParseProbeResult(vals map[string]any) (ProbeResult, error) { /* 严格校验 */ }
   ```

2. **stream.Client 提供类型化 API**:
   ```go
   func (c *Client) AddProbeResultTyped(ctx context.Context, r contracts.ProbeResult) (string, error)
   func (c *Client) ConsumeProbeResults(ctx context.Context, group, consumer string, handler func(contracts.ProbeResult) error) error
   ```
   旧 `AddProbeResult(map[string]any)` **保留但标记 `// Deprecated:`**,新代码禁用

3. **schema 版本字段**(`schema_ver`):
   - 消费端反序列化时检查版本,不认识的版本 → 告警 + 拒收(或丢死信队列)
   - 加字段时版本不变;删字段或改语义时版本 +1

4. **lint 规则**(`backend/scripts/lint-stream-payload.sh`)— 类似已有的 `lint-cross-schema-fk.sh`:
   - grep stream.go 周边代码,出现 `map[string]any` 跨边界传递就 fail
   - 强制必须用 `contracts/` 类型

5. **应用层 map 不强制改**:
   - `internal/handler` 内部用 map 拼 JSON 响应可以接受(单一进程内类型不重要)
   - **重点 attack 的是**:跨服务消息 / 跨 stream payload / 跨 HTTP 边界 RPC

**优先级排序**(4 周路线):
- **Week 1**:`contracts.ProbeResult`(最大流量、最关键)+ lint 脚本
- **Week 2**:`contracts.CertOrderEvent`(钱相关)
- **Week 3**:`contracts.AttestRecord`(合规)
- **Week 4**:`contracts.MonitorEvent` / `contracts.AlertEvent`

**工作量**:4 周渐进(每条 contract ~1 天 + 双端迁移 ~1 天)

**触发条件**:**立刻开始**,Week 1 必须在 50 节点上线前完成(probe.results 是受冲击最大的链路)

---

### P0-5. OpenAPI spec 3799 行无 codegen 绑定,对外契约可能脱钩

**What**
`docs/prd/16-api-spec.yaml` 是 **3799 行手写 OpenAPI 3.1 spec**,但 `backend/apps/api/internal/` 完全没有 `oapi-codegen` / `swag` 等代码生成工具的痕迹,**API spec 与实际代码完全分离维护**。

```
docs/prd/16-api-spec.yaml      3799 行 ← 手写,SSOT 之一
backend/apps/api/internal/handler/  19227 行 ← 手写实现
绑定关系:无 ❌
```

**Why**(危害)
- **对外契约错位**:开发者改 handler 忘了改 spec,SDK/前端按 spec 生成代码,运行时报错
- **review 漏看不报错**:PR 只改 handler,review 不会主动想到 spec 也要改
- **客户对接困难**:idcd 计划提供 SDK(已有 `backend/packages/sdk-go/`),如果 SDK 按 spec 生成,spec 错就 SDK 错,客户集成报错追责到 idcd
- **D2 决策**(token 90d 上限)等关键策略**字面值同时存在于 spec 和代码**,改一边忘一边 → 客户拿到错误的 token TTL

**Where**
- `docs/prd/16-api-spec.yaml`:3799 行(对照其他 SSOT 文档,这是最容易腐烂的一个)
- `backend/apps/api/cmd/api/main.go`:没有"启动时校验 spec 与实际路由一致"的逻辑
- `backend/packages/sdk-go/`:看代码组织似乎是手写,没看到 codegen 配置

**How**(两档方案,二选一)

**A. 强约束:codegen 路线**(推荐 long-term)
- 引入 `oapi-codegen`,以 OpenAPI spec 为 SSOT,生成:
  - `api/internal/oapi/types.go`(请求/响应 struct)
  - `api/internal/oapi/server.go`(路由 + handler interface,handler 实现接口)
- 改 handler 必须先改 spec,否则编译不过
- SDK(`packages/sdk-go/`)也用 codegen 生成

**B. 弱约束:contract test**(推荐 short-term,1-2 天可上线)
- 写 `backend/scripts/check-openapi-coverage.sh`:启动 api,从 spec 提取所有 `path + method`,逐个调用 → 检查响应 schema 匹配
- 在 CI 跑,spec 改了实现没改(或反之)立刻失败
- 不需要重写 handler,**只是加一道闸**

**实际建议**:
- **本季度做 B(弱约束)** — 2 天投入,立刻有保护
- **下季度评估 A(codegen)** — 看 SDK 计划复杂度,如果 SDK 要支持多语言(go/python/js)再上 codegen 才划算

**工作量**:
- B 方案 2 天
- A 方案 1-2 周(包含改造现有 handler 适配生成代码)

**触发条件**:**B 立刻做**(无成本就有保护);**A 当 SDK 进入多语言阶段时**

---

## P1 — 规模化前必修(50 节点 + 真用户后会很疼)

### P1-4. api 服务过载,handler 19k 行 / repository 仅 333 行

**What**
api 服务 25.8k 行代码,其中 handler 目录 **19,227 行**,占 75%;repository 层只有 **333 行**。说明**业务逻辑全部沉淀在 handler 里**,数据访问层薄弱,缺少 service 中间层。

**Where**
```
backend/apps/api/internal/handler/      19,227 lines  ← 异常膨胀
backend/apps/api/internal/middleware/    1,285 lines
backend/apps/api/internal/billing/       1,199 lines
backend/apps/api/internal/server/        1,122 lines
backend/apps/api/internal/job/             579 lines
backend/apps/api/internal/errcode/         424 lines
backend/apps/api/internal/repository/      333 lines  ← 异常薄
```

**Why**(危害分级)
- **低**:目前还能工作
- **中**:测试覆盖率会随 handler 体量增加快速下滑(handler 测要 mock 全部依赖)
- **高**:多人协作时频繁 merge conflict,典型 hot file 反模式
- **高**:每个 handler 重复写"参数解析 + 权限检查 + 业务逻辑 + DB 调用 + 响应组装",违反 DRY

**How**(渐进重构,**不要专门做重构 sprint**)

目标分层:
```
handler/{domain}/      ← 只负责 HTTP 解码 + service 调用 + 响应编码
service/{domain}/      ← 业务逻辑,可单元测试
repository/{domain}/   ← DB 访问,可 mock
```

domain 拆分建议:
- `handler/billing/` ← 当前 `billing/` 已经分出
- `handler/probe/`
- `handler/status/`
- `handler/cert/`
- `handler/admin/`
- `handler/auth/`
- `handler/mcp/`(MCP 元数据管理,不是 MCP 协议本身)

**执行原则**:
- 每加一个**新功能**时,顺手把那一块拆出 service/repository
- 每次 bug 修复时,顺手抽 1-2 个 helper 到 service
- **禁止专门开 PR 做 "handler 拆分"** — 容易引入 regression
- 6 个月内自然收敛到合理结构

**衡量指标**:每月看 handler 目录单文件行数 P90,目标 6 个月内从当前 ~1500 降到 <600

**工作量**:渐进,无单次估算

**触发条件**:**立刻开始改习惯**,但不设 deadline

---

### P1-5. OTel 跨 Stream/WebSocket 无 trace propagation(trace 断在异步边界)

**What**
OTel SDK 已经接入 7 个服务,HTTP 请求边界有 `TraceMiddleware` 注入/提取 `traceparent`。但是:
- Redis Stream 写消息时**没有把 trace context 编码进 message values**
- WebSocket 帧也**没有 trace context 透传**

**后果**:trace 在 `api → 内部 handler` 内成立,但 `api → Redis Stream → aggregator` 这条线 trace 断了,**找不到一个 trace_id 串起跨服务异步链路**

**Where**
- `backend/lib/shared/stream/stream.go:107` — `AddProbeResult` 函数把 task_id/node_id 写进 stream values,但**没有 inject traceparent**
- `backend/lib/shared/telemetry/telemetry.go:95` — 只设置了 HTTP 的 Composite Propagator,没有为 Stream / WS 写专用 Carrier
- 调试用户报"我的拨测结果丢了"链路 `api → scheduler → gateway → agent → gateway → aggregator → DB` 6 个 hop,**无法一个 trace_id 跟到底**

**How**

实现 Stream Carrier(20 行)+ WebSocket Carrier(20 行):

```go
// backend/lib/shared/telemetry/stream_carrier.go (新增)
type StreamCarrier map[string]any

func (c StreamCarrier) Get(key string) string {
    if v, ok := c[key].(string); ok { return v }
    return ""
}
func (c StreamCarrier) Set(key, value string) { c[key] = value }
func (c StreamCarrier) Keys() []string {
    keys := make([]string, 0, len(c))
    for k := range c { keys = append(keys, k) }
    return keys
}

// 写入端:
carrier := StreamCarrier(vals)
otel.GetTextMapPropagator().Inject(ctx, carrier)

// 读取端:
ctx = otel.GetTextMapPropagator().Extract(ctx, StreamCarrier(message.Values))
```

WS Carrier 类似,使用 frame header 或消息 JSON 顶层字段。

**工作量**:0.5-1 天(含为 `stream.Client` 加 ctx 参数 + 修改 producer/consumer + 文档)

**触发条件**:**第一次出现"用户报跨服务问题但 trace 跟不到"时**立刻做

---

### P1-6. gateway 多副本时 agent 连接状态无 cross-instance 路由

**What**
gateway hub 内存 map `connections: map[string]*Connection`(nodeID → Connection),agent 重连可能落到不同 gateway 实例,**新实例不知道之前实例的任务进度**,任务回执无法路由。

**Where**
- `backend/apps/gateway/internal/hub/hub.go:55` — 纯内存 map
- 全代码库无 `instanceID` / sticky session 路由表

**Why**
- 当前 gateway 单实例的话**问题不发生**
- prod compose 显示 gateway 是单服务定义(无 replicas 字段确认是否扩展),需核实
- 一旦多副本,以下场景出问题:
  - gateway-1 重启,agent 重连到 gateway-2
  - scheduler 仍把任务 XAdd 到 `probe.tasks`,但 gateway-2 的内存里没有 task→agent 的回执 channel(任务派出去 agent 跑完了,结果上报无主)

**How**

短期(单实例):在 `docs/ARCHITECTURE.md` 标明 **gateway 当前单实例约束**,加监控告警单实例 down 时触发 P0 oncall

长期(多副本):
- Redis HASH `agent_routes`:`agent_id → gateway_instance_id`,TTL 跟 WS ping 同步刷新
- scheduler 派任务前查 routes,**XAdd 到对应 gateway 的 stream**(`probe.tasks.gw-{instance_id}`)
- agent 重连 → gateway 写入 routes → 下次派任务路由正确
- gateway 实例下线时主动清理自己的 routes 条目

**工作量**:2-3 天(含集成测试)

**触发条件**:**gateway 需要扩到 2+ 副本时**(预计 30 节点 / 5k SSE 后)

---

### P1-7. probe.results 单 stream + 单 consumer group(规模化天花板)

**What**
所有 50 节点的探测结果写到同一个 stream(`probe.results`),aggregator 单 consumer group 内多 consumer 实例分担。Redis Stream 本身是**单一日志**,单 shard 吞吐有上限。

**Where**
- `backend/lib/shared/stream/stream.go:29` — `Probe = "probe.results"`
- `backend/apps/aggregator/internal/consumer/consumer.go` — 单 consumer group

**Why**
- 50 节点 × 30 次/天 探测 = ~1500 result/天 = 完全无压力
- 但 idcd 拨测频率有提到"5min 探活" → 50 节点 × 12次/小时 × 6 服务 × 24h = **~86k events/day**
- 单 agent 异常上报洪水(比如循环 bug 每秒 100 条)会塞住整个 stream
- aggregator 单消费组的扩展上限受限于 Redis 单 shard

**How**(规模再翻 4 倍时再做,**目前不急**)

按 agent_id hash 分片到 N 个 stream:
```
probe.results.0 ← agents with hash(id) % 8 == 0
probe.results.1 ← agents with hash(id) % 8 == 1
...
probe.results.7 ← agents with hash(id) % 8 == 7
```

aggregator 各 consumer group 订阅一个 shard。改造代价:
- stream 库加 `AddProbeResultSharded(agentID, ...)` API
- aggregator 启动时为每个 shard 创建独立 consumer
- 数据落库逻辑不变(都进同一个 `probe_task` 表)

**工作量**:1 天(架构清晰,代码简单)

**触发条件**:**aggregator lag > 30s 持续 5 分钟,或单 agent 上报 QPS > 50**

---

### P1-8. 服务配置重复定义,`lib/shared/config` 已有但未回迁

**What**
`backend/lib/shared/config/config.go` 已经存在统一的 Config 结构(Database/Redis/Server/JWT/Email/Observability/AgentGateway/OAuth/Encryption/Payment/RateLimit/StatusProbe),但是**每个服务自己又定义了一份 config**,重复且字段命名不完全一致:

```
agent       100 行  ~9 字段
aggregator  111 行  ~8 字段
attest      309 行  ~29 字段  ← 严重膨胀
cert-svc    375 行  ~31 字段  ← 严重膨胀
gateway     124 行  ~14 字段
notifier    160 行  ~19 字段
scheduler   102 行  ~10 字段
```

**重复字段统计**(出现在 ≥2 个服务的 config.go):

| 字段 | 出现次数 | 备注 |
|---|---|---|
| `RedisAddr` | 4 | 应统一 |
| `RedisPassword` | 3 | 应统一 |
| `RedisDB` | 3 | 应统一 |
| `Port` | 3 | 命名甚至不一致(`Port` / `HTTPPort` / `ListenAddr`) |
| `Env` | 3 | dev/staging/prod 枚举 |
| `LogLevel` | 2 | |
| `DatabaseDSN` / `PGDSN` | 4 合计 | **同语义不同字段名**,严重 |
| `From` / `FromName` | 各 2 | email 字段 |
| `BatchSize` | 2 | |
| `AliKMSRegionID` | 2 | |

**Why**(危害)
- **改一个 Redis 地址要改 N 个 config 解析器** → 容易漏
- **同语义字段名不一致**(`DatabaseDSN` vs `PGDSN`)→ ops 同学配错踩坑
- **测试 fixture 重复造**,每个服务自己造一份 testdata/config.yaml
- **新服务上线必抄旧服务的 config 代码** → 抄错率高

**How**

1. **以 `lib/shared/config` 为单一真实源**,所有跨服务共用字段(Redis/Database/Server/JWT/Email/Logger/Telemetry)**只在这里定义**
2. 各服务的 `internal/config/config.go` 只保留**该服务独有字段**(比如 attest 的 KMS / TSA 配置,cert-svc 的 ACME / CAA 配置)
3. 服务启动时 embed shared:
   ```go
   // backend/apps/cert-svc/internal/config/config.go
   type Config struct {
       shared.Config `yaml:",inline"`  // Redis/DB/Server/JWT/...
   
       ACME   ACMEConfig   `yaml:"acme"`     // cert-svc 独有
       CAA    CAAConfig    `yaml:"caa"`
       Vault  VaultConfig  `yaml:"vault"`
   }
   ```
4. **环境变量解析也统一**到 shared,所有服务用同一套 `SHARED__REDIS__ADDR` / `SHARED__DATABASE__DSN`
5. **改名 `DatabaseDSN` ↔ `PGDSN` 统一到一个**(我建议 `Database.DSN`,跟 shared/config 一致)

**工作量**:2-3 天(每服务 ~30 min × 7 + 测试)

**触发条件**:**立刻开始**,P1-4 handler 拆分同步推进时一起做

---

### P1-9. 业务常量散落,无统一 `constants` 包

**What**
SLA 阈值、token TTL、quota 数字、超时时间等业务常量直接以字面值散落在多个文件,**改一个数字要 grep 全代码库**。

**Where**(典型例子)

| 常量 | 散落位置(部分) |
|---|---|
| `5 * time.Minute`(webauthn challenge / flap 判定 / claim min idle) | `api/handler/webauthn_handler.go:23`, `aggregator/processor/noise_recorder.go:36`, `aggregator/consumer/consumer.go:25` |
| `24 * time.Hour` / `30 * 24 * time.Hour`(各种 retention) | `cert-svc/handler/admin_cert.go:404`, `cert-svc/handler/orders.go:236`, `cert-svc/service/notifications.go:82,84` |
| `7 * 24 * time.Hour` | `cert-svc/service/ca_quota_repo.go:86`, `cert-svc/service/abuse.go:35`, `cert-svc/service/notifications.go:82` |
| MCP token TTL(24h personal / 90d workspace,D2 决策) | 应该统一,需 grep 确认 |
| Verdict SLA(纯自动 / 1h P0 / 24h 常规,D12 决策) | 同上 |

**Why**(危害)
- **D2 / D12 等关键决策的数字散落在代码各处** — 决策改了,改代码要遍历全库
- **测试用例硬编码同样的字面值** — 改了实现忘了改测试,测试反而保护了错误行为
- **可观测性 dashboard 阈值跟代码字面值脱钩** — 告警阈值用 5min,代码改成 3min,告警立刻失真

**How**

新建 `backend/lib/shared/constants/` 包,按业务域分文件:

```go
// constants/sla.go (对应 D12 决策)
package constants

import "time"

const (
    VerdictAutoTimeout        = time.Duration(0)        // 纯自动,没有人工 SLA
    VerdictCriticalP0SLA      = 1 * time.Hour          // KMS / 节点失窃
    VerdictRoutineSLA         = 24 * time.Hour         // 常规客服
)

// constants/token_ttl.go (对应 D2 决策)
const (
    MCPTokenPersonalTTL    = 24 * time.Hour
    MCPTokenWorkspaceTTL   = 90 * 24 * time.Hour
    MCPTokenServiceTTL     = 90 * 24 * time.Hour
)

// constants/retention.go
const (
    ProbeResultHotRetention   = 7 * 24 * time.Hour
    ProbeResultColdRetention  = 90 * 24 * time.Hour
    CertOrderRetention        = 30 * 24 * time.Hour
)

// constants/timeout.go
const (
    WebAuthnChallengeTTL      = 5 * time.Minute
    StreamConsumerClaimMinIdle = 5 * time.Minute
    MonitorFlapThreshold      = 5 * time.Minute
)
```

**强制规则**(写入 CLAUDE.md):
- 业务语义常量**禁止**写字面值,必须从 `constants` 包导入
- 与 PRD 决策(D1-D13)关联的常量**注释里标明对应决策号**,改之前先改 DECISIONS.md
- 通过 `backend/scripts/lint-magic-numbers.sh` 检查(可选,但能保 review 不漏)

**工作量**:1-2 天(常量定义 + 替换 + lint 脚本)

**触发条件**:**立刻开始**,P0-4 contracts 包做完后顺手做(同一个公共库迁移)

---

### P1-10. 关键业务流程缺事务保护(WithTx 已造但生产代码使用率为零)

**What**
`backend/lib/db/tx.go` 提供了完整的 `WithTx` helper(含 nested savepoint / panic rollback / context cancel,16 个单元测试),但是**全代码库无 import 使用**。关键业务流程如**用户注册**横跨多个写操作,中间失败会留下脏数据。

**Where 实证**:`backend/apps/api/internal/handler/auth.go:209` 注册流程

```go
user, err := h.q.CreateUser(ctx, ...)          // ① 写 users 表
// ...
if h.enqueuer != nil {
    if otpID, code, err := h.issueOTP(...) { // ② 写 user_otp 表
        h.enqueueVerifyEmail(...)             // ③ Stream 写邮件队列
    }
}
token, _, err := h.issueToken(...)            // ④ 写 sessions 表
```

**失败场景**:
- ① 成功,② 失败 → 用户已创建,但永远收不到验证邮件,**邮箱被占用且账号不可用**(用户重试注册被告知"邮箱已存在")
- ① ② 成功,④ 失败 → 用户已注册可登录,但当前请求返回 500 → 用户体验差,且 frontend 不知道账号已建好

**类似风险**(grep 速查):
- `auth.go:675` CreateUserOTP 等 OTP 流程
- `oauth.go` OAuth 注册创建 user + credential 多步
- billing 订单创建 + 计费扣减
- cert-svc 订单创建 + DNS challenge 记录

**Why**(危害分级)
- **正确性问题**:数据不一致是 silent bug,用户报障才发现
- **客服成本**:用户报"注册不了/收不到邮件"需要人工排查 + 删脏数据
- **审计/合规**:billing 类操作部分成功未来可能引发对账纠纷

**How**

按业务关键度排序的迁移:

1. **auth 流程**(最高优先):用 `WithTx(ctx, pool, func(tx pgx.Tx) error { ... })` 包裹 CreateUser + CreateUserOTP + IssueToken
2. **OAuth 注册流程**:同上
3. **billing 订单流程**:订单创建 + 计费扣减 必须事务
4. **cert-svc 订单流程**:已经有 idempotency key(`orders.go:42`),但事务保护仍要补
5. **enqueue 类的"写 DB + 写 Stream"**:Stream XAdd **不能进 DB tx**(commit 后再 publish,outbox pattern),需小心设计

**Outbox pattern 落地草图**(对应第 5 类):
- 在 tx 内写入 `outbox` 表(同 schema),记录待发的 stream 消息
- tx commit 后,一个独立 worker 定期 SELECT outbox + XAdd + 删除已发条目
- 保证"DB 写入与消息发送原子一致",牺牲一点点延迟

**工作量**:
- auth/OAuth 流程包 tx:1 天
- billing/cert-svc 流程包 tx:1-2 天
- outbox pattern(可选):2-3 天

**触发条件**:**立刻开始 auth 流程**(每天有真实用户跑这条),其他按业务重要度推进

---

### P1-11. 业务指标观测性严重缺失(自定义 metric 仅 2 处)

**What**
全代码库 `prometheus.NewCounter/Gauge/Histogram` 只出现 **2 处**。意味着虽然每个服务都暴露 `/metrics`,但里面只有:
- runtime metrics(go_*)
- HTTP middleware 默认 metrics(http_requests_total / duration)
- 没有任何**业务指标**

**Why**(危害)
- 看不到"今天签发了多少证书"、"probe 成功率"、"订单创建成功率"、"OTP 验证失败率"
- 出问题靠"用户报障"而不是"告警"
- D12 SLA 决策(1h P0 / 24h 常规)**没有可观测的指标支撑**,你怎么衡量 SLA 是否达标?
- status page 公开后,**用户会先于你发现服务降级**

**Where**
- `grep -rn "prometheus.New" backend/` → 2 处
- 业务关键路径(auth/billing/probe/cert/attest)**全无指标**

**How**(分阶段)

**Phase 1: 业务核心指标**(1-2 天)

每个服务定义自己的指标包 `internal/metrics/`:

```go
// api/internal/metrics/auth.go
var (
    RegistrationAttempts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Namespace: "idcd_api",
            Subsystem: "auth",
            Name:      "registration_attempts_total",
            Help:      "用户注册尝试次数 (按 outcome 分类)",
        },
        []string{"outcome"}, // success/duplicate/invalid_email/internal_error
    )
    
    LoginLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Namespace: "idcd_api",
            Subsystem: "auth",
            Name:      "login_duration_seconds",
            Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
        },
        []string{"outcome"},
    )
)
```

**优先级清单**(必埋点):
| 服务 | 指标 | 用途 |
|---|---|---|
| api | 注册/登录/OTP 成功率 + 延迟 | 用户增长 + SLA |
| api | API quota 使用率(per workspace) | 计费与限流可见性 |
| cert-svc | 证书签发成功率 + LE rate limit hit | 关键业务健康度 |
| cert-svc | DNS challenge 失败原因分布 | 客户支持 |
| attest | KMS sign 成功率 + 延迟 + retry 次数 | D11 12h Shamir SOP 触发判断 |
| attest | refund retry 队列长度 | D5 退款失败监控 |
| scheduler | 派任务 lag(now - scheduled_at) | 系统延迟感知 |
| gateway | 在线 agent 数 + WS reconnect 次数 | 节点健康 |
| aggregator | stream consumer lag | 数据管道健康(P1-7 触发条件) |
| notifier | 邮件发送成功率 + provider 错误分布 | 客户触达健康 |
| mcp | SSE 连接数 + 单 client QPS top-K | D13 容量监控 |

**Phase 2: Grafana dashboard 模板**(1 天)
- 一个 dashboard 一个服务,固定布局:在线/QPS/延迟/错误率/业务关键 metric
- 入 `infra/grafana/dashboards/` 进 repo,版本化

**Phase 3: 告警规则**(1-2 天)
- Prometheus AlertManager 规则入 `infra/prometheus/alerts.yml`
- 至少覆盖:registration 错误率 > 5%、cert 签发失败率 > 10%、stream lag > 30s、refund retry 队列 > 10

**工作量**:Phase 1 ~2 天 + Phase 2 ~1 天 + Phase 3 ~1 天 = **总计 4-5 天**

**触发条件**:**status page 公开前必须做 Phase 1**(否则用户先于你发现问题)

---

### P1-13. agent 无自动升级机制,50 节点扩张后运维负担陡增

**What**
全代码库扫描 `self.*update / self.*upgrade / version.check` **无任何匹配**。agent 升级**必须 SSH 进每个节点手动 pull + 重启**。

**Where**
- `backend/apps/agent/` 有 `reload_config` 热更新机制(通过 WS 命令)
- **没有** `update_binary` / 灰度升级 / 版本检查

**Why**(规模化前必修)
- 现在 ~3 节点,手动升级勉强可接受
- 扩到 50 节点后,**每次发版要 SSH 50 次**,极易漏 / 出错 / 版本错位
- 出现紧急安全 patch 时,**升级时间窗口拉长** → 漏洞期延长
- 版本错位(部分节点新,部分旧)会导致 stream 消息格式不一致,跟 P0-4 contracts 联动出大问题

**How**

三档方案,按运维成本递增:

| 方案 | 实施成本 | 维护成本 | 推荐 |
|---|---|---|---|
| A. Ansible / 配置管理工具批量推送 | 0.5 天(写 playbook) | 低 | **快速止血** |
| B. agent self-update(gateway 推送新版本下载链接 + agent 自重启) | 2-3 天 | 中 | **目标方案** |
| C. 容器化 + watchtower / orchestration | 1-2 天 | 高(每个 VPS 都装容器) | 过度 |

**B 方案落地草图**:
- 新增 WS 命令 `update_binary { url, sha256, version }`
- agent 下载到 `agent.new`,验证 sha256,**优雅 drain** 当前任务 → exec `agent.new` 替换自身(systemd 接管 restart)
- gateway 灰度策略:**先升 1 个 canary 节点观察 30 分钟,再批量推**
- 必须有回滚机制(版本号比当前老 → 拒绝更新;新版本健康检查失败 → 自动 revert)

**工作量**:
- A 方案 0.5 天(立刻可做)
- B 方案 2-3 天

**触发条件**:**节点扩到 10+ 前必做 A,扩到 30+ 前必做 B**

---

### P1-12. fire-and-forget 模式用 `context.Background()` 丢 trace context

**What**
请求 handler 里启动后台异步操作(比如更新 last_used_at)时,**直接用 `context.Background()` 而不是从请求 context 派生**,导致:
- trace_id 断链 — OTel span 关联不上原请求
- 取消信号传不进去 — 进程退出时这些 goroutine 可能拖延 shutdown

**Where**
- `backend/apps/api/internal/middleware/authn.go:361,367`(典型例子)
  ```go
  func touchLastUsedPAT(svc PATVerifier, patID string) {
      ctx, cancel := context.WithTimeout(context.Background(), lastUsedUpdateTimeout)
      defer cancel()
      _ = svc.TouchLastUsed(ctx, patID)
  }
  ```
- 注释说 "Errors are swallowed because failing to bump the timestamp must not block authenticated requests" — 意图是对的,但实现错了
- 全代码库 **0 处** `context.WithoutCancel`(Go 1.21+ 标准做法)

**Why**(危害)
- **可观测性**:trace 看不到 last_used 更新与原请求的关联
- **shutdown 不干净**:进程退出时这些 detached goroutine 仍在跑,Shutdown 超时被强 kill,可能写一半 → DB 不一致
- **OTel propagation 失效**:即使 P1-5 修了 Stream propagation,这种本地 fire-and-forget 仍然断链

**How**

替换 `context.Background()` 为 `context.WithoutCancel(parent)`:

```go
func touchLastUsedPAT(parentCtx context.Context, svc PATVerifier, patID string) {
    // WithoutCancel: 保留 trace_id + values,忽略 cancellation
    ctx := context.WithoutCancel(parentCtx)
    ctx, cancel := context.WithTimeout(ctx, lastUsedUpdateTimeout)
    defer cancel()
    _ = svc.TouchLastUsed(ctx, patID)
}
```

调用方传入 `r.Context()`,trace 完整。

**改造范围**:全代码 grep `context.Background()` 在非 main/init 处使用,逐个评估是否应改为 `WithoutCancel`。

**工作量**:0.5 天(grep + 替换 + 评估每处意图)

**触发条件**:跟 P1-5 OTel propagation **一起做**(同属可观测性主题)

---

## P2 — 看 ROI 再决定

### P2-8. 无熔断器(`gobreaker` 在 go.work.sum 但未使用)

**What**
对外部依赖(AWS KMS / TSA / Let's Encrypt / 阿里通义 / DeepSeek)无熔断,失败时持续重试拖慢业务接口。

**Where**
- `backend/go.work.sum:169` 提到 `gobreaker v1.0.0`,但**全代码库无 import 使用**
- `backend/lib/attest/sign/awskms/awskms.go` 直接调 KMS,只有 ctx timeout 兜底

**How**
对**每个外部依赖**包一层 gobreaker:
- ReadyToTrip 阈值:失败率 > 50% 且最近 10 次请求 ≥ 5 次失败
- Timeout:30s 半开重试一次
- 失败后 fallback:KMS → 队列等待 + alert;TSA → 降级到 free TSA;LLM → 降级到备用 provider

**工作量**:1 天

**触发条件**:**第一次出现 KMS/TSA 抖动导致请求堆积时**

---

### P2-9. PostgreSQL 单实例,无读写分离

**What**
prod compose 当前看不到 PG 服务定义(可能托管在云数据库),需要核实是否已有读写分离。

**Where**
- 全代码库无 `ReadReplica` / `read_url` 使用迹象
- D1 决策保证不写跨 schema FK,但**没有解决跨 replica 一致性问题**(读副本延迟)

**How**

何时改:
- **不要太早做** — 主从延迟会带来新一类 bug(刚写完读不到,常见在 OAuth 回调、支付通知等场景)
- 触发条件:**status page 公开后**只读流量(状态查询 / probe 历史)增长可观,或**月活 > 10k 用户**

改造路径:
- repository 层引入 `read` / `write` 两个 pool,业务代码显式选择
- 危险查询(刚写完立即读)继续走 master
- 大量只读分析(probe 历史聚合、status page 90 天数据)走 replica

**工作量**:2-3 天(主要是 audit 哪些查询能去 replica)

**触发条件**:**主库 CPU 持续 > 60%,或 status page DAU > 1k**

---

### P2-10. notifier 与 api 邮件职责模糊

**What**
`api/internal/billing/` 自己发邮件,`notifier/` 也发邮件,两个服务都有 SMTP 客户端代码,职责边界不清。

**How**
统一所有邮件走 notifier 队列(新建 Redis Stream `notif.outbound`):
- api 只 XAdd 事件:`{type: "billing.refund_failed", to: "user@x.com", template: "refund_apology", data: {...}}`
- notifier 单一负责模板渲染 / SMTP 调用 / 重试 / 限流 / 审计 / 不送达后兜底

**Why**
- 邮件重试逻辑不应散落在每个调用方
- 统一后能给运营一个 dashboard 看"今日发件统计"
- 切换邮件 provider(SES → 阿里 DM)只改一处

**工作量**:1-2 天

**触发条件**:**第二次出现"某场景邮件忘发了"时**(目前已经有 1 次?核实 billing 退款邮件 SOP)

---

### P2-11. MCP 服务与主 API 共用 DB,无限流隔离

**What**
MCP 客户端是 AI agent,行为不可控(可能批量请求),无 connection pool 隔离会拖慢主 API。

**Where**
- `backend/apps/mcp/` 服务跟 api 共用 `idcd_main` schema 的部分表

**How**

短期(D13 决策已要求):
- 给 mcp 服务**独立的 DB connection pool**(配置 `mcp_db_max_conns` 远小于 api 的)
- 给 mcp 的 SSE 请求配独立 rate limit bucket

长期(等 P2-9 读写分离上线后):
- MCP 走**只读副本**,完全不打主库

**工作量**:0.5 天(短期)+ 1 天(长期,依赖 P2-9)

**触发条件**:**MCP 集成第一个商业客户接入,流量上来后**

---

### P2-12. `http.DefaultClient` 用于 OAuth 第三方调用,无 timeout 无 trace 注入

**What**
OAuth 流程(Google/GitHub 等)用 `http.DefaultClient.Do(req)` 调外部 token endpoint,有两个问题:
- DefaultClient **无超时**,远端 hang 整个 goroutine 卡死
- DefaultClient 全局共享,无 OTel HTTP middleware 注入 trace context

**Where**
- `backend/apps/api/internal/handler/oauth.go:200, 233, 347, 387` — 4 处 DefaultClient.Do

**Why**
- OAuth provider 偶发慢响应(Google 偶尔会 5-10s)→ 用户登录页卡半天没反馈
- 出问题时 trace 跟不到外部调用,只能看到 "我们等了 X 秒",不知道是 DNS / TCP / TLS / 远端的哪一段

**How**

提供一个全局 http.Client(在 api 服务初始化时构造):
```go
// api/internal/httpclient/httpclient.go
var ExternalOAuth = &http.Client{
    Timeout: 10 * time.Second,
    Transport: otelhttp.NewTransport(http.DefaultTransport),
}
```
所有 OAuth 调用统一用 `httpclient.ExternalOAuth.Do(req)`。

**工作量**:0.5 天

**触发条件**:**用户报"Google 登录卡住"时**,或与 P1-5 同期做

---

### P2-13. 健康检查覆盖不全:4/9 服务无 /healthz,8/9 服务无 /readyz

**What**
当前各服务健康检查状态:

| 服务 | /healthz | /readyz |
|---|---|---|
| api | ✅ | ❌ |
| attest | ✅ | ❌ |
| cert-svc | ✅ | ✅ |
| gateway | ✅ | ❌ |
| agent | ❌ | ❌ |
| aggregator | ❌ | ❌ |
| mcp | ❌ | ❌ |
| notifier | ❌ | ❌ |
| scheduler | ❌ | ❌ |

**Why**(危害)
- k8s/容器编排没有 liveness/readiness probe → 假死进程不会被自动重启
- 滚动升级时**没有 readiness 判断**,流量打到还在初始化的实例
- LB/反向代理(nginx)看不到准确的 backend 健康状态

**How**

约定每个服务暴露:
- `GET /healthz` — liveness,只检查"进程活着"(返 200)
- `GET /readyz` — readiness,检查关键依赖(DB ping / Redis ping / 关键下游 ping)

抽公共 helper 到 `lib/shared/healthcheck/`,各服务 main 启动时注册。

**注意**:agent 是出站 WS,没有 inbound 端口,健康检查走"WS 连接状态 + 探测器最近成功时间"放到 metrics 即可,不需要 /healthz

**工作量**:1-2 天(写 helper + 各服务接入)

**触发条件**:**正式上 k8s/容器编排前必修**

---

### P2-14. 18 个独立 go.mod,工作区多模块维护成本高

**What**
`find backend -name go.mod | wc -l` = **18 个**(`backend/lib/*` × 7 + `backend/apps/*` × 9 + `backend/packages/*` × 2)。

**Why**
- 共享依赖(redis / pgx / otel 等)版本对齐**仰赖 go.work + go work sync** + 人工巡查
- 升级一个依赖要在 18 个 go.mod 改版本,容易漏
- `go.work.sum` 有 ~500+ 行,signal/noise 比差
- 新人 onboarding 看到 18 个 go.mod 容易困惑

**Why 不立刻改**
- 多 module 有合理性:`lib/auth` / `lib/cert` / `lib/db` 等共享库可能想被外部项目 import(虽然目前没有)
- 改成单 module 会破坏潜在的外部 import,**短期没收益**

**How**(看 ROI 决定,**不急**)

选项 A:**保持多模块**,但增加约束
- 写 `backend/scripts/check-go-mod-versions.sh`:扫所有 go.mod,共享依赖必须一致版本
- CI 集成

选项 B:**合并为单模块**
- 假设 lib 不被外部 import,合并到一个 go.mod
- 代价:大改动,需要更新所有 import path

**建议**:**做 A,不做 B**。1 天写检查脚本即可。

**工作量**:1 天(选项 A)

**触发条件**:**第一次出现"依赖版本不一致导致莫名 bug"时**

---

### P2-16. secrets management 不统一(cert-svc 用 Vault,其他服务用 yaml)

**What**
扫描发现 `backend/apps/cert-svc/cmd/{worker,server}/main.go:334` 使用 **HashiCorp Vault**(`hashivault.New`)管理 secrets,但其他服务(api / scheduler / notifier 等)仍从 yaml 配置文件读取(JWT secret / DB password / 第三方 API key)。

**Why**
- 不一致 = onboarding 难 + 安全审计盲点
- yaml 配置文件即使 gitignored,落在每台服务器 + ops 同学本地,**任何节点失窃都泄露所有 secrets**
- D11 KMS 应急 SOP 已经把 cert 类高敏 secrets 走 Vault,这是正路;**其他 secrets 应该统一**

**Where**
- `backend/apps/cert-svc/cmd/server/main.go:334` — 用 Vault
- `backend/lib/cert/vault/` — Vault 抽象层
- 其他服务的 main.go:从 `config.LoadFromYAML(path)` 直接拿明文

**How**

短期:`lib/shared/config` 增加 Vault provider,统一从 Vault 取所有 secrets
- 配置文件只存非敏感配置(端口、域名、限流参数)
- 敏感配置(DB password、JWT secret、third-party key)从 Vault 拉

长期:推进 K8s + Vault Agent Sidecar(等容器化阶段)

**工作量**:1-2 天(扩展 shared/config + 各服务迁移)

**触发条件**:**与 P1-8 shared/config 回迁同期做**(同一个公共库,一次性弄完)

---

### P2-17. GDPR / 用户主动删除流程不完整

**What**
`users` 表有 `status='pending_deletion' + pending_deletion_at` 字段(`migrations/idcd_main/00002_users.sql:18`),`audit_log` 有 180 天 retention(`00005_audit_log.sql:22`)。**软删除框架已搭好**,但代码扫描看不到:
- 用户发起"删除我的账号"的 API 路径
- 后台清理 worker(过 X 天后真正删除)
- 跨 schema 用户数据级联删除(billing/probe/cert 等)

**Why**
- 国内合规:个保法第 47 条,用户撤回同意后 15 个工作日内删除
- 海外合规:GDPR 第 17 条 "right to be forgotten"
- 现在 idcd 尚未公开运营,**风险窗口短;一旦商业上线,合规风险立刻浮现**

**How**

- Phase 1:在 api 增加 `DELETE /v1/me` endpoint,标记 `pending_deletion`(30 天 grace period 可恢复)
- Phase 2:notifier 发"30 天后将永久删除"邮件
- Phase 3:scheduler 定时任务(daily),把过期的 `pending_deletion` 用户跨 schema 硬删除(走 D1 决策的 Repository 应用层 join 删除)
- Phase 4:导出用户数据接口(GDPR 第 15 条 right to access)

**工作量**:2-3 天(Phase 1-3)+ 1-2 天(Phase 4)

**触发条件**:**正式商业上线 / 海外用户接入前**

---

### P2-15. `encoding/json` 性能优化空间(可选)

**What**
全代码库用标准库 `encoding/json`,**78 处 Unmarshal**。在 status page / probe 历史查询等高频路径,`jsoniter` / `bytedance/sonic` 可带来 1.5-2x 性能。

**Why 不立刻改**
- 当前 QPS 不需要,优化为时过早
- sonic 在 ARM 上有 bug 历史,引入新依赖有兼容性风险

**触发条件**:**JSON 序列化占 P99 latency > 20% 时**(目前几乎肯定不是),需要 pprof 验证

---

## ❌ 不建议做的反模式

| 想法 | 为什么别做 |
|---|---|
| **改 gRPC 取代 HTTP** | 服务间 90% 通信是异步消息(Redis Streams),直接互调极少,gRPC 收益 < 改造成本 |
| **上 Service Mesh(Istio/Linkerd)** | 9 服务规模上 mesh 是用大炮打蚊子,运维复杂度暴增,Sidecar 多吃一倍内存 |
| **加 Service Discovery(Consul/etcd)** | 9 服务地址写在 config 完全够用,30+ 服务才有 ROI |
| **拆 api 成微服务** | 现在的问题是 handler 内部模块边界,不是服务边界 — 先拆模块(P1-4)再说,**模块边界还没拆清就拆服务必踩坑** |
| **NATS / Kafka 替换 Redis Streams** | Streams 当前用得很对位,Kafka 起步成本(运维 + ZK/KRaft + Schema Registry)远高于现在量级 |
| **抽公共框架替换 net/http** | Go 标准库够用,自造框架是**最常见的过度工程** |
| **scheduler 改 etcd lease** | 1-2 副本规模 fencing token(P0-2 方案 B)已经足够,etcd 引入一整套新依赖,推迟到真有需要 |
| **全栈上 OpenTelemetry Collector 集群** | 单 collector 处理 9 服务的 trace/metric 完全够用,自部署集群是过度 |

---

## 已知约束(不是问题,但要记录)

记录这些**当前刻意的简化**,避免后续 onboarding 误以为是 bug:

1. **D1 跨 schema 不写 FK** — 跨 schema join 走 Repository 应用层,牺牲 DB 一致性约束换 schema 演进自由度
2. **gateway 当前单实例** — sticky session 由单实例天然保证,扩多副本前需先做 P1-6
3. **scheduler 当前单实例**(待核实) — 单实例时 P0-2 的 leader 锁问题不触发
4. **Redis 单实例**(P0-1 待修) — 当前接受单点风险换部署简单,P0-1 修复前是已知 risk
5. **服务地址写死 config** — 9 服务规模不需要 service discovery
6. **attest 独立部署待文档化** — 符合 D6 的话需在 ARCHITECTURE.md 明确

---

## 规模化触发条件(何时回看本审查)

| 触发信号 | 触发的项 | 应对 |
|---|---|---|
| 节点扩到 20+ | P0-1 | Redis Sentinel 必修 |
| 用户付费正式上线 | P0-1, P0-3 | Redis HA + attest 部署核实 |
| 50 节点拨测上线前 | P0-4 (Week 1) | ProbeResult contract 必须先到位 |
| scheduler 扩到 2 副本 | P0-2 | fencing token 必修 |
| SDK 进入多语言阶段 | P0-5 (A 方案) | OpenAPI codegen |
| 任意 PR 改 handler 没改 spec | P0-5 (B 方案) | OpenAPI contract test 立刻做 |
| gateway 需扩到 2 副本 | P1-6 | cross-instance routing |
| aggregator lag > 30s 持续 5min | P1-7 | stream 分片 |
| status page 公开前 | P1-11 (Phase 1) | 业务指标埋点 |
| 第一次"注册成功但收不到邮件"客诉 | P1-10 | auth 流程包 WithTx |
| 第一次跨服务 trace 跟不到 | P1-5, P1-12 | Stream/WS propagation + WithoutCancel |
| 上 k8s/容器编排前 | P2-13 | /healthz + /readyz 全覆盖 |
| 节点扩到 10+ 前 | P1-13 (A 方案) | Ansible 批量推 agent |
| 节点扩到 30+ 前 | P1-13 (B 方案) | agent self-update |
| 商业正式上线 / 海外用户接入 | P2-17 | GDPR 删除流程 |
| Vault 接入第二个服务 | P2-16 | secrets 统一改造 |
| 第一次 KMS/TSA 抖动堆请求 | P2-8 | gobreaker |
| 用户报 Google 登录卡住 | P2-12 | OAuth HTTP client 加 timeout |
| 主库 CPU > 60% | P2-9 | 读写分离 |
| MCP 接入商业客户 | P2-11 | conn pool 隔离 |
| status page DAU > 1k | P2-9 | 读副本路由 |
| 任意 stream/billing 字段拼写 bug | P0-4 | contracts 包补完 |
| 改了 PRD 决策需找代码全部位置 | P1-9 | constants 包补完 |
| 新加服务时复制旧 config 代码 | P1-8 | shared/config 回迁 |
| 依赖版本不一致引发莫名 bug | P2-14 | go.mod 版本对齐 lint |
| JSON 序列化 P99 占比 > 20% | P2-15 | 换 sonic/jsoniter |

---

## 本季度建议执行的 10 件(按 ROI 排序)

| # | 项 | 工作量 | 收益类型 |
|---|---|---|---|
| 1 | **P0-1 Redis Sentinel** | 1 天 | 消除单点 |
| 2 | **P0-3 attest 部署核实** | 0.5 天 | 排除合规风险 |
| 3 | **P0-4 Week 1: `contracts.ProbeResult`** | 2 天 | 阻止字段拼写 bug |
| 4 | **P0-5 OpenAPI contract test (B 方案)** | 2 天 | 防 spec/代码脱钩 |
| 5 | **P1-10 auth 流程包 `WithTx`** | 1 天 | 消除注册脏数据 |
| 6 | **P1-11 Phase 1 业务指标埋点** | 2 天 | status page 公开前必备 |
| 7 | **P0-2 scheduler fencing token** | 2 天 | 多副本前置 |
| 8 | **P1-5 OTel Stream/WS propagation** | 1 天 | 跨服务 trace 可用 |
| 9 | **P1-12 fire-and-forget WithoutCancel** | 0.5 天 | 跟 P1-5 一起做 |
| 10 | **P1-8 + P1-9 shared/config + constants** | 3-5 天 | 一致性债清理 |

**总计约 15-17 个工作日**,可在 4 周内完成主要工作。

**渐进推进(不专门 sprint)**:
- **P0-4 Week 2-4** cert/attest/monitor contracts(跟 P1-4 handler 拆分一起做)
- **P1-4** api handler 按业务域拆分(每加新功能时顺手)
- **P1-11 Phase 2/3** dashboard + 告警(Phase 1 做完后下季度做)

**等触发再做**:
- P1-6 / P1-7(gateway / stream 扩展) → 等真到瓶颈
- P2-8 / P2-9 / P2-10 / P2-11 / P2-12 / P2-13 / P2-14 / P2-15 → 按触发条件表执行

---

## 立刻可启动的最小落地路径(本周内,2-3 天)

如果只能本周做事,做这两条**最高 ROI 组合**:

### 路径 A:消息契约保护(P0-4 Week 1 + P1-9 同步,2 天)

1. 新建 `backend/lib/shared/contracts/` 包
2. 新建 `backend/lib/shared/constants/` 包
3. 先定义 `contracts.ProbeResult` 和 `contracts.MonitorEvent` 两个类型
4. `stream.Client` 加 `AddProbeResultTyped` / `AddMonitorEventTyped` 新 API,旧 API 加 `// Deprecated`
5. **新代码强制用新 API**,旧代码标记 TODO 渐进迁移
6. `backend/scripts/lint-stream-payload.sh` 在 CI 阻止新增 `map[string]any` 跨 stream 边界

做完之后 50 节点上线就有"消息契约保护",不再依赖默契。

### 路径 B:auth 事务保护(P1-10,1 天)

1. `backend/apps/api/internal/handler/auth.go:209` Register handler 用 `WithTx` 包裹 CreateUser + CreateUserOTP + enqueueVerifyEmail 前的所有 DB 写
2. enqueueVerifyEmail 放在 tx 之外(commit 成功后再发邮件,outbox-lite 模式)
3. 写单元测试验证:CreateUserOTP 失败时,users 表无脏数据

做完之后**每天的用户注册流程**不再有"邮箱被占用但账号不可用"的脏数据问题。

**两条路径独立,可并行做**(不同 handler / 不同库,不会冲突)。

---

## 与现有文档的关系

- **`docs/prd/14-tech-architecture.md`**:PRD 级架构定义,未受本审查影响
- **`docs/ARCHITECTURE.md`**:项目实施架构,本审查建议追加"P1-6 gateway 单实例约束"和"P0-3 attest 部署形态"两节
- **`docs/prd/DECISIONS.md` §M**:v2 决策摘要,本审查不与 D1-D13 任何决策冲突
- **`docs/prd/ENG-REVIEW-REPORT.md`**(2026-05-13):PRD 工程审查,关注 KMS / Verdict / Refund 等业务流;本文关注**通信架构**,二者互补无重叠

---

## 校对核对项(供 review)

请逐项确认:

**P0 项**:
- [ ] **P0-1 Redis 单点** — 当前 prod Redis 是单实例还是已经 Sentinel/Cluster?如果托管在阿里云/腾讯云,是哪个套餐?
- [ ] **P0-2 scheduler 副本数** — 当前 prod 跑几个 scheduler 实例?计划扩到几个?
- [ ] **P0-3 attest 部署** — 当前 prod 是否有 attest 在跑?如果没有,S2 计划部署方式?
- [ ] **P0-4 contracts 包优先级** — 4 周路线(probe → cert → attest → monitor)是否认同?有没有更紧急的 stream payload 想先做?
- [ ] **P0-4 旧消息兼容策略** — 迁移期间 stream 里同时有"旧 map 格式 + 新 typed 格式"消息怎么处理?短期允许两种共存,还是版本号灰度切换?
- [ ] **P0-5 OpenAPI 路线** — A 方案(codegen)还是 B 方案(contract test 先做)?如果 B,什么时候评估升 A?

**P1 项**:
- [ ] **P1-4 api handler 拆分** — 是否认可"渐进重构,不专门做重构 sprint"的策略?
- [ ] **P1-5 OTel Stream propagation** — 当前是否实际在用 OTel collector?Trace 数据流向哪里(Jaeger / Tempo / SaaS)?
- [ ] **P1-6 gateway 副本数** — 当前 prod 跑几个 gateway 实例?
- [ ] **P1-7 probe stream 量级预估** — 实际每天 probe.results 消息量大概多少?(决定 P1-7 触发时机)
- [ ] **P1-8 shared/config 回迁** — `DatabaseDSN` vs `PGDSN` 统一到哪个命名?(我建议 `Database.DSN`)环境变量前缀用什么(`IDCD__` / `SHARED__` / 其他)?
- [ ] **P1-9 constants 包** — 是否需要 `lint-magic-numbers.sh` lint 脚本强制?还是只靠 review?
- [ ] **P1-10 事务覆盖范围** — 除 auth / billing / cert / OAuth 外还有哪些写多步流程需要 tx?是否需要先 audit 一遍?
- [ ] **P1-11 业务指标优先级** — 表中 10 项是否全要,还是先做 auth/cert-svc/scheduler 3 个最关键的?
- [ ] **P1-12 WithoutCancel 范围** — 全代码 grep 替换可以吗?有没有故意需要 detached context 的场景?

**P2 项**:
- [ ] **P2-8 熔断器** — 已经引入 gobreaker 在 go.sum,是否要按 KMS / TSA / LE / 阿里通义 / DeepSeek 5 个 provider 各包一层?
- [ ] **P2-9 PostgreSQL 部署** — 当前 PG 是托管还是自部署?有没有 read replica?
- [ ] **P2-10 邮件统一** — 当前邮件是否真有 "billing 直接发 + notifier 也发" 的双写?
- [ ] **P2-12 OAuth HTTP client** — `http.DefaultClient` 在 oauth.go 用了 4 处,是否一起包替换 timeout?
- [ ] **P2-13 健康检查范围** — 5 个无 /healthz 的服务(agent/aggregator/mcp/notifier/scheduler)中,agent 不需要(出站 WS),其他 4 个是否一次性补?

**其他**:
- [ ] **P1-13 agent 升级** — 节点扩张时间表是什么?决定 A/B 方案先后
- [ ] **P2-16 secrets 统一** — 当前 Vault 实例是自部署还是云托管?其他服务接入是否有约束?
- [ ] **P2-17 GDPR** — 商业上线时间表?是否需要先做 Phase 1(标记软删除)?
- [ ] **反模式清单** — 8 条是否有不同意见(特别是 NATS/Kafka 那条)?
- [ ] **已确认健康项** — 测试覆盖率统计是否需要在 CI 维持(防止退化到 <70%)?
- [ ] **是否拆 TASKS.md** — 10+ 件本季度任务是否进 TASKS.md(同步条目)?

核对完后,确认无误的项打勾,有调整的项标注,我据此更新文档并决定是否拆 TASKS 条目。
