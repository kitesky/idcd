# 06 · 状态页（Status Page）

> 关联：OVERVIEW.md §4.5、§11 决策 #6
> 阶段主体：S2 上线 MVP，S3 完善
> 是否登录：管理需登录，公开访问无需
> 品牌名占位：`idcd`

---

## 1. 模块定位

状态页让用户对外公示服务运行状况，是**重要的拉新引擎**：

1. **用户拉新**：免费状态页带 `Powered by idcd` 水印，每个访问者都是潜在新用户
2. **付费转化**：去水印 + 自定义域名是 Pro 起的核心付费点
3. **SaaS 完整性**：监控告警没有状态页就少一条腿
4. **企业销售钩子**：S4 白标 + 私有状态页面向企业大客户

### 关键指标

| 指标 | S2 目标 | S3 目标 |
|---|---|---|
| 创建状态页用户占比（活跃用户中） | ≥ 20% | ≥ 40% |
| 状态页 SEO 索引（外部反向曝光） | — | ≥ 1000 个站 |
| 通过状态页拉新（识别 `?ref=status`） | ≥ 5% | ≥ 15% |
| 自定义域名（付费转化标志） | — | ≥ 10% 付费用户使用 |

---

## 2. 状态页类型

| 类型 | 访问方式 | 适用 | 档位 |
|---|---|---|---|
| 公开状态页（默认子域） | `<slug>.status.idcd.com` | 个人 / SaaS 公开公示 | Free 起 |
| 公开状态页（自定义域） | `status.<userdomain>.com` | 自有品牌 | Pro 起 |
| 受密码保护的状态页 | 同上 + 密码 | 半私 | Team 起 |
| 私有状态页（登录后可见） | 同上 + 邀请制 | 内部用 | Team 起 |
| 多状态页 | 同一账号建 N 个 | 多产品线 | 按配额 |

### 各档配额（与 09-billing.md 同步）

| 档位 | 状态页数 | 自定义域名 | 去水印 | 私有 / 密码 |
|---|---|---|---|---|
| Free | 1 | ❌ | ❌ | ❌ |
| Pro | 1 | ✅ | ✅ | ❌ |
| Team | 3 | ✅ | ✅ | ✅ |
| Business | 10 | ✅ | ✅ | ✅ |

---

## 3. 公开访问页面结构

### 3.1 页面组成

```
┌──────────────────────────────────────────────────────┐
│  [Logo]  Title                              [中/EN]   │
│  Slogan / Description                                 │
├──────────────────────────────────────────────────────┤
│  ✅ 所有系统运行正常 ／ ⚠ 部分降级 ／ ❌ 重大故障          │
│  最后更新时间：YYYY-MM-DD HH:MM:SS                       │
├──────────────────────────────────────────────────────┤
│  [可用率概览]                                          │
│  - 90 天总体 99.98%                                    │
│  - 当月 99.99%                                         │
├──────────────────────────────────────────────────────┤
│  [服务分组]                                            │
│  Web 服务                                              │
│    ● Web 主站            ●●●●●●●●●●●● 99.99%             │
│    ● API 网关            ●●●●●●●○●●●● 99.85%             │
│  数据库                                                │
│    ● 主库                ●●●●●●●●●●●● 100%               │
│  CDN                                                  │
│    ● 国内 CDN            ●●●●●●●●●●●● 99.95%             │
│    ● 海外 CDN            ●●●●●○●●●●●● 99.62%             │
│                                                        │
│  ●●●●●●●●●●●● = 90 天逐日小条（绿=UP，黄=降级，红=DOWN）  │
├──────────────────────────────────────────────────────┤
│  [当前 / 进行中事件]                                    │
│  ⚠ 海外 CDN 部分节点降级 - 进行中（2 小时前开始）           │
│   2026-05-12 15:00 已识别问题，正在与服务商沟通              │
│   2026-05-12 14:30 收到监控告警                          │
├──────────────────────────────────────────────────────┤
│  [计划维护]                                            │
│  📅 数据库主库升级 - 2026-05-15 02:00-04:00              │
├──────────────────────────────────────────────────────┤
│  [历史事件]                                            │
│  2026-05-10 API 网关响应延迟（已解决，持续 18 分钟）        │
│  ...                                                  │
├──────────────────────────────────────────────────────┤
│  [订阅更新]                                            │
│  [邮件订阅]  [RSS]  [Webhook]                          │
├──────────────────────────────────────────────────────┤
│  Powered by idcd      (Free 档显示，付费档隐藏)       │
└──────────────────────────────────────────────────────┘
```

