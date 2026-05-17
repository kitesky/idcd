# 09 · 商业化：订阅、计费、聚合支付、CPS、Verdict 件价、Compliance 年订、Agent Pro

> 关联：OVERVIEW.md §4.8、§6.1 收入结构、§6.2 定价档、§11 决策 #4 #5 #6 + §11.K
> 关联(v2):DECISIONS.md §K5 / §K6 / §H6;18-evidence-and-attestation.md §2;19-ai-agent-observability.md §3.5
> 阶段主体：S2 全量上线（与监控告警同步），含 Verdict MVP + Compliance Starter;S3 Agent Pro + Compliance Pro;S4 增量
> 品牌名占位：`idcd`

---

## 1. 模块定位

商业化模块决定 `idcd` 是否能活下来。覆盖：

1. **订阅 SaaS**（主要收入）：4 档订阅 + 月/年付
2. **按量计费**（API + 短信 + 语音）：开发者市场
3. **CPS / 联盟**（旁路收入）：主机商推广 + 用户推荐
4. **支付通道**：聚合支付 主通道（含微信支付/支付宝 ，因经营性 ICP 暂不办）
5. **发票 / 退款 / 审计**：合规闭环

### 设计原则

1. **定价透明**：无隐藏条款，差额可秒算
2. **配额硬上限**：超额优雅降级（不直接停服）+ 引导升级
3. **跨境无感**：聚合支付 一站式处理国内外用户（含微信支付/支付宝），用户体验一致
4. **可审计**：每笔交易、每次扣量、每次退款都有完整记录

### 关键指标

| 指标 | S2 末（8 月） | S3 末（14 月） |
|---|---|---|
| 付费用户数 | 200 | 1,500 |
| MRR（月经常性收入） | ¥10k | ¥80k+ |
| Free → Paid 转化率 | ≥ 3% | ≥ 5% |
| 月流失（churn） | ≤ 8% | ≤ 5% |
| 年付占比 | ≥ 20% | ≥ 40% |
| API 收入占比 | < 5% | ≥ 15% |
| 退款率 | ≤ 2% | ≤ 1% |

---

## 2. 订阅档与定价

### 2.1 4 档定价（与 OVERVIEW §6.2 同步）

| 档位 | 月付 | 年付（8 折） | 监控 | 频率 | 节点 | API 配额 | 告警通道 | 团队 | 状态页 |
|---|---|---|---|---|---|---|---|---|---|
| Free | ¥0 | ¥0 | 5 | 5min | 5 | 100/天 | 邮件 | 1 | 1（带水印） |
| Pro | ¥29 | ¥278 | 50 | 1min | 全部 | 5,000/天 | 全通道 | 1 | 1 |
| Team | ¥99 | ¥950 | 200 | 30s | 全部 | 30,000/天 | 全通道 + 排班 | 5 | 3 |
| Business | ¥299 | ¥2,870 | 1,000 | 10s | 全部 | 200,000/天 | 全通道 + 升级 | 20 | 10 |

> Enterprise 档 S4 才上，定价议价，不在公开定价页展示。

### 2.2 出海定价（美元，S3 起开放）

| 档位 | Monthly | Annual |
|---|---|---|
| Free | $0 | $0 |
| Pro | $7 | $67 |
| Team | $24 | $230 |
| Business | $79 | $760 |

> 美元定价不是简单汇率换算，参考 UptimeRobot / Better Stack 的国际定位。

### 2.3 计费维度（超额按量）

| 维度 | Free | Pro | Team | Business | 超额单价 |
|---|---|---|---|---|---|
| 监控数 | 5 | 50 | 200 | 1000 | ¥1/个/月（最多 +20% 数量） |
| 状态页数 | 1 | 1 | 3 | 10 | ¥10/个/月 |
| API 调用 | 100/天 | 5k/天 | 30k/天 | 200k/天 | ¥0.5/1k 次 |
| API 总月配额 | 3,000 | 150k | 900k | 6M | 同上 |
| 短信 | 0 | 0 | 包含 30 条/月 | 包含 100 条/月 | ¥0.08/条 |
| 语音 | — | — | — | 包含 10 通/月 | ¥0.5/通 |
| 数据保留延长 | — | — | — | — | ¥30/月 (1 年) |
| 团队成员数 | 1 | 1 | 5 | 20 | ¥10/人/月 |

### 2.4 加购项（Add-on）

