# 03 · 账号、团队、API Key、MCP Token、Organization(v2)

> 关联：OVERVIEW.md §4.2、§4.13(Compliance)、§4.14(MCP);DECISIONS §K3 / §K-账号
> v2 新增章节:§5a MCP Token 管理(三档:personal / workspace / service);§6.8 Compliance 组织级身份(营业执照 + 法人 + 公章授权 + 多 email)
> 阶段主体:S1 基础账号(仅邮箱注册),S2 完整账号 + API Key + organization 实体,S3 团队 + MCP token,S4 SSO
> 品牌名占位:`idcd`

---

## 1. 模块定位

账号系统是所有付费 / 个性化功能的入口。需要做到：

1. **门槛极低**：邮箱注册 30 秒搞定，是公开工具用户的自然转化下一步
2. **安全可控**：2FA、登录异常告警、密码规则
3. **协作可扩展**：从单人 → 团队 → 组织 → 企业 SSO 逐档扩
4. **合规友好**：PIPL 注销 / 数据导出原生支持

### 关键指标

| 指标 | S1 | S2 | S3 |
|---|---|---|---|
| 注册转化率（公开工具 → 注册） | — | ≥ 3% | ≥ 5% |
| 注册完成率（开始注册 → 完成） | ≥ 70% | ≥ 80% | ≥ 85% |
| 2FA 启用率（付费用户） | — | ≥ 20% | ≥ 50% |
| 账户被盗事件 | 0 | 0 | 0 |

---

## 2. 注册与登录

### 2.1 注册方式

| 方式 | P | S | 备注 |
|---|---|---|---|
| 邮箱 + 密码 | P0 | S1 | 主方式 |
| 微信扫码 | P0 | S2 | 国内用户主流 |
| GitHub OAuth | P0 | S2 | 开发者首选 |
| 手机号 + 验证码 | P1 | S2 | 部分国内用户偏好 |
| Google OAuth | P1 | S2 | 海外用户 |
| 钉钉 / 飞书扫码 | P2 | S3 | 企业用户场景 |
| 企业 SAML SSO | P3 | S4 | 仅企业版 |

### 2.2 注册流程（邮箱）

```
Step 1: 填邮箱 + 密码 + 同意协议
   → 立即创建账号（unverified 状态）
   → 发送验证邮件
   → 用户可立即登录使用（但有提示"请验证邮箱解锁全部功能"）

Step 2: 用户点验证邮件链接
   → 标记 email_verified
   → 解锁完整功能

Step 3:（可选引导）
   → 加入欢迎邮件
   → 引导首个监控创建
   → 提示绑定告警通道
```

#### 决策依据
- **不强制完成验证才让用**：转化漏斗最大化
- 但未验证账号有"3 天宽限期"，逾期不验证则锁定（可继续验证）

### 2.3 密码规则

- 长度 ≥ 8 位
- 必须含字母 + 数字（弱密码警告）
- 与邮箱前缀不可相同
- 检查 HIBP API（Have I Been Pwned，可选优化）
- 哈希：Argon2id（memory_cost 64MB, time_cost 3, parallelism 4）

### 2.4 登录

- 邮箱 + 密码
- 失败 5 次/15 分钟 → 锁定登录 30 分钟（用户名 + IP 双维度）
- 异地登录提醒（邮件 + 站内）
- 登录会话：JWT access (15 min) + refresh (30 days)
- "记住我"延长 refresh 到 90 天

### 2.5 登录方式切换 / 绑定

- 用户可在 `/app/settings/security` 绑定多个登录方式
- 至少保留一种密码登录（或备用邮箱）
- 解绑前需确认有其他可用方式

### 2.6 找回密码

- 邮箱发送一次性 token（15 分钟有效）
- 重置后所有现有 session 失效
- 重置成功通知（站内 + 邮件）

---

## 3. 双因素认证（2FA）

### 3.1 类型

| 类型 | P | S |
|---|---|---|
| TOTP（Authenticator App） | P0 | S2 |
| 备份恢复码（10 个一次性） | P0 | S2 |
| 短信验证码 | P2 | S3 |
| Email OTP | P2 | S3 |
| WebAuthn / Passkey | P2 | S3 |

### 3.2 启用流程