### 3.2 状态等级

| 等级 | 含义 | 颜色 |
|---|---|---|
| Operational | 全部正常 | 绿 |
| Degraded Performance | 性能降级（响应慢但可用） | 黄 |
| Partial Outage | 部分节点 / 部分功能不可用 | 橙 |
| Major Outage | 重大故障，多数不可用 | 红 |
| Under Maintenance | 计划维护 | 蓝 |

### 3.3 历史可用率展示

- 90 天逐日小条：每条代表 1 天，颜色按当天最严重事件
- 鼠标悬停显示当天事件摘要
- 支持切换 30 天 / 90 天 / 180 天 / 365 天

### 3.4 事件时间线

- 事件分阶段记录：identified（已识别）→ monitoring（监控中）→ resolved（已解决）
- 每个阶段时间 + 备注
- 维护者可手动添加更新

---

## 4. 服务（Component）模型

### 4.1 服务定义

一个"服务"是状态页上展示的最小单位（一行）。可以由：

- **关联监控**：选择 1 或 N 个监控，状态由这些监控聚合而成
- **手动维护**：纯人工管理状态（适合非自动监控场景）

### 4.2 服务聚合规则

当一个服务关联多个监控时：

| 规则 | 说明 |
|---|---|
| ANY_DOWN | 任一监控异常 → 服务异常 |
| ALL_DOWN | 全部监控异常 → 服务异常 |
| MAJORITY_DOWN | 多数异常 → 服务异常 |
| WEIGHTED | 加权（按用户配置每个监控的权重） |

### 4.3 服务分组（Section）
- 可分组：Web / API / 数据库 / CDN ...
- 每组在状态页内独立展示

### 4.4 不同地区视图（S3）

- 一个服务可在多个区域显示状态（"中国电信" / "美国西岸" / "欧洲" 分列）
- 后台关联的监控按区域聚合

---

## 5. 事件（Incident）管理

### 5.1 事件来源

| 来源 | 创建方式 |
|---|---|
| 自动事件 | 关联监控的 alert_event 自动同步（可配置过滤）|
| 手动事件 | 维护者在后台手动创建 |
| API 创建 | 通过 API 创建事件（CI/CD 集成）|

### 5.2 事件字段

```yaml
id: inc_xxx
title: "海外 CDN 部分节点降级"
status: investigating | identified | monitoring | resolved | postmortem
impact: minor | major | critical | maintenance
affected_services: [comp_cdn_oversea, comp_web]
started_at: ...
resolved_at: ...
updates:
  - status: investigating
    body: "收到监控告警..."
    posted_at: ...
    posted_by: user_id | system
  - status: identified
    body: "已确认是服务商问题..."
    posted_at: ...
visibility: public | private (Team+)
notify_subscribers: true
auto_close_on_recovery: true   # 监控恢复时自动关闭
```

### 5.3 事件状态流转

```
investigating → identified → monitoring → resolved
      ↓ (可跳)         ↓ (可跳)         ↓
   resolved        resolved          postmortem
```

### 5.4 维护事件
- 计划维护是特殊事件类型
- 提前创建（不立即可见 / 公示）
- 维护窗口期间状态自动转 Under Maintenance
- 窗口结束自动转 resolved（或保持，由维护者决定）

### 5.5 自动事件创建（监控驱动）
- 监控触发告警 → 可自动创建 / 不创建（用户配置）
- 自动创建时模板可配（标题、初始 body）
- 监控恢复后可自动 resolve

### 5.6 事件更新（Postmortem）
- 事件解决后，维护者可发布"事故复盘"
- Markdown 编辑器、附图
- 复盘单独可见的二级页
- **v2 NEW: LLM 自动起草工作流(详 §5.8)**

### 5.7 自动事故时间线(v2 NEW)

> 事件创建后,系统自动写入结构化时间线;减少维护者"手动写每一条更新"的负担。

#### 自动时间线条目类型

| 来源 | 时间线条目示例 |
|---|---|
| 监控告警 | "14:30 监控 [官网首页] 触发异常,5/5 节点失败" |
| 节点恢复 | "14:50 美国节点全部恢复,国内节点仍异常" |
| 路由变化 | "14:35 检测到 AS4837 路由抽风,影响国内访问"(配合 traceroute 监控) |
| 关联事件 | "14:32 同一区域内 [API 服务] 也触发异常,可能同源" |
| 处理动作 | "14:35 [user_xxx] Ack 此事件"(从 04 alert_event Ack 同步) |
| 监控恢复 | "15:08 所有节点恢复" |