| Add-on | 价格 | 说明 |
|---|---|---|
| 额外 50 个监控 | ¥30/月 | 在档位上加量 |
| 额外团队成员 5 人 | ¥40/月 | |
| 短信 100 条包 | ¥8 | 一次性，按用扣 |
| 语音 20 通包 | ¥10 | 一次性 |
| 自定义域名状态页 | 免费（Pro+） | 已含 |
| 长期数据归档（3 年起） | ¥50/月 | |

### 2.5 折扣 / 优惠

| 类型 | 力度 |
|---|---|
| 年付 | 8 折（相当于 2 个月免费）|
| 学生认证 | 5 折（需邮箱 .edu）|
| NGO / 公益 | 5 折 |
| 早鸟终身（S1 限定 100 人） | 一次性 ¥299（永久 Pro） |
| Pro+ 用户购买 Verdict 件价 | 9 折(¥199→¥179 等) |
| Compliance 年订续费 | 9 折 |
| 推荐返利（CPS） | 见 §6 |

### 2.6 Verdict 报告件价档(v2 NEW, 详见 18-evidence §2.1)

| 模板 | 目标场景 | 价格 | 含税 |
|---|---|---|---|
| **故障取证(Incident)** | 故障复盘 | ¥199/份 | 实收 ~¥185 |
| **SLA 索赔证据** | 企业 vs CDN/云 SLA 争议 | ¥299/份 | 实收 ~¥280 |
| **合规自证(等保/审计)** | 等保测评 / 内部审计 | ¥499/份 | 实收 ~¥470 |
| **争议取证(法务)** | 法律纠纷 / 高保真留证 | ¥999/份 | 实收 ~¥940 |

**特性**:
- 一次性付费,非订阅,Pro+ 用户 9 折
- 报告永久可验签(只读),Verdict 平台保证 6 年归档
- 包含:多节点交叉验证 + RFC3161 时间戳 + KMS 签名 + PDF + 公开验签
- 不含:司法鉴定结论(平台不冒充鉴定意见)
- 失败自动退款承诺(自检失败 → 1 小时内自动退款 + 工单介入)

**生成流程**(详见 09 §13.5 + 18 §3.2):
```
用户选模板 + 目标 + 时间窗 → 数据可用性预检 → 聚合支付结账 → 异步生成 →
自检通过 → 邮件通知 + 站内可下载 → 6 年归档
```

### 2.7 Compliance 企业年订(v2 NEW, 详见 18-evidence §2.2)

面向企业法务 / 采购 / 合规 / 等保测评准备的"持续证据 + 周/月度报告"年订。

| 档位 | 年费 | 监控数 | 报告频率 | 历史回溯 | Verdict 免费件 | 司法鉴定所对接 |
|---|---|---|---|---|---|---|
| **Compliance Starter** | ¥3,000/年 | 50 | 月度 | 12 个月 | — | — |
| **Compliance Pro** | ¥12,000/年 | 200 | 周度 | 24 个月 | 5 份(任意模板) | — |
| **Compliance Enterprise** | ¥30,000/年(议价) | 1,000 | 任意 | 6 年 | 不限 | ✅ S3+ 通道(K7) |

**关键特性**:
- 持续证据存证(每次拨测都进 Attestation 归档)
- 周/月度自动生成签名报告,邮件 + 控制台下载
- 不包含告警 / 监控管理 等基础功能(需配 Pro / Team / Business 订阅)
- 可与订阅档叠加:`Team(¥99/月)+ Compliance Pro(¥1000/月) = ¥1099/月`
- 年付方式 only(月付选项不开)

### 2.8 Agent Pro 档(v2 NEW, 详见 19-ai-agent §3.5;D2 计量边界锁定)

面向企业 AI Agent 团队 + 重度 MCP 客户端用户的独立 SKU。

| 档位 | 月付 | 年付(8 折) | MCP units/day(独立池)| Concurrent SSE | Priority |
|---|---|---|---|---|---|
| **Agent Pro** | ¥299/月 | ¥2,870/年 | 1,000,000 | 500 | 优先调度 |

**关键特性(v2 D2 锁定)**:
- **MCP units 与 API 配额完全独立**(D2):用户控制台看到两条独立量表
  - 用户拿订阅档(如 Pro)+ Agent Pro 时:`MCP units = 1,000,000/day(Agent Pro 加大)`,`API calls = 5,000/day(Pro 订阅档)`
- service account token 不限数量(IP 白名单强制,详 03 §5a + 12 §22)
- 所有 token 最长 90 天 auto_renewal(D2,无永久 token)
- 优先调度(节点选择 + 队列权重)
- Compliance Enterprise 客户默认包含

