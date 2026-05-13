# 10 · 节点与 Agent 系统(v2)

> 关联：OVERVIEW.md §4.9、§7 阶段路线图、§11 决策 #2 #3、§11.K
> 关联(v2):DECISIONS.md §K-节点;18-evidence(Verdict 引用节点数据);12 §21 节点失窃应急 SOP
> 阶段主体：S1 自有节点全量上线；S2 增强(短期 mTLS + CRL/OCSP + OTA 3 级灰度 + Anchor 偏差告警);S3 众包试点
> 品牌名占位:`idcd`

---

## 1. 模块定位

节点系统是 `idcd` 的**核心壁垒和最大成本中心**。一切拨测、监控、API 的最终结果都依赖节点真实执行。本模块定义：

1. 节点的**类型与角色**（自有 / 众包 / 专属）
2. **Agent 程序**的协议、安全、版本
3. **调度系统**如何分配任务
4. **健康监控**与异常剔除
5. **公开节点目录**（透明度卖点）
6. **众包模型**（S3 试点，激励 + 反作弊）

### 设计原则

1. **结果可信优先**：节点要有标识、有签名、有水印
2. **节点是普通服务器**：不依赖特定云厂、特定网络环境
3. **Agent 可观测、可升级、可剔除**
4. **众包节点天生不可信**：默认怀疑 + 多重验证
5. **节点能力非均质**：分级 + 路由 + 权重

### 关键指标

| 指标 | S1 目标 | S3 目标 |
|---|---|---|
| 自有节点数量 | 100+ | 维持 + 增加冷门地区 |
| 众包节点数量 | 0 | 500+ |
| 节点平均可用率 | ≥ 95% | ≥ 98% |
| 任务成功率 | ≥ 97% | ≥ 99% |
| 调度延迟 P95（任务下发到执行开始） | ≤ 2s | ≤ 1s |
| 节点心跳异常自动剔除时间 | ≤ 90s | ≤ 60s |

---

## 2. 节点类型与角色

### 2.1 节点类型

| 类型 | 来源 | 信任级别 | 计费 | 阶段 |
|---|---|---|---|---|
| **Owned IDC** | 自购国内多线 VPS / 物理机 | 高 | 内部成本 | S1 |
| **Owned Cloud** | 自购海外云 VPS（Hetzner / Vultr / RackNerd…） | 高 | 内部成本 | S1 |
| **Anchor**（锚定节点） | 自有但稳定性更高、配置更好的节点 | 最高 | 内部成本 | S2 |
| **Community**（众包） | 第三方贡献的 VPS / 软路由 / 家宽 | 中 | 积分激励 | S3 |
| **Dedicated**（专属） | 给企业客户专门部署的节点 | 高 | 企业版包 | S4 |
| **Private**（私有部署） | 客户机房内部署的 Agent | 客户自负 | 企业版 | S4 |

### 2.2 节点角色（横切）

| 角色 | 说明 |
|---|---|
| Tier 1 | 高频任务承载、关键监控用、SLA 高 |
| Tier 2 | 一般拨测、可承载部分监控 |
| Tier 3 | 仅参与一次性公开测试、不承载付费监控 |
| Anchor | 用作"基准节点"，对照其他节点结果偏差 |
| Probe-only | 只跑拨测任务 |
| Speedtest | 兼任测速服务端（带宽测试） |

> 一个节点可有多个角色 tag，由调度系统按需路由。

### 2.3 节点元数据

```yaml
id: nd_jp_tk_01_vultr
type: owned_cloud
status: active                  # active|drain|maintenance|disabled
tier: 1                          # 1|2|3
roles: [probe, speedtest, anchor]

# 地理
country: JP
country_name: "Japan"
region: "Kanto"
city: "Tokyo"
latitude: 35.6762
longitude: 139.6503
timezone: "Asia/Tokyo"

# 网络
ipv4: "203.0.113.10"             # 节点出口（可能多个，多 IP）
ipv6: "2001:db8::1"
asn: 20473
asn_org: "Vultr Holdings"
isp_category: "datacenter"       # datacenter|residential|mobile

# 容量
cpu_cores: 1
memory_mb: 1024
bandwidth_mbps_in: 100
bandwidth_mbps_out: 100
max_concurrent_tasks: 50
max_rps: 30

# 能力
capabilities:
  http: true
  https: true
  http2: true
  http3: true
  ping_icmp: true
  ping_v6: true
  tcping: true
  udp: true
  dns: true
  traceroute: true
  mtr: true
  speedtest: true
  websocket: false
  browser: false

# 软件
agent_version: "1.4.2"
os: "Debian 12"
kernel: "6.1.0-x"

# 运维
provider: "Vultr"
deployed_at: "2026-04-10"
last_health_check_at: ...
last_seen_at: ...
contact: "ops@idcd.com"
notes: ""
```

---

## 3. Agent 程序设计

### 3.1 技术栈
- **语言**：Go（静态编译，跨平台）
- **二进制**：单文件 5–15 MB，无外部依赖
- **运行方式**：systemd / docker 容器 / 二进制直跑
- **目标平台**：linux/amd64 / linux/arm64 / linux/arm（树莓派）
- **资源占用**：内存 < 50 MB（空闲），CPU < 5%（满任务）

### 3.2 Agent 二进制结构

```
agent
├── core
│   ├── transport       # WebSocket + mTLS 长连
│   ├── scheduler       # 本地任务调度
│   ├── reporter        # 结果上报、批合并
│   ├── healthcheck     # 自检 / 自上报
│   └── updater         # OTA 升级
├── probes
│   ├── http
│   ├── ping
│   ├── tcping
│   ├── dns
│   ├── traceroute / mtr
│   ├── udp
│   ├── speedtest
│   └── websocket (S3)
└── config              # 启动配置（密钥、控制中心地址）
```

### 3.3 任务执行边界（硬编码白名单）

**Agent 不接受任意命令执行**。能力是硬编码的：