#### 自动 → 手动覆盖
- 用户可在每个自动时间线条目下"添加备注 / 重写 / 隐藏"
- 维护者可手动插入"信号性"条目(如"已联系 CDN 厂商工单 INC-12345")
- 自动条目带有"自动" badge,与手动条目视觉区分

### 5.8 LLM 自动起草工作流(v2 NEW, 决策 §K4 配套, 与 07 §6 联动)

> 事件触发后,系统**同时生成两个 LLM 起草草稿**:
> 1. **对外公告草稿**:发到状态页 + 邮件订阅者 + 微信 / 钉钉(短 / 礼貌 / 不带技术细节)
> 2. **内部复盘草稿**:详 07 §6,给 SRE 团队内部 review(长 / 技术细节 / 根因建议)
>
> 两个草稿都**强制人工审核**,AI 标识 + sanitize + 离线 eval ≥ 4.0/5 才允许新 prompt 上线。

#### 公告草稿生成时机

```
事件创建(status=investigating)
    │
    ▼  T+5m: 等待事件结构稳定(避免立即起草后又改)
    │
    ▼
LLM 起草"对外公告":
    - 一句话描述影响("我们正在调查影响美国西部访问的网络异常")
    - 礼貌措辞(不甩锅 / 不断言责任方)
    - 包含"持续更新"承诺 + 后续更新频率(每 30 分钟)
    │
    ▼
草稿存到 status_incident.public_announcement_draft 字段
status=ai_drafted
    │
    ▼
维护者收邮件 + 站内通知:"AI 已为你起草事故公告,请审核 → 发布"
    │
    ▼
[强制人工审核]
    维护者编辑 → 一键发布到状态页 + 推送订阅者
    或:维护者勾"自动发布"(仅高级用户,且开关默认 OFF) → 跳过审核直发(不推荐)
```

#### Prompt 约束(避免幻觉)

- 输出严格 Markdown(无 HTML)
- 长度 50-300 字符(短公告)
- **禁止**:断言责任方 / 提及具体人名团队 / 承诺具体恢复时间 / 提供赔偿数字
- **必须**:致歉 / 影响范围 / 持续更新承诺 + 频率 / 联系方式

#### 离线 eval

- 每月 50 个真实事故公告,人工打分 ≥ 4.0/5 才允许新版 prompt
- 数据集:用户实际发布的 vs LLM 起草的对比;评估"措辞专业性 / 信息完整性 / 措辞礼貌性 / 无幻觉"

#### 与 07 §6 内部复盘的关系

- 公告草稿(短 / 对外)和复盘草稿(长 / 内部)**同一 LLM 调用**,不同 prompt 模板
- 公告 在 T+5m,复盘 在事件 resolved 后 T+5m
- 公告发布后变更不可逆(发出去就出去了);复盘可反复编辑直到 published

---

## 6. 自定义品牌

### 6.1 视觉自定义（Pro 起）

| 项 | 配置 |
|---|---|
| Logo | 上传 PNG/SVG，含 dark mode 版 |
| Favicon | 上传 |
| 主色 | HEX 选色 |
| 字体 | 系统字体 / Google Fonts / 自定义 webfont URL |
| Hero 区背景 | 纯色 / 渐变 / 图片 |
| 顶部 / 底部自定义 HTML | 嵌入分析、客服 Widget |
| 自定义 CSS | 高级用户用 |
| 移除 `Powered by` | ✅（Pro 起） |

### 6.2 内容自定义

- Title / Slogan / Description（含 SEO meta）
- 自定义链接（导航栏放官网 / 支持 / 文档链接）
- 联系方式（邮件 / 链接）
- 多语言版本（中/英，S2 中文；S3 加英文）

### 6.3 域名自定义（Pro 起）

#### 流程
1. 用户在控制台输入 `status.example.com`
2. 系统生成 CNAME 目标：`<slug>.status.idcd.com`
3. 用户去自己 DNS 商配 CNAME
4. 系统轮询验证 → 验证通过
5. 自动签发 Let's Encrypt 证书（ACME HTTP-01）
6. 自动续期（每 60 天 / 提前 30 天）

