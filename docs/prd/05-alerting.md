# 05 · 告警系统（通道、策略、事件）

> 关联：OVERVIEW.md §4.4、04-monitoring.md §14
> 阶段主体：S2 全量上线（核心商业模块的孪生）
> 是否登录：**必须登录**
> 品牌名占位：`idcd`

---

## 1. 模块定位

告警系统是监控的"最后一公里"。监控发现异常的价值只有通过告警通道送达到对的人才能兑现。`idcd` 的告警系统要做到：

1. **送达可靠**：通道冗余 + 重试 + 多通道并发
2. **不打扰**：智能抑制、静音、合并
3. **可路由**：按团队 / 时间 / 严重程度路由到不同人
4. **可追溯**：每条告警有完整轨迹（触发→派发→签收→恢复）
5. **支持二次通知**：异常 5/30/60 分钟后未恢复升级

### 关键指标

| 指标 | S2 目标 | S3 目标 |
|---|---|---|
| 告警从事件创建到第一通道送达 P95 | ≤ 30s | ≤ 15s |
| 通道送达成功率 | ≥ 99% | ≥ 99.9% |
| 用户主动确认率（acknowledged） | — | ≥ 50% |
| 误报标记率 | ≤ 5% | ≤ 1% |

---

## 2. 告警通道全集

### 2.1 通道总览

| ID | 通道 | 双向 | 计费 | P | S |
|---|---|---|---|---|---|
| C01 | 邮件 | 单向 | 内部成本 | P0 | S2 |
| C02 | Webhook | 单向 | 免费 | P0 | S2 |
| C03 | 微信（公众号模板消息 / 个人微信 Bot） | 单向 | 内部成本 | P0 | S2 |
| C04 | 企业微信机器人 | 单向 | 免费 | P0 | S2 |
| C05 | 钉钉机器人 | 单向 | 免费 | P0 | S2 |
| C06 | 飞书机器人 | 单向 | 免费 | P0 | S2 |
| C07 | Telegram Bot | 单向 | 免费 | P1 | S2 |
| C08 | Slack | 单向 | 免费 | P1 | S2 |
| C09 | Discord | 单向 | 免费 | P1 | S2 |
| C10 | 短信 | 单向 | **按条计费** | P2 | S3 |
| C11 | 电话语音 | 单向 | **按通计费** | P3 | S4 |
| C12 | App Push（自家 App） | 双向 | 免费 | P3 | S4 |
| C13 | PagerDuty / Opsgenie 集成 | 单向 | 免费 | P2 | S3 |

### 2.2 通道详细规格

#### C01 邮件
- 发送基础设施：自建 SMTP（多 IP 出口）+ 备用 SES/SendGrid
- 单封邮件可配模板（事件类型 + 严重程度）
- 同一事件多次通知：合并为线程（同一 Subject + In-Reply-To）
- 退订机制（合规）：每封邮件底部退订链接
- 限速：单用户/单收件人 100 封/小时

#### C02 Webhook
- 配置：URL + Method (POST/PUT) + 自定义 Headers + 签名密钥（HMAC-SHA256）
- Payload 模板：可自定义 JSON 模板，注入变量 `{{ event.type }}` `{{ monitor.name }}` `{{ duration }}`
- 重试：失败时按指数退避（5s, 30s, 2m, 10m, 1h, 6h），最多 6 次
- 签名：`X-Brand-Signature: t=<ts>,v1=<hmac>`
- 超时：连接 5s / 响应 10s

#### C03 微信（重点：国内主战场）

**子通道 1：公众号模板消息**
- 用户关注 `idcd` 服务号 → 在站内绑定 → 收消息
- 优势：合规、稳定、可贴卡片
- 限制：模板消息有审核

**子通道 2：个人微信 Bot（通过企业微信第三方应用 / WxPusher / Server酱 等）**
- 用户提供 WxPusher Token / Server酱 SCKEY
- 走第三方推送服务
- 不强依赖某一家，多源兜底