| 任务类型 | 实现 | 风险 | 限制 |
|---|---|---|---|
| HTTP/HTTPS | Go net/http | 中 | URL Scheme 白名单、超时、最大响应大小 |
| Ping | raw socket（需 cap）| 低 | 包数、间隔下限 |
| TCPing | TCP dial | 低 | 仅常用端口或用户已购权限 |
| DNS | miekg/dns | 低 | DNS server 白名单 + 用户自定义二级校验 |
| Traceroute | UDP/ICMP | 中 | 跳数上限 |
| UDP probe | 受限 payload | 高 | 默认禁用，仅 53/123/161 类端口开放 |
| Speedtest | 节点之间互测 | 中 | 频率限制 |

> 任何不在白名单内的"任务"都被 Agent 拒绝并上报异常。

### 3.4 安全设计

#### mTLS 双向认证(v2 增强:短期证书 + 主动撤销)

- Agent 启动时持有 client cert(部署时签发)
- 连控制中心 WSS:必须双向 TLS 验证
- 服务端 cert 由内部 CA 签发
- **v2 NEW:Agent 客户端证书短期(默认 7 天,可配 30 天)**
  - 老证书过期前 24 小时自动 renewal(走当前 valid mTLS 通道换新)
  - renewal 必须验证节点 fingerprint(MAC / CPU 指纹)与历史一致;突变 → 拒绝 + P1 告警(疑似失窃)
  - 失效证书保留 7 天用于审计追溯(只读,不能 renew)
- **v2 NEW:CRL/OCSP 主动撤销路径**
  - 内部 CA 维护 CRL(每分钟刷新);OCSP responder 实时回应
  - Gateway 在每次 WSS 握手时校验客户端证书的 OCSP 状态(可缓存 60 秒)
  - 节点怀疑失窃 → 控制台一键 revoke → 1 分钟内 OCSP 拒绝该证书 → Gateway 主动关闭该节点的所有 active 连接 → 节点从调度池剔除 → 1 小时内完全踢出
- **v2 NEW:节点 fingerprint 突变告警**:每次 renewal / 心跳带节点 fingerprint(MAC / CPU 指纹 / hostname / kernel version);突变 → P1 告警 + 节点 drain 等待人工审核

#### 任务签名
- 每个下发任务带控制中心签名（Ed25519）
- Agent 校验签名后才执行
- 防止中间人注入任务

#### 节点身份认证
- 启动 token：一次性的 enrollment token（部署脚本生成）
- 用 token 换取**短期** cert(v2:从"长期"改为"7 天短期 + 自动 renewal")
- Token 可在控制台撤销

#### 数据上报签名
- 结果带节点签名（防伪造）
- 防止"假 Agent"伪造数据

#### v2 NEW: 应急撤销 SOP(详 12 §21 节点失窃应急 SOP)

```
T+0   检测告警(任一路径:OCSP 异常 / Anchor 偏差 / fingerprint 突变 / 流量异常)
T+1m  自动:节点从调度池 drain
T+5m  自动:撤销节点客户端证书(CRL 推送 + OCSP 拒绝)
       Gateway 主动关闭该节点 active WSS 连接
T+15m 人工审核(避免误剔除)
T+30m 若确认:相关时间窗内拨测结果标记"低置信"
       影响的 Verdict 报告自检 → 必要时重新生成或退款
T+1h  节点机器重装 + 重新签发证书 + 重新入池
```

### 3.5 任务上报水印

每条 `probe_result` 强制包含：

```yaml
node_id: nd_jp_tk_01_vultr
agent_version: 1.4.2
executed_at: 2026-05-12T15:23:45Z
target: "https://example.com"
client_request_id: "..."       # 关联到发起方
signature: "ed25519:..."
```

这是反滥用与合规的关键证据链。

---

## 4. 通信协议

### 4.1 连接模式
- Agent 主动连控制中心 WSS（`agent-wss.idcd.com:443`）
- 一条长连接维持，断线指数退避重连
- 同时支持 HTTP fallback（部分受限网络）

### 4.2 消息类型

```
A → S (Agent → Server):
  hello { node_id, agent_version, capabilities, ip... }
  heartbeat { ts, load, in_progress_tasks }
  task_ack { task_id, accepted | rejected | reason }
  task_progress { task_id, percent, partial_result }
  task_result { task_id, success, raw, summary, duration_ms }
  log { level, message }

S → A (Server → Agent):
  task_dispatch { task_id, type, params, timeout, signature }
  task_cancel { task_id }
  control { cmd: pause | drain | resume | upgrade | reload_config }
  ping
```

### 4.3 协议格式
- 默认 JSON（开发友好、可调试）
- 可选 Protobuf（性能场景）
- 消息含序列号、批 ID（结果可批合并）

### 4.4 任务下发流程

```
Scheduler 选定 task + 节点
  → 序列化 task_dispatch (含签名 + TTL)
  → push 给 Agent
  → Agent 校验签名 + 资源充足 → task_ack(accepted)
  → 执行 probe
  → 上报 task_progress (可选)
  → 完成后上报 task_result
Scheduler 等待结果 / 超时
  → 超时后自动重试 (路由到其他节点)
```

---

## 5. 调度系统（Scheduler）

### 5.1 任务来源

| 来源 | 频率 | 优先级 |
|---|---|---|
| 公开工具一次性测试 | 突发 | 中 |
| 一键诊断 | 突发 | 中 |
| 监控定时任务 | 高频可预测 | 按订阅档（Free 低、Business 高） |
| API 请求 | 突发 | 按 API Key 档位 |
| 节点健康检查（内部） | 周期 | 高 |

### 5.2 调度策略

#### 5.2.1 节点筛选
按任务定义中的 `node_selection`（见 04-monitoring.md §3.1）：
- 地理：country / region / city
- 网络：ISP / ASN
- Tier 与 Tag
- IP 版本支持

筛选后得到候选池（候选池一般 ≥ 任务需要数量的 3 倍）。

#### 5.2.2 节点打分（候选池内）
- 健康度（最近 100 次任务成功率）
- 负载（当前并发任务数 / 容量）
- 与目标的地理/ASN 距离（速度）
- 历史与目标交互的稳定度（缓存：节点 X → 目标 Y 历史成功率）
- Anchor 节点偏好系数

#### 5.2.3 选 N 个
- 按 `pool_size` 选 top-N
- 添加少量随机扰动（防过度聚集）

#### 5.2.4 错峰
- 同 `interval_sec` 的监控自动分配偏移（hash(monitor_id) % interval）
- 避免每分钟整点突发

### 5.3 优先级队列

