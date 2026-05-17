# 17 · 详细路线图与里程碑(v2.0)

> 关联：OVERVIEW.md §7、所有模块的"阶段交付清单"
> 用途：把分散在各模块的阶段计划聚合，对齐时间、依赖、人力、验收
> 品牌名占位：`idcd`
> v2.0 修订:S2 新增 Evidence MVP(M7-M8);S3 新增 MCP server + Agent obs(M9-M14);LLM 故障复盘从 P3/S4 提至 P1/S2

---

## 1. 总览：4 阶段时间轴

| 阶段 | 月数 | 月份（自项目启动） | 主题 | 主要交付 | 关键转折 |
|---|---|---|---|---|---|
| **S1** | 0–4 月 | M1–M4 | MVP 上线 + SEO 基本盘 | 工具站 + 拨测 + 节点 + 合规 | 公开上线 |
| **S2** | 4–8 月 | M5–M8 | 商业化启动 | 监控告警 + 状态页 + 订阅付费 + API beta | 首笔 MRR |
| **S3** | 8–14 月 | M9–M14 | 规模化 + API GA | API + SDK + CLI + 团队 + 众包节点 | MRR > ¥50k |
| **S4** | 14+ 月 | M15+ | 企业化 | 私有部署 + SSO + 销售驱动 | 首张企业合同 |

---

## 2. 按月细分（M1-M14 详细）

### M1：基础设施 + 节点骨架
- 仓库初始化（monorepo / CI）
- Cloudflare + 域名 + 公司主体 ICP 备案启动
- PostgreSQL + TimescaleDB + Redis 部署
- Agent 1.0 原型：HTTP + Ping
- Agent Gateway WSS + mTLS CA
- 5-10 个种子节点部署
- 内部 dogfood：自家产品监控自家
- 合规四件套草稿（服务条款 / 隐私 / Cookie / AUP）

### M2：拨测能力齐全
- Agent 1.0 完整能力：HTTP/Ping/TCPing/DNS/Traceroute/MTR
- Scheduler 基础（筛选 + 打分 + 错峰）
- 节点心跳 + 自动 drain
- 反滥用底盘：黑名单 + Turnstile + 限速
- API Gateway v1 alpha（公开拨测）
- 节点扩至 30+

### M3：前端工具站
- Next.js 主站 + 50 个工具页
- 一键诊断 + 报告分享
- 公开节点目录 `/nodes`
- SEO 基础（sitemap / robots / Schema.org / hreflang）
- 基础账号系统（邮箱注册 + 验证）
- 帮助中心 30 篇
- 节点扩至 60+