> 何时推荐?月 MCP 调用量 > 30k 的开发者 / Agent 服务。
> **计量池关系**(D2):订阅档(Free/Pro/Team/Business)各有独立 MCP units 额度(分别 100 / 5,000 / 30,000 / 200,000 per day);Agent Pro 是 MCP units 加大独立 SKU,**与订阅档完全独立**,不替换、不互相消耗。详 19 §3.5 表格。

---

## 3. 支付通道（聚合支付系统）

### 3.1 通道选型

| 通道 | 适用 | 费率 | 备注 |
|---|---|---|---|
| **聚合支付**（主通道,通过 `payment-go-sdk`） | 国内 | ~1% | 同时承接微信支付 + 支付宝,由聚合支付服务商统一结算 |
| 对公转账 | 大额企业 | 0% | 手动确认 |
| 微信支付（自家商户号） | 国内 | 0.6% | ⚠️ 需经营性 ICP,S3+ 视量再评估 |
| 支付宝（自家商户号） | 国内 | 0.55% | ⚠️ 同上 |

> **重要决策（C8）**:经营性 ICP 许可证暂不办理。
>
> **解决方案**:S1-S2 完全依赖 **聚合支付服务商**(对接微信 + 支付宝),通过 `packages/payment-go-sdk` 调用,内部由聚合方分发到具体通道。优点:
> - 1% 上下费率,远低于跨境 MoR 的 5%+$0.5
> - 资金 T+1 清算,人民币结算,不涉及境外主体
> - 商户资质由聚合服务商代办,无需自行 ICP/微信/支付宝商户号申请
>
> **S3+ 视量**:若月流水 > ¥500k 可考虑直连微信 / 支付宝商户号,把聚合通道费率从 ~1% 压到 0.6%/0.55%。

### 3.2 聚合支付架构

```
User 选择档位 + 周期
  → 跳到 /app/billing/checkout/<order_id>
  → 选择支付方式
       • 微信 / 支付宝（国内）
       • 
       • 对公转账（企业）
  → 自家 Payment Gateway 路由到对应通道
  → 通道完成支付
  → 回调（webhook + 主动轮询双保险）
  → 订单状态 paid
  → 订阅生效
```

### 3.3 通道适配层（Adapter）

每个通道一个 adapter，统一接口：

```
interface PaymentAdapter {
  createCheckout(order) -> { redirect_url, payment_url, qrcode_url }
  verifyWebhook(req) -> boolean
  parseWebhookEvent(req) -> { order_id, status, paid_at, ... }
  refund(order, amount, reason) -> { refund_id, status }
  queryStatus(order) -> status
}
```

### 3.4 聚合支付特别说明
- 通过 `packages/payment-go-sdk` 调用,客户端不区分微信/支付宝,聚合服务商按用户选择路由
- 国内人民币 T+1 清算,无跨境结算与汇率风险
- 费率 ~1%,远低于 MoR(5%+$0.5),不必为海外用户/英文 invoice 场景买单(本期不出海)
- 缺点:聚合服务商兜底依赖第三方,需在合同里锁定 SLA + 资金安全条款

### 3.5 微信支付 / 支付宝特别说明
- 国内必须实名 + 企业资质（个体户也行）
- 需要在 `idcd` 公司主体下开通商户号
- 经营性 ICP 许可证是开通"网站类目"的前提
- 跨境收款不可（境外用户付不了人民币）

### 3.6 对公转账（企业用）
- 用户上传打款凭证
- 财务后台核对 → 手动标记订单完成
- 适合大额、企业月结
- 提供形式发票（pro forma invoice）

### 3.7 支付安全
- 所有支付回调走 HTTPS + 签名验证
- 防重放（订单 ID + 状态机不可逆）
- 异常订单自动告警（金额异常、回调失败等）
- 对账：每天自动对账 通道流水 vs 内部订单

---

## 4. 订阅生命周期

### 4.1 状态机

```
[trial]            (S3 上线，14 天免费试用 Pro)
   ↓
[active]           ── 续费失败 → [past_due] → 7 天后 → [canceled]
   ↓                ↓
[past_due]         (用户更新支付 → active)
   ↓
[canceled]         (用户主动取消 / 续费失败 7 天)
   ↓
[expired]          (周期结束，仍未续费)
```

### 4.2 创建订阅