```
P0 内部健康检查 / 紧急维护
P1 Business 用户监控
P2 Team 用户监控
P3 Pro 用户监控 + API key 付费
P4 Free 用户监控 + 公开工具登录用户
P5 公开工具未登录用户
```

资源紧张时按优先级丢任务（不丢 P0-P3）。

### 5.4 重试与降级
- 节点超时 / 拒绝 → 自动选另一节点重试
- 重试上限 = pool_size（保证不会无限放大负载）
- 全部失败 → 任务标记失败 + 触发告警

### 5.5 节点亲和性
- 监控倾向于"上次成功的同一节点"（趋势可比较）
- 但每 N 次至少切换一次（防节点对单一目标"作弊"）

---

## 6. 节点健康监控

### 6.1 心跳
- Agent 每 30s 发一次 heartbeat
- 服务端 90s 无心跳 → 标记节点 `unreachable`
- 5 分钟仍无心跳 → 自动 drain（不再下发新任务）
- 24h 仍无心跳 → 标记 `offline`，等人工或自动 reprovision

### 6.2 健康指标
- **成功率**：最近 1h / 24h / 7d 任务成功率
- **延迟稳定性**：单节点对一组锚定目标（如 google / baidu）的 RTT 抖动
- **资源**：CPU、内存、磁盘、网络 in/out
- **任务积压**：未消费任务数

### 6.3 自动剔除规则
- 1h 成功率 < 70% → 自动 drain（仅做读，新任务路由其他节点）
- 24h 持续异常 → 标记 disabled
- 锚点对比偏差 > 阈值（"该节点对所有目标 RTT 都明显异常"）→ 提醒人工

### 6.4 节点恢复
- Drained 节点 6h 后自动重新评估
- 健康度回升 → 自动启用

### 6.5 锚定基准 + 偏差实时告警(v2 增强)

- 维护一组"已知良好目标"(每个区域 5-10 个;详 §10 §10 决策 E7 第三方基准:baidu / google / cloudflare)
- 节点定期对这些目标拨测(默认每 5 分钟,Anchor 节点每 1 分钟)
- 异常偏差 → 节点本身有问题
- 用于发现"看起来在线但实际网络异常"的节点
- **v2 NEW: 偏差告警阈值(分级,v2 D10 标记为 placeholder)**

> **v2 D10 锁定**:当前 ×2 / ×3 / ×5 阈值为 **S1 初期 placeholder**,无历史数据校准。
> **S2 上线前必须完成 30 天 baseline 数据校准报告**,基于真实 anchor 偏差分布重新调整阈值;不同区域 / 时段差异化。
> 详 17-roadmap S1 末 + S2 初 里程碑。

| 偏差严重度 | 判定(S1 placeholder) | 自动处理 | 告警 |
|---|---|---|---|
| 轻度 | 单节点对 anchor RTT 偏差 > 同区域中位数 ×2,持续 5 分钟 | 标记 "low_confidence" | 内部 W1 |
| 中度 | 偏差 > 中位数 ×3,持续 10 分钟 | 自动 drain + 任务路由其他节点 | 内部 P2 |
| 高度 | 偏差 > 中位数 ×5,或与其他节点结果出现"系统性矛盾"(如 anchor 不可达但同区域其他节点可达) | 立即 drain + **当前时间窗内该节点拨测结果标记"低置信"** + 影响的 Verdict 报告自检 | 内部 P1 + 运维告警 |
| 致命 | 节点连续返回"我说你看不见 baidu"但其他节点说"看得见" | 立即 disable + 走 12 §21 失窃应急 SOP | P0 |

**S1 末 → S2 初 阈值 calibration 流程(v2 D10)**:
- S1 上线后采集 30 天 baseline 数据(anchor 偏差分布、按 region × ASN × 时段聚合)
- 创始人 + CC 协助分析,产出 calibration 报告:
  - 真实合法节点抖动占多少 σ?
  - 不同区域 / 时段(白天 vs 凌晨)阈值是否需要差异化?
  - 推荐 P95 / P99 阈值替代 ×2 / ×3 / ×5
- 报告入 `/admin/calibration/anchor-thresholds-2026-XX.md`
- 阈值更新写入 `anchor_deviation_threshold` 配置表(可热更新,不需 deploy)

- **v2 NEW: 数据污染自动恢复(v2 D-Concern8 增强 — 向前回溯审查)**:
  - 节点被判定"高度偏差"后,**该节点过去 N 分钟(由偏差持续时间决定)的所有拨测结果**自动标记 `confidence=low`
  - **向前回溯审查(D-Concern8)**:如果偏差持续 10 分钟,系统不仅标记最后 10 分钟,**还要回溯审查前 30 分钟数据**;若发现偏差有渐进式增长趋势(从 0 → 阈值)→ 扩大标记窗口到前 30 分钟
  - 防御场景:攻击者"先 8 分钟正常 + 后 2 分钟造假"→ 向前回溯检测异常增长 → 不仅标记后 2 分钟也标记前异常增长期
  - Aggregator 计算可用率时剔除 `low` 数据
  - Verdict 报告引用该数据的 → 触发"补充重新生成"或自动退款流程
  - **若排除 low_confidence 节点后节点数 < 3** → 报告**拒绝生成 + 自动退款**(防止数据强度不足)

#### Anchor 节点的特殊地位

- Anchor 自身偏差检测:Anchor 节点也对**其他 Anchor 节点**做交叉验证;3 个 Anchor 中 1 个偏差 → 该 Anchor 自动降级为 Tier1 节点
- Anchor 部署位置:每个主要区域 ≥ 3 个 Anchor(国内 BGP / 海外大区);避免单点
- Anchor 提供方:**优先用与节点提供商不同的厂商**(避免 ASN 重合);例如节点用 RackNerd,Anchor 用 Hetzner

### 6.6 节点数据污染恢复流程(v2 NEW)

```
T+0  Anchor 偏差告警(中度或更高)
T+1m 节点 drain(从调度池移除)
T+5m 系统自动:
      - 该节点过去 5-30 分钟(由偏差严重度决定)的拨测结果标记 confidence=low
      - 触发 Aggregator 重算受影响监控的可用率
      - 触发 Verdict 报告自检(若有引用该节点数据)
T+15m 运维人工 review:确认是节点故障 vs 真实失窃
       若失窃 → 走 12 §21 应急 SOP
       若故障 → 节点重启 / 重装 / 联系机房
T+1h  节点完全踢出 或 恢复(取决于审核)
T+24h post-mortem 写入 audit_log
```

