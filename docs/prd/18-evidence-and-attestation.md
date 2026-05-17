# 18 · 证据与公证(Evidence-as-a-Service)

> 关联:OVERVIEW.md §1.2 长版定位、§4.13;DECISIONS.md §K1/§K2/§K5/§K7;14-tech-architecture.md(独立子域 attest.idcd.com);09-billing.md §2.6(Verdict 件价档)
> 阶段主体:S2 MVP(签名报告 + RFC3161);S3 完整(多家 TSA 容灾 + 司法鉴定所合作通道);S4 加 HSM
> 品牌名占位:`idcd`

---

## 1. 模块定位

**Evidence-as-a-Service 是 `idcd` 的核心护城河**,把"多地拨测 + 监控告警"从工具升级为可作证据的网络第三方公证。

### 1.1 一句话

`idcd` 出具的拨测/监控历史**带签名 + 时间戳**,可在合规、采购、SLA 索赔、运维甩锅、内部审计场景中作为**一手观测证据**使用。

### 1.2 我们不是什么

- ❌ **不是司法鉴定结论** — 我们不替代司法鉴定机构;Verdict 报告标注"一手观测数据 + 第三方背书",不冒充鉴定意见
- ❌ **不是 SLA 仲裁方** — 我们提供数据,但 SLA 争议由当事方法律程序解决
- ❌ **不是网络违法证据链** — 我们不为任何主动攻击/侦察行为出报告;合规一票否决

### 1.3 设计原则

1. **签名先于美观**:报告内容是次要的,可被独立验证才是核心
2. **时间戳是事实**:任何报告必须含 RFC3161 时间戳;无时间戳的报告不发出
3. **多节点交叉**:任何主张("X 时刻不可用")必须有 ≥3 节点交叉验证;否则标注"低置信"
4. **痕迹可追溯**:每份报告留存原始 raw 数据 + 节点 ID + 任务签名;6 年留存合规
5. **法律边界明确**:文案与定价中反复强调"非鉴定结论",避免被误用

### 1.4 关键指标

| 指标 | S2 末 | S3 末 |
|---|---|---|
| Verdict 报告月生成量 | 100 | 1,000 |
| 报告生成成功率 | ≥ 99% | ≥ 99.5% |
| 报告自检验签通过率(每日抽样) | 100% | 100% |
| 报告生成 P95 延迟 | ≤ 90s | ≤ 60s |
| TSA 可用性(主备汇总) | ≥ 99.9% | ≥ 99.95% |
| Verdict 收入占比 | ≥ 10% | ≥ 25% |

---

## 2. 产品形态

### 2.1 Verdict 报告(单次件价)

用户在控制台或公开页输入"目标 + 时间窗 + 场景模板",`idcd` 生成一份带签名 + 时间戳的 PDF 报告。

**场景模板**(初版 4 类):

| 模板 | 目标用户 | 价格 | 内容要点 |
|---|---|---|---|
| **SLA 索赔证据** | 企业用户 vs CDN/云厂商 | ¥299/份 | 时间窗内每分钟可用性 + 跨节点偏差 + 失败原因分类 |
| **故障取证(Incident)** | 故障复盘 | ¥199/份 | 时间线 + 影响范围 + 跨节点表现 + LLM 草拟根因建议 |
| **合规自证(等保/审计)** | 等保测评准备 | ¥499/份 | 持续 30/90/180 天的连续观测 + 多节点 + 关键事件清单 |
| **争议取证(法务)** | 法律纠纷 | ¥999/份 | 高保真:每节点 raw + 路由 + DNS 解析 + TLS 链 + 签名 + TSA |

> 聚合支付费率 ~1%, 实收约 ¥197-989。**毛利高于订阅档**(因为单次生成成本固定 + 信任溢价)。

### 2.2 Compliance 企业年订

| 档位 | 年费 | 包含 |
|---|---|---|
| Compliance Starter | ¥3,000/年 | 50 监控持续证据存证 + 月度报告 + 12 个月历史回溯 |
| Compliance Pro | ¥12,000/年 | 200 监控 + 周度报告 + 24 个月历史 + 5 份免费 Verdict 件 |
| Compliance Enterprise | ¥30,000/年(议价) | 1000 监控 + 任意频率报告 + 6 年历史 + 不限 Verdict + 司法鉴定所对接通道(S3+)|