1. 用户进入 `/app/settings/security` → 启用 2FA
2. 系统生成 secret + QR Code
3. 用户用 Authenticator 扫码 → 输入 6 位码确认
4. 显示 10 个备份码，强制下载/打印
5. 启用成功

### 3.3 登录流程
- 密码 + 验证码
- 输错 3 次锁 5 分钟
- 备份码每个仅可用一次

### 3.4 关闭 2FA
- 需密码 + 当前 TOTP / 备份码确认
- 关闭后通知用户

### 3.5 强制 2FA
- Team / Business 档可由 owner 强制成员启用 2FA
- 企业版可强制所有成员

---

## 4. 个人资料

### 4.1 字段

```yaml
user:
  id: u_xxx
  email: user@example.com
  email_verified: true
  phone: "+86 138****0000"      # 可选
  phone_verified: false
  username: zhangsan            # 唯一，2-32 字符
  display_name: 张三
  avatar_url: https://...
  bio: "..."
  locale: zh-CN                 # zh-CN | en
  timezone: Asia/Shanghai
  notification_preferences: {...}
  created_at, updated_at
```

### 4.2 头像
- 自定义上传（200KB 内）
- 默认 Gravatar
- 默认彩色字母头像（无 Gravatar 时）

### 4.3 偏好
- 主题：浅色 / 深色 / 跟随系统
- 语言：中文 / 英文
- 时区
- 默认仪表盘视图
- 邮件订阅偏好

---

## 5. API Key 管理

### 5.1 设计原则
- **最小权限**：每个 Key 可指定权限范围
- **可观测**：每个 Key 的调用次数、最近使用时间
- **可撤销**：随时禁用 / 删除
- **不可恢复**：Secret 仅创建时显示一次

### 5.2 Key 结构

```yaml
api_key:
  id: ak_xxx
  owner_id: u_xxx                # 或 team_id
  name: "生产环境监控"
  prefix: "idc_live_xxxxxxxx"    # 可见前缀
  secret_hash: "..."             # SHA-256 哈希，不可逆
  scopes: ["read", "probe", "monitor.read"]
  rate_limit_override: null      # 默认按订阅档；可单独调整
  allowed_ips: ["1.2.3.4/32"]    # 可选 IP 白名单
  allowed_origins: ["api.client.com"]  # 浏览器场景
  expires_at: null               # 可设过期
  last_used_at: ...
  last_used_ip: ...
  usage_total: 12345             # 累计调用次数
  status: active | revoked | expired
  created_at, revoked_at
```

### 5.3 Scope（权限范围）

| Scope | 说明 |
|---|---|
| `probe` | 调用一次性拨测 API |
| `probe:read` | 读取拨测历史 |
| `monitor:read` | 读监控配置 + 状态 |
| `monitor:write` | 增删改监控 |
| `alert:read` | 读告警事件 |
| `alert:write` | Ack / 评论 |
| `status:read` | 读状态页内容 |
| `status:write` | 改状态页 / 创建事件 |
| `billing:read` | 读账单（仅 owner） |
| `admin` | 管理团队（仅 owner） |

### 5.4 创建流程

```
User → API Key 管理 → 创建
  → 输入名称 + 选 scopes + 可选 IP 白名单 + 可选过期
  → 创建 → 显示完整 secret（仅此一次）
  → 用户复制
  → 关闭弹窗后 secret 不可再查
```

### 5.5 列表与使用统计

- 列：名称 / Prefix / Scopes / 最近使用 / 累计调用 / 状态 / 操作
- 详情：调用次数曲线（7d / 30d）、按 endpoint 分布、错误率
- 操作：编辑名称 / scope / 限速 / 撤销

### 5.6 撤销与失效
- 立即撤销：Key 不可再用（所有正在进行的请求受影响）
- 软撤销（grace 1 小时）：渐进式撤销，避免大批量任务突然失败

### 5.7 自动安全检测
- Key 出现在公开 GitHub / GitLab：自动失活 + 通知
- Key 异常调用模式（IP 突变、流量飙升）：触发告警

---

## 5a. MCP Token 管理(v2 NEW, 详 19 §3.4)

> MCP token 与 API Key 是**两套独立体系**:API Key 走 `api.idcd.com`,MCP token 走 `mcp.idcd.com`。两者鉴权 / 计量 / 撤销 / 审计完全分开。理由:MCP 是面向 AI Agent 的产品,凭证泄露风险面 / 使用习惯 / 计费模式都与 REST API 不同。

