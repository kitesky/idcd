# 11 · 管理后台(v2)

> v2 新增:/attest(Verdict 工单 + KMS 密钥仪式 + Verdict 申诉 + TSA 健康)/ /mcp(MCP 运维)/ /leaderboard(排行榜后台 + 厂商退出申请)/ /nodes/revoke + /nodes/anchor-divergence(节点失窃应急 + Anchor 偏差监控)
> v2 关键 SOP:Verdict 工单兜底(§15.4) / KMS 密钥仪式(§15.5) / 排行榜厂商退出(§15.6) / 运营工具

> 关联：OVERVIEW.md §4.10、12 合规与审计
> 阶段主体：S1 节点 + 反滥用核心后台；S2 完整后台 + 数据看板；S3+ 运营增强
> 品牌名占位：`idcd`
> 访问域名：`admin.idcd.com`（内网/堡垒机后）

---

## 1. 模块定位

管理后台是 `idcd` 的**指挥舱**。直接影响：

1. 节点稳定性（运维侧）
2. 反滥用响应速度（安全侧）
3. 客户体验（客服 / 退款）
4. 经营决策（数据看板）
5. 内容运营（公告、Banner、博客）

### 设计原则

1. **不放公网**：必须通过 VPN / 堡垒机访问
2. **最小权限**：每个角色仅看必要数据
3. **全审计**：所有操作有日志，敏感操作双人审批
4. **简洁高效**：信息密度高，操作可批量
5. **支持移动应急**：值班场景需移动可用（简化版）

### 关键指标

| 指标 | S1 | S2 | S3 |
|---|---|---|---|
| 节点故障 → 自动剔除时间 | ≤ 90s | ≤ 60s | ≤ 30s |
| 滥用工单首次响应 | ≤ 24h | ≤ 12h | ≤ 4h |
| 退款工单平均处理 | — | ≤ 24h | ≤ 8h |
| 后台关键页面 P95 | ≤ 1s | ≤ 800ms | ≤ 500ms |

---

## 2. 角色与权限（与 12 合规模块对应）

| 角色 | 关键权限 | 双人审批 |
|---|---|---|
| **Super Admin** | 全部 | 关键变更需 |
| **Tech Admin** | 节点 / 调度 / 系统配置 | 系统配置变更 |
| **Security Officer** | 黑名单 / 滥用 / 用户封禁 | 用户大批封禁 |
| **Customer Success** | 工单 / 退款（< ¥1000）/ 通知用户 | 大额退款 |
| **Finance** | 财务 / 发票 / 退款审核 | 大额操作 |
| **Content Editor** | 博客 / 公告 / Banner / 文档 | 涉及付费策略 |
| **Read-only** | 数据看板 / 列表只读 | 无 |
| **Customer Sales**（S4） | 企业客户管理 | 合同变更 |

---

## 3. 后台导航结构