```
User → 升级页 → 选档位 + 周期
  → 创建订单 (status=pending)
  → 跳支付
  → 支付成功 webhook → 订单 paid
  → 创建 subscription 记录
       • plan_id, period (monthly|yearly), price, currency,
       • starts_at, current_period_start, current_period_end
       • renewal: auto | manual
       • payment_method_id (绑定的支付方式)
  → 解锁配额
  → 发送收据邮件 + 发票（如选开）
```

### 4.3 自动续费

- 续费前 7 天：邮件提醒 + 站内通知
- 周期结束当天：扣款（微信支付 自动；支付宝按时弹通知）
- 扣款失败：3 天内重试 3 次（递增间隔）
- 仍失败：状态 past_due → 7 天宽限期 → canceled

### 4.4 升级（Upgrade）

- 升级立即生效
- 按时间比例计算差额：
  - 例：Pro 月付 ¥29，用了 10 天，剩 20 天 → 退余额 ¥29 × 20/30 = ¥19.33
  - 立即扣 Team ¥99 - 余额抵扣 = ¥79.67
- 用户可见"差额明细"
- 周期保持（不重新计算开始日）

### 4.5 降级（Downgrade）

- 降级**生效在下个周期**（避免立即损失功能）
- 当前周期内提示"将在 YYYY-MM-DD 切换为 Pro"
- 用户可随时撤销降级

### 4.6 取消

- 取消立即生效"不续费"
- 当前周期内仍享用所有功能
- 周期结束后变 expired → 数据保留 90 天 → 删除业务数据
- 退款另议（见 §5）

### 4.7 切换支付方式 / 通道
- 用户可绑定多个支付方式
- 默认支付方式用于自动续费
- 切换通道生效在下次扣款

---

## 5. 退款政策

### 5.1 自助退款（首次订阅）
- 7 天内无理由退款（首次订阅）
- 自助按钮 → 立即退回原通道
- 退款后订阅立即终止 → 降级 Free

### 5.2 中途退款
- 工单 / 邮件申请
- 客服审核：
  - 合理：按使用天数比例退余额
  - 不合理：拒绝（如已使用 50% 以上）
- 一般 3-5 工作日到账

### 5.3 强制退款
- 服务严重故障 / 协议变更不接受 / 法律要求
- 按情况全额或按时间退

### 5.4 不退款情况
- 已使用超过 50% 周期且非首次订阅
- 涉及违规被封禁
- 短信 / 语音等已消耗的 add-on

### 5.5 退款审计
- 每笔退款记录原因 + 操作人 + 凭证
- 大额（> ¥1000）需双人审批

---

## 6. 推荐返利（用户级 CPS）

### 6.1 模式
- 每个登录用户有唯一推广链接：`idcd.com/?ref=u_xxx`
- 通过链接注册 + 付费的用户 → 推荐人获得返利

### 6.2 返利规则
- 推荐人：被推荐人首次付费的 20%（一次性）
- 被推荐人：注册自动得 ¥20 优惠券（首次订阅可用）
- 仅首次订阅触发返利
- 返利记入 `idcd` 余额，可：抵扣下次续费 / 提现到微信 / 支付宝（达 ¥100 起）

### 6.3 反作弊
- 同 IP / 同支付方式 / 同邮箱后缀的"自推自" → 不计返利
- 异常推荐模式自动审核
- 反复异常 → 取消推广资格

### 6.4 推广素材
- 自动生成宣传海报 / 二维码
- 简单 Banner / Widget 嵌入用户网站

---

## 7. 主机商 CPS（合作伙伴）

### 7.1 形式
- 在拨测 / 工具页右侧或顶部展示主机商 / VPS 推广位
- 旁路收入，不影响主业体验

### 7.2 来源
- 第三方联盟：腾讯云 / 阿里云 / 华为云 / Cloudflare / RackNerd / Hetzner 等
- 直接合作：与 IDC 主机商谈定向链接

### 7.3 展示规则
- 仅相关位置展示（如 VPS 测速结果旁推 VPS 商）
- 不在控制台 / 付费用户页面打扰

### 7.4 数据归因
- 通过 ref / utm 参数
- 月度对账 + 收款

---

## 8. 发票管理

### 8.1 类型

| 类型 | 适用 |
|---|---|
| 电子普通发票（增值税普通） | 个人用户 |
| 电子专用发票（增值税专票） | 一般纳税人企业 |
| 形式发票（Pro Forma Invoice） | 海外用户 / 对公转账前 |
| 聚合支付电子发票 | 普票自动出具 |