### 2.3 报告嵌入(可选 add-on)

- 企业用户可在自家文档/邮件/PPT 中嵌入"由 `idcd` 签名 + 时间戳"的小卡片(iframe / OG)
- 任意第三方可在 `idcd` 网站独立验签
- 不出现"鉴定"字样,只出现"由 `idcd` 出具的观测记录"

---

## 3. 技术架构

### 3.1 独立子域 + 独立服务

| 组件 | 子域 / 路径 | 职责 |
|---|---|---|
| Attestation API | `attest.idcd.com` | 报告生成请求、状态查询、独立验签接口 |
| Attestation Worker | (内部) | 异步生成 PDF / 签名 / 时间戳 / 归档 |
| Public Verify | `attest.idcd.com/verify` | 任意第三方上传 PDF/JSON 验签 |
| Report Archive | (内部 S3/WORM) | 6 年只增不删归档 |

### 3.2 完整数据流(端到端,v2 D4 WAL 化)

> **关键设计(v2 D4)**:`attestation_record` 表充当本流程的 Write-Ahead Log(WAL)。
> Worker 进入每个 step 前先查 `attestation_record WHERE report_id=$1 AND action=$2 AND status='success'`;
> 若已成功,跳过该 step 并复用 external_id;否则执行 step,完成后写 `attestation_record(action, status=success, external_id, idempotency_key)`。
> KMS sign 调用启用 idempotency token(AWS KMS / 阿里云 KMS 均支持),防止 worker crash 后重试导致 KMS audit log 重复 sign。
> 任何 step 失败重试上限 3 次(retry_count 字段),超出转 DLQ。

```
[用户付款 聚合支付 Webhook]
   │ verdict_order(status=PAID)
   ▼
[Order Service] 入 verdict_generation_queue (Redis Stream, idempotency by order_id)
   │
   ▼
[Verdict Generator Worker]
   │ -- 进入 step 前总查 attestation_record;已成功则跳过;crash 重启后从 last record 续跑 --
   │
   ├─ 1) 拉取目标历史(TimescaleDB)              ⇩ 数据空? → REJECT_REFUND
   ├─ 2) 多节点交叉验证(>=3 节点,排除 low_confidence 节点) ⇩ 一致性 <50% → 标记低置信 / 节点 <3 → 拒绝生成 + REFUND
   ├─ 3) LLM 解读 + 根因建议(可选,失败降级模板) ⇩ LLM 失败 → fallback 模板继续(不入 WAL,可重试无副作用)
   ├─ 4) 渲染 PDF(含节点地图 + 时间序列 + 表格 + 法律边界声明硬编码段落) ⇩ 超大字段截断 + 标注
   ├─ 5) 计算内容哈希(SHA-256)               (纯计算,不入 WAL)
   ├─ 6) 签名(KMS sign API + idempotency token) → 写 attestation_record(action=signed, external_id=kms_req_id)
   │    ⇩ KMS 失败 → retry up to 3 次,3 次都失败 → DLQ + P1 告警 → 流程转失败路径
   ├─ 7) 申请时间戳(RFC3161 TSA, 主→备→第三) → 写 attestation_record(action=tsa_stamped, external_id=tsa_serial)
   │    ⇩ 三家全失败 → DLQ → 失败路径
   ├─ 8) 嵌入签名 + 时间戳到 PDF(PAdES)        (基于已写入的 attestation 数据,可重做无副作用)
   ├─ 9) (可选 S3+)区块链锚定 → 写 attestation_record(action=anchored, external_id=tx_hash);失败则跳过 + 标记"未上链"
   ├─10) 归档到 S3 WORM(只增不删) → 写 attestation_record(action=s3_archived, external_id=s3_etag)
   │
   ▼
[Self-Verify Worker(独立进程 + 独立 VPC subnet + 独立 KMS 客户端实例)]
   │ -- 仅调用 attest.idcd.com/verify 公开 HTTP 接口,不复用主 Worker 任何内部代码 / 配置 / 缓存 --
   │ 1. 从 S3 重读归档 PDF
   │ 2. 提取嵌入签名 + 时间戳
   │ 3. 走公开 verify 路径验证 → 写 attestation_record(action=self_verified, status=success|failure)
   │    ⇩ 自检失败 → DLQ + 失败路径(全额自动退款 + 道歉邮箱)
   │
   ▼
[Notify] 邮件 + 站内 + (可选 webhook)             ⇩ 通知失败 → 重试 3 次
   │
   ▼
verdict_order(status=DELIVERED)
   │
   ▼
[Periodic Audit Worker(独立)] 每日抽样 10 份历史报告独立验签 ⇩ 失败 → 立即停止新生成 + P0 告警

────────────────────────────────────────────────────────────
失败路径(任一 step DLQ 或 self-verify failure):
────────────────────────────────────────────────────────────
[Refund Worker]
   1. 调用 聚合支付 refund API
      ⇩ 失败 → 5min retry → 仍失败 → 30min retry
      ⇩ 30min 后仍失败 → verdict_order(status=refund_failed) + P0 告警(创始人本人)
   2. **无论 refund API 是否成功,30min 内必发用户道歉邮箱**:
      "由于 [失败类别] 无法生成报告,已发起全额退款 ¥XXX。
       若 1-3 工作日内未到账,请回复此邮件,我们手动处理。"
      → verdict_order.refund_apology_sent_at = now()
   3. 后续如手动 refund 成功 → status=refunded;否则保留 refund_failed 在 admin dashboard 等待处理
```