```
admin.idcd.com
├── /                       全局仪表盘
├── /users                  用户管理
│   ├── /                   列表
│   ├── /<id>               详情
│   ├── /<id>/activity      活动日志
│   ├── /<id>/security      安全（重置密码、踢下线、关 2FA）
│   ├── /<id>/team          其所在团队
│   └── /<id>/impersonate   "扮演"登录该用户（看其视角）
├── /teams                  团队管理
├── /subscriptions          订阅与计费
├── /orders                 订单管理
│   ├── /refunds            退款管理
│   └── /invoices           发票管理
├── /tickets                工单中心
│   ├── /support            一般支持
│   ├── /abuse              滥用举报
│   ├── /security           安全事件
│   └── /verdict            v2 NEW: Verdict 生成异常工单兜底
├── /monitors               用户监控（查 / 排障）
├── /nodes                  节点管理
│   ├── /                   列表
│   ├── /<id>               详情
│   ├── /enrollment         入网申请审核
│   ├── /community          众包节点（S3）
│   ├── /health             节点健康看板
│   ├── /tasks              任务队列监控
│   ├── /revoke             v2 NEW: 紧急吊销节点证书(失窃应急)
│   └── /anchor-divergence  v2 NEW: Anchor 偏差实时监控 + 自动剔除日志
├── /abuse                  反滥用控制台
│   ├── /denylist           黑名单管理
│   ├── /rate-limit         限速规则
│   ├── /reports            举报队列
│   └── /watchlist          观察名单
├── /audit                  审计日志
│   ├── /admin              管理员操作
│   ├── /system             系统事件
│   └── /export             审计导出
├── /attest (v2 NEW)        Evidence / Attestation 运维
│   ├── /verdict-tickets    Verdict 生成失败工单队列
│   ├── /verdict-disputes   被报告对象的申诉队列
│   ├── /key-ceremony       KMS 密钥仪式后台(3-of-5 quorum 操作)
│   ├── /key-events         密钥操作审计(只读)
│   ├── /tsa-health         TSA 健康度 + 切换日志
│   └── /self-verify-fails  自检失败队列(P0)
├── /mcp (v2 NEW)           MCP server 运维
│   ├── /tokens             所有 MCP token 列表(可强制撤销)
│   ├── /sessions           活跃 session(可强制断连)
│   ├── /usage-anomalies    异常突增告警队列(疑似失窃)
│   └── /tool-stats         tool 调用统计 + 客户端兼容矩阵
├── /leaderboard (v2 NEW)   CDN/云排行榜运维
│   ├── /current            本月草稿(发布前 review)
│   ├── /history            历史月报告
│   ├── /optout-requests    厂商退出申请队列
│   ├── /methodology        测试方法学版本管理
│   └── /vendor-feedback    厂商反馈通道
├── /content                内容运营
│   ├── /blog               博客文章
│   ├── /docs               帮助中心
│   ├── /announcements      公告
│   ├── /banners            首页 / 控制台 Banner
│   └── /seo                SEO 工具页文案 + meta
├── /system                 系统配置
│   ├── /plans              订阅档位 / 价格
│   ├── /coupons            优惠券
│   ├── /referral           推广返利策略
│   ├── /quotas             配额阈值
│   ├── /channels           告警通道（adapter 配置）
│   └── /flags              功能开关 / 灰度
├── /analytics              数据分析
│   ├── /growth             增长（注册、付费）
│   ├── /revenue            收入
│   ├── /usage              用量（API、监控）
│   ├── /retention          留存
│   └── /cohort             同期群
└── /tools                  内部工具
    ├── /debug              问题排障器
    ├── /db-query           SQL 查询（审计 + 双人）
    └── /scripts            一次性脚本运行（S3）
```

---

## 4. 用户管理

### 4.1 列表
- 列：ID / 邮箱 / 注册时间 / 订阅 / 状态 / 累计消费 / 最近登录 / 操作
- 筛选：状态 / 订阅档 / 注册渠道 / 标签 / 异常标记
- 搜索：邮箱 / username / 手机号 / 订单号 / API Key 前缀
- 批量：导出 / 标签 / 公告群发

### 4.2 详情页
- 基础信息（已实名 / 邮箱 / 手机 / 团队）
- 订阅与账单概览
- 监控总数 / 当前异常
- API Key 列表
- 最近活动日志
- 登录记录 + 异常登录
- 工单历史

### 4.3 安全操作
- 强制重置密码
- 强制踢下线（吊销所有 session）
- 关闭 2FA（紧急救助场景）
- 标记为可疑（额外审核）
- 临时停用 / 永久封禁
- 用户注销（紧急情况强制走流程）

### 4.4 客服扮演（Impersonate）
- 极敏感操作：仅 Customer Success 高级 + Super Admin 可用
- 进入"以该用户身份"的只读视图
- 全程记录审计（含每个页面访问）
- 用户控制台同时显示"管理员正在协助你"提示
- 用户可主动关闭"允许扮演"开关（默认开）

### 4.5 注释 / 标签
- 用户标签：VIP / 风险 / 学生 / 企业 / 流失风险
- 内部备注（不可见给用户）

---

## 5. 节点管理（核心运维）

### 5.1 节点列表
- 列：ID / 类型 / 国家 / 城市 / ASN / Tier / 状态 / 当前负载 / 24h 成功率 / Agent 版本
- 颜色：状态色 + tier 色（Tier1 蓝 / 2 灰 / 3 黄）
- 筛选：类型（owned / community / dedicated）、状态、地区、tier
- 批量：drain / 重启 / 升级 / 改 tier / 改 tag

### 5.2 节点详情
- 完整元数据（含 IP、ASN、提供商）
- 健康曲线（24h / 7d / 30d）
- Anchor 基准对比图
- 当前在执行的任务列表
- 最近 100 条日志
- 配置项（任务并发、能力开关）
- 操作：drain / reactivate / restart agent / 升级 / disable

### 5.3 入网申请
- 自有节点：管理员部署后自动接入，无需审核（但首次进入待审 24h 内）
- 众包节点：人工审核 + 自动检测
- 审核界面：节点信息、基准测试结果、用户信息、ASN 合规性