### 8.2 开票流程

```
User → /app/billing/invoices → 申请开票
  → 选订单 + 抬头 + 税号 + 邮箱
  → 提交 → 财务审核（自动 + 人工）
  → 调用税务接口开具
  → 邮箱推送 PDF + 站内可下载
  → 红冲 / 重开机制
```

### 8.3 抬头管理
- 用户可保存多个抬头（个人 + 公司）
- 默认抬头一键开票

### 8.4 自动开票
- 用户开启"自动开票" → 每次付款自动开
- 失败邮件通知人工处理

### 8.5 合规
- 增值税法要求（销售方义务）
- 保留 10 年财税档案
- 配合税务局核查

---

## 9. 配额管理与超额处理

### 9.1 配额维度
- 监控数、状态页数、API 调用量、短信、团队成员、节点池大小、频率

### 9.2 超额提醒
- 80% → 邮件提醒
- 95% → 站内 + 邮件
- 100% → 强制限制（不再创建新监控；API 返 429）

### 9.3 软超额（按量计费维度）
- 短信 / 语音 / API 超额自动转入按量计费
- 用户预先勾选"允许超额按量"
- 超出后扣余额；余额不足 → 停发 + 邮件提醒

### 9.4 配额降级
- 订阅档位下调 → 现有资源数量超新档位
- 不删除，但只读（不可改 / 不可新增）
- 用户可主动归档 / 删除多余资源

---

## 10. 余额（Credits）系统

### 10.1 用途
- 抵扣短信 / 语音 / API 超额
- 抵扣下次续费
- 推荐返利存入

### 10.2 充值
- 微信 / 支付宝 直接充值
- 最小 ¥50 起
- 永不过期（合规要求，避免被认定"预付卡"）

### 10.3 扣减
- FIFO（先充先扣）
- 每次扣减有明细：时间 / 用途 / 数量 / 余额
- 自动扣量阈值（余额 < 配置值 → 邮件提醒）

### 10.4 提现
- 推荐返利可提现（实名验证 + 达 ¥100）
- 充值余额不可提现（防洗钱合规）

---

## 11. 信息架构（控制台）

```
/app/billing
├── /                  概览（当前订阅 + 余额 + 用量）
├── /subscription      订阅详情 + 升级 / 降级 / 取消
├── /payment-methods   绑定的支付方式
├── /orders            订单历史
├── /invoices          发票管理
├── /credits           余额明细
├── /usage             用量明细（API / 短信 / 监控数等）
├── /referral          推广返利
└── /checkout/<id>     支付页（结账）
```

---

## 12. 数据模型

```
plan
  id, code (free|pro|team|business),
  name, description,
  prices (jsonb: { CNY: { monthly, yearly }, USD: {...} }),
  limits (jsonb: monitors, status_pages, api_quota, ...),
  visible (true|false),  -- enterprise=false
  created_at

subscription
  id, owner_id (user|team), plan_id,
  status (trial|active|past_due|canceled|expired),
  period (monthly|yearly), currency,
  current_period_start, current_period_end,
  trial_ends_at,
  cancel_at_period_end (bool),
  payment_method_id,
  created_at, canceled_at

payment_method
  id, owner_id, type (wechat|alipay|bank),
  external_id (在支付商的标识),
  display (脱敏卡号等), is_default,
  created_at

order
  id, owner_id, type (subscription|addon|topup|invoice),
  plan_id (nullable), amount, currency, tax,
  status (pending|paid|failed|canceled|refunded),
  payment_method_id, payment_channel,
  external_txn_id,
  created_at, paid_at, refunded_at

refund
  id, order_id, amount, reason,
  status, requested_by, approved_by,
  refunded_at, external_refund_id

invoice
  id, order_id, owner_id,
  title (抬头), tax_id (税号),
  type (普票|专票|海外),
  amount, status,
  file_url, issued_at

credit_ledger
  id, owner_id, change (正/负),
  balance_after,
  source (topup|refund|referral|usage|other),
  reference_id, created_at

usage_event
  id, owner_id, dimension (api_call|sms|voice|...),
  amount, unit, occurred_at,
  reference_id (api_key|monitor_id|...)

referral
  inviter_user_id, invitee_user_id,
  registered_at, first_paid_order_id,
  commission_amount, paid_to_inviter_at

coupon
  code, description, type (percent|amount),
  value, valid_from, valid_until,
  max_uses, used_count, applicable_plans
```

---

## 13. 关键流程