### 3.3 签名密钥架构(决策 §K2 + v2 D11 单路径,Pre-4 2026-05-13 调整)

> **v2 Pre-4 调整**:S2 仅走 Shamir 3-of-5 主路径(目标 12h),Backup HSM 加速通道**推迟到 S4 企业越劤时补**。S2 上线初 Verdict 量小(~100 份/月),12h SLA 偶尔滑至 24h+ 风险可控。

```
                Root Key (offline, Shamir 3-of-5 quorum,唯一应急路径)
                            │
                ┌───────────┴───────────┐
                │                       │
        Sign Key (online, KMS)   Backup Sign Key (cold)
        90-day rotation                30-day cold rotation
                │
                ▼
        Cloud KMS (AWS KMS / 阿里云 KMS)
                │
                ▼
        每份报告调一次 KMS sign API(启用 idempotency token)
        审计日志记录 key_id + key_version + caller + report_id

        [S4 补:Backup HSM(YubiHSM2 ¥3000)1-of-1 物理重组加速通道,
               企业 due diligence 要求"4h 加速"时再加]
```

**起步选型**:云 KMS(AWS KMS 或阿里云 KMS,根据收款主体地区);S4 企业客户严苛 due diligence 时升级 HSM。

**Root key 仪式**(首次部署 SOP):
1. 在 air-gap 笔记本生成 root key
2. 切分为 5 份(Shamir SSS),由 5 个不同人员/律所/可信第三方保管
3. 任何 sign key 轮换需要 3-of-5 重组 root key
4. 全过程录像 + 公证 + 上传 `idcd.com/transparency/key-ceremony`(借鉴 DNSSEC root key ceremony 范式作为信任背书)

**Backup HSM 独立重组通道(S4 才补,Pre-4 2026-05-13 调整)**:
- **当前 S2-S3 不采购**:理由 — Verdict 量小 + 1 人创业暂可接受 12h SLA 偶尔滑至 24h+
- **S4 推进时点**:企业客户 due diligence 要求"4h 加速"时,或 S3 末月 Verdict 量超 500 份/月时
- **届时形态**:YubiHSM2(¥3000)+ 离线保险柜;1-of-1 创始人物理获取 + 解锁密码;启用后 7 天内补做完整 Shamir 仪式
- **替代:S2 上线前演练 12h 主路径**(详 11 §15.5),记录每步实际耗时基线;若 5 持有人响应慢于预期,可加快"短期加速候选人"沟通(如增加 1-2 个备用持有人)

### 3.4 时间戳(RFC3161 TSA)

**主备三家**:
1. **DigiCert TSA**(主) — 商业可信、广泛验证
2. **GlobalSign TSA**(备) — 商业可信
3. **国家授时中心(NTSC)TSA** — 国内司法场景认可度高,S2 末接入