#### 边界
- 仅支持单个自定义域 per 状态页（Pro/Team）；Business 可多个
- 必须是用户实际拥有的域名（DNS 控制权）
- 不允许指向 `*.tld` 的根域名（CNAME 限制）

---

## 7. 订阅与通知

### 7.1 订阅方式
- **邮件订阅**：访客提供邮箱 → 双向确认（防滥用）→ 重大事件邮件
- **RSS / Atom**：自动生成 feed URL
- **Webhook**：访客提供 URL（高级，仅 Team+ 用户的状态页支持）
- **微信 / 钉钉 / 飞书订阅**（S3）：访客扫码绑定，企业内部通知

### 7.2 通知策略
- 默认仅 `major` / `critical` 事件通知（避免轻微事件骚扰订阅者）
- 维护事件可选通知
- 复盘发布可选通知

### 7.3 退订
- 每封邮件底部退订链接
- 一键退订所有状态页

---

## 8. 嵌入与 API

### 8.1 嵌入组件
- **Status Badge**：`<a href="..."><img src="https://api.idcd.com/badge/<slug>"></a>`
- **JS Widget**：嵌入小型状态指示器（一行）
- **iframe**：完整状态页嵌入

### 8.2 公开 API（S2 启用）

| Endpoint | 说明 |
|---|---|
| `GET /v1/status/<slug>/summary` | 整体状态摘要 |
| `GET /v1/status/<slug>/components` | 服务列表 |
| `GET /v1/status/<slug>/incidents` | 事件列表 |
| `GET /v1/status/<slug>/uptime?days=90` | 可用率数据 |

- 公开访问无需鉴权
- 限速：IP 维度 60 次/分钟

### 8.3 集成
- Slack / Discord：状态变化自动同步到群（外部用户可关注的状态）
- IM 平台插件（S3）

### 8.4 实时观众面板(v2 NEW, S3)

> 重大事故场景:公司 CEO + 客服 + 工程师 + 销售各自截图微信群,场面混乱。**给一个共享实时大屏**。

#### 入口与权限
- 控制台 `/app/status-pages/<id>/live-room`
- 团队级功能(Team / Business / Compliance Enterprise);Free / Pro 不可用
- 临时密码访问(URL + 6 位密码,密码 1 小时过期 + 可续期)— 让非团队成员(如外部 CDN 厂商工程师)也能进
- 最多 50 人同时在线观看

#### 显示内容(实时)
- 当前事件状态 + 影响时间累计计时器
- 多节点拨测实时结果热力图(每 10 秒刷新)
- 关键监控指标实时折线图(可用率 / 响应时间 / 错误率)
- 聊天侧栏:观众可发消息(类弹幕,不打扰主信息)
- "处理人当前在做什么"标签(`Ack by`、`investigating`、`escalated to vendor` 等状态切换实时同步)

#### 推送机制
- WebSocket 长连接(后端 02 §7.1 同栈)
- 每 5 秒推一次数据快照(避免过细)
- 离线超 30 秒自动重连
- 移动端响应式优化(SRE 手机看)

#### 数据隐私
- 实时观众面板**不展示内部技术细节**(数据库 query / 内部 service 名);只展示用户视角的影响
- 聊天历史保存 24 小时后自动删除
- 临时密码访问者的 IP / 操作 留审计 30 天

#### 转化锚点
- 重大事故复盘时,实时观众面板的截图 / 录屏可作为"展示给客户/CEO 的素材"
- "把这次事故生成 Verdict 报告 →"(关联 18 模块)

---

## 9. 信息架构（IA）

### 9.1 控制台页面

```
/app/status-pages
  列表 + 新建按钮

/app/status-pages/<id>
  ├── /dashboard       概览（当前状态 + 当月可用率）
  ├── /components      服务（组件）管理
  ├── /incidents       事件管理(含 LLM 起草 + 自动时间线)
  ├── /maintenance     计划维护
  ├── /subscribers     订阅者管理
  ├── /design          视觉自定义
  ├── /domain          自定义域名 + SSL
  ├── /settings        基本设置 + 隐私
  ├── /analytics       状态页访问统计
  └── /live-room       v2 NEW: 实时观众面板(Team+)
```

### 9.2 公开 URL

```
<slug>.status.idcd.com                  默认子域
status.<userdomain>.com                    自定义域
/incidents/<id>                            单个事件页
/incidents/<id>/postmortem                 复盘页
/uptime                                    历史可用率详情
/subscribe                                 订阅页
/api                                       API 文档跳转
```

---