### 5.4 任务队列监控
- 当前队列长度
- 各优先级队列分布
- 节点维度的任务积压
- 慢任务（执行时间 > N 秒）排行
- 失败率高的节点 / 目标排行

### 5.5 实时调试
- 选一个节点 → 下发测试任务（如 ping anchor）
- 看实时返回
- 用于"节点是真的挂了还是只是高延迟"判断

---

## 6. 反滥用控制台

### 6.1 黑名单管理

#### 类别（与 12 §3.1.A 对应）
- 技术性黑名单（不可编辑，仅查看）
- 敏感目标黑名单（可编辑，需双人）
- 动态黑名单（可编辑）
- 用户白名单（每个用户可看到）

#### 操作
- 添加 / 删除 / 修改
- 批量导入（CSV / 第三方情报）
- 历史记录（谁加的、什么时候、原因）
- 测试：输入 target 看是否命中

### 6.2 限速规则
- 默认规则（按订阅档）
- 用户级覆盖（针对特定用户调高 / 调低）
- 目标级覆盖（特定 host 全站限速）
- API 级覆盖（特定 endpoint）
- 实时生效（无需重启）

### 6.3 举报队列
- 工单列表（按紧急度排序）
- 详情：举报内容、关联资源、自动分析结果
- 处理操作：
  - 关闭（假举报 / 已处理）
  - 警告用户
  - 临时停止用户活动
  - 永久封禁 + 退款
  - 加入黑名单
- 处理记录：双人审批 if 大批量

### 6.4 观察名单
- 高频测试目标自动入列
- 短期内多 IP 测试同一目标 → 告警
- 人工审核是否升级到黑名单

### 6.5 风险评分（S3）
- 用户级风险分（基于行为、注册渠道、IP 信誉、目标分布）
- 高风险自动加大 Turnstile 校验、限速更严
- 低风险给更大额度

---

## 7. 订单与计费管理

### 7.1 订单列表
- 列：订单号 / 用户 / 类型 / 金额 / 通道 / 状态 / 时间
- 筛选 + 搜索 + 导出
- 总金额、当日 / 当月对账

### 7.2 退款审核
- 列表：退款申请 / 状态 / 金额 / 原因
- 详情：订单关联、用户历史、首次/中途
- 操作：同意 / 拒绝 / 部分退
- 大额（> ¥1000）：双人审批
- 退款失败重试 / 改通道

### 7.3 发票审核
- 用户提交开票 → 自动审核（一般）/ 人工审核（专票、海外）
- 人工干预红冲 / 重开

### 7.4 余额管理
- 充值订单审核（对公转账等）
- 异常扣减回滚

### 7.5 配额调整
- 单用户临时调高（客户关怀场景）
- 临时折扣 / 赠送配额

### 7.6 财务对账
- 每日自动对账：通道流水 vs 内部订单
- 不一致自动告警

---

## 8. 工单中心

### 8.1 工单类别

| 类别 | 处理人 | SLA |
|---|---|---|
| 一般支持 | Customer Success | 24h 首次响应 |
| 计费问题 | Finance / CS | 24h |
| 退款 | Finance / CS | 48h 决策 |
| 滥用举报 | Security Officer | 24h 首次 |
| 安全事件 | Security Officer | 1h |
| 企业客户 | Customer Sales（S4） | 4h |

### 8.2 工单流程
- 用户提交 → 自动归类 + 编号
- 自动关联用户信息、订单、监控
- 客服回复 → 邮件 + 站内通知用户
- 状态：open / waiting_user / resolved / closed
- SLA 计时
- 客户满意度评分（resolve 后）

### 8.3 客服工具
- 富文本回复 + 模板（常见问题预设回复）
- 内部备注（不可见用户）
- 协作（转交、@ 同事）
- 知识库自动推荐相关文档

### 8.4 工单分析
- 各类工单数量、平均处理时长、满意度
- 高频问题识别 → 推动 FAQ / 产品改进

---

## 9. 内容运营

### 9.1 博客
- 文章 CRUD（Markdown 编辑器）
- 分类 / 标签 / 作者 / 封面
- 草稿 / 发布 / 定时发布
- 评论审核（如开）
- SEO 字段（title、description、og 图）
- 多语言版本

### 9.2 帮助中心
- 文档树（章节 + 文章）
- 版本管理（产品多版本时）
- 搜索（含站内搜索引擎）
- 用户反馈（"这篇有帮助吗"）

### 9.3 公告
- 全站公告 / 控制台公告 / 邮件公告
- 目标用户：所有 / 付费 / 特定档位 / 标签
- 时效（开始 + 结束）
- 显示位置（顶部 Banner / 弹窗 / 站内消息）