### 5a.1 三种 token 形态(v2 D2 锁定:所有 token 最长 90 天,无永久)

| 形态 | 适用 | 有效期 | Auto-renewal | IP 白名单 | 签发方式 |
|---|---|---|---|---|---|
| **Personal** | 开发者本地(Cursor / Claude Code) | **24h** | OAuth-like 自动 refresh | 可选 | 用户控制台手动签发,只展示一次 |
| **Workspace** | 团队级生产(Team / Business 订阅绑定) | **90d** | 过期前 24h 自动 renewal(基于 last_used_at;超 30 天未用不续) | 强烈推荐 | 团队管理员签发 |
| **Service** | 生产 Agent 服务(长期运行) | **90d** | 过期前 24h 自动 renewal(同上) | **强制** | 团队管理员签发;无白名单不签发 |

> **v2 D2 原则**:不存在"永久 / 长期"token。所有 token expires_at 必填,最长 90 天。auto_renewal 让用户体验等同永久,但 90 天滚动失效大幅降低凭证泄露损失上限。

### 5a.2 Token 结构(v2 D2)

```yaml
mcp_token:
  id: mcpt_xxx
  owner_id: u_xxx           # 或 team_id;跨 schema 不写 FK(详 15 §4.X)
  type: personal | workspace | service
  name: "Cursor on MacBook"
  token_hash: "..."         # 不存原文,只哈希
  token_display: "mcpt_***1234"   # 后 4 位展示
  scope:
    tools: [idcd_ping, idcd_http_probe, idcd_diagnose, ...]
    regions: [CN, US, EU]    # 可限制节点区域
  ip_whitelist: ["1.2.3.4/32", "5.6.7.0/24"]    # service 必填
  expires_at: timestamptz   # v2 D2: NOT NULL,最长 90 天
  auto_renew: true          # v2 D2: workspace/service 默认 true,personal OAuth-like 自动 refresh
  last_renewed_at: timestamptz
  revoked: false
  revoke_reason: null
  created_at, last_used_at
```

### 5a.3 创建流程

```
User → /app/mcp → "签发新 token"
  → 选 type (personal / workspace / service)
  → service 强制要求 IP 白名单输入
  → 选 scope (tools 多选 + 区域筛选)
  → 选有效期
  → 创建 → 显示完整 token (仅此一次) + 提示"请勿提交到代码库"
  → 用户复制 (或自动复制 + 二维码扫码到设备)
  → 关闭弹窗 token 不可再查
```

### 5a.4 撤销

- 一键撤销:即时生效,所有 active session 强制断开
- 自动撤销:
  - Personal:90 天未使用自动撤销
  - 异常突增告警(详 12 §22 应急 SOP)→ 系统先降级到 read_only 再人工确认撤销
  - GitHub 扫描发现公开:自动失活 + 通知(类 API Key)

### 5a.5 与 API Key 的明确区别(v2 D2)

| 维度 | API Key | MCP Token |
|---|---|---|
| 子域 | `api.idcd.com` | `mcp.idcd.com` |
| 协议 | HTTPS REST | MCP(JSON-RPC stdio + HTTP+SSE) |
| 计量 | API 配额(详 09 §2.3) | **MCP units(完全独立池,v2 D2)** |
| 计费 | 订阅档配额 | 各订阅档 MCP units 独立额度;Agent Pro 是 MCP units 加大独立 SKU |
| 鉴权 | Bearer token / Basic | MCP session token + 可选 IP 校验 |
| 有效期 | 无强制过期(由用户管理) | **最长 90 天 + auto_renewal(v2 D2)** |
| 用户场景 | 自家产品集成 idcd 数据 | AI Agent 在 Cursor / Codex 调用 idcd |
| 撤销影响 | 该 Key 所有调用 | 该 token 所有 active session 立即断开 |

### 5a.6 控制台 UI

```
/app/mcp
├── /                       概览(active token 数 / 本月调用 / 异常告警)
├── /tokens                 token 列表 + 签发新 token
│   ├── /<id>               详情(用量 / 调用历史 / IP / scope)
│   └── /<id>/revoke        撤销
├── /usage                  用量明细(按 tool / 按 client / 按时间)
├── /sessions               活跃 session(可强制断开)
└── /docs-quick             5 分钟接入指南(链接 mcp.idcd.com/docs)
```