### 13.1 升级 Pro 月付（国内用户）

```
User → /pricing → 点 Pro 升级
  → /app/billing/checkout/<order_id> (订单 pending)
  → 选微信支付
  → 后端调微信统一下单 → 拿到 prepay_id
  → 前端展示二维码（PC）/ 调起小程序（移动）
  → 用户扫码完成支付
  → 微信回调 webhook → 校验签名 → 更新订单 paid
  → 创建 subscription (active)
  → 解锁配额
  → 发邮件收据
  → 前端轮询订单状态变为 success → 跳转 "升级成功"页
```

### 13.2 出海年付订阅

```
User → /pricing (en) → 选 Team Annual
  → 聚合支付 checkout 跳转
  → 用户填邮箱 + 卡 + 公司信息（VAT 自动计算）
  → 聚合服务商处理支付清算
  → Webhook 通知订单 paid + 含税金额
  → 创建 subscription
  → 系统自动出具普票
```

### 13.3 续费失败处理

```
T0   扣款失败（卡过期 / 余额不足）
       → 订阅状态 active → past_due
       → 邮件 + 站内通知"更新支付方式"
T1   24h 后重试 → 仍失败
T3   72h 后再重试 → 仍失败
T7   订阅状态 past_due → canceled
       → 周期结束后变 expired
       → 数据保留 90 天 → 删除
```

### 13.4 申请退款

```
User → /app/billing/orders → 选订单 → 退款
  → 选原因 + 描述
  → 7 天内首次订阅：自动通过，立即退原通道
  → 否则：工单送客服 → 客服审核 → 同意 / 拒绝
  → 同意：触发支付商 refund API
  → 状态变 refunded
  → 余额扣回 / 订阅终止
  → 邮件通知
```

### 13.5 Verdict 件价下单 → 报告生成 → 自检失败自动退款(v2 NEW + D4/D5/D6 锁定)

> **CRITICAL 流程**:这是 Verdict 模块的"信任根之外"的兜底承诺。用户付费后未拿到合格报告 = 品牌直接死亡风险。
>
> **三件套 CRITICAL GAP 闭合方案(v2 D4 + D5 + D6)**:
> 1. **D4 WAL**:`attestation_record` 表充当 step-level WAL;每 step 完成写一条 `(action, status=success, external_id, idempotency_key)`;Worker crash 后续跑时先查 attestation_record 跳过已成功 step。KMS sign 调用启用 idempotency token,防止重试导致 KMS audit log 重复 sign。详 18 §3.2。
> 2. **D5 Refund retry queue**:聚合支付 refund API 失败本身有 retry queue(5min retry → 30min retry);**无论 refund 是否成功,30min 内必发用户道歉邮箱**;30min 后仍失败 → `verdict_order(status=refund_failed)` + P0 告警 + admin dashboard 入兜底队列。
> 3. **D6 Self-verify 独立**:Self-Verify Worker 不同进程 / 不同 VPC subnet / 独立 KMS 客户端实例 / 仅调 attest.idcd.com/verify 公开接口。详 18 §3.5。