### 9.4 Banner / 横幅
- 首页营销位
- 控制台促销位
- 时段控制
- A/B 测试（S3）

### 9.5 SEO 工具页文案
- 每个工具页的 description / FAQ / 应用场景独立管理
- 中英双语
- 历史版本对比

### 9.6 邮件模板
- 注册欢迎 / 验证邮件 / 告警邮件 / 账单邮件 / 续费提醒
- 模板化 + 变量
- A/B 测试（S3）

---

## 10. 系统配置

### 10.1 订阅档位
- 价格、配额、限制、可见性
- 改价格历史记录
- 变价的影响（已订阅用户不受影响 / 新订阅生效）

### 10.2 优惠券
- 类型（百分比 / 固定金额 / 永久 / 限次）
- 适用范围（用户群 / 档位 / 周期）
- 有效期、领取上限、使用上限
- 推广链接

### 10.3 推广返利配置
- 返利比例 / 现金奖励
- 反作弊规则

### 10.4 配额阈值
- 全局：单用户 RPS、并发任务等

### 10.5 告警通道 Adapter 配置
- 第三方密钥（短信、SES）
- 公众号 token / appsecret
- 这些密钥在 KMS，后台仅展示状态 + 测试发送

### 10.6 功能开关 / 灰度（Feature Flags）
- 灰度规则：% / 用户 ID / 团队 / 地区
- 立即生效
- 用于：新功能灰度、紧急下线某能力

---

## 11. 数据看板（经营分析）

### 11.1 增长看板
- 注册数（日 / 周 / 月）
- 注册来源（直接 / 搜索 / 推荐 / SEO 工具页）
- 激活率（注册→首个监控）
- 漏斗：访问 → 注册 → 首次行为 → 付费

### 11.2 收入看板
- MRR / ARR / 当月新增
- 流失率 / 净收入留存（NRR）
- 各档位收入分布
- 月付 vs 年付比例
- 渠道：直接 / 推荐 / 优惠码

### 11.3 用量看板
- 总监控数 / 总拨测次数 / 总告警数
- API 调用量 / 各 endpoint TOP
- 节点总负载

### 11.4 留存 / 同期群
- 注册队列的 7d / 30d / 90d 留存
- 付费用户 6 / 12 月留存

### 11.5 SaaS 健康度指标
- CAC（如有投放）
- LTV
- LTV / CAC 比
- 平均订单价（ARPU）
- Churn Rate

### 11.6 实时仪表
- 在线用户数
- 当前 API RPS
- 当前节点状态
- 当前任务队列长度

### 11.7 自定义查询（高级）
- SQL 查询（需 Super Admin / 数据分析角色）
- 内置常用模板
- 查询审计

---

## 12. 审计日志

### 12.1 管理员操作日志
- 谁、什么时候、对什么资源、做了什么操作、前后值
- 关键操作：用户封禁、退款、配额调整、黑名单变更、系统配置

### 12.2 自动审计
- 双人审批未完成的操作记录
- 异常行为（连续大量操作、非工作时间操作）告警

### 12.3 日志保留
- 6 个月起步（合规要求）
- 安全相关 1 年起
- WORM 存储（不可篡改）

### 12.4 日志导出
- 用于合规审查、年度审计
- 司法配合时按需提供

---

## 13. 安全与访问控制

### 13.1 访问路径
- VPN（员工连入企业网络）
- 堡垒机（额外跳板）
- 后台域名仅内网解析（或公网但 IP 白名单）

### 13.2 认证
- 强制 2FA（TOTP + WebAuthn）
- 短期 session（4h）
- 异常登录检测

### 13.3 操作录屏（高级）
- 关键操作页面自动录屏（仅服务端，无客户端代理）
- 用于复盘 / 审计

### 13.4 离职流程
- 立即吊销账号 + API Key + VPN
- 关联资源转交
- 审计快照

---

## 14. 数据模型（管理后台特有）

```
admin_user
  id, email, role, status,
  totp_secret, webauthn_credentials,
  last_login_at, created_at

admin_session
  id, admin_user_id, ...

admin_audit_log
  -- 见 12 §9.2

ticket
  id, type (support|abuse|security|billing|refund),
  user_id, subject, body,
  status (open|waiting_user|resolved|closed),
  assignee_admin_id,
  priority, sla_due_at,
  created_at, resolved_at, satisfaction_score,
  metadata (jsonb)

ticket_message
  id, ticket_id, author (user|admin), body,
  is_internal_note, attachments

feature_flag
  id, key, description, rules (jsonb),
  enabled, created_at, updated_at

announcement
  id, type (global|console|email),
  audience (all|paid|tags|...),
  title, body, banner_image,
  starts_at, ends_at, created_by

approval
  id, action_type, requested_by, target,
  reason, status (pending|approved|rejected),
  approver_admin_id, approved_at,
  executed_at, original_payload (jsonb)
```