### M4：S1 收官 + 免费证书 S1
**v2 NEW(免费证书 S1,详 [20-free-cert.md §15.1](./20-free-cert.md#151-s14-周)):**
- W1 cert.* schema 落地 + apps/cert-svc HTTP 骨架 + Let's Encrypt adapter
- W2 ACME 完整状态机 + WAL 重放 + Pebble 集成测试
- W3 Cloudflare DNS provider + 手动模式 + DNS 凭据加密
- W4 前端申请向导 + 订单页 + 下载弹窗 + staging 端到端冒烟

**S1 验收(证书)**:登录用户 5 分钟内完成 Cloudflare 模式签发 `*.example.com` 并部署 Nginx 验证 HTTPS。

- 节点扩至 100+
- 完整反滥用测试（红队演练）
- ICP 备案完成
- 公开内测（限量邀请 100 人）
- SEO 提交各搜索引擎
- 监控（Prometheus / Loki / Sentry）就位
- 公开发布
- **v2 NEW: S1 末 30 天 baseline 数据采集启动**(D10):为 Anchor 偏差阈值校准准备
- ~~Backup HSM 采购(D11)~~ — **2026-05-13 Pre-4 调整为 S4 才补**;S2 仅走 12h Shamir 主路径,接受 SLA 风险

### M5：监控模块 + Evidence 信任根准备(v2)
- Monitor Service（M01-M08 完整）
- 反误报机制（quorum + 连续 N 次）
- 维护窗口 + 分组 + 标签
- 监控列表 / 详情 / 创建
- 客户端 SSE 推送实时结果
- 海外公司主体注册（Paddle 收款主体准备）
- **v2 NEW: KMS 选型确定(AWS KMS 或阿里云 KMS,据收款主体定)**
- **v2 NEW: Root key 仪式准备(寻找 5 个 quorum 持有者 + 律所公证流程)**
- **v2 NEW: Anchor 阈值 calibration 报告**(D10):基于 S1 末 30 天 baseline 数据完成 + 写入 admin 配置
- **v2 NEW: LLM 复盘 eval 数据集首版 bootstrap**(D8,~25h):30 公开事故(AWS/CF/Azure)+ 20 内部 dogfood,创始人手动标注

### M6：告警模块 + Evidence 核心实现(v2)
- Alert Service 完整
- 通道：邮件 + Webhook + 企微 + 钉钉 + 飞书 + 微信 + Telegram + Slack + Discord
- 告警策略 + 升级 + 抑制
- 模板系统 + 测试发送
- 通道健康度自检
- 历史事件 + Ack + 评论
- **v2 NEW: attest.idcd.com 子域 + 独立部署**
- **v2 NEW: KMS 集成 + 首次 Root key 仪式(录像 + 公证)**
- **v2 NEW: KMS 应急 SOP 模拟演练**(D11,1-2 days):S2 上线前必做,演练 **12h Shamir 3-of-5 主路径**(Backup HSM 加速通道 S4 才补),实测每步耗时基线 + 5 持有人响应时间
- **v2 NEW: RFC3161 TSA 客户端(DigiCert + GlobalSign 双家)**
- **v2 NEW: PDF 签名(PAdES B-T)+ 内嵌时间戳**
- **v2 NEW: 公开验签接口 /verify + Attestation Worker(D6 Self-Verify Worker 独立部署)**
- **v2 NEW: attestation_record WAL 实施**(D4):step-level idempotency + KMS sign idempotency token

### M7：状态页 + 计费 + Evidence MVP 上线(v2)
- 状态页（含自动事件创建 + 自定义域 + ACME）
- 订阅档位 + 月年付
- **主支付通道：Paddle（MoR，含微信支付 + 支付宝 + 卡）**
- 余额 + 发票（Paddle 自动出具）
- 退款（自助 + 工单）
- 首笔商业订单
- *国内自家微信 / 支付宝商户号：S3+ 视情况通过合作方代收引入*
- **v2 NEW: Verdict 件价 4 模板上线(¥199/299/499/999)**
- **v2 NEW: Compliance Starter 年订(¥3k)上线**
- **v2 NEW: 13.5 WAL 状态机(D4)+ 自检失败自动退款流程(D5)+ 工单 SLA 三档(D12)**
- **v2 NEW: Verdict 失败链路 staging 演示**(D4/D5/D6,CRITICAL GAP 必演示):注入 KMS/TSA/S3/refund 失败,验证 30min 内用户收到道歉邮箱 + DLQ 告警
- **v2 NEW: Verdict 滥用举报通道 + 申诉流程**

### M8：S2 收官 + Evidence 稳定运行(v2)
- API beta 邀请制开放
- OpenAPI 文档站（Mintlify / 自建 Nextra）
- 管理后台主要功能
- 数据看板基础
- 用户案例 5 篇
- 内容运营启动（博客 5-10 篇 + **/leaderboard 首篇 CDN 月度报告**)
- 公开商业化（去除"内测"标签）
- **v2 NEW: LLM 故障复盘自动起草上线(从 P3 提前)— 离线 eval ≥ 4.0/5 才上线**
- **v2 NEW: Verdict 自检 daily 抽样审计 + transparency 页**
- **v2 NEW: Verdict 件价首月销售目标 ≥ 50 份**
- **v2 NEW(免费证书 S2 收官,详 [20-free-cert.md §15.2](./20-free-cert.md#152-s24-周))**:
  - W5 ZeroSSL + Buypass adapter + CA 路由策略(70% 配额阈值切备份)+ CAA 预检
  - W6 阿里云 / DNSPod / Route53 / GCP DNS provider + 真 KMS 接入(阿里云 / AWS / Vault 三路径)
  - W7 自动续期 cron + retry 队列(3 次后停 + 强告警)+ 撤销 + PKCS#12 导出
  - W8 abuse-detection + admin 面板 + Prometheus /metrics + PRD 8 项文档同步

### M9：API 完善 + MCP server alpha 准备(v2)
- API v1 公开 GA（仍可邀请制提高限速）
- 沙箱（test Key）
- API 用量统计与计费集成
- Webhook 接收完整
- 自定义仪表盘 v1
- SLA 月报基础
- **v2 NEW: mcp.idcd.com 独立子域 + 基础部署**
- **v2 NEW: MCP 协议(Anthropic spec)实现:JSON-RPC stdio + HTTP+SSE**
- **v2 NEW: Personal token 签发/撤销机制**

### M10：节点 + 浏览器监控 + MCP 核心 tools(v2)
- 浏览器监控 M11（Headless Chrome 独立集群）
- Speedtest 节点
- IPv6 全面支持
- 节点 OTA 灰度升级(3 级灰度 1%/10%/100%)
- 节点扩至 150+
- Anchor 锚定基准 + **Anchor 偏差实时告警上线**
- 高级节点告警与诊断
- **v2 NEW: MCP 8 核心 tools(ping/http/dns/trace/ssl/diagnose/ip/whois)**
- **v2 NEW: MCP alpha 邀请制(100 开发者)**
- **v2 NEW: Cursor + Claude Code 兼容性测试**

### M11：SDK + CLI + Agent obs 监控类型(v2)
- 官方 SDK：JS / Go / Python
- CLI 工具完整
- "按量纯付费"档（开发者市场）
- Terraform Provider
- 推荐返利上线
- 主机商 CPS 接入
- **v2 NEW: Agent obs 监控类型 M21/M22/M23(LLM endpoint / Tool API / RAG)**
- **v2 NEW: idcd-mcp-py + idcd-mcp-ts 自家 SDK(MIT 开源 + npm/pypi 发布)**
- **v2 NEW: Compliance Pro 年订(¥12k)上线**

### M12：团队 + 协作 + MCP GA(v2)
- Team / Org（多用户）
- 角色权限完整
- 团队级 API Key + 订阅
- 强制 2FA（团队配置）
- WebAuthn / Passkey
- 钉钉 / 飞书登录
- 团队级状态页 / 监控
- **v2 NEW: MCP server GA:13 tool 全量 + Workspace + Service account token**
- **v2 NEW: Codex CLI 兼容性 + Anthropic MCP gallery 提交**
- **v2 NEW: Agent Pro 档(¥299/月)上线**
- **v2 NEW: MCP 文档站 mcp.idcd.com/docs 上线**

### M13：众包节点 + Evidence 增强(v2)
- Agent 开源（MIT）
- 众包节点完整闭环：申请 → 自助加入 → 自动观察 → 自动入池 → 三级自动剔除
- 反作弊：指纹 + 蜜罐 + 一致性 + Echo
- 积分体系 + 兑换
- 申诉流程
- 节点贡献排行榜
- 出海英文站雏形
- **v2 NEW: 第三家 TSA 接入(NTSC 国家授时中心)— 提升国内司法场景认可度**
- **v2 NEW: 司法鉴定所合作通道初步对接(1-2 家)**
- **v2 NEW: 报告嵌入卡片(iframe / OG)**

### M14：S3 收官 + Evidence GA + Agent obs GA(v2) + 免费证书 S3
- 出海英文站完整（500+ 工具页 + 50 篇博客 + **/leaderboard 中英双语**）
- 排班（On-Call rotation）
- 告警噪音分析
- 月/季度 SLA 报告
- 故障复盘自动起草(M8 已上线,M14 完善 prompt + 多模板)
- 状态页订阅推送（Webhook / 微信 / 钉钉）
- 移动端响应式优化
- MRR ¥50k+ 目标(v2 含 Verdict + Compliance + Agent Pro 占 30%)
- **v2 NEW: 区块链锚定 alpha(Polygon 或 Arweave,可选 add-on)**
- **v2 NEW: transparency 公开仪表盘(key ceremony / TSA 健康度 / 节点 / 申诉)**
- **v2 NEW: MCP 月调用量 ≥ 1M;Agent obs 监控数 ≥ 5,000**
- **v2 NEW: TimescaleDB 容量评估报告 + ClickHouse PoC**(D14):S3 末必备,基于单日 monitor_check 新增 + P99 write latency 监控指标,准备好 CK 代码与切换 SOP
- **v2 NEW: KMS 应急 SOP 年度演练**(D11):S3 末再做 1 次模拟,与 M6 演练数据对比 + 监控趋势
- **v2 NEW(免费证书 S3,详 [20-free-cert.md §15.3](./20-free-cert.md#153-s34-6-周))**:
  - 付费 CA 通道试水(GoGetSSL / DigiCert reseller,详 §20)
  - 团队席位 + 凭据共享(企业向)
  - Webhook 通知(签发 / 续期 / 撤销事件外推)
  - D-FC-10 MCP tool 评估(与 19-ai-agent 协同)

### M15+（S4，按需展开)
- Enterprise 档定价正式上线
- SSO（SAML / OIDC）
- 私有部署（On-Premises）
- 专属节点
- 白标状态页
- 多步骤事务监控
- 销售线索系统
- 合同管理
- 等保 三级 / SOC 2
- **v2 NEW: Compliance Enterprise 年订(¥30k 议价档)**
- **v2 NEW: HSM 硬件密钥升级(企业 due diligence 触发)**
- **v2 NEW: 白标 Attestation API + 白标 MCP server**
- **v2 NEW: Agent Output Quality 监控 M24**
- **v2 NEW: BYOK(自带签名密钥,企业版)**

---

## 3. 模块化时间线对照表

> 横轴月份，纵轴模块。● 主要交付月。

```
模块           M1  M2  M3  M4  M5  M6  M7  M8  M9  M10 M11 M12 M13 M14 M15+
─────────────────────────────────────────────────────────────────────────────
02 公开工具    ●   ●   ●   ●   ·   ●   ·   ·   ●   ·   ·   ·   ·   ·   ·
03 账号        ·   ●   ●   ●   ·   ·   ·   ●   ·   ●   ·   ●   ·   ·   ●
04 监控        ·   ·   ·   ·   ●   ·   ·   ·   ·   ●   ●   ·   ·   ·   ●
05 告警        ·   ·   ·   ·   ·   ●   ·   ·   ·   ·   ·   ·   ·   ●   ·
06 状态页      ·   ·   ·   ·   ·   ·   ●   ·   ·   ·   ·   ·   ·   ●   ●
07 报表        ·   ·   ·   ·   ·   ·   ·   ●   ●   ·   ·   ·   ·   ●   ●
08 API         ·   ●   ·   ·   ·   ·   ·   ●   ●   ·   ●   ·   ·   ·   ●
09 计费        ·   ·   ·   ·   ·   ·   ●   ●   ●   ·   ●   ●   ·   ·   ●
10 节点        ●   ●   ●   ●   ·   ·   ·   ·   ·   ●   ·   ·   ●   ·   ●
11 后台        ●   ●   ·   ●   ·   ●   ●   ●   ·   ·   ·   ·   ●   ·   ●
12 合规        ●   ●   ●   ●   ·   ·   ●   ●   ·   ·   ·   ·   ·   ·   ●
13 SEO         ·   ·   ●   ●   ·   ·   ·   ●   ·   ·   ·   ·   ·   ●   ·
14 技术        ●   ●   ●   ●   ●   ●   ●   ·   ●   ●   ·   ●   ·   ·   ●
15 数据模型    ●   ●   ●   ·   ●   ●   ●   ·   ·   ·   ●   ●   ●   ·   ·
16 API spec    ·   ●   ·   ·   ·   ·   ●   ●   ●   ·   ●   ●   ·   ·   ●
18 Evidence(v2)·   ·   ·   ·   ●   ●   ●   ●   ·   ·   ·   ·   ●   ●   ●
19 AI/MCP(v2)  ·   ·   ·   ·   ·   ·   ·   ●   ●   ●   ●   ●   ·   ●   ●
```

---

## 4. 关键依赖关系(v2)

```
节点系统 (10) ─┬─→ 公开工具 (02)
              ├─→ 监控 (04)
              ├─→ 一键诊断
              ├─→ Evidence (18) ─ 报告引用的节点必须在公开目录 + Anchor 偏差检测
              └─→ MCP (19) ─ 100 节点是 Agent obs 的天然 backbone

合规 (12) ─┬─→ 所有公开功能（前置）
          ├─→ 计费 (10) 涉及税务
          ├─→ 后台 (11) 审计
          ├─→ Evidence (18) 非鉴定结论声明 + 滥用举报
          └─→ MCP (19) 凭证泄露应急

监控 (04) ─→ 告警 (05) ─→ 状态页 (06)
                       └→ 报表 (07) ─→ Evidence (18) (Verdict 是 reports 的 supercharged)
                                  └→ LLM 故障复盘起草 (v2 P1 提前)

账号 (03) ─→ 计费 (09) ─→ API 限速 (08)
                       ├→ 团队 / 状态页归属
                       ├→ Verdict 件价订单 / Compliance 年订
                       └→ MCP token / Agent Pro 档

API (08) + 计费 (09) ─→ 公开商业化
              └→ MCP (19) ─ MCP server 独立子域,API 配额共享但计量独立

KMS / TSA ─→ Evidence (18) ─ 信任根
LLM 服务 ─┬→ Evidence (18) ─ 报告解读 + 根因建议
        └→ MCP / Agent obs (19) ─ 故障复盘起草

技术架构 (14) → 所有其他模块(v2: 含 attest / mcp / kms 三栈)
数据模型 (15) → 所有需持久化模块(v2: 新增 verdict_* / mcp_* / agent_obs_*)
API spec (16) ↔ API (08) ↔ SDK (08 §15) + MCP(19) + Verdict(18)

SEO (13) ─→ 主要靠 02 公开工具页 + /leaderboard(v2 NEW) + /verdict 报告分享页(v2)
```

---

## 5. 人力假设与角色

### 5.1 S1（M1–M4）—— 极简启动
- **创始团队 1-2 人**（一人多角色）
  - 全栈 + 运维
  - SEO 内容（外包部分文案）

### 5.2 S2（M5–M8）—— 商业化期
- 全栈工程师 2-3 人
- 前端工程师 1 人
- 产品 / 客服 0.5 人（创始人兼）
- 设计 0.5 人（外包 / 兼职）

### 5.3 S3（M9–M14）—— 扩张期
- 后端 2-3 人
- 前端 2 人
- DevOps / SRE 1 人
- 安全 / 反滥用 0.5-1 人
- 产品 1 人
- 客户成功 / 客服 1-2 人
- 内容 / 运营 1 人

### 5.4 S4（M15+）—— 企业化
- + 销售 2 人
- + 客户成功（企业向）1-2 人
- + 法务 / 合规 0.5 人（外包）
- + 财务 0.5 人（外包）

### 5.5 招聘节奏
- M2 招第一个工程师（前端 / 全栈）
- M6 招第二个工程师 + 设计兼职
- M9 招 DevOps / SRE
- M11 招内容运营
- M13 招客户成功
- M15+ 启动销售岗

---

## 6. 里程碑事件（外部可见）

| 里程碑 | 月份 | 内容 |
|---|---|---|
| **域名 + 公司主体备案** | M1 | 国内合规起点 |
| **节点 100+ 上线** | M4 | 产品可信度基础 |
| **公开内测** | M4 | 100 邀请用户 |
| **公开发布** | M4 | 解除邀请限制 |
| **首笔商业订单** | M7 | 商业化里程碑 |
| **首份 Verdict 报告售出(v2)** | M7 | Evidence 商业化里程碑 |
| **首笔 Compliance 年订(v2)** | M8 | 企业商业化里程碑 |
| **MRR 突破 ¥10k** | M8 | S2 KPI |
| **API beta** | M8 | 开发者市场启动 |
| **API GA** | M9 | 开发者正式可用 |
| **MCP server alpha(v2)** | M10 | Agent 时代启动 |
| **MCP server GA + Anthropic MCP gallery 上架(v2)** | M12 | Agent 公开市场 |
| **众包节点开放** | M13 | 节点生态启动 |
| **MRR 突破 ¥50k** | M14 | S3 KPI(v2 含 Evidence + Compliance + Agent Pro 占 30%) |
| **出海英文站完整** | M14 | 国际化里程碑 |
| **企业版定价上线** | M15 | S4 启动 |
| **首张企业合同(含 Compliance Enterprise)** | M16-18 | S4 验证 |
| **MRR ¥200k+** | M24 | 年度目标(v2 Evidence + Compliance 翻倍贡献) |

---

## 7. 验收标准（每阶段）

### S1 验收（M4 末）
- ✅ 100+ 节点稳定运行（成功率 > 95%）
- ✅ ICP 备案完成 + 服务可访问
- ✅ 50+ 工具页 SEO 文案完整，至少 100 关键词进百度首页
- ✅ 一键诊断可生成可分享报告
- ✅ 反滥用底盘上线，红队演练通过
- ✅ 内部 dogfood 监控自家
- ✅ 公开发布 + 公关稿
- ✅ 日 UV 1000+
- ✅ 注册用户 500+

### S2 验收（M8 末)
- ✅ 监控类型 M01-M09 完整
- ✅ 告警通道 9 个上线
- ✅ 状态页（含自定义域 + ACME）
- ✅ 4 档订阅 + 聚合支付完整
- ✅ 海外公司主体注册完成（Paddle 收款主体已就位）
- ✅ MRR ≥ ¥10k(含 Verdict + Compliance 贡献 ≥ ¥3k)
- ✅ 付费用户 200+
- ✅ API beta 邀请制开放
- ✅ 日 UV 5000+
- ✅ 注册用户 5000+
- ✅ **v2 NEW: Verdict 件价售出 ≥ 100 份**
- ✅ **v2 NEW: Compliance Starter 年订 ≥ 5 个**
- ✅ **v2 NEW: Root key 仪式完成 + 公开 transparency**
- ✅ **v2 NEW: LLM 故障复盘 eval ≥ 4.0/5 上线**
- ✅ **v2 NEW: /leaderboard 首篇 CDN 月度报告发布**

### S3 验收（M14 末)
- ✅ API v1 GA + 3 个 SDK + CLI
- ✅ 团队 / 角色 / API Key 团队级
- ✅ 浏览器监控
- ✅ 众包节点开放，500+ 众包节点
- ✅ 出海英文站完整
- ✅ 排班 + SLA 报告
- ✅ MRR ≥ ¥50k(v2 含 Evidence + Compliance + Agent Pro 占 30% = ¥15k)
- ✅ 付费用户 1500+
- ✅ 日 UV 50,000+
- ✅ 注册用户 50,000+
- ✅ API 月调用量 ≥ 50M
- ✅ **v2 NEW: MCP 月调用量 ≥ 1M;Agent obs 监控数 ≥ 5,000**
- ✅ **v2 NEW: Compliance Pro 年订 ≥ 30 个**
- ✅ **v2 NEW: Verdict 月生成 ≥ 1,000 份;自检通过率 100%**
- ✅ **v2 NEW: MCP gallery 提交 + Anthropic 官方收录**
- ✅ **v2 NEW: NTSC 第三家 TSA 接入 + 司法鉴定所 1-2 家初步合作**

### S4 验收（M24 末，远期目标）
- ✅ Enterprise 档付费用户 ≥ 5
- ✅ 私有部署案例 1-3 个
- ✅ MRR ≥ ¥200k
- ✅ ARR ≥ ¥2.5M
- ✅ NRR ≥ 105%
- ✅ Churn ≤ 3%

---

## 8. 风险与应对

### 8.1 进度风险

| 风险 | 应对 |
|---|---|
| ICP 备案延迟 | 节点先在海外起；公司主体提前注册 |
| **经营性 ICP 不办** ⚠️ | **决策 C8：不办**。主走 Paddle MoR（境外主体收款）；S3 视情况引入合作方代收。详见 DECISIONS.md §H1 |
| Agent 复杂度被低估 | M1-M2 做最简版，后续迭代 |
| SEO 起势慢 | 内容运营前置 + 社区曝光双路 |
| 反滥用规则被绕过 | 红队演练 + 第三方安全审查 |
| **v2 NEW: Evidence MVP 延期(Root key 仪式 + KMS 集成)** | M5 启动 KMS 选型 + Root key 仪式准备,M6 仪式 + 集成,M7 上线;若延期回退到 only sign(无 TSA)灰度 |
| **v2 NEW: LLM 故障复盘 eval < 4.0/5 阻塞上线** | 默认不上线;若 M8 仍未达标,延后到 M10-M11,期间仅人工模板 |
| **v2 NEW: MCP server 客户端兼容性差** | M9 alpha 严控测试矩阵;Cursor/Claude Code 客户端版本相对稳定,Codex 兼容性 M12 之前可选 |

### 8.2 市场风险

| 风险 | 应对 |
|---|---|
| 同行降价 / 加速跟进 | 差异化（API + 一键诊断 + 国内化）+ 不打价格战 |
| 同行 DDoS / 投诉 | Cloudflare 全套 + 法务应对预案 |
| 政策变化（拨测被监管） | 主动沟通 + 合规优先 + 关键功能砍 |
| 经济下行影响付费 | 免费档保活 + Pro 价格亲民 + 重运营 |

### 8.3 团队风险

| 风险 | 应对 |
|---|---|
| 招人慢 / 找不到合适 | 远程招聘 + 早期高股权吸引 |
| 创始人精力分散 | 明确分工 + 早期外包非核心 |
| 关键人员离开 | 文档完整 + 关键密钥多份保管 |

### 8.4 财务风险

| 风险 | 应对 |
|---|---|
| 烧钱过快 | 节点成本严控 + 早期免费档限严 |
| 收款卡顿（微信 / 支付宝） | Paddle 兜底 + 多渠道 |
| 退款率高 | 严控质量 + 客服培训 |

---

## 9. 调整机制

### 9.1 季度复盘
- 每季度末检视：实际 vs 计划，根因分析
- 调整下季度计划

### 9.2 月度 OKR
- 每月初对齐 OKR（结果导向）
- 月末打分

### 9.3 双周冲刺
- 工程双周冲刺
- 站立会 + 回顾 + 计划

### 9.4 关键决策
- 涉及阶段调整：创始人决策 + 团队共识
- 涉及方向调整：复盘 + 数据驱动 + 不轻易转向

---

## 10. 资源需求估算

### 10.1 S1 起步（M1–M4）

| 项目 | 金额（人民币） |
|---|---|
| 节点 VPS（100 个低配） | ~¥6,000（年付折算 ¥1,500/月）|
| 控制集群 VPS（杭州 + 法兰克福）| ~¥600/月 |
| Cloudflare（Free 起步，Pro 后期） | ¥0-200/月 |
| 域名续费 + 商标注册 | ¥3,000 一次性 |
| 设计外包 | ¥5,000-10,000 |
| 营业执照 / 公司注册 | ¥1,000 |
| 等保咨询（暂缓） | — |
| **总计 4 个月** | **~¥35,000-50,000** |

### 10.2 S2（M5–M8）

| 项目 | 金额（人民币） |
|---|---|
| 节点（保持） | ¥1,500/月 |
| 控制集群（扩容） | ¥1,500/月 |
| 工程师工资 × 2 | ¥30,000/月 起 |
| 海外公司主体注册（香港 / 新加坡） | ¥5,000-15,000 |
| Paddle 接入 / KYC | ¥0（免费）|
| 短信 / 邮件 通道预付 | ¥2,000 |
| 内容外包 | ¥5,000/月 |
| **总计 4 个月** | **~¥200,000-300,000** |

### 10.3 S3（M9–M14）—— 月支出加速

预计每月 ¥100,000-200,000（含 4-6 人团队），半年 ~¥600k-1.2M。

需 MRR ¥50k+ 自给 + 留足 6-12 月现金缓冲。

### 10.4 融资节点（如需要）
- M6-M8：天使 / 种子轮（已有 MVP + 早期收入 + 数据）
- M14-M18：A 轮（验证 PMF + 月增长）

---

## 11. 阶段对应章节交付物（汇总）

### S1 交付（所有模块的 S1 部分）
| 模块 | S1 交付要点 |
|---|---|
| 02 | 50+ 工具页 + 一键诊断 + 报告分享 |
| 03 | 邮箱注册 / 登录 / 验证 / 找回 |
| 10 | 100+ 节点 + Agent 1.0 + Scheduler + 调度 |
| 12 | 备案 + 协议 + 黑名单 + 限速 + Turnstile |
| 13 | SEO 基础 + sitemap + 50 工具页文案 |
| 14 | 仓库 + Cloudflare + PG/Timescale/Redis + mTLS |
| 15 | 用户 / 节点 / 任务 / 缓存表 |
| 11 | 节点 / 黑名单 / 用户基础后台 |

### S2 交付
| 模块 | S2 交付要点 |
|---|---|
| 03 | OAuth / 2FA / API Key 完整 |
| 04 | M01-M09 监控类型完整 |
| 05 | 9 通道 + 策略 + 升级 + 抑制 + 模板 |
| 06 | 状态页 + 自定义域 + ACME + 订阅 + **LLM 自动起草 incident(v2)** |
| 07 | 仪表盘 + 月度 SLA + **LLM 故障复盘自动起草(v2 P1 提前)** |
| 08 | API beta + 文档站 + 沙箱 |
| 09 | 4 档订阅 + 聚合支付 + 发票 + 退款 + **Verdict 件价 + Compliance Starter(v2)** |
| 11 | 工单 + 订单 + 内容运营 + 数据看板 + **KMS 仪式后台 + Verdict 工单(v2)** |
| 12 | + **Verdict 非鉴定声明 + 排行榜权威白名单(v2)** |
| 13 | 博客 + 案例 + 内容运营后台 + **/leaderboard 首篇月度报告(v2)** |
| **18 (v2 NEW)** | Evidence MVP:attest.idcd.com + KMS + Root 仪式 + RFC3161 双 TSA + PDF 签名 + 公开验签 + 4 模板 + 自检 daily + 滥用举报 |

### S3 交付
| 模块 | S3 交付要点 |
|---|---|
| 03 | 团队 / Org / WebAuthn + **MCP token 三种形态(v2)** |
| 04 | 浏览器监控 / 心跳 / Terraform + **Agent obs M21/M22/M23 监控类型(v2)** |
| 05 | 短信 / 排班 / PagerDuty |
| 06 | 私有 / 嵌入 / Slack 集成 |
| 07 | 自定义仪表盘 / 高级分析 |
| 08 | SDK / CLI / API GA + **MCP 章节指向 19(v2)** |
| 09 | 出海定价 / 14 天试用 / 余额提现 + **Compliance Pro + Agent Pro 档(v2)** |
| 10 | 众包完整闭环 + Anchor + OTA + **Anchor 偏差实时告警(v2)** |
| 11 | 风险评分 / 移动端应急 / SQL 查询 |
| 12 | 等保 / 渗透测试 / Bug Bounty |
| 13 | 出海英文站完整 + **/leaderboard 中英双语持续 + MCP 集成博客(v2)** |
| **18 (v2 NEW)** | Evidence GA:NTSC 第三家 TSA + LLM 解读 + Compliance Pro + 报告嵌入卡片 + 区块链锚定 alpha + 司法鉴定所 1-2 家 + transparency 公开仪表盘 |
| **19 (v2 NEW)** | MCP alpha + GA(13 tool 全量)+ 自家 SDK + Codex 兼容 + Anthropic gallery + Agent obs M21-M23 上线 + Agent Pro 档 |

### S4 交付
| 模块 | S4 交付要点 |
|---|---|
| 03 | SAML / SSO / SCIM |
| 04 | 多步骤事务 / 高级 SLA |
| 06 | 白标 / 多状态页 |
| 09 | Enterprise / 合同 / 信用付 + **Compliance Enterprise(¥30k 议价)(v2)** |
| 10 | 专属节点 / 私有部署 |
| 11 | CRM / 销售线索 |
| 12 | 等保三级 / SOC 2 |
| **18 (v2 NEW)** | HSM 硬件密钥 + 司法鉴定所深度合作 + 白标 Attestation API + 法定保留 10 年 |
| **19 (v2 NEW)** | OpenAI Agents Protocol(若需)+ 白标 MCP server + M24 Agent Output Quality |

---

## 12. 决策点汇总（所有模块的 Open Decisions）

随着 PRD 完成，已累积约 100+ 开放决策点散布在各模块。建议在以下里程碑前集中评审：

- **M0 启动前**：品牌名 / 域名细节 / 国内主控厂商 / 节点选择
- **M3 公开前**：定价档具体数字 / 免费额度 / 注销冷静期 / Free 状态页 watermark 文案
- **M6 商业化前**：发票流程 / 退款政策 / 推荐返利比例 / 学生折扣
- **M9 API GA 前**：API 限速具体数字 / 沙箱 mock 目标 / SDK 仓库结构
- **M13 众包前**：申请门槛 / 积分提现 / 反作弊阈值
- **M14 出海前**：英文版子路径 vs 子域 / Stripe 是否启用

---

## 13. 文档更新与责任

- 本 PRD 是 v1 蓝图，**每季度复盘 + 更新**
- 重大决策变更必须更新对应模块 PRD
- 实际执行偏差累积 → 提前调整路线图
- 团队成员入职第一周读完 OVERVIEW + 自己模块 PRD
- PRD 与代码不同步即视为重大问题，CI 加 lint 提醒

---

## 14. 收尾

至此 PRD v2.0 完成(v1.0 + EXPANSION 增量)。下一步：

1. **品牌名最终决策**（仍 pending，影响所有文案）
2. **v1.0 的 8 个关键决策点的具体数字敲定**（定价 / 配额等）
3. **v2.0 的 9 项 K 决策已锁(见 DECISIONS.md §K),后续监督执行**
4. **v2.0 的 8 项 Reviewer Concerns 列为实施前必处理项(见 DECISIONS.md §L)**
5. **进入工程实施**：按 M1 计划启动;v2 增量 M5-M14 按本文档执行
6. **季度复盘**：每季度回看 PRD 与实际，调整下季度方向
7. **PRD 三层重组**:S3 中期把 PRD 重组为 Core(02-07) / Extension(18-19) / Platform(03/08-16) 三层目录,降低新员工 onboarding 成本