---

## 7. 公开节点目录（透明度卖点）

### 7.1 公开页面（`/nodes`）

#### 列表
- 列：ID（脱敏） / 国家 / 城市 / ISP / ASN / Tier / 状态 / 当前负载 / 加入时间
- 总数 + 按区域聚合（地图视图）
- 状态分布饼图（在线 / 离线 / 维护中）

#### 节点详情页（`/nodes/<id>`）
- 节点完整元信息（除 IP，看决策点）
- 最近 24h 健康曲线
- 锚定基准对比图
- 部署日期、运维方（如适用）

### 7.2 公开 API
- `GET /v1/nodes` 返回公开元数据
- `GET /v1/nodes/<id>` 详情
- `GET /v1/nodes/<id>/health` 健康指标

### 7.3 公开 / 不公开的字段决策

| 字段 | 公开 | 备注 |
|---|---|---|
| 节点 ID（脱敏，如 `nd_jp_tk_01`） | ✅ | 透明度 |
| 国家 / 城市 | ✅ | |
| ISP / ASN | ✅ | |
| 出口 IP | ❓ | **开放决策点**：暴露便于用户加白名单，但可能被恶意攻击 |
| 提供商（Vultr / Hetzner...） | ✅ | 已是公开信息（ASN 可查） |
| 容量 / 当前负载 | ✅（粗粒度） | 不暴露精确数字 |
| 部署日期 | ✅ | 老节点更可信 |
| 历史可用率 | ✅ | 信任建立 |

---

## 8. 节点部署与运维

### 8.1 部署 Playbook
- 一行命令安装（curl + bash）
- 或 Ansible playbook（批量）
- 或 Terraform 模块（IaC）

### 8.2 启动流程
```
1. 下载 agent 二进制
2. 生成 enrollment token（控制台后台预先发放）
3. 启动 agent --token=xxx --controller=wss://...
4. Agent 用 token 换 cert
5. 注册节点元信息（自动检测 IP/AS/地理）
6. 等待人工审核（首次接入）
7. 审核通过 → 启用
```

### 8.3 升级（OTA, v2 修订:3 级灰度)
- 控制中心可批量下发 upgrade 指令
- Agent 校验签名 → 下载新二进制 → 自启动 → 老进程优雅退出
- **v2 修订: 灰度三级**:
  - **L1 (1%)**:随机 1-2 个节点;观察 1 小时;错误率 / 任务失败率 / Anchor 偏差全部在基线 ×2 以内 → 进入 L2
  - **L2 (10%)**:扩到 10 节点;观察 4 小时;同样指标 → 进入 L3
  - **L3 (100%)**:全量推送(分批,每 5 分钟推 20%)
- 任何阶段错误率突增 > 基线 ×2 → **自动回滚 + P1 告警 + 暂停后续灰度**
- 紧急安全补丁(CVE 高危)可走快速通道:L1 (1%) 5 分钟 → L3 (100%) 直接,但**必须运维双人确认**

### 8.3a 节点 fingerprint 与 mTLS renewal(v2 NEW)

- Agent 每次启动 / heartbeat 上报 fingerprint:`{mac, cpu_signature, hostname, kernel_version, distro}`
- 服务端比对上次记录的 fingerprint:
  - 一致 → 正常
  - 突变 → 上报 P1 告警 + Agent drain + 走 12 §21 应急 SOP
  - 首次上报 → 记录为基线
- mTLS renewal:每 7 天自动 renew,renewal 时再次 fingerprint 比对 + Anchor 偏差检测;任何异常拒绝 renewal → 证书自然过期 → 该节点失效

### 8.4 配置热更新
- 控制台可远程改 Agent 的运行参数（任务并发、限速、能力开关）
- 通过 control 消息推送，无需重启

### 8.5 节点故障排查工具
- 控制台 → 节点详情 → "诊断" Tab
- 一键拉取最近 100 条日志
- 触发自检（ping anchor / DNS 解析 / HTTP 出口测试）

### 8.6 节点退役
- 标记 `drain` → 不再下发新任务
- 等待已分配任务完成
- 标记 `disabled` → 从调度中移除
- 物理停机

---

## 9. 节点容量规划

### 9.1 容量模型
- 单 1C/1G 小机：50-100 任务/分钟（基础拨测）
- Speedtest 节点：建议 ≥ 2C/2G + 100Mbps 带宽，单机 5-10 并发
- 浏览器监控节点（M11）：独立集群，2C/4G 起步，单机 5 并发

### 9.2 容量预测
- 监控总数 × 平均频率 / 60 = 每秒任务数
- 每任务平均 3 节点 → 节点 RPS = 任务 RPS × 3
- 单节点 RPS 上限 → 需要节点数

### 9.3 弹性策略
- 突发流量（公开工具）有专属节点池（不影响付费监控）
- 节点池预留 30% 余量
- 接近容量上限自动扩容（运维人工触发，S3 自动化）

### 9.4 区域容量分布建议（S1 100+ 节点）

| 区域 | 节点数 | 备注 |
|---|---|---|
| 中国大陆电信 | 5+ | 上海/广州/北京等核心机房 |
| 中国大陆联通 | 5+ | 同上 |
| 中国大陆移动 | 5+ | 同上 |
| 中国大陆 BGP 多线 | 10+ | 江浙沪、华南、华北 |
| 中国香港 | 4 | 多家 ISP |
| 中国台湾 | 1-2 | 受限节点 |
| 日本 | 5 | 东京 + 大阪 |
| 新加坡 | 3 | |
| 韩国 | 2 | |
| 越南 / 泰国 / 印尼 / 马来 / 菲律宾 | 各 1-2 | 东南亚覆盖 |
| 印度 / 巴基斯坦 | 各 1-2 | |
| 中东（迪拜 / 沙特 / 以色列） | 各 1 | |
| 欧洲（德 / 法 / 英 / 荷 / 瑞 / 意 / 西 / 北欧） | 各 1-2，共 10+ | |
| 俄罗斯 + 东欧（波兰 / 乌克兰） | 3+ | |
| 美国西 / 中 / 东 | 各 3，共 9 | |
| 加拿大 / 墨西哥 | 各 1-2 | |
| 巴西 + 南美 | 3+ | |
| 澳洲 / 新西兰 | 各 1-2 | |
| 南非 + 非洲 | 2+ | |
| **合计** | **≥100** | |