**子通道 3：企业微信应用号**
- 企业用户配置自建应用 → 推送给内部成员
- 用于 Team / Business 档

#### C04/C05/C06 企微/钉钉/飞书机器人
- 配置：用户拷贝 Webhook URL（机器人创建时拿到）+ 签名密钥
- 支持富文本 / Markdown / 卡片
- 支持 @ 指定成员（手机号或 UserID）
- 检测推送失败的常见错误（机器人被踢出群、Token 失效）

#### C07 Telegram
- 用户在 `idcd` 控制台生成绑定码 → `/start <code>` 触发 `idcdBot` → 完成绑定
- 推送：标准 Markdown
- 支持按用户 / 群组 / Channel 推送

#### C08/C09 Slack / Discord
- 通过 Incoming Webhook（用户在 Slack 创建 App）
- 富卡片支持

#### C10 短信
- 国内：阿里云 / 腾讯云 SMS（需企业资质 + 内容报备）
- 国外：Twilio
- 费用：国内 ¥0.045/条、国外 $0.005-0.02/条
- 按条扣余额，超额停发
- 仅 Team/Business 可用，且单监控每天最多 5 条

#### C11 电话语音
- 国内：阿里云语音通知
- 用 TTS 念告警内容
- ¥0.08-0.15/通
- 仅 Business / Enterprise 可用

#### C12 自家 App Push
- iOS / Android App（远期 S4）
- 双向：可在 App 内 ack / 静音

#### C13 PagerDuty / Opsgenie
- 标准 Webhook 适配
- 让 `idcd` 成为这些专业告警平台的 source

---

## 3. 告警策略（Alert Policy）

告警策略是"什么时候、用哪个通道、通知谁、要不要升级"的规则集合。一个监控关联一个策略，多个监控可共享一个策略。

### 3.1 策略数据结构

```yaml
id: ap_xxx
name: "默认（生产环境）"
owner_id: u_xxx

# 通知规则（多组，按 severity / 时间 / 监控筛选）
rules:
  - match:                          # 匹配条件
      severity: [critical]          # 严重程度
      tags_any: ["prod"]            # 监控标签
      time_window:                  # 时间窗口
        timezone: "Asia/Shanghai"
        days: [mon, tue, wed, thu, fri]
        start: "09:00"
        end: "22:00"
    channels:                       # 命中后用哪些通道
      - { type: email, to: ["a@x.com"] }
      - { type: wecom_robot, webhook: "...", at_mobiles: ["13800138000"] }
    delay_sec: 0                    # 立即派发
    repeat:
      enabled: true
      interval_sec: 600             # 异常未恢复每 10 分钟提醒
      max_count: 3                  # 最多重复 3 次

  - match:                          # 第二条规则：夜间走电话
      severity: [critical]
      tags_any: ["prod"]
      time_window:
        days: [mon, tue, wed, thu, fri]
        start: "22:00"
        end: "09:00"
    channels:
      - { type: voice, to: ["13800138000"] }
    delay_sec: 0

# 升级策略（escalation）
escalation:
  enabled: true
  steps:
    - after_sec: 300                # 5 分钟没人 ack
      channels:
        - { type: email, to: ["manager@x.com"] }
    - after_sec: 900                # 15 分钟没人 ack
      channels:
        - { type: voice, to: ["13800138001"] }   # 老板电话

# 告警合并（抑制）
suppression:
  group_by: [monitor_id, severity]  # 按字段分组合并
  group_wait: 30                    # 进入分组后等 30s，看是否还有新事件
  group_interval: 300               # 同分组每 5 分钟最多一次

# 静默
mute:
  enabled: false
  reason: ""
  until: null

# 恢复通知
on_recovery:
  enabled: true
  channels: [email, wecom_robot]
```

### 3.2 策略匹配优先级
- 多条 rule 按数组顺序优先匹配
- 第一条匹配的 rule 被使用
- 不匹配 → 不发送（用户主动取消通知）