**失败策略**:主失败 → 5 秒内切备 → 备失败 → 切第三 → 三家全失败,报告生成暂停,P0 告警(年内预期发生次数 0-1 次)。

### 3.5 自检(Self-verify, v2 D6 独立性边界明确)

每份报告生成后立即由独立 Self-Verify Worker 执行自检。

**独立性边界**(v2 D6 — 严格):
- **不同进程**:Self-Verify Worker 在独立 docker container 运行,不与 Verdict Generator Worker 共享进程空间
- **不同 VPC subnet**:部署在独立 subnet,与 Generator Worker 之间仅暴露 attest.idcd.com/verify HTTPS 接口,无内部 RPC
- **独立 KMS 客户端实例**:Self-Verify 通过 verify 接口走 KMS GetPublicKey,**不复用** Generator Worker 的 KMS sign 客户端实例 / 配置 / 缓存
- **不同代码路径**:Self-Verify 仅调用 `attest.idcd.com/verify` 公开 HTTP 接口验证,与外部第三方用户走的代码路径完全一致;不存在"内部捷径"
- **物理隔离**:S2 起在 docker compose 中明确两个 service 不同 hostname,后期可拆为不同物理机

**目的**:Generator Worker 与 Self-Verify Worker bug 互相不可能同时存在(不同代码路径 + 不同进程),自检失败 = 真实失败,自检通过 = 真实通过。

**自检流程**:
- 重新从 S3 读 PDF
- 提取嵌入签名 + 时间戳
- 调用 `attest.idcd.com/verify` 公开接口验证
- 失败 → DLQ → 失败路径(refund retry + 30min 道歉邮箱 + P0 告警,详 §3.2)
- 写 `attestation_record(action=self_verified, status=success|failure)` 为 WAL 终态

### 3.6 公开验签(信任公开化)

`attest.idcd.com/verify` 提供任何第三方上传 PDF 验签的页面:
- 上传 PDF
- 系统解析嵌入签名 + 时间戳
- 显示:签名链 / 公钥指纹 / 签名时间 / TSA 颁发者 / 内容哈希
- 显示验证结果:✅ 有效 / ❌ 失效(原因)
- 验签不需登录、不限频(纯只读)

**重要**:验签接口由独立轻量服务承担,与主 Attestation API 解耦,即使主服务挂了验签也不挂。

---

## 4. 与其他模块的接口

| 模块 | 接口 |
|---|---|
| `04-monitoring.md` | Verdict 报告输入 = 监控历史(TimescaleDB);Compliance 档持续观测来源 |
| `07-reports-and-dashboards.md` | Verdict 是 reports 的 supercharged 版本(带签名时间戳),非订阅档专属 |
| `09-billing.md` §2.6 | Verdict 件价订单 + Compliance 年订订单 |
| `10-nodes-and-agents.md` | 报告引用的节点 ID 必须在公开节点目录可查;Anchor 偏差不可信节点结果不入报告 |
| `11-admin.md` | Verdict 工单兜底 + KMS 密钥仪式后台 + 自检失败队列 |
| `12-compliance-and-abuse.md` | "Verdict 非鉴定结论"声明、滥用报告(诬告竞品)处理流 |
| `14-tech-architecture.md` | 独立子域 attest.idcd.com;KMS 集成;TSA 客户端集成 |
| `15-data-model.md` | 新增表:verdict_orders / verdict_reports / attestation_records / tsa_responses |
| `16-api-spec.md` | `/v1/verdict/*` `/v1/attest/*` `/v1/verify/*` 端点 |

---

## 5. 数据模型(参与 15 模块)