> 实际部署按预算和可获取的低配 VPS 调整。

---

## 10. 众包节点（S3 试点）

> 核心设计目标：**用户自助加入 → 系统自动发现 → 自动评估 → 自动入池 → 自动剔除** 的全闭环。99% 情况零人工，1% 边缘场景走人工兜底。

### 10.1 模式

- Agent **开源**（MIT/Apache 2.0）：开发者可审计、贡献、自部署
- 任何人（满足申请门槛）可贡献 VPS / 软路由 / 家宽设备 → 拿积分 → 兑换查询额度或 Pro 会员
- 系统将众包节点视为**默认不可信**，全流程自动检测、自动管理

### 10.2 申请门槛（防黑产薅羊毛）

申请贡献节点的账号必须满足：

| 条件 | 要求 |
|---|---|
| 账号年龄 | ≥ 7 天 |
| 邮箱已验证 | 必须 |
| 已通过基础认证 | 任一：成功添加 1 个监控用 ≥ 7 天 / 完成一次实名 / 已付费过 |
| 未在黑名单 | 用户、邮箱域名、注册 IP 三维度检查 |

不满足 → 申请页直接禁用并提示如何达成。

### 10.3 节点完整状态机

```
   [provisioning]              用户拿到 token 但 Agent 还没连上
        │ (Agent 首次连接)
        ▼
   [enrolling]                 已连接，正在自动注册 + 自检
        │ (基础校验通过)
        ▼
   [observing]                 观察期 24-72h，仅执行公开测试 + honey-task
        │
        ├── 基准达标 ──────────► [active T3]    自动入池，参与公开测试
        │
        └── 基准不达标 ────────► [rejected]      自动拒绝 + 邮件说明
                                                ↑
   [active T3] ───30天稳定──► [active T2]      自动晋升
        │
   [active T2] ───90天稳定──► [active T1]      自动晋升（最高众包等级）
        │
        ├── 1h 成功率<70% ───► [drained]        软剔除，已分配任务跑完
        │      │
        │      └── 6h 后健康度恢复 → 回 active；持续异常 → 进 disabled
        │
        ├── 24h 持续异常 ────► [disabled]       从调度池移除，公开目录显示离线
        │      │
        │      └── 30 天内仍无心跳 → 进 offline
        │
        ├── 蜜罐失败/严重作弊 ► [banned]        永久封禁，积分清零，用户进黑名单
        │
        └── 7 天无心跳 ──────► [retired]        自动退役，积分冻结
```

每个状态转换都是**自动触发**，对应明确的检测信号（见 §10.6）。

### 10.4 自助加入流程（完全自助）

```
1. 用户登录控制台 → /app/nodes/contribute
   ↓
2. 检查申请门槛（§10.2），不达标 → 提示
   ↓
3. 选择部署方式：
   • 一行命令（VPS）
   • Docker compose
   • Ansible role
   • 树莓派镜像
   ↓
4. 系统生成 enrollment token（24h 过期 + 单用户最多 5 个待用 token）
   ↓
5. 用户在目标机器上跑：
   curl -fsSL https://get.idcd.com/agent | sh -s -- --token=xxx
   ↓
6. Agent 启动 → 连控制中心 → 状态变 enrolling
   ↓
7. 自动注册流程（无需人工）：
   • 反查 IP / ASN / 地理（GeoIP + RIR）
   • 自检能力（icmp 可用 / 端口出口 / IPv6 / 内存等）
   • 跑一组基准任务（ping 5 个 anchor + DNS 解析 + HTTP 出口测试）
   • 上报指纹（IP + ASN + machine-id + 启动时间）
   ↓
8. 系统自动判定（毫秒级）：
   a) 基础校验通过（ASN/IP 不在黑名单、地理不与同 owner 节点冲突、能力达标）
      → 状态变 observing → 进入观察期
   b) 异常（VPN/TOR/已知作弊 ASN/能力不足）
      → 状态变 rejected + 邮件告知用户具体原因
   ↓
9. 用户在控制台实时看节点状态（"观察中 12h / 72h"）
```

整个流程**不需要管理员介入**，正常情况下 5 分钟内完成 enrolling。

### 10.5 自动发现 & 入池（observing 阶段）

观察期 24-72h 内：

#### 自动下发任务
- **Honey-task**（蜜罐）：每小时 2-5 次，已知预期结果
- **Echo-task**（回声）：跟随其他节点跑同一任务，用于一致性对比
- **公开测试任务**：少量真实用户的公开拨测任务（带降权）

#### 持续评估指标
| 指标 | 通过门槛 |
|---|---|
| 心跳完整率 | ≥ 95% |
| 蜜罐命中率 | ≥ 95% |
| Echo 一致性（与多数偏差） | ≤ 阈值 |
| 出口延迟稳定性（mdev） | ≤ 阈值 |
| 资源占用合理性（不异常波动） | 通过 |

#### 自动判定
- 24h 后：若所有指标满足且地理覆盖有价值 → 直接升 T3 active 入池
- 72h 后：仍不满足 → 自动 rejected + 通知用户改进建议
- 边缘情况（多项接近门槛）→ 自动延长观察 48h + 通知用户

**关键**：稀缺地区节点（如非洲/南美/中亚）门槛自动放宽 20%（这些地区少有对照样本）。

### 10.6 自动剔除（三级，全自动）

| 等级 | 触发条件 | 动作 | 恢复路径 |
|---|---|---|---|
| **L1 Drain（软剔除）** | • 5 min 无心跳<br>• 1h 成功率 < 70%<br>• 单节点对 ≥ 5 个目标的延迟突增 > 3σ | 暂停下发新任务，已分配任务跑完；调度池移除 | 6h 后自动重评估，健康恢复 → 自动回 active |
| **L2 Disable（停用）** | • 24h 持续异常<br>• 锚定基准对比偏差超阈值 ≥ 6 小时<br>• Drain 累计 ≥ 3 次/7 天 | 节点目录显示"离线"；自动邮件通知贡献者 | 用户修复后控制台手动"重新启用" → 走简化版 observing（12h） |
| **L3 Ban（封禁）** | • Honey-task 结果异常（恶意改结果）<br>• 同指纹多账号注册<br>• 对自家关联域名异常优待<br>• 多源情报命中（如 IP 出现在 abuseipdb） | 永久封禁 + 积分清零 + 用户进黑名单 + 通知 | 申诉流程（§10.10） |