### 3.3 严重程度（Severity）
- `critical`：核心服务异常（用户配置时勾选）
- `warning`：性能退化、证书快过期
- `info`：维护开始 / 结束、配置变更通知

### 3.4 默认策略
- 新用户注册自动创建 `default` 策略：邮件 + Webhook（如配置）
- 监控创建时若未指定策略，使用 default

---

## 4. 通道连接（用户视角）

### 4.1 通道管理页（`/app/alerts/channels`）

#### 列表
- 已绑定通道列表：类型、名称、状态（正常 / 异常）、最近测试时间
- 操作：测试发送 / 编辑 / 删除 / 暂停

#### 绑定流程
- 选择通道类型 → 进入对应绑定向导
- 引导式（截图 + 步骤）：
  - 钉钉：截图教如何加机器人、复制 Webhook、配置签名
  - 微信：扫码关注公众号 → 后台显示已绑定
  - Webhook：填 URL + 测试发送
- 绑定完成自动发送一条 "测试消息"

#### 通道健康
- 系统自动定期验活（每 24h）
- 连续 3 次推送失败 → 标记"异常"，邮件通知用户
- 用户可手动"测试通道"

### 4.2 个人通知偏好（`/app/settings/notifications`）
- 每种事件类型默认走哪些通道
- 工作日 / 周末 / 假期不同偏好
- "请勿打扰"时段
- 静默全部告警（紧急情况）

---

## 5. 告警事件（Event）管理

### 5.1 事件列表（`/app/alerts/events`）

#### 视图
- 列：状态 / 严重程度 / 监控名 / 触发时间 / 持续时长 / Ack 状态 / 解决人 / 备注
- 状态色：● 开启（红）/ ● 已确认（黄）/ ● 已解决（绿）
- 筛选：状态 / 严重程度 / 时间范围 / 监控组 / 标签
- 时间线视图（按天聚合，一眼看到"今天有多少事件"）

### 5.2 事件详情（`/app/alerts/events/<id>`）

- 事件基础信息：触发时间、持续时长、严重程度、关联监控
- 触发原因：哪些节点失败、断言哪条没过
- **派发轨迹**：每条通知（通道、收件人、派发时间、送达状态、签收时间）
- **协作区**：
  - Ack 按钮（一键确认，停止后续重复通知）
  - 添加备注（事件分析、根因、处理人）
  - 标记"误报"反馈到学习系统
- **关联事件**：同一监控历史事件
- **恢复信息**：何时恢复、恢复持续时间

### 5.3 Ack（签收）流程
- 用户在控制台、邮件链接、Webhook 回调中可 Ack
- Ack 后：
  - 停止重复通知
  - 但不影响升级（除非升级策略配置了 ack 后停止）
  - 记录 ack_by / ack_at
- 解决（resolve）：恢复时自动 resolve，也可手动提前 resolve（强制忽略）

### 5.4 协作 / 评论
- 团队成员可在事件下评论（沟通处理）
- @ 提醒队友（通过对应通道推送）
- 上传截图 / 链接（故障复盘材料）

---

## 6. 抑制与合并（防告警风暴）

### 6.1 同监控合并
- 同一监控连续异常 → 只发一次首条告警 + 周期性"仍未恢复"提醒

### 6.2 同分组合并
- 一组监控同时挂（按 group_by）→ 合并成一条聚合告警（"5 个监控同时异常"）
- 适合 IDC 故障、CDN 故障批量场景

### 6.3 依赖抑制（S3+）
- 配置"A 依赖 B"：B 挂时 A 的告警被抑制（避免父子级双重打扰）
- 例：DNS 挂时所有依赖该域名的 HTTP 监控告警被抑制

### 6.4 维护窗口
- 维护窗口内告警自动静默
- 但仍记录事件（用于复盘）

---

## 7. 升级策略（Escalation）

### 7.1 时间阶梯升级
- 配置：事件未 Ack 后 N 分钟升级一次
- 每一阶可配不同通道 / 收件人