```
verdict_order
  id (v_xxx), owner_id, template (sla|incident|compliance|legal),
  target (domain|url|ip), time_window_start, time_window_end,
  status (pending|paid|generating|delivered|failed|refunded),
  price_cny, ext_order_id,
  created_at, paid_at, delivered_at

verdict_report
  id (r_xxx), order_id,
  pdf_url (S3 path), pdf_size, content_hash (sha256),
  signature (KMS), signature_key_id, signature_key_version,
  tsa_provider (digicert|globalsign|ntsc), tsa_response_blob, tsa_time,
  blockchain_anchor (optional, jsonb),
  nodes_used (jsonb: [node_id, ...]), node_consistency_pct,
  llm_used (bool), llm_model, llm_prompt_version,
  self_verify_status (pass|fail|pending), self_verify_at,
  confidence_label (high|medium|low),
  created_at, archived_url (WORM永久)

attestation_record
  id, report_id, action (signed|tsa_stamped|anchored|verified),
  external_id (TSA serial / chain tx hash),
  payload_hash, created_at

tsa_response
  id, provider, request_hash, response_blob,
  serial_number, issued_at, valid_until,
  status (success|failure|timeout), created_at

key_ceremony_log
  id, action (root_gen|root_split|sign_key_rotate|emergency_revoke),
  actor (jsonb: [user_id or external_identity, ...]),
  evidence_url (录像 / 公证 PDF),
  created_at
```

---

## 6. API 端点(参与 16 模块)

```
POST   /v1/verdict/quote          预估价格 + 数据可用性预检
POST   /v1/verdict/orders         创建订单(返回 聚合支付 checkout url)
GET    /v1/verdict/orders/:id     订单状态
GET    /v1/verdict/reports/:id    报告详情 + PDF 下载链接
POST   /v1/verdict/reports/:id/share  生成分享 token(可设过期)

POST   /v1/attest/verify          上传 PDF 验签(不需登录,纯只读)
GET    /v1/attest/key/:key_id     查询公钥 + 元数据
GET    /v1/attest/ceremony        密钥仪式公开记录(transparency)

POST   /v1/compliance/subscriptions      创建 Compliance 年订
GET    /v1/compliance/reports             列出年订生成的周/月度报告
```

**Verify 接口返回字段(v2 D-Concern1)**:`POST /v1/attest/verify` 返回必含 `report_type` 字段,默认为 `observation_only`,明确告知第三方"本报告是一手观测数据,不是司法鉴定结论"。第三方解析时可基于此字段决定如何使用:

```json
{
  "valid": true,
  "signature_chain": "...",
  "public_key_fingerprint": "...",
  "signed_at": "2026-05-13T01:00:00Z",
  "tsa_provider": "digicert",
  "content_hash": "sha256:...",
  "report_type": "observation_only",
  "legal_disclaimer": "本报告为 idcd 提供的一手观测数据,不构成司法鉴定结论。"
}
```

---

## 7. 安全 / 合规

### 7.1 密钥安全(决策 §K2 + v2 D-Concern5 revoke 期间历史验签)
- 所有 sign key 短期 90 天轮换,过期密钥保留用于历史验签(只读)
- root key 不在任何在线系统出现,任何在线签名密钥失窃**不影响 root**
- KMS 调用全审计;key_usage_rate 突增告警(可能被滥用)
- 应急撤销 SOP:任何 sign key 怀疑泄露 → revoke + rotate + 通知所有历史报告持有者验签自检

**revoke 期间的关键设计(v2 D-Concern5)**:
- Attestation Service **生成新报告暂停**(切只读模式)
- **但 attest.idcd.com/verify 公开验签接口持续可用**:已发的历史报告仍可被验签,使用对应 key_version 的 public key
- `verify` 接口实施:根据请求中的报告 metadata 中的 `signature_key_id + key_version`,从 KMS GetPublicKey 拉取对应版本 public key(过期/撤销的 key 仍保留 public key 只读)
- 用户通知模板:"出于密钥安全考虑,我们正在轮换签名密钥;您手中的报告**仍可在 attest.idcd.com/verify 验证**;新报告生成将在 12-24h 内恢复"

### 7.2 滥用防控(配合 12 模块)
- 拒绝以下目标的 Verdict 报告:
  - 12 §3 所有黑名单类别(政府/金融/友站等)
  - 用户**没有验证所有权**的非公开网站(可选自验:DNS TXT / HTTP file)
  - 单 24 小时被 ≥3 不同用户请求 Verdict 的同一目标(可能恶意诬告)→ 人工审核
- 报告中**禁止主观推断罪责**;LLM 输出必须经过 prompt 约束 + 后处理过滤
- "司法鉴定"字样在产品/营销中**全面禁用**