### 5a.7 安全教育

- 控制台第一次签发 token 时**强制弹窗**:
  - "请勿提交到代码库 / 配置文件;请用环境变量"
  - "请使用 short-lived Personal token,不要用 service token 做开发"
  - "service token 必须配置 IP 白名单,否则不签发"
- 文档站给出 Cursor / Claude Code / Codex 三种客户端的"正确配置示范"
- 每月给 active token 拥有者发"安全月报":列出过去 30 天 token 调用情况 + 异常提醒

---

## 6. 团队 / 组织（S3）

### 6.1 概念

- **个人账号**：默认所有人都是个人账号
- **团队（Team）**：多个用户协作（Team 档起）
- **组织（Org）**：企业级管理（Business + 起，S4 完善）

### 6.2 团队成员角色

| 角色 | 权限 |
|---|---|
| Owner | 全部权限 + 计费 + 解散团队 + 移交所有权 |
| Admin | 管理成员 + 全部业务功能 |
| Member | 业务功能 + 看见账单概要 |
| Viewer | 只读 |
| Billing | 仅账单管理（财务专用） |

### 6.3 团队邀请

- Owner / Admin 发邀请链接 + 邮箱
- 受邀人接受 → 自动加入团队
- 邀请链接 7 天过期
- 拒绝 / 撤销邀请

### 6.4 资源归属

| 资源 | 归属 |
|---|---|
| 监控 | 团队（创建者可识别） |
| 告警策略 | 团队 |
| API Key | 创建者 + 团队 |
| 通知通道 | 创建者 / 团队 |
| 状态页 | 团队 |
| 订阅 / 账单 | 团队（Owner 控制） |

### 6.5 切换团队
- 顶部下拉切换"当前工作区"
- 个人 vs 团队 1 vs 团队 2
- 切换后所有资源视图重置

### 6.6 离开团队 / 解散

- 成员可随时离开（自己资源转给 Owner 或匿名化）
- Owner 不能离开（必须先移交）
- 解散：所有数据按"用户注销"流程处理

### 6.7 组织（Org，企业版）

- 多团队聚合
- 组织级 SSO / 域名邮箱绑定 / 集中计费
- 子团队隔离 + 跨团队角色

### 6.8 Compliance 组织级身份(v2 NEW, S2 起;详 18 §2.2)

> Compliance 年订(¥3k/¥12k/¥30k)是企业 SKU,需要比"个人 + 团队"更**法律 / 合规可追溯的身份模型**。本节定义 Compliance 客户必须有的额外身份字段。

#### 6.8.1 组织实体(Organization Entity)

Compliance Starter/Pro/Enterprise 客户**必须**绑定到一个 organization 实体(在 team 之上),包含:

```yaml
organization:
  id: org_xxx
  legal_name: "Beijing Example Co., Ltd."     # 法律全称
  display_name: "Example"                     # 展示名
  business_type: company | ngo | gov | individual
  registration:
    country: CN
    business_license_no: "91110108MA00XXXXXX"
    tax_id: "91110108MA00XXXXXX"               # 同营业执照
    legal_representative: "张三"                # 法人姓名
    registered_address: "..."
  contact:
    primary_email: "admin@example.com"
    primary_phone: "+86 138 XXXX XXXX"
    billing_email: "finance@example.com"      # 账单邮箱
    legal_email: "legal@example.com"          # 法务通知(报告异议 / 申诉)
    technical_email: "ops@example.com"        # 技术联络
  verification:
    status: pending | verified | rejected
    verified_at: ...
    verified_by: admin_user_id
    evidence_urls: [...]                       # 营业执照 / 法人身份证(脱敏)
  compliance_settings:
    sla_required: 99.5 | 99.9 | 99.95         # Compliance Pro+ 可选
    data_retention_years: 6 | 10              # 法定保留期(中国 6 年 / 出海 10 年)
    auto_legal_notification: true             # 涉及法律事件自动通知 legal_email
    audit_export_allowed: true                # 允许批量导出审计日志
  created_at, updated_at
```

#### 6.8.2 团队与组织的关系