---

## 15. 关键流程

### 15.1 滥用举报处理

```
工单到达 abuse 队列
  → 自动归类 + 编号
  → 系统自动关联：
       • 涉及的 user_id / target / report_id
       • 相关历史
       • 自动检测特征（黑名单匹配等）
  → 分配给 Security Officer
  → SO 审核（24h SLA）
  → 决策：
      a) 假举报：关闭 + 通知举报人
      b) 一般违规：警告用户 + 记录
      c) 严重违规：立即停止该用户活动
         → 需 SO + 另一管理员 双人审批
         → 用户 status -> suspended
         → 退款（如适用）
         → 邮件通知用户原因
      d) 涉嫌违法：留档 + 法务介入
  → 通知举报人结果
  → 留档 6 个月
```

### 15.2 退款流程

```
用户提交退款（控制台）
  → 7 天内首次订阅：系统自动通过 + 退原通道
  → 否则：进入工单
  → CS / Finance 审核
  → 同意：
      • < ¥1000：直接处理
      • ≥ ¥1000：等另一管理员审批
      → 调通道 refund API
      → 余额扣回 / 订阅终止
      → 邮件通知
  → 拒绝：邮件说明原因
```

### 15.3 大批量用户操作（如批量送 30 元优惠券）

```
Marketing Admin → 创建批量任务
  → 选目标用户（标签、注册时间、订阅档）
  → 配置内容
  → 提交审批
  → Super Admin / Finance 审批（涉及成本）
  → 任务异步执行
  → 完成后报告（成功数、失败原因）
```

### 15.4 Verdict 工单分类与三档 SLA(v2 NEW + D5/D12)

> 详 09 §13.5 + 18 §3.2;Verdict 是付费产品,生成失败 = 品牌灾难。**v2 D12 锁定 3 档 SLA**,纯自动化 / 1h 仅 P0 / 24h 常规,适配 1 人创业现实。

#### 工单分类 SLA 表

| 工单类别 | 触发条件 | SLA | 处理方 |
|---|---|---|---|
| **A:纯自动**(Verdict 生成失败) | Generator 3 次重试失败 / Self-Verify failure | **30min 内自动**:聚合支付 refund + 道歉邮箱(无需人工) | Refund Worker(无人工)|
| **B:Refund 失败**(D5) | 聚合支付 refund retry 3 次仍失败 → `refund_failed` 状态 | **道歉邮箱已发** + admin dashboard 入队 + P0 告警创始人 | 创始人手动联系聚合服务商客服 / 银行 |
| **C:1h P0 本人响应** | KMS 怀疑泄露 / 节点失窃 / 系统性失败(≥5 失败/小时) | **1h 内创始人响应**(手机告警 7×24) | 创始人本人 |
| **D:24h 常规客服** | 用户自助退款被拒 / 用户报障 / 申诉 / 一般咨询 | **24h 内**(出差/假期都可) | Operator / 创始人邮件批处理 |

#### 类别 A:Verdict 生成失败自动流程(D5)

```
T+5min Generator 3 次自动重试都失败 / Self-Verify failure → DLQ
       → 系统自动:
         - verdict_order(status=failed)
         - Refund Worker 启动(详 09 §13.5)
         - 不创建人工工单(纯自动)

T+10min Refund retry 1:聚合支付 refund API
        ├─ success → status=refunded → 邮件通知用户
        └─ failure → refund_attempt_count=1

T+15min **强制道歉邮箱**(无论 refund 是否成功):
       "由于 [失败类别] 无法生成报告,已发起全额退款 ¥XXX。
        若 1-3 工作日内未到账,请回复此邮件,我们手动处理。"

T+45min Refund retry 2:聚合支付 refund API
        ├─ success → status=refunded
        └─ failure → refund_attempt_count=3 → 转类别 B

       系统性问题检测:同时段 ≥5 失败/小时 → 转类别 C(P0 创始人手机告警)
```

#### 类别 B:Refund_failed 状态人工兜底

```
T+45min refund_attempt_count=3 → verdict_order(status=refund_failed)
        → admin dashboard "refund_failed" 队列
        → P0 告警(创始人手机)
        → 用户已收到 T+15min 道歉邮箱(知情)

T+1-24h 创始人 review:
        - 联系支付通道客服(明确退款失败原因)
        - 必要时手动银行转账 + 留档
        - 退款到账后 → status=refunded → 二次通知用户
        - 入 audit_log + 月度 Verdict 健康月报
```