### 7.3 合规边界
- Verdict 报告作为"一手观测数据"使用合法
- 不冒充司法鉴定意见,不出具"建议判决"类内容
- 司法鉴定所合作通道(S3+):需要鉴定时,Verdict 报告作为输入数据交给合作鉴定所出具正式鉴定结论

### 7.4 报告滥用举报
- 任何被报告对象可在 `attest.idcd.com/dispute/:report_id` 提出申诉
- 平台 24 小时内响应;违规报告下架 + 退款 + 公开 transparency 记录
- 累计 3 次违规的用户冻结 Verdict 权限

---

## 8. 阶段交付清单

### S2(M5-M8)— Evidence MVP
- attest.idcd.com 独立子域 + 独立部署
- KMS 集成 + 首次 root key 仪式(M5-M6)
- RFC3161 TSA 客户端(DigiCert + GlobalSign 双家)
- PDF 签名(PAdES)+ 内嵌时间戳
- 4 个场景模板(SLA / 故障取证 / 合规自证 / 争议取证)
- Verdict 件价 + Compliance Starter 年订
- 公开验签页 `/verify`
- 自检 daily 抽样审计
- 滥用举报通道

### S3(M9-M14)— Evidence GA
- 第三家 TSA 接入(NTSC 国内授时中心)
- LLM 根因分析草拟(改进 prompt + 多模板)
- Compliance Pro 档 + 司法鉴定所合作通道初步对接
- 报告嵌入卡片(iframe / OG)
- 区块链锚定 alpha(以太坊主网 / Polygon,可选)
- 报告分享 token 过期/密码保护
- transparency 公开仪表盘(key ceremony / TSA 健康度 / 历史报告数 / 申诉处理)

### S4(M15+)— Enterprise
- HSM 硬件密钥(YubiHSM / CloudHSM)
- 司法鉴定所深度合作 + 资质背书
- 白标 Attestation API(企业用自家域名出具报告,后台仍用 `idcd` 签)
- 法定保留期延长合规(10 年)
- Compliance Enterprise 议价档

---

## 9. 关键风险

| 风险 | 缓解 |
|---|---|
| 单家 TSA 服务不可用导致全停 | ✅ 主备三家 + 自动切换 |
| 签名密钥泄露 | ✅ 短期密钥 + Root 离线 + 撤销 SOP + 公开 transparency |
| 用户用 Verdict 报告"诬告竞品" | ✅ 目标黑名单 + 所有权可验证 + 滥用申诉通道 + 文案明确"非鉴定结论" |
| LLM 复盘草稿出现幻觉/造谣 | ✅ Prompt 约束 + 必须人工审核 + 输出 sanitize + AI 标识 |
| 报告 PDF 被篡改后蒙混 | ✅ 验签接口 + 内容哈希 + TSA 时间戳;任何篡改立即被检出 |
| 与司法鉴定机构边界争议 | ✅ 全文不用"鉴定"字样;明确定位"一手观测";合作鉴定所通道兜底 |
| 国内"民法 + 个人信息 + 反不正当竞争"红线 | ✅ 12 §3 合规边界 + 法律顾问 review 报告模板 |

---

## 10. 决策记录(已锁定,见 DECISIONS.md §K)

- ✅ **K1** MCP / Attestation / Evidence 三者均独立子域(sub-product 阵型)
- ✅ **K2** 签名密钥起步用云 KMS + 离线 root + 90 天 sign key 轮换;HSM S4 评估
- ✅ **K5** Verdict 4 个件价档(¥199/299/499/999)+ Compliance 3 个年订档(¥3k/12k/30k)
- ✅ **K7** 司法鉴定所合作 deferred 到 S3 中后期
- ⏳ **K-OPEN-1** 区块链锚定具体链选(以太坊 vs Polygon vs ARWeave)— S3 评估时决
- ⏳ **K-OPEN-2** Verdict 报告"过期可读"语义 — 已有报告永久可验签(只读);S2 末敲定 UI 措辞

### 待定(非紧迫)
- [ ] PAdES 签名等级(B-B 起步 vs B-T 含 TSA vs B-LT 长期归档),S2 实施时定
- [ ] 是否允许用户自带签名密钥(企业版 BYOK),S4 评估