## 10. SEO 与可发现性

### 10.1 默认子域 SEO
- 每个状态页是独立子域，可被搜索引擎收录
- `<title>` `<meta description>` 用户可配置
- 自动生成 sitemap.xml
- 历史事件作为内容沉淀（"<品牌> 故障 5月 12日" 等长尾词）

### 10.2 反向带量
- 公开状态页带 `Powered by idcd` 链接（Free 档）
- 累计每个状态页都是 SEO 反链

### 10.3 OG 卡片
- 自动生成状态卡片（状态色 + 标题 + 当前状态）
- 用户分享到微博 / Twitter 自动展示

---

## 11. 数据模型

> v2 NEW:status_incident 增加 LLM 起草字段 / 自动时间线引用;新增 status_live_room / status_live_room_session 两张表(实时观众面板)。
> v2 字段详见 15-data-model.md(可与 04 / 07 模块共享 LLM 起草字段命名)

```
status_page
  id, owner_id, slug, name, description,
  default_domain (slug.status.idcd.com),
  custom_domain, custom_domain_verified_at,
  cert_status, cert_expires_at,
  visibility (public|password|private),
  password_hash,
  design (jsonb: logo, colors, fonts, css...),
  watermark_enabled (true/false 按订阅档),
  i18n (jsonb: zh, en),
  created_at, updated_at

status_component
  id, status_page_id, section_id,
  name, description, position,
  source_type (monitor|manual|api),
  monitor_ids (uuid[]),
  aggregation_rule, current_status,
  last_changed_at

status_section
  id, status_page_id, name, position

status_incident
  id, status_page_id, title, status, impact,
  affected_components (uuid[]),
  visibility, notify_subscribers,
  auto_close_on_recovery,
  source (auto|manual|api),
  related_alert_event_id,
  started_at, resolved_at, postmortem_published_at

status_incident_update
  id, incident_id, status, body (markdown),
  posted_by, posted_at

status_maintenance
  id, status_page_id, title, body,
  scheduled_start, scheduled_end,
  actual_start, actual_end,
  affected_components, notify_subscribers

status_subscriber
  id, status_page_id, channel (email|webhook|rss|wecom...),
  contact (email|webhook url|...),
  verified_at, subscribed_at,
  notify_on_minor, notify_on_maintenance

status_page_uptime_daily
  status_page_id, component_id, date,
  total_seconds, up_seconds, degraded_seconds, down_seconds,
  events_count
```

---

## 12. 关键流程

### 12.1 用户首次创建状态页

```
User → /app/status-pages → 新建
  → Step 1：基础（名称 + slug + 描述）
  → Step 2：添加服务（从已有监控中选择 N 个）
  → Step 3：分组
  → Step 4：视觉（Logo + 主色，可跳过用默认）
  → Step 5：预览
  → 完成 → 跳转到公开链接
```

### 12.2 自动事件创建流程

```
Monitor 触发 DOWN 事件
  → 检查 monitor 是否关联到 status component
  → 检查 component 的"自动事件"配置
  → 命中规则：自动创建 status_incident
      • status = investigating
      • impact = major/minor (按 severity 推断)
      • title = 模板生成
      • affected_components = [comp]
  → 通知订阅者（如配置）
  → 状态页实时刷新

Monitor 恢复
  → 自动 resolve incident
  → 通知订阅者恢复
```

### 12.3 自定义域 + SSL 流程

```
User 在域名页输入 status.example.com
  → 系统返回 CNAME 目标：abc.status.idcd.com
  → User 在 DNS 配 CNAME
  → 系统每 30s 查询验证（最多 24h）
  → 验证成功
  → 触发 ACME HTTP-01 / DNS-01 流程申请证书
  → 证书签发完成
  → Nginx / Caddy 自动加载证书
  → 用户访问 https://status.example.com 直接命中
```

---

## 13. 可用率计算

### 13.1 计算口径
- **窗口**：30 / 90 / 365 天
- **粒度**：原始数据按 60s 切片，每片"up" / "degraded" / "down"
- **公式**：`uptime = 1 - down_seconds / total_seconds`

### 13.2 维护窗口计入吗？
- 默认：**不计入分母**（"忽略维护时间"，更友好的可用率）
- 用户可切换："计入分母"（更严格）

### 13.3 SLA 报告（S3）
- 月度 / 季度自动生成
- 含 SLA 承诺（如 99.95%）、实际可用率、差额
- 可分享 / 导出 PDF