- 一个 organization 可挂多个 team(场景:跨部门 / 子公司)
- Team 仍是日常协作单元;organization 是"统一计费 + 统一合规"层
- Compliance 订阅绑定到 organization,而非 team
- Verdict 报告**可选**以 organization legal_name 为出具方(添加增信)

#### 6.8.3 验证流程

```
Owner 在 /app/org/setup 填写组织信息
  → 上传营业执照 + 法人身份证(脱敏)+ 加盖公章的服务申请表
  → 提交 → admin pending
  → admin 审核(5 工作日 SLA):
    - 验证营业执照真实(工商局公示查询)
    - 验证申请人为法人或授权人(企业邮箱 + 法人签字授权)
  → 通过 → organization.status = verified → 允许下单 Compliance 年订
  → 拒绝 → 邮件说明原因 + 申诉途径
```

#### 6.8.4 组织级 audit 增强

- audit_log 中所有涉及 Verdict / Compliance / 大额支付 的事件 → 自动 copy 到 legal_email
- 用户违规被封禁前 → 必须通知 organization 的 legal_email(给 24h 反馈窗口)
- 数据删除(用户注销)对 organization 内成员 → 触发 organization 级 Owner 审批

#### 6.8.5 与 03 §6 个人 / 团队的区别

| 维度 | 个人 | Team | Organization (v2 Compliance) |
|---|---|---|---|
| 身份验证 | 邮箱 | 邮箱 + 团队 Owner 验证 | **营业执照 + 法人身份 + 公章授权** |
| 法律实体 | 自然人 | 一般无法律实体 | **明确法律实体 + 法定代表人** |
| 数据保留 | 30 天 - 5 年 | 同上 | **6-10 年法定保留** |
| 违规处理 | 立即封禁 | 同上 | **24h 法定通知 + 申诉窗口** |
| Verdict 出具方 | 用户名 / 邮箱 | 团队名 | **可用 organization legal_name(增信)** |
| 审计导出 | 不支持 | 不支持 | **支持批量导出 + 法务通道** |

---

## 7. 审计日志（账号侧）

### 7.1 记录的事件

- 登录成功 / 失败 / 异地
- 密码修改 / 重置
- 2FA 启用 / 关闭
- API Key 创建 / 撤销
- 团队成员变更（邀请、加入、退出、角色改）
- 资源权限变更
- 数据导出 / 注销请求

### 7.2 字段

```
audit_log
  id, owner_id (user|team),
  ts, actor_user_id, action,
  resource_type, resource_id,
  client_ip, user_agent, location,
  result (ok|fail), error_reason,
  metadata (jsonb)
```

### 7.3 用户可见
- `/app/settings/audit-log`
- 自己所有操作日志
- 团队 Admin/Owner 可见团队成员操作日志
- 默认保留 90 天（Free），1 年（Pro+），3 年（Business）

### 7.4 导出
- CSV / JSON
- 用于合规 / 内部审计

---

## 8. 通知偏好（个人）

### 8.1 维度
- 事件类型：告警 / 账单 / 系统更新 / 营销 / 安全
- 通道：邮件 / 站内 / 个人微信
- 时间：工作时间 / 非工作时间 / 全天 / 自定义

### 8.2 营销邮件
- 默认订阅，可一键退订（合规）
- 退订记录 6 个月不再发

### 8.3 关键通知（不可退订）
- 安全：异地登录、2FA 变更、密码重置
- 账单：扣款失败、续费提醒（提前 30/7/1 天）
- 合规：服务条款变更（提前 30 天）

---

## 9. 数据导出与注销（PIPL 合规）

### 9.1 数据导出
- 入口：`/app/settings/data`
- 一键点击 → 异步生成 ZIP → 邮件下载链接
- ZIP 内含：账号信息 / 监控配置 / 拨测历史 / 告警 / 订单 / 节点（如适用）
- 多格式：JSON（机器） + Markdown（可读） + CSV（表格）
- 链接 7 天有效

### 9.2 注销流程
- 详见 12-compliance-and-abuse.md §14.3
- 此处只关心账号侧：
  - 密码 + 邮箱验证码 + 协议二次确认
  - 30 天冷静期
  - 不能注销的情况：欠费、调查中、近期已经注销过

### 9.3 GDPR / PIPL 数据请求

- 用户主动通过控制台完成的就走自助
- 邮件来信申请通过 `privacy@idcd.com` 工单处理（30 天内响应）

