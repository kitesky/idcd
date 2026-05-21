# Runbook: Gateway 单实例约束

> 关联约束:**P1-6**（`docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md`）
> 关联架构文档:`docs/ARCHITECTURE.md` §2.6

---

## 1. 约束描述

Agent Gateway（`backend/apps/gateway/`，端口 8084）当前为**单实例部署**。所有 Agent 节点的 WebSocket 连接均落在同一进程的内存 map（`connections: map[string]*Connection`）中。

**后果**：gateway 进程重启、OOM 或宿主机故障时，100+ 节点的 Agent **同时断连**，所有进行中的任务回执丢失，直到 Agent 自动重连后恢复。

---

## 2. 监控设置

### 2.1 必须监控的指标

| 指标 | 采集方式 | 告警阈值 | 级别 |
|---|---|---|---|
| `gateway_up` | Prometheus `up{job="gateway"}` | == 0 持续 15s | **P0** |
| `gateway_ws_connections_total` | gateway `/metrics` 端点 | 骤降 > 50%（1min 窗口）| **P0** |
| `gateway_process_restarts_total` | container restart count | > 0 in 5min | **P1** |
| `gateway_memory_rss_bytes` | cAdvisor / process exporter | > 80% container limit | **P1** |
| `container_oom_events_total{name="gateway"}` | Docker / cAdvisor | > 0 | **P0** |

### 2.2 Grafana 告警规则示例

```yaml
# Prometheus alerting rule
- alert: GatewayDown
  expr: up{job="gateway"} == 0
  for: 15s
  labels:
    severity: P0
  annotations:
    summary: "Agent Gateway 不可达"
    description: "gateway 进程 down，所有 Agent WebSocket 连接已断开。"
    runbook_url: "docs/RUNBOOKS/gateway-single-instance.md"

- alert: GatewayConnectionDrop
  expr: |
    (gateway_ws_connections_total - gateway_ws_connections_total offset 1m)
    / gateway_ws_connections_total offset 1m < -0.5
  for: 0s
  labels:
    severity: P0
  annotations:
    summary: "Agent Gateway 连接数骤降 >50%"

- alert: GatewayHighMemory
  expr: process_resident_memory_bytes{job="gateway"} / container_spec_memory_limit_bytes > 0.8
  for: 2m
  labels:
    severity: P1
  annotations:
    summary: "Gateway 内存使用 >80%，有 OOM 风险"
```

---

## 3. 故障恢复流程

### 3.1 Gateway 进程意外退出

**预期恢复时间**：< 60s（Docker restart policy + Agent 30s 重连）

| 步骤 | 操作 | 预期结果 |
|---|---|---|
| 1 | 确认 gateway container 状态 | `docker ps -a \| grep gateway` |
| 2 | 检查退出原因 | `docker logs gateway --tail 100` — 看 OOM / panic / signal |
| 3 | 若 Docker restart policy 已自动恢复 | 确认 `gateway_up == 1` + 连接数恢复 |
| 4 | 若未自动恢复,手动重启 | `docker compose -f infra/docker/docker-compose.core.yml up -d gateway` |
| 5 | 验证 Agent 重连 | 观察 `gateway_ws_connections_total` 在 30s 内恢复到断连前水位 |
| 6 | 检查任务回执 | 断连期间发出的任务回执会丢失；Scheduler 会在下一轮调度重发 |

### 3.2 Gateway OOM Kill

| 步骤 | 操作 |
|---|---|
| 1 | 确认 OOM：`dmesg \| grep -i oom` 或 `docker inspect gateway \| grep OOMKilled` |
| 2 | 临时增大内存限制：修改 `docker-compose.core.yml` 中 gateway 的 `mem_limit` |
| 3 | 重启：`docker compose -f infra/docker/docker-compose.core.yml up -d gateway` |
| 4 | 根因分析：检查连接数增长趋势，排查 Agent 异常重连风暴或内存泄漏 |

### 3.3 宿主机故障

| 步骤 | 操作 |
|---|---|
| 1 | 确认宿主机状态（SSH / 云控制台）|
| 2 | 若宿主机可恢复 → 启动 Docker → `docker compose up -d` |
| 3 | 若宿主机不可恢复 → 在备用机上拉起 gateway（更新 DNS `agent-wss.idcd.com` 指向新 IP）|
| 4 | DNS TTL 当前 300s，完全切换需 5min |
| 5 | Agent 会在 DNS 刷新后自动重连到新实例 |

---

## 4. Agent 重连行为

Agent 客户端内置重连机制（`backend/apps/agent/`）：

- 断连检测：WebSocket ping/pong，5s 超时
- 重连策略：指数退避（1s → 2s → 4s → 8s → 16s → 30s cap）
- 最大重连间隔：30s
- 重连成功后：重新注册 node_id + 上报 buffer 中暂存的 probe 结果（SQLite 24h buffer，D17）

**正常情况下，gateway 重启后 30s 内所有 Agent 应完成重连。** 若超过 60s 仍有大量 Agent 未重连，检查：
- DNS 是否正确解析
- mTLS 证书是否过期
- 网络层（防火墙 / 安全组）是否变更

---

## 5. 长期修复计划

**触发条件**：gateway 需要扩到 2+ 副本时（连接数接近单实例 10k 上限，或 SLA 要求消除单点）。

### Phase 1：连接状态外置（Redis）

- 将 `connections` map 从内存迁移到 Redis HASH `agent_routes`
- key：`agent_id`，value：`gateway_instance_id`
- TTL 与 WebSocket ping 同步刷新（30s）
- 任务回执通过 Redis Pub-Sub 跨实例广播

### Phase 2：Sticky LB

- Cloudflare Load Balancer 按 `agent_id` hash 路由
- 同一 Agent 始终落在同一 gateway 实例
- 实例下线时自动 rehash，Agent 重连到新实例

### Phase 3：多区部署

- 中/美/欧各部署 gateway 实例（已在 ARCHITECTURE.md §2.1 规划）
- Agent → 就近 gateway，一致性哈希路由

---

## 6. 相关文档

- `docs/ARCHITECTURE.md` §2.6 — 约束说明与缓解措施
- `docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md` P1-6 — 原始审查记录
- `docs/prd/10-nodes-and-agents.md` — Agent 节点设计
- `docs/TROUBLESHOOTING.md` — 故障排查总索引