```
T0    User → /pricing → 选 Verdict 模板 + 目标 + 时间窗
        → /v1/verdict/quote 数据可用性预检
        → 若数据不足 / 目标黑名单 / 节点<3 → 拒绝下单 + 解释
        → 数据 OK → 创建 verdict_order(status=pending)
T+0    User 跳聚合支付 checkout → 完成支付
T+10s  聚合支付 webhook → verdict_order(status=paid)
        → 入 verdict_generation_queue(Redis Stream, idempotency by order_id)
T+15s  Generator Worker 拉 task
        → verdict_order(status=generating)
        → 进入每 step 前查 attestation_record;若已 success 则跳过(WAL replay)
        → step 6 KMS sign 启用 idempotency token;step 完成写 attestation_record(action=signed, external_id=kms_req_id, status=success)
        → step 7 TSA stamp 类同 → attestation_record(action=tsa_stamped, ...)
        → step 10 S3 archive → attestation_record(action=s3_archived, ...)
        (任何 step 失败 → step.retry_count++ 重试,3 次都失败 → 入 DLQ → 失败路径)

成功路径:
T+90s  Self-Verify Worker(独立进程)从 S3 重读 PDF → 走 attest.idcd.com/verify 公开接口
        → attestation_record(action=self_verified, status=success)
        → verdict_order(status=delivered)
        → 邮件 + 站内通知 + 可选 webhook → 用户下载 PDF + 永久可验签

失败路径(Critical Gap 兜底,v2 D5):
T+5min Generator step 三次重试失败 / Self-verify 失败 → DLQ
        ↓
        【Refund Worker】
        Step 1: 调用 聚合支付 refund API
                ├─ success → verdict_order(status=refunded) → 通知用户(成功)
                └─ failure → refund_attempt_count=1, refund_last_error=记录 → 5min 后 retry
T+10min Step 1 retry: 聚合支付 refund API 再调用
                ├─ success → status=refunded → 通知用户
                └─ failure → refund_attempt_count=2 → 30min 后 retry
T+15min 【强制道歉邮箱(无论后续 refund 成功否)】
        发送用户:"由于 [失败类别] 无法生成报告,已发起全额退款 ¥XXX。
                   若 1-3 工作日内未到账,请回复此邮件,我们手动处理。"
        → verdict_order.refund_apology_sent_at = now()
T+45min Step 2 retry: 聚合支付 refund API 再调用
                ├─ success → verdict_order(status=refunded) → 通知用户(已通过道歉信)
                └─ failure → refund_attempt_count=3 → 终止自动 retry
                            → verdict_order(status=refund_failed)
                            → P0 告警(创始人 7×24 手机)
                            → admin dashboard "refund_failed" 队列
                            → 人工手动处理(联系聚合服务商支持 / 手动退款)
T+24h  人工 review:若手动 refund 成功 → status=refunded;否则保留 refund_failed
```

**SLA 承诺**(写在 /pricing /verdict 页):
- 90% 报告在 60 秒内生成
- 99% 报告在 5 分钟内生成
- 任何自检失败的订单 → **30 分钟内** 发送道歉邮箱 + 触发全额退款流程
- 退款最迟 1-3 工作日到账(支付通道风控 / 银行处理时间);超出时间用户回复邮件 → 创始人手动跟进
- 任何 refund 失败 → 道歉邮箱已发(用户先知道) + 创始人 P0 告警 + admin dashboard 入队

### 13.6 Compliance 年订续费(v2 NEW)

与订阅档续费流程类似,但:
- 仅年付,无月付
- 续费前 30 天邮件提醒(因企业财务流程长)
- 续费失败 → past_due → 30 天宽限 → canceled(比订阅档 7 天宽限更长)
- 取消后:历史报告归档持续可访问(6 年合规要求),仅新报告生成停止

### 13.7 Agent Pro 档下单(v2 NEW)

- 与 Pro/Team 订阅档下单流程相同
- 关键差异:开通后必须配置 service account token 的 IP 白名单(强制),否则状态 active 但 service account token 签发受限
- 用户可在 /app/mcp 中查看 Agent Pro 独立用量(不与订阅档 API 配额合并)

---

## 14. 与其他模块接口

| 模块 | 接口 |
|---|---|
| `03-account-system.md` | 订阅与 user / team 绑定 |
| `04-monitoring.md` | 监控数 / 频率 / 节点池受档位限制 |
| `05-alerting.md` | 通道限速、短信 / 语音按量计费 |
| `06-status-pages.md` | 状态页数 / 自定义域受档位限制 |
| `08-open-api.md` | API 调用计量 + 配额 |
| `11-admin.md` | 后台订单管理 + 退款审批 |
| `12-compliance-and-abuse.md` | 财税档案 10 年保留 |

---

## 15. 阶段交付清单

### S2（4–8 月）必交付
- 4 档定价 + 月 / 年付
- 支付通道：聚合支付 为主 (含微信/支付宝 );企业用户对公转账
- 升级 / 降级 / 取消 / 续费完整
- 发票（普票自动开 + 专票申请;聚合支付电子普票）
- 退款（自助首次 + 工单中途）
- 余额系统
- 配额管理 + 超额提醒
- 短信 / 语音超额按量
- 推荐返利基础
- 主机商 CPS 落地（旁路）
- **v2 NEW: Verdict 件价(4 模板) — MVP,与 18-evidence MVP 同期(M7-M8)**
- **v2 NEW: Compliance Starter 年订(¥3k/年) — MVP(M7-M8)**
- **v2 NEW: Verdict 自检失败自动退款流程(13.5 CRITICAL 流程)**