---

## 10. 信息架构

```
/app/
├── /                       工作区首页（仪表盘）
├── /switch                 切换工作区
└── /settings
    ├── /profile            个人资料
    ├── /security           密码 + 2FA + 会话管理
    ├── /api-keys           API Key 管理
    ├── /notifications      通知偏好
    ├── /team               团队（Team+ 才显示）
    │   ├── /members        成员管理
    │   ├── /roles          角色与权限
    │   └── /invitations    邀请管理
    ├── /audit-log          审计日志
    ├── /data               数据导出
    └── /danger             注销账号
```

---

## 11. 数据模型

```
user
  id, email, email_verified_at,
  phone, phone_verified_at,
  username UNIQUE, display_name, avatar_url, bio,
  locale, timezone,
  password_hash (argon2id),
  password_changed_at,
  status (active|locked|pending_deletion|deleted),
  pending_deletion_at,
  email_marketing_opted_in,
  created_at, updated_at, last_login_at

user_credential (多登录方式)
  id, user_id, type (password|wechat|github|google|...),
  external_id, metadata (jsonb), linked_at

user_2fa
  user_id, type (totp|sms|webauthn),
  secret (encrypted), backup_codes (encrypted),
  enabled_at, disabled_at

user_session
  id, user_id, refresh_token_hash,
  client_ip, user_agent, device,
  created_at, expires_at, revoked_at

api_key
  id, owner_type (user|team), owner_id,
  name, prefix, secret_hash,
  scopes (text[]),
  rate_limit_override (jsonb),
  allowed_ips (cidr[]), allowed_origins (text[]),
  expires_at,
  last_used_at, last_used_ip,
  usage_total,
  status (active|revoked|expired),
  created_by, created_at, revoked_at

team
  id, name, slug, plan_id, owner_id,
  created_at, deleted_at

team_member
  team_id, user_id, role (owner|admin|member|viewer|billing),
  joined_at, invited_by, left_at

team_invitation
  id, team_id, email, role,
  token, invited_by, accepted_at, expires_at

audit_log
  id, owner_id, ts, actor_user_id, action,
  resource_type, resource_id,
  client_ip, user_agent, location,
  result, error_reason, metadata

data_export_request
  id, user_id, status, requested_at, completed_at,
  download_url, expires_at
```

---

## 12. 关键流程

### 12.1 公开工具用户 → 注册

```
未登录用户用公开工具
  → 看到 "保存历史 + 加入持续监控" 弹窗
  → 点击注册
  → 进入快速注册（邮箱 + 密码 + 协议）
  → 立即登录 → 之前的临时测试记录绑定到账号
  → 引导 "添加你的第一个监控"
```

### 12.2 团队 owner 邀请成员

```
Owner → /app/settings/team/members → 邀请
  → 输入邮箱 + 选角色
  → 系统发邀请邮件 + 控制台显示"待接受"
  → 受邀人收到邮件 → 点击链接
  → 已登录：弹"接受 / 拒绝"
  → 未登录：先注册 / 登录，再接受
  → 加入后 owner / admin 收到通知
```

### 12.3 切换工作区

```
顶部下拉点击其他工作区
  → 当前 session 关联新 workspace_id（保存到 user_session）
  → 全站资源视图刷新
  → URL 不变（workspace 是 session 属性）
```

---

## 13. 安全控制要点

### 13.1 密码相关
- 不存明文 / 不可见
- 不通过日志 / 错误信息泄露
- 重置邮件链接一次性
- 历史密码记录（最近 5 个，禁止重用）

### 13.2 Session
- HttpOnly + Secure + SameSite=Lax cookie
- Refresh Token 旋转（用一次失效一次）
- 全局登出（吊销所有 refresh token）

### 13.3 API Key
- 创建后 Secret 不可再看
- 哈希存储
- IP / Origin 白名单

### 13.4 暴力破解防护
- 登录失败限速（IP / 用户名 / 组合）
- 注册限速（IP / 邮箱域）
- 重置密码限速

### 13.5 异常登录检测
- 异地（不同国家 / ASN）
- 新设备
- 短时间多次失败后的成功
- 立即邮件通知 + 站内提示

---

## 14. 与其他模块接口