L1 / L2 完全自动；L3 中"单节点单一信号触发"完全自动，"短时间内大批量触发"自动转人工 review（防误杀）。

### 10.7 自动检测信号（时序）

```
┌──────────────────────────────────────────────────┐
│  实时（毫秒级）                                     │
│    • Agent 连接 / 断开 → 状态机迁移                  │
│    • 节点指纹冲突检测 → 立即 Ban                     │
│    • Agent 协议版本不兼容 → 立即 Drain               │
└──────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────┐
│  每分钟                                            │
│    • 心跳超时检查 → Drain                          │
│    • 任务超时率 → 调度优先级降低                     │
└──────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────┐
│  每 5 分钟                                         │
│    • 滑动窗口成功率 → Drain                         │
│    • 在执行任务积压 → 自动降并发                     │
└──────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────┐
│  每小时                                            │
│    • 锚定基准对比 → 累积异常分                       │
│    • 蜜罐任务命中率统计                              │
│    • Echo 一致性分布 → 异常者扣分                    │
│    • 节点对单一目标的优待检测                        │
└──────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────┐
│  每日 02:00                                       │
│    • 全量节点健康评分                                │
│    • Tier 自动晋升 / 降级                            │
│    • 累积分超阈值节点 → Disable                       │
│    • 同 ASN 节点数量配额检查                          │
│    • 积分日结                                        │
└──────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────┐
│  按需                                              │
│    • GitHub / 公开网络扫描泄露 token → 立即吊销        │
│    • 第三方滥用情报源同步（AbuseIPDB / Spamhaus）       │
│    • 多源情报命中 → Ban                              │
└──────────────────────────────────────────────────┘
```

### 10.8 信任分级（与状态机配合）

| Tier | 入选条件（全自动判定） | 参与任务类型 |
|---|---|---|
| T3 | observing 通过 | 公开测试、一次性 API 调用 |
| T2 | T3 持续 30 天 + 蜜罐命中率 ≥ 99% + Echo 一致性偏差 ≤ 阈值 | + Free 档监控 |
| T1 | T2 持续 90 天 + 用户社区好评 + 提供稀缺地区 | + **Pro 档监控（默认参与，用户可关）** |

> **决策 E1**：T1 众包节点默认参与 Pro 档付费监控（用户在监控配置中可关闭"接受众包节点"）。
> 众包节点**永远不参与 Team/Business 档的关键监控**（除非 S4 与贡献者签订专门合作协议升为 Dedicated）。

### 10.9 积分体系

#### 获得
- 完成正确任务：1 分
- 蜜罐命中：×2
- Echo 与多数一致：×1
- 与多数偏差大：×0.3 或负
- 稳定运行：每日 5 分
- 稀缺地区：×2-×5 加成
- T2/T1 节点：×1.5 / ×2 加成

#### 扣除
- Drain 一次：-50 分
- Disable 一次：-500 分
- Ban：清零

#### 用途
- 兑换 API 调用额度（100 分 = 1k 次）
- 兑换 Pro 月会员（按市场价折算）
- 加入"贡献者社区"特殊权益（年度礼品 / 周边）
- **不可提现**（防黑产洗钱）

### 10.10 反作弊（自动检测）

#### 多维节点指纹（自动）
- IP / ASN
- 内核 / 系统信息 / machine-id / boot id
- CPU info / 磁盘 UUID
- SSH host key 哈希
- 启动时间漂移（同源 VPS 启动序列可识别）
- 网络指纹（出口 MTU / TCP 时间戳 / TLS 客户端指纹）

同指纹 → 立即 Ban；高相似度 → 标记观察。

#### 一致性检测（自动）
- 同任务下发 ≥ 5 节点 → 结果做聚类
- 与多数共识偏离 > 阈值 → 累积异常分

#### 行为分析（自动）
- 节点对特定目标长期偏离基准 → 异常分
- 节点对该用户 owner 关联域名异常优待 → 异常分
- 节点拒绝特定任务比例异常 → 异常分

#### Honey-task（自动 + 关键）
- 系统在任务流中随机插入已知结果的任务
- 节点不知道哪个是蜜罐
- 蜜罐结果错误 → 立即 L2 或 L3

#### 同 ASN 限权（自动）
- 同 ASN 众包节点上限：10 个（默认）
- 单 owner 节点数上限：5 个（Free 用户）/ 10 个（Pro+）
- 触达上限 → 自动拒绝新申请

#### 跨用户协同检测（自动）
- 多用户 VPS 来自同 ASN / 同提供商集群账号 → 标记观察
- 协同申请时间 / IP 段相似 → 触发风控

### 10.11 人工兜底边界（1% 场景）

**完全自动覆盖 99%，人工只在下面这些边缘情况介入**：

| 场景 | 人工动作 |
|---|---|
| 短时间内（< 1h）同地区 / 同 ASN 大批量 Ban（≥ 10 个）| 暂停自动 Ban，进入 Security Officer 队列人工 review |
| 某地区第一个节点的 observing 失败 | 人工确认是真异常还是缺对照样本 |
| 用户申诉（"我被错封了"） | Security Officer 审核 |
| Honey-task 系统自身可能故障的情况下大量异常 | 自动暂停 Honey-task 判定 + 告警内部 |
| 节点贡献排行榜 TOP 10 的节点出现异常 | 自动 Drain 但触发管理员通知，避免误伤明星贡献者 |

人工兜底队列在管理后台 `/admin/abuse/community-nodes-review`，SLA：24h 内处理。

### 10.12 申诉流程