### S3（8–14 月）增量
- 出海定价（USD）+ Stripe 备选
- 14 天 Pro 免费试用
- 高级优惠券 / 学生认证 / NGO
- 余额提现
- 自动开票
- 团队成员 / 状态页加购
- 早鸟终身计划
- **v2 NEW: Compliance Pro 年订(¥12k/年)— M11-M12**
- **v2 NEW: Agent Pro 档(¥299/月)— 与 MCP server GA 同期(M12)**
- **v2 NEW: Verdict 件价 Pro+ 用户 9 折优惠**
- **v2 NEW: 按报告嵌入卡片加购(企业 add-on)**

### S4（14+ 月）增量
- Enterprise 档定价正式上线
- **v2 NEW: Compliance Enterprise 年订(¥30k/年议价档)— 含司法鉴定所对接 + HSM 升级**
- 合同管理（年付大客户）
- 信用付（先用后付，T+30）
- 多币种切换
- 海外税务深度优化
- **v2 NEW: 白标 Attestation API / MCP server(企业自家域名)**

---

## 16. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 经营性 ICP 未办，无法直连微信 / 支付宝 | ✅ 已采决策 C8：走 聚合支付；S3+ 视情况引入合作方代收 |
| 海外税务踩坑 | 不涉及海外税务 |
| 退款率高 → 微信通道关停 | 严控退款流程 + 客服培训 |
| 优惠券被薅羊毛 | 风控规则 + 限领 + 黑名单 |
| 聚合支付通道费率上涨 | S3 引入 Stripe 备选 |
| 价格战导致利润压缩 | 差异化（API + 一键诊断）+ 不打价格战 |
| 跨境收款合规 | 聚合支付 + 不直接处理外汇 |
| **v2 NEW: Verdict 付费后生成失败 = 品牌灾难** | ✅ 13.5 attestation_record 充 WAL + step-level idempotency + Self-Verify 独立部署(D4/D6);聚合支付 refund retry queue + 30min 强制道歉邮箱(D5);P0 告警 + admin dashboard "refund_failed" 队列 |
| **v2 NEW: 聚合支付 refund API 自身失败** | ✅ 13.5 refund retry queue(5min→30min);refund_failed 状态保留兜底;道歉邮箱 30min 内必发,优于退款到账;P0 告警创始人手机 7×24 |
| **v2 NEW: Compliance 年订客户中途取消(企业财务流程)** | 取消生效在下周期 + 历史报告永久可访问;协议明确"年订不退款"但允许 30 天宽限 |
| **v2 NEW: Agent Pro 用户 MCP 调用爆账单(死循环)** | 80%/95%/100% 三级提醒 + 用户可设硬上限自动停服;Service account token 强制 IP 白名单 |
| **v2 NEW: Verdict 报告被滥用(诬告竞品)** | 12 §3 目标黑名单 + 单 24h 多用户同目标 → 人工审核 + 申诉通道 |

---

## 17. 决策记录（已锁定，见 DECISIONS.md）

### v1.0 决策(C 节)
- ✅ **C1** 早鸟终身计划：**不做**
- ✅ **C2** 推荐返利比例：**30%**（仅首次付费触发）
- ✅ **C3** Pro 14 天免费试用：**S3 中期推出**
- ✅ **C4** 7 天无理由退款：**首次订阅提供**
- ✅ **C5** "按量纯付费"档：**S3 推出**（¥0.5 / 1k 调用）
- ✅ **C6** 学生 / NGO 折扣：**5 折**（.edu / NGO 资质审核）
- ✅ **C7** 短信 / 语音：**订阅档赠送配额 + 超额按量**
- ✅ **C8** 经营性 ICP：**暂不办理**，主走聚合支付；详见 DECISIONS.md §H1

### v2.0 决策(K 节, 2026-05-12 plan-ceo-review EXPANSION)
- ✅ **K5** Verdict 件价档:¥199 / ¥299 / ¥499 / ¥999 共 4 档
- ✅ **K5b** Compliance 年订档:¥3k / ¥12k / ¥30k 共 3 档(议价档 ¥30k)
- ✅ **K-计费 Agent Pro 档**:¥299/月(S3 GA 推出)
- ✅ **K-计费 Verdict 付费失败兜底**:WAL 状态机 + 自检失败 + 自动退款 + 工单介入(详 13.5)

### 待定（不紧迫）

- [ ] 余额提现：仅推荐返利 vs 充值都可（运营时再定）
- [ ] 信用付（B 端常见）：S4 评估
- [ ] **v2 NEW** Verdict Pro+ 用户折扣是否扩大到 8 折(S2 末根据销售数据定)
- [ ] **v2 NEW** Compliance 年订是否支持季度付(S3 评估,目前年付 only)