| 模块 | 接口 |
|---|---|
| 全部需登录功能 | 提供身份认证 |
| `08-open-api.md` | API Key 校验 |
| `09-billing.md` | 订阅与 user/team 绑定 |
| `04-monitoring.md` | 资源归属（团队 / 个人） |
| `05-alerting.md` | 通知偏好 |
| `11-admin.md` | 管理员可看 / 改用户数据 |
| `12-compliance-and-abuse.md` | 数据导出 / 注销执行 |

---

## 15. 阶段交付清单

### S1（0–4 月）
- 邮箱注册 + 登录 + 验证 + 找回密码
- 个人资料基础
- 单工作区（无团队）
- 公开工具用户的临时历史绑定
- 数据导出（基础）
- 注销（基础）

### S2（4–8 月）
- 微信 + GitHub OAuth + Google OAuth + 手机号
- 2FA TOTP + 备份码
- 完整 API Key 管理
- 完整审计日志
- 异地登录提醒
- 通知偏好
- **v2 NEW: organization 实体 + 营业执照 / 法人验证流程(Compliance 客户必备)**
- **v2 NEW: organization legal_email / billing_email / technical_email 多联络人**

### S3（8–14 月）
- 团队 / 角色 / 邀请
- 切换工作区
- 团队级 API Key
- 强制 2FA（团队配置）
- WebAuthn / Passkey
- 钉钉 / 飞书登录
- **v2 NEW: MCP token 三种形态(personal / workspace / service)+ 控制台 /app/mcp**
- **v2 NEW: organization 多 team 聚合 + 统一计费 + Compliance 订阅绑定**

### S4（14+ 月）
- 组织（Org）层级
- SAML / OIDC SSO
- SCIM 用户同步
- 高级审计 + 不可篡改日志库
- **v2 NEW: organization 级 BYOK(自带 Verdict 签名密钥)**
- **v2 NEW: 白标 organization 身份(Verdict 报告用客户公司名出具)**

---

## 16. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 弱密码用户被撞库 | HIBP 检查 + 2FA 引导 + 异地登录提醒 |
| API Key 泄露 | GitHub 扫描自动失活 + 调用模式异常告警 |
| 邀请链接被转发 | 链接绑定邮箱 + 7 天过期 + 一次性 |
| 团队权限设计过于复杂 | S3 起步先 4 个角色，企业版再加细粒度 |
| 数据导出包过大 | 异步生成 + 分卷 + 邮件链接 |
| 多账号薅羊毛 | 注册需邮箱验证 + 邀请码（可选） + IP / 设备指纹 |
| **v2 NEW: MCP token 泄露(Cursor 配置被偷)** | 短期 token + 默认 24h Personal + service 强制 IP 白名单 + 异常突增告警 + 一键撤销 |
| **v2 NEW: organization 营业执照造假(伪 Compliance 客户)** | 工商局公示查询 + 法人邮箱验证 + 加盖公章授权 + 拒绝率监控 |
| **v2 NEW: Verdict 报告以 organization 名义出具被滥用** | 必须 verified 状态;违规自动暂停 organization Compliance 权限 |

---

## 17. 决策记录（已锁定，见 DECISIONS.md）

### v1.0
- ✅ **A2** 注销冷静期：**30 天**
- ✅ **A3** 2FA：**Free 也允许**
- ✅ **A4** Username：**不必填，可后补**；默认以邮箱前缀作 display name

### v2.0 (K 节, 2026-05-12)
- ✅ **K3** MCP token 三档:personal(1h-7d) / workspace(1-90d) / service(长期 + 强制 IP 白名单)
- ✅ **K-账号 organization 实体**:Compliance 客户必须 verified(营业执照 + 法人 + 公章授权)
- ✅ **K-账号 multi-email**:legal_email / billing_email / technical_email 分开,法律事件自动通知

### 待定（不紧迫）

- [ ] 邮箱别名（同一邮箱多账号）：默认禁止
- [ ] 团队档位起点：Team 起（PRD 默认）
- [ ] API Key 是否强制过期上限
- [ ] 异地登录处理：邮件 vs 强制再认证
- [ ] **v2 NEW** organization 验证是否需要现场到访 / 视频核验(Compliance Enterprise ¥30k 档):S3 评估
- [ ] **v2 NEW** MCP token 是否支持 OAuth-like flow(避免手动复制粘贴):S3 末评估