#### 类别 C:P0 本人响应(KMS / 节点失窃 / 系统性失败)

详 §15.5 KMS 应急 SOP(D11) + 12 §21 节点失窃 SOP。

#### 类别 D:24h 常规客服

- 用户工单走邮件 + 控制台 /support/tickets
- 创始人每日 1-2 次批处理(早 / 晚)
- 出差 / 假期前预先回复"24h 响应"自动回复
- 非紧急问题不打扰创始人个人时间

#### 工单看板(admin dashboard)
- **类别 A 实时统计**:今日 Verdict 生成成功率 / 自检通过率 / 自动退款率
- **类别 B refund_failed 队列**:所有 refund 失败订单,按超时排序
- **类别 C P0 告警历史**:KMS / 节点失窃 / 系统性失败事件
- 按失败 step 分类
- 同时段集中失败的"系统性问题"自动聚合告警(≥ 5 失败/小时 → P0 → 创始人手机)

### 15.5 KMS 密钥仪式 SOP(v2 NEW, 决策 §K2 配套)

> Verdict 签名密钥是产品信任根,**密钥仪式必须严格执行**,所有操作进 `/attest/key-ceremony` 后台,留全程录像 + 公证。

#### 首次 root key 生成仪式(M5-M6)

```
1. 准备(M5):
   - 在 air-gap 笔记本(物理隔离)生成 Ed25519 root key
   - 用 Shamir 3-of-5 SSS 拆分为 5 份
   - 准备 5 个独立保管点:
     • 创始人(银行保险箱)
     • 法务公司 A(电子证据保管)
     • 律所 B(密封信封)
     • 可信工程师 C(海外银行保险箱)
     • 第 5 持有人(可信第三方 / 律师 / 备用工程师;S4 补 Backup HSM 后改为冷备硬件)
   - 准备录像设备 + 公证人到场

2. 仪式日(M6):
   - 录像开始,公证人见证
   - air-gap 生成 root key + 5 份 Shamir 切片
   - 每份切片密封信封 + 序列号 + 接收人签字
   - 当场销毁原始 key 文件(air-gap 笔记本格式化)
   - 公证人出具公证书

3. 验证(M6):
   - 3-of-5 召回测试:用其中 3 份重组 root key,签发首份测试 sign key
   - 用 sign key 签一份测试报告 → 走完整 verify 流程通过
   - 测试 sign key 退役 + 记录

4. 公开 transparency(M6):
   - 录像剪辑(剔除 PII)+ 公证书 + 切片接收人公开身份(职务 / 联系方式)
   - 发布到 attest.idcd.com/transparency/key-ceremony
   - 借鉴 DNSSEC root key ceremony 范式
```

#### 90 天定期 sign key 轮换

```
T-7d  系统自动提醒:sign key 即将到期
T-1d  操作员准备 → 后台 /attest/key-ceremony/rotate
T+0   走 3-of-5 quorum:
       - 3 名 quorum 持有人 24h 内提交切片(走加密通道)
       - 系统重组 root key → 签发新 sign key
       - 旧 sign key 标记 read_only(用于历史报告验签)
       - 应用层切到新 sign key
T+1h  全部 Attestation Worker 切到新 sign key
       验证:发起测试报告 → verify 通过
T+24h key_ceremony_log 写入完整审计
       自动通知所有 quorum 持有人"轮换完成"
```

#### 应急撤销仪式(v2 D11 Pre-4 2026-05-13 调整:12h Shamir 单路径,详 12 §20)

> **v2 D11 + Pre-4 锁定**:S2 仅走 **12h Shamir 3-of-5 单路径**;Backup HSM 加速通道**推迟到 S4** 企业越劤时补。
> 接受周日凌晨 sign key 泄露场景下 SLA 偶尔滑至 24h+ 的现实风险(S2 上线初 Verdict 量 ~100 份/月,可控)。
>
> **必演练**:S2 上线前必做 1 次完整模拟,实测 5 持有人响应时间,作为 SLA 基线。

**触发**:sign key 怀疑泄露(KMS 异常调用 / 用户报告伪造报告 / 内审发现)