### 7.2 排班升级（S3）
- On-Call Rotation：定义"主班 → 副班 → 经理"轮换
- 自动按当前时间找到值班人
- 排班人不响应 → 自动升级到下一位

### 7.3 应用场景
- 普通工程师 → 5 分钟内 ack
- 没人 ack → leader 收到
- 还没人 ack → 全员通知

---

## 8. 排班（On-Call Rotation）— S3

### 8.1 排班规则
- 周期：每天 / 每周 / 自定义
- 成员队列：[张三、李四、王五]
- 切换时间：每周一 09:00 切换
- 临时换班：成员之间互换

### 8.2 排班日历
- 看板：未来 4 周谁值班
- 飞书 / Slack 同步排班

### 8.3 替换 / 请假
- 成员可申请请假，自动顺延或指定替班

---

## 9. 模板系统

### 9.1 消息模板字段

可在 webhook payload / 邮件正文 / 机器人消息中使用：

```
事件级：
  {{ event.id }}             事件 ID
  {{ event.type }}           down | up | degraded
  {{ event.severity }}       critical | warning | info
  {{ event.started_at }}     ISO 时间
  {{ event.duration_sec }}   持续秒数
  {{ event.affected_nodes }} 受影响节点
  {{ event.reason }}         自动生成的原因摘要

监控级：
  {{ monitor.id }}
  {{ monitor.name }}
  {{ monitor.type }}
  {{ monitor.target }}
  {{ monitor.tags }}

链接：
  {{ event.url }}            事件详情页 URL
  {{ event.ack_url }}        一键 Ack URL
```

### 9.2 模板编辑器
- 富文本编辑器 + 变量插入下拉
- 预览：用一个示例事件渲染
- 测试发送

### 9.3 预置模板
- 简洁版（一行）
- 详细版（含完整上下文）
- 富卡片版（钉钉/飞书/企微）
- 故障复盘版（Markdown 详细）

---

## 10. 告警分析与报表

### 10.1 告警统计仪表盘
- 本月告警数 / 同环比
- TOP 10 异常监控
- TOP 10 频繁告警的标签 / 分组
- 通道送达成功率
- 平均 MTTR（平均恢复时间）
- 平均 MTTA（平均 ack 时间）

### 10.2 告警噪音分析
- 自动识别"短期内频繁触发又恢复"的监控 → 建议调整阈值
- 抑制后避免的告警数（"本月通过合并节省了 N 次打扰"）

### 10.3 告警导出
- CSV / JSON / PDF
- 用于安全审计 / 客户报告

---

## 11. 数据模型概览

```
channel
  id, owner_id, type, name, config (jsonb),
  health (ok|fail|paused),
  last_test_at, last_test_result,
  created_at

alert_policy
  id, owner_id, name, rules (jsonb),
  escalation (jsonb), suppression (jsonb),
  mute (jsonb), on_recovery (jsonb),
  is_default, created_at

alert_event
  id, monitor_id, owner_id, type (down|up|degraded),
  severity, started_at, ended_at,
  duration_sec, reason, affected_nodes (jsonb),
  acknowledged_by, acked_at,
  resolved_by, resolved_at, resolved_kind (auto|manual),
  is_false_positive, notes

alert_notification
  id, event_id, channel_id, channel_type,
  payload (jsonb), sent_at,
  delivery_status (sent|failed|retrying),
  attempts, last_error, latency_ms

alert_comment
  id, event_id, user_id, body, mentioned_users, created_at

oncall_schedule  (S3)
  id, team_id, name, rotation_type, members (jsonb), timezone

oncall_shift  (S3)
  id, schedule_id, user_id, start_at, end_at, swapped_from
```

---

## 12. 关键流程

### 12.1 单事件派发流程