---

## 14. 状态页访问数据分析

### 14.1 流量看板（控制台内）
- 状态页 PV / UV（每日 / 每月）
- 来源（直接 / 搜索 / 引用）
- 设备分布
- 高频访问时间段

### 14.2 集成 Google Analytics / 百度统计
- 用户可填自己的 GA / 百度统计 ID
- 自动注入埋点

---

## 15. 与其他模块的接口

| 模块 | 接口 |
|---|---|
| `04-monitoring.md` | 服务关联监控 + 状态聚合 |
| `05-alerting.md` | 事件可关联到告警事件（同一事件库） |
| `08-open-api.md` | 状态页公开 API + 控制 API |
| `09-billing.md` | 配额校验（状态页数、自定义域、订阅人数） |
| `12-compliance-and-abuse.md` | 自定义域名 / 子域名审核（禁止违规内容） |
| `13-content-and-seo.md` | 状态页本身的 SEO 策略 |

---

## 16. 阶段交付清单

### S2（4–8 月）
- 公开状态页（默认子域 + 自定义域）
- 服务 + 分组 + 状态聚合
- 事件管理 + 自动事件 + 复盘
- 计划维护
- 视觉自定义（Pro 起）
- 邮件 / RSS 订阅
- Status Badge + 基础 API
- 默认水印（Free） / 去水印（Pro 起）
- **v2 NEW: 自动事故时间线(监控告警 / 节点恢复 / 路由变化自动写入)**
- **v2 NEW: LLM 公告草稿自动生成 + 强制人工审核 + AI 标识**
- **v2 NEW: 公告 / 复盘双轨 LLM 起草(对外公告 + 内部复盘,与 07 §6 联动)**

### S3（8–14 月）
- 多区域视图
- 私有 / 密码保护状态页
- Webhook / 微信 / 钉钉订阅
- 嵌入 Widget
- 月度 SLA 报告
- 集成 Slack / Discord
- 状态页访问数据看板
- **v2 NEW: 实时观众面板(Team+)— 共享实时大屏 + 临时密码访问外部观众**

### S4（14+ 月）
- 白标版（去掉所有 `idcd` 痕迹）
- 多状态页统一管理（Business+）
- 企业级私有部署版

---

## 17. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 自定义域名 CNAME 配置错被滥用（指向钓鱼内容） | 域名实名 + 内容自动扫描 + 用户协议 |
| Free 用户大量创建状态页占用资源 | 每用户 1 个 + Slug 唯一抢占 |
| 状态页 SEO 关键词与正主网站冲突 | 默认 `<noindex>` 可选，子域有限收录 |
| 自动事件创建被误触发污染状态页 | 默认关闭自动事件，引导用户主动开启 |
| 自定义域 SSL 签发失败循环 | 自动重试 + 失败告警 + Fallback 到默认子域 |
| **v2 NEW: LLM 公告草稿被维护者直接发布造成幻觉/谩骂** | 强制人工审核(自动发布默认 OFF)+ AI 标识 + sanitize + 离线 eval ≥ 4.0/5 |
| **v2 NEW: 实时观众面板被滥用做"网络战观战"** | Team+ 限制 + 临时密码 + 50 人上限 + 不展示内部技术细节 |
| **v2 NEW: 自动事故时间线写错关联事件** | 关联置信度阈值;低置信不自动加入,推荐到"建议关联"等待维护者确认 |

---

## 18. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **G5** 状态页默认子域：**`<slug>.status.idcd.com`**
- ✅ **A5** Free 档 watermark：**页脚文字 "Powered by idcd"**（带链接，不含 Logo）
- ✅ **A8** 维护窗口：**默认不计入可用率分母**

### v2.0 (K 节, 2026-05-12)
- ✅ **K4** LLM 公告/复盘自动起草:S2 上线;强制人工审核 + AI 标识 + 离线 eval ≥ 4.0/5
- ✅ **K-status 自动事故时间线**:监控告警 / 节点恢复 / 路由变化 自动写入,关联置信度阈值
- ✅ **K-status 实时观众面板档位**:Team / Business / Compliance Enterprise;Free / Pro 不可用

### 待定（不紧迫）

- [ ] 邮件订阅双向验证（防爬虫批量注册）：建议默认开启
- [ ] 复盘文档：独立 URL（已采用默认）
- [ ] **v2 NEW** 公告"自动发布"是否完全禁止(目前默认 OFF):S2 末根据用户反馈决