```
T+0    P0 告警 → 创始人(7×24 手机)+ 安全负责人 + 法务即时介入
T+5m   冻结当前 sign key + Attestation Service 切**只读**模式
       - 停止新 Verdict 生成
       - **attest.idcd.com/verify 公开验签接口持续可用**(D-Concern5)
       - 已发的历史报告仍可被验签(使用对应 key_version 的 public key)

T+30m  召集 5 个 quorum 持有人,Signal 加密 + PGP 邮件双轨:
       - 创始人(银行保险箱,自身已知)
       - 法务公司 A
       - 律所 B
       - 工程师 C(海外)
       - 第 5 持有人(待定:可信第三方 / 律师 / 备用工程师)

T+4h   ≥3 持有人确认收到 + 准备提交切片
       (若 4h 仍 <3 持有人响应 → 进入 SLA 滑动预警,通知用户事件持续中)
T+8h   3 名持有人提交切片(走加密通道 / 物理寄回)
T+10h  系统重组 root → 签发新 sign key
T+11h  切换 Attestation Service → 新 sign key
T+12h  恢复 Verdict 新报告生成 + 验证

T+24h  通知所有历史报告持有者 "建议主动验签自检"
       - SELECT DISTINCT owner_id FROM verdict_order WHERE delivered_at IS NOT NULL
       - 邮件发送 + 自助验签链接
T+48h  transparency 公开事件 → attest.idcd.com/transparency/incidents
T+72h  post-mortem + key_ceremony_log 完整审计
```

**SLA 滑动场景**(Pre-4 调整后接受):
- 若 T+4h ≥3 持有人未响应 → 通知用户"事件持续中,将在 X 小时内 update";
- 实际 SLA 可能滑至 24-36h(周日凌晨 / 假期场景);
- S4 企业越劤时补 Backup HSM 加速通道(YubiHSM2 ¥3000,1-of-1 物理重组,目标 4h)

**演练记录(S2 上线前必做)**:
- 模拟 sign key 泄露场景,记录:
  - 5 持有人实际联络耗时(分钟级)
  - 加密通道传输切片实测耗时
  - 系统切换 sign key 实测耗时
- 演练报告入 `/attest/key-ceremony` 后台,作为 SLA 基线
- 每年定期演练 1 次,记录实际趋势

### 15.6 排行榜厂商退出申诉流程(v2 NEW, 决策 §K6 配套)

> /leaderboard/optout 表单提交进 `/leaderboard/optout-requests` 队列。

```
T+0   厂商在 /leaderboard/optout 提交:
       - 主体认证材料(营业执照 / 商标证)
       - 申请人姓名 / 职务 / 联系方式
       - 退出原因
       - 自动生成 ticket_id

T+1d  Content Admin 初审:
       - 验证主体真实性
       - 验证申请人代表资格(企业邮箱 / 公开 PR 联络人)
       - 若信息不全 → 邮件要求补充

T+5d  最终决策:
       - 同意(默认):
         • 厂商从公开排行榜下架
         • transparency 记录 "应 X 申请退出"
         • 已发布的历史报告**不删除**,但加水印
         • 邮件通知厂商
       - 拒绝(罕见,如主体造假):
         • 邮件说明原因 + 申诉途径

T+30d 复审窗口:厂商可在 30 天内申请重新加入(走相同流程)
```

#### 厂商方法学反馈通道(发布前 48h)

每月排行榜发布前 48h,**预先通知主流厂商(top 10)**:
- 邮件附草稿摘要(不发完整数据)
- 提供 48h 反馈窗口
- 厂商可指出"测试方法学问题"(如"你测的是已下线的 IP")
- 评估:
  - 数据错误 → 修正(罕见,因为公开边缘 IP 是稳定的)
  - 方法学不合理 → 可调整说明(但不修具体数字)
  - 厂商不满但方法学正确 → 维持发布

---

## 16. 移动端应急（S3）

### 16.1 场景
- 值班 SRE / SO 在外
- 紧急告警 / 滥用事件 / 用户大批投诉

### 16.2 简化功能
- 节点状态查看
- 紧急用户封禁（限定预设原因）
- 滥用工单一键"立即停止"
- 告警通道紧急下线（如群发故障）

### 16.3 安全
- 仍需 2FA + IP 白名单
- 关键操作仍走双人审批

---

## 17. 与其他模块接口