```
Monitor: state UP → DOWN
  → 创建 alert_event (status=open)
  → 选择监控关联的 alert_policy
  → 评估 policy.rules：
      • 匹配 severity + 时间窗 + tags
      • 选中第一条匹配 rule
  → 通过 suppression 检查：
      • 是否在 group_wait 内（等待合并）
      • 是否在 group_interval 内（被抑制）
      • 命中 → 入等待队列
      • 未命中 → 立即派发
  → 派发到所有 channels：
      • 渲染模板
      • 调用通道 adapter 发送
      • 记录 alert_notification
      • 失败按指数退避重试
  → 启动重复提醒计时器（repeat.interval）
  → 启动升级计时器（escalation.steps）
```

### 12.2 Ack 流程

```
User 通过控制台 / 邮件链接 / Webhook 反向回调 Ack
  → alert_event.acked_at = now, acked_by = user_id
  → 停止 repeat 通知
  → 视升级配置可选停止 escalation
  → 派发 Ack 通知到相关频道（"小李已确认此事件"）
```

### 12.3 恢复流程

```
Monitor: state DOWN → UP
  → alert_event.resolved_at = now, resolved_kind = auto
  → 派发 on_recovery 通道
  → 停止所有计时器
  → 计算 MTTA / MTTR 写入事件
```

---

## 13. 反滥用 / 限速

| 维度 | 限制 |
|---|---|
| 单用户每分钟通知数 | Free 10 / Pro 60 / Team 300 / Business 1000 |
| 单通道每分钟通知数 | 100（防机器人接口被封） |
| 单 Webhook 目标每分钟 | 60 |
| 短信 / 语音单月配额 | 按订阅档 + 充值 |
| 邮件单用户每小时 | 100 |
| 用户连续告警风暴检测 | 自动启用合并 + 邮件告知用户 |

---

## 14. 安全与合规

- Webhook 签名（HMAC-SHA256），用户验证消息来源
- 邮件 DKIM/SPF/DMARC 全配置（防被识别成垃圾邮件）
- 短信内容报备（国内合规）
- 告警内容脱敏选项（POST body 隐藏、Header 脱敏）
- 通知接收人手机号 / 邮箱加密存储
- GDPR：用户注销时所有 alert_notification 中的 PII 被清除

---

## 15. 与其他模块的接口

| 模块 | 接口 |
|---|---|
| `04-monitoring.md` | 接收 monitor 状态变化事件 |
| `06-status-pages.md` | 事件可关联到状态页 incident |
| `07-reports-and-dashboards.md` | MTTA / MTTR / 噪音统计 |
| `08-open-api.md` | 通道 / 策略 / 事件的 CRUD API |
| `09-billing.md` | 短信 / 语音按量计费 |
| `11-admin.md` | 后台可看用户通知配额、批量调整 |

---

## 16. 阶段交付清单

### S2（4–8 月）
- 通道：C01–C09
- 策略：rules + repeat + on_recovery + 静默 + 维护窗口
- 事件：列表 / 详情 / Ack / 评论 / 误报反馈
- 抑制：同监控、同分组合并
- 升级：固定时间阶梯
- 模板：预置 4 类 + 自定义编辑器
- 报表：基础仪表盘

### S3（8–14 月）
- 通道：C10 短信、C13 PagerDuty / Opsgenie
- 排班 + 排班升级
- 依赖抑制
- 告警噪音分析
- 移动端响应式优化

### S4（14+ 月）
- 通道：C11 电话语音、C12 自家 App Push
- SLA 报告 + 合同绑定
- 高级审计日志

---

## 17. 决策记录（已锁定，见 DECISIONS.md）

- ✅ **C7** 短信 / 语音：**订阅档赠送配额 + 超额按量**
- ✅ **F2** 个人微信 Bot：**自家服务号模板消息 + 第三方 fallback**（Server酱 / WxPusher）

### 待定（不紧迫）

- [ ] Ack 后是否停止升级（业内分歧）
- [ ] 维护窗口内告警是否记录事件（默认建议记录但不通知）
- [ ] 排班档位起点：建议 Team 起（PRD 默认）