```
用户被 Ban / Disable
  → 邮件包含申诉链接（含原因详情）
  → 用户点击 → /app/nodes/appeal/<id>
  → 填表：陈述 + 证据（截图、配置）
  → 提交 → 进入 Security Officer 队列
  → 7 天内响应：
      a) 误判 → 解除 + 恢复积分 + 道歉模板邮件
      b) 维持 → 维持封禁 + 详细说明
      c) 部分误判 → 解除 Ban 但保持降级
  → 用户对处理结果可再次申诉（仅一次）
```

每个用户的申诉次数和成功率记录在内部档案，**反复无理申诉者**进入静默处理列表。

### 10.13 退出（用户主动）

- 控制台一键"退役该节点"
- 进入 30 天保留期（积分仍可使用）
- 30 天后节点元数据脱敏（保留贡献历史聚合数据，不识别个人）
- 节点积分按"已锁定"处理，可兑换不可累积

### 10.14 数据模型补充（新增）

```
community_node_application
  id, user_id, requested_at,
  enrollment_token, token_used_at,
  approved_at, rejected_at, rejection_reason,
  resulting_node_id

community_node_observation
  id, node_id, started_at, ended_at,
  honey_total, honey_passed,
  echo_total, echo_consistent,
  baseline_score, decision (passed|rejected|extended)

community_node_status_event
  id, node_id, from_state, to_state,
  triggered_by (auto|admin_user_id),
  signal_type, signal_details (jsonb),
  occurred_at

community_node_appeal
  id, user_id, node_id, related_event_id,
  statement, evidence (jsonb),
  status (pending|upheld|reversed|partial),
  reviewed_by, reviewed_at, decision_note,
  user_satisfied (true|false|null)

community_node_fingerprint
  node_id, fingerprint_hash,
  components (jsonb), confidence,
  duplicate_of_node_id (nullable)

honey_task_template
  id, target, expected_result_summary (jsonb),
  task_type, params (jsonb), enabled,
  detection_threshold
```

### 10.15 关键流程图：自助加入到自动入池

```
[用户登录] → 点击 "贡献节点"
     ↓
检查门槛（账号 ≥ 7d、邮箱验证、已通过基础认证）
     ↓
生成 enrollment_token (24h 有效)
     ↓
[用户在 VPS 上跑 install.sh]
     ↓
Agent 连接控制中心 (mTLS + token 换 cert)
     ↓
[provisioning] → [enrolling]
     ↓
自动注册：地理 / ASN / 能力 / 指纹
     ↓
基础校验：黑名单 / 重复 / 能力达标
     ↓
   ┌─不通过─→ [rejected] + 邮件原因
   │
   └通过
     ↓
[observing] 24-72h
     ↓
持续接收 honey + echo + 真实公开任务
     ↓
自动评估：心跳 / 蜜罐 / Echo / 延迟稳定性
     ↓
   ┌─不达标─→ [rejected] + 邮件改进建议
   │       └─边缘──→ 延长 48h
   │
   └达标
     ↓
[active T3] → 自动入调度池
     ↓
持续运行 + 自动监控（每分钟 / 每小时 / 每日）
     ↓
[T3 → T2 → T1] 自动晋升 或
[active → drained → disabled → banned] 自动剔除
```

---

## 11. 专属与私有节点（S4 企业版）

### 11.1 专属节点（Dedicated）
- 企业付费"购买"指定数量的 `idcd` 自有节点专属使用
- 节点资源 100% 服务该企业
- 价格：年付几千到几万元每节点

### 11.2 私有节点（Customer-deployed）
- 企业在自己机房部署 Agent
- 控制中心仍是 `idcd` 的 SaaS（或私有控制中心，企业 Plus）
- 用于：内网监控、内部 DNS 解析、合规要求数据不出域

### 11.3 私有部署版（On-Premises）
- 整套系统（控制中心 + 数据库 + 节点）部署到客户机房
- 仅 Enterprise 客户
- 提供升级订阅 + 技术支持

---

## 12. 数据模型

```
node
  id, type, status, tier, roles (text[]),
  country, region, city, latitude, longitude, timezone,
  ipv4, ipv6, asn, asn_org, isp_category,
  cpu_cores, memory_mb, bandwidth_mbps_in, bandwidth_mbps_out,
  max_concurrent_tasks, max_rps,
  capabilities (jsonb),
  agent_version, os, kernel,
  provider, deployed_at, last_seen_at,
  enrolled_by (admin_user_id|null),
  owner_id (null|community_user_id),  -- 众包节点的贡献者
  trust_level (1|2|3),                -- T1/T2/T3
  is_anchor, contact, notes,
  created_at, updated_at

node_heartbeat
  node_id, ts, load_metrics (jsonb), in_progress_tasks

node_health_metric_hour
  node_id, bucket_at,
  total_tasks, succ_tasks, fail_tasks,
  avg_latency_ms, p95_latency_ms,
  uptime_seconds

node_capability_check
  node_id, capability, last_checked_at, status, error

node_enrollment_token
  token, owner_id (community|admin),
  used_at, expires_at, target_node_id

community_node_points
  node_id, user_id, total_points,
  daily_bonus, task_bonus, penalty,
  last_calc_at

honey_task
  id, target, expected_result (jsonb), enabled
```

---

## 13. 关键流程

### 13.1 任务调度全流程

```
任务提交（监控触发 / API / 公开工具）
  → Scheduler 选节点池：
      • 应用 node_selection 筛选
      • 候选池打分排序
      • 选 top-N
      • 错峰偏移
  → 对每节点：
      • 构造 task_dispatch (含 TTL/签名)
      • Push 到 Agent (WSS)
  → Agent：
      • 校验签名
      • 资源充足 → ack accepted
      • 执行 probe
      • 上报 task_result
  → Scheduler / Aggregator：
      • 收集 N 个节点结果
      • 任意失败 → 路由到候选下一节点重试
      • 全部到位或 deadline → 聚合
      • 生成 monitor_check 或 probe 结果
  → 异常判定 → 触发告警引擎
```

### 13.2 节点入网流程

```
管理员 / 申请人 → 控制台
  → 申请 enrollment token
  → 复制部署脚本（含 token）
  → 在目标服务器跑 install.sh
  → Agent 启动 → 用 token 换 cert
  → 上报 capability / 地理 / 网络
  → 进入"待审"
  → 管理员审核（自有自动通过；众包人工 + 自动联合）
  → 启用
  → Anchor 基准测试通过
  → 加入调度池
```