| 模块 | 接口 |
|---|---|
| `03-account-system.md` | 用户管理（封禁、改密、扮演） |
| `04-monitoring.md` | 用户监控的诊断查看 |
| `05-alerting.md` | 通道配置 + 工单滥用处理 |
| `09-billing.md` | 订单、退款、发票管理 |
| `10-nodes-and-agents.md` | 节点管理 + 入网审核 + **节点失窃应急吊销(v2)** |
| `12-compliance-and-abuse.md` | 黑名单 / 限速 / 审计 + **应急 SOP 触发入口(v2)** |
| `13-content-and-seo.md` | 博客、文档、SEO 文案管理 + **/leaderboard 后台(v2)** |
| **`18-evidence-and-attestation.md` (v2 NEW)** | Verdict 工单兜底 + KMS 密钥仪式 + Verdict 申诉处理 |
| **`19-ai-agent-observability.md` (v2 NEW)** | MCP token 强制撤销 + 异常突增告警接收

---

## 18. 阶段交付清单

### S1（0–4 月）
- 节点管理（列表 / 详情 / drain / 重启）
- 任务队列基础看板
- 黑名单管理（技术 + 敏感目标 + 动态）
- 限速规则可视化
- 滥用举报工单（基础）
- 管理员账号系统 + 2FA + 审计日志
- 全局仪表盘（简版）
- 用户管理（列表 / 详情 / 封禁）

### S2（4–8 月）
- 完整工单中心
- 订单 + 退款 + 发票
- 内容运营（博客 / 文档 / 公告 / Banner / SEO）
- 系统配置（档位 / 优惠券 / 配额 / 功能开关）
- 数据看板（增长 / 收入 / 用量 / 留存）
- 客服扮演（Impersonate）
- 观察名单
- 双人审批工作流
- **v2 NEW: /attest 全套(Verdict 工单 / KMS 密钥仪式 / TSA 健康 / Verdict 申诉)**
- **v2 NEW: /leaderboard 后台(草稿审核 / 厂商退出申请 / 方法学版本)**
- **v2 NEW: /nodes/revoke 紧急吊销 + /nodes/anchor-divergence 偏差监控**
- **v2 NEW: Verdict 健康月报(成功率 / 退款率 / 失败 root cause)**

### S3（8–14 月）
- 风险评分系统
- 移动端应急
- A/B 测试管理
- 高级数据分析（SQL 查询 + 模板）
- 众包节点审核流程
- 操作录屏
- **v2 NEW: /mcp 全套(token 管理 / session 监控 / 异常告警 / tool 统计)**
- **v2 NEW: Anchor 偏差自动剔除日志 + 数据污染恢复看板**

### S4（14+ 月）
- 企业客户管理（CRM 集成）
- 合同管理
- 销售线索
- SaaS 健康度高级分析（CAC、LTV、Cohort）
- **v2 NEW: 白标 Attestation 多租户管理后台**
- **v2 NEW: HSM 密钥仪式后台(从云 KMS 升级)**

---

## 19. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 后台账号被盗 → 大规模影响 | 强制 2FA / VPN / 堡垒机 / 双人审批 / 审计 / 离职流程 |
| 客服扮演被滥用 | 全程录像 + 用户告知 + 频次审计 |
| 双人审批拖慢效率 | 仅限关键操作 + 工作时间内审批 + 紧急流程 |
| 内部 SQL 查询误操作 | DBA 角色分离 + 只读副本 + 危险语句拦截 |
| 数据看板影响生产 | 用只读副本 + 缓存 |

---

## 20. 决策记录（已锁定，见 DECISIONS.md）

- ✅ **B1** 后台部署：**阿里云**（与主控同集群，内网隔离）
- ✅ **G4** VPN：**自建 WireGuard**
- ✅ **G3** 客服扮演（Impersonate）：**默认允许，用户可在设置中关闭**

### v2.0 (K 节, 2026-05-12)
- ✅ **K-后台 Verdict 工单兜底**:1 小时 SLA + 自动退款 + P1 告警 + 月报
- ✅ **K-后台 KMS 密钥仪式**:首次 root 仪式 + 90 天 sign key 轮换 + 应急撤销 全部走双人 quorum
- ✅ **K-后台 排行榜厂商退出**:5 工作日响应 + 48h 厂商方法学反馈窗口
- ✅ **K-后台 节点紧急吊销**:1 分钟内 CRL/OCSP 推送 + 1 小时内完全踢出

### 待定（不紧迫）

- [ ] 操作录屏：仅关键页面（默认）+ 存储分级
- [ ] 工单系统：S2 自建（MVP），M5-M6 视体量评估是否接 Zendesk / Help Scout
- [ ] 大额退款审批阈值：¥1000（PRD 默认）
- [ ] **v2 NEW** Verdict 工单 SLA 是否升级为 30 分钟(Compliance Enterprise 客户):S3 评估
- [ ] **v2 NEW** KMS 仪式 quorum 是否从 3-of-5 升到 5-of-7(企业 due diligence 要求):S4 评估