### 13.3 节点异常处理

```
节点 X 5 分钟无心跳
  → 状态 active → drain
  → 已分配任务不撤回，但不再下发新任务
  → 监控未完成结果超时 → 任务路由其他节点
  → 30 分钟无心跳 → unreachable
  → 24h 无心跳 → offline
  → 管理员邮件告警
  → 用户透明：状态页节点目录显示离线
```

---

## 14. 与其他模块的接口

| 模块 | 接口 |
|---|---|
| `02-public-tools.md` | 公开工具调用 Scheduler |
| `04-monitoring.md` | 监控调用 Scheduler |
| `05-alerting.md` | 节点异常触发告警（内部） |
| `07-reports.md` | 节点健康指标 / SLA 报告 |
| `08-open-api.md` | API 调用走相同的调度路径 |
| `11-admin.md` | 节点管理后台 |
| `12-compliance-and-abuse.md` | 节点任务带水印用于追溯；任务白名单 |
| `14-tech-architecture.md` | Agent ↔ Scheduler 协议详情 |

---

## 15. 阶段交付清单

### S1（0–4 月）
- Agent 1.0：HTTP / Ping / TCPing / DNS / Traceroute / MTR
- mTLS + 任务签名 + 上报水印
- Scheduler 基础（筛选 + 打分 + 错峰 + 重试）
- 心跳 + 自动 drain / offline
- 公开节点目录（/nodes）
- 100+ 自有节点部署
- 安装脚本 + Ansible playbook
- 节点容量预警

### S2（4–8 月）
- Agent 1.x：UDP / HTTP/2 HTTP/3 / Speedtest
- 节点能力分级（Tier）+ 角色 tag
- Anchor 锚定基准
- OTA 灰度升级
- 节点告警与自动恢复
- 节点诊断工具（控制台）
- IPv6 全面支持

### S3（8–14 月）
- Agent 1.x：WebSocket / 浏览器拨测（独立 Worker 集群）
- **众包节点完整闭环**：
  - 自助加入流程（一行命令 + 控制台引导）
  - 节点状态机（10 状态 + 转换自动化）
  - 自助观察期评估（蜜罐 + Echo + 锚定基准）
  - 三级自动剔除（Drain / Disable / Ban）
  - 时序自动检测（实时 / 分钟 / 小时 / 每日）
  - 信任分级 + 积分体系
  - 反作弊：多维指纹 / 蜜罐 / 一致性 / 同 ASN 限权
  - 人工兜底队列（大批量异常 + 申诉）
  - 申诉流程
  - 公开贡献排行榜
- IaC（Terraform 模块）

### S4（14+ 月）
- 专属节点（企业版）
- 私有部署（On-Premises）
- 高级路由策略（基于成本 / SLA 合同）
- 多控制中心容灾

---

## 16. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 同行 DDoS 我们的 Agent 出口 IP | Cloudflare 前置 + 多 IP 出口 + 节点目录 IP 不公开 |
| 自有节点被运营商投诉为"扫描" | 限速 + 拒测黑名单 + 节点合规 ToS + 申诉响应通道 |
| Agent 漏洞被利用 | 任务白名单硬编码 + 签名校验 + 沙箱化（S3）|
| OTA 升级失败导致大量节点掉线 | 灰度策略 + 自动回滚 + 紧急 kill switch |
| 众包节点被用作内网穿透 | 任务 target 严格白名单 + DNS 解析校验 + 拒绝私网段 |
| 节点元数据被反向利用攻击 | IP 公开 vs 隐私的权衡（见开放决策点） |
| 调度系统成为单点 | S2 起多区部署 + Redis 队列分片 |
| **v2 NEW: 节点失窃 → 伪造拨测结果污染历史** | 短期 mTLS 证书(7d) + CRL/OCSP 主动撤销 + fingerprint 突变告警 + Anchor 偏差实时检测 + 数据污染自动恢复(详 §6.6 / 12 §21) |
| **v2 NEW: Anchor 节点自身被攻陷误判正常节点** | 多 Anchor 交叉验证 + 不同厂商部署 + 异常 Anchor 自动降级 |
| **v2 NEW: Verdict 报告引用了"低置信"节点数据** | Anchor 偏差实时检测 + Aggregator 自动剔除 + Verdict 报告自检 + 必要时退款重新生成 |

---

## 17. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **A7** 公开节点目录：**完全公开出口 IP**
- ✅ **E5** Agent 开源：**S3 与众包同步开源 MIT**
- ✅ **E6** 节点首批厂商：**分散 8+ 家**（避免同 ASN 集中）
- ✅ **E7** Anchor 锚定目标：**第三方**（baidu / google / cloudflare 等）
- ✅ **E4** 众包申请门槛：**账号 ≥ 7 天 + 邮箱验证 + 基础认证**
- ✅ **E3** 众包大批量 Ban 转人工阈值：**1 小时同 ASN ≥ 5 个**
- ✅ **E2** Honey-task 占比：**3-5%**
- ✅ **E1** 众包参与 Pro 监控：**T1 默认参与**（用户可关闭）；T2/T3 仅 Free

### v2.0 (K 节, 2026-05-12)
- ✅ **K-节点 mTLS 短期证书**:Agent 客户端 cert 默认 7 天,可配 30 天;自动 renewal + fingerprint 校验
- ✅ **K-节点 CRL/OCSP 主动撤销**:1 分钟内推送,1 小时内全节点完全踢出
- ✅ **K-节点 Anchor 偏差实时告警**:4 级阈值(轻 / 中 / 重 / 致命)+ 数据污染自动恢复
- ✅ **K-节点 fingerprint 突变告警**:每次 renewal / 心跳校验,突变 → P1 告警 + drain
- ✅ **K-节点 OTA 3 级灰度**:1% → 10% → 100%,失败自动回滚

### 待定（不紧迫）

- [ ] 节点贡献者公开 attribution（"by xxx" 显示在节点目录）：建议提供选项让贡献者自选
- [ ] 申诉成功后是否额外赔偿（积分加倍 / 礼品）：建议小额积分加倍即可
- [ ] **v2 NEW** Anchor 节点是否提供"商业 SLA 监控"作为收费项(给企业客户更高 SLA 保证):S4 评估
