# idcd.com 产品需求文档 — 高层总览（OVERVIEW）

> 版本：**v2.0（EXPANSION 模式 / 2026-05-12 + Eng Review 锁定 / 2026-05-13)** — 在 v1.0 基础上叠加 Evidence-as-a-Service + AI Agent Observability + MCP server 三栈
> 域名：**idcd.com 单域名，中英双语同站**（`/zh` `/en` 子路径或 Accept-Language 自动）
> 状态：20 份模块 PRD(含 18-evidence / 19-ai-agent) + DECISIONS.md(§K 9 项 EXPANSION 决策 + §M 14 项 Eng Review 决策)就绪
> 工程实施前必读:**ENG-REVIEW-REPORT.md**(verdict + 18 finding) + **ENG-REVIEW-TODOS.md**(11 项 TODO + Worktree 并行化)
> 文档维护：本文件是全景骨架；具体细节看各模块 PRD；所有决策汇总见 **DECISIONS.md**
> v2.0 变更摘要：详见 §11.K(scope)/ §11.M(eng review)/ DECISIONS §K + §M / 2026-05-12 CEO Plan + 2026-05-13 Eng Review

---

## 0. 文档说明

- 本 OVERVIEW 用于**对齐产品全貌**：愿景、定位、用户、功能版图、商业模型、合规、阶段。
- 不包含字段级、接口级细节，那些会在各模块 PRD 中展开。
- 优先级：`P0` 必做核心、`P1` 重要、`P2` 加分、`P3` 远期/可选。
- 阶段：`S1` MVP 上线（0–4 月）、`S2` 商业化（4–8 月）、`S3` 规模化（8–14 月）、`S4` 企业化（14 月+）。

---

## 1. 产品愿景与定位

### 1.1 一句话定位

**面向中文互联网的网络可观测平台 + 可证据的网络第三方公证 + AI Agent 时代的可观测枢纽**——三栈一体的全栈网络质量服务。

### 1.2 长版定位(v2.0 重写)

idcd 是**三栈叠加**的产品阵型，不是单一品类的"另一个监控 SaaS"：

**栈 1：基础工具与监控 SaaS(v1.0 已有)**
- **个人站长 / 独立开发者**：低门槛工具、一键诊断、好用即买单
- **中小企业运维 / SRE**：监控告警替代监控宝 / UptimeRobot，价格亲民、三网拨测、本地化强
- **DevOps 工具链开发者**：通过 API 把全球节点的网络数据嵌入自家产品

**栈 2：Evidence-as-a-Service(2026 v2.0 新增,详见 18-evidence)**
- **企业法务 / 采购 / 合规岗**：要在 SLA 索赔 / 故障取证 / 等保自证 / 网络纠纷场景中拿到**可被独立验证的第三方观测证据**(签名 + RFC3161 时间戳 + 多节点交叉验证)
- 售价 ¥199-999/份(件价) 或 ¥3k-30k/年(企业年订)
- 不冒充司法鉴定结论；定位为"一手观测数据 + 第三方背书"
- **护城河 = trust 积累 + 时间戳沉淀 + 公开验签**，3 个月内不可被同行复制

**栈 3：AI Agent Observability + MCP Server(2026 v2.0 新增,详见 19-ai-agent)**
- **企业 AI Agent / LLM 应用团队**：Agent 调用 N 个 LLM/工具/API，任何出口故障 = 整链失败；需要从全球 N 个区域持续验证 LLM endpoint / tool / RAG 端点稳定性
- **使用 Cursor / Claude Code / Codex 的开发者**：通过 `mcp.idcd.com` 直接 import，在 AI 助手会话中调用 `idcd_ping` / `idcd_diagnose` 等工具
- 售价 $99-999/月 (Agent Pro 档) + Compliance Enterprise 年订绑定

三栈共享 100 节点 + 反滥用底盘 + 账号系统 + 计费系统，但**独立子域 / 独立计量 / 独立 SLA / 独立 PRD 模块**(sub-product 阵型,见决策 §K1)。

### 1.3 产品价值主张（差异化）

| 维度 | 同类竞品 | idcd v2.0 差异化 |
|---|---|---|
| 节点定义 | 数字注水（按线路拆分） | 透明节点目录（每节点公开 ASN、运营商、IP 段） |
| 测试体验 | 一个工具一个页面 | "一键诊断报告"（DNS+SSL+HTTP+Ping+备案+路由 一站式） |
| 监控 SaaS | 监控宝偏贵、UptimeRobot 偏国外 | 国内化定价 + 海外节点覆盖 + 三网真实拨测 |
| API | ipinfo.io 只查 IP | 全栈网络数据（拨测+DNS+SSL+WHOIS+备案+ASN） |
| 数据透明 | 黑盒 | 任意一次测试可复现、可分享（带签名报告 URL） |
| 反滥用 | 弱、被滥用做 DDoS 工具 | 强限速、强黑名单、强可追溯（独家亮点） |
| **可证据 (NEW v2)** | 截屏 / Excel 应付,无第三方背书 | **签名 + RFC3161 时间戳 + 公开验签 + 多节点交叉 + 6 年归档** |
| **AI Agent 集成 (NEW v2)** | 拨测/监控厂商无 MCP server | **`mcp.idcd.com` 独立子域 + 13 个 MCP tool + 三档 token** |
| **故障复盘 AI 起草 (NEW v2)** | 无,或仅企业版深埋 | **P1/S2 上线,LLM 自动草拟时间线/根因/公告 + 强制人工审核** |
| **权威排行榜 (NEW v2)** | 厂商自吹 | **idcd.com/leaderboard 月度 CDN/云独立测评,公共边缘 + 退出通道** |

### 1.4 不做什么（边界）

- 不做端口扫描、漏洞扫描（合规红线）
- 不做应用层 APM、前端性能 RUM（远期 S4 再议）
- 不做内网监控（私有部署版除外）
- 不做"测对手网站"的恶意工具（即便用户付费也拒绝）
- **不冒充司法鉴定结论**(Evidence 模块明确边界,详见 18-evidence §1.2)
- **不替 Agent 执行业务逻辑**(MCP server 仅提供可观测能力,不参与决策)
- **不主动适配所有 Agent 协议**(主跟 Anthropic MCP;OpenAI Agents Protocol 仅在有付费需求时接入)
- **不签发永久 / 长期 token**(v2 D2):所有 MCP token 都有过期日,最长 90 天 auto_renewal;UX 等同永久但失窃损失上限 90 天
- **不允许 verify 接口看到内部签名捷径**(v2 D6):Self-verify Worker 与 Generator Worker 物理隔离,自检走与第三方完全一致的代码路径

---

## 2. 目标用户与场景

### 2.1 用户画像

#### Persona A：站长老王（个人站长）
- 35 岁，运营 3-5 个个人站，月流水几千元，用宝塔面板
- **痛点**：网站偶尔 502、CDN 偶尔抽风，半夜被用户吐槽才知道
- **预期付费**：9.9–29 元/月
- **关键功能**：免费 5 个监控 + 微信/邮件告警 + 状态页

#### Persona B：SRE 小李（中小企业运维）
- 28 岁，互联网公司运维，负责 30+ 微服务的可用性
- **痛点**：内部 Prometheus 监控不了"用户视角"的可用性，CDN 多地不一致难发现
- **预期付费**：99–299 元/月
- **关键功能**：多地拨测、API 接入、Webhook、SLA 报告、团队协作

#### Persona C：DevOps 开发者老张
- 30 岁，给自家产品集成网络数据
- **痛点**：要在产品里展示"用户访问 X 的延迟分布"，自建节点太贵
- **预期付费**：按调用量，月 ¥200–2000
- **关键功能**：稳定 API、SDK、计量准确、文档完善

#### Persona D（远期 S4）：大型企业采购
- 跨国互联网公司，要可定制、可审计、可私有部署
- **预期付费**：年付 5–20 万元
- **关键功能**：白标、专属节点、SLA 合同、SAML SSO、审计日志

### 2.2 核心场景

1. **临时排障**（高频低粘性）：用户网站打不开 → 来 idcd 多地测一下 → 走了
2. **持续监控**（低频高粘性）：注册账号 → 添加监控 → 出问题告警 → 续费
3. **公开状态页**（社交传播）：把 status.xxx.com 挂在 idcd → 给客户看 → 拉新
4. **API 集成**（开发者高 ARPU）：调用 API 嵌入自家产品
5. **网络诊断报告**（专业场景）：故障复盘需要一份"第三方证据" → 生成 PDF 报告

---

## 3. 竞品分析（节选）

| 竞品 | 类型 | 优势 | 劣势 | idcd 差异化策略 |
|---|---|---|---|---|
| boce.com | 国内拨测 | 节点多、SEO 强 | UI 老、无监控告警、无 API | 现代 UI + 监控 SaaS + 开放 API |
| 17ce.com | 国内拨测 | 老牌、知名 | 节点缩水、停滞 | 节点持续运营 + 内容更新 |
| itdog.cn | 国内拨测 | 持续 Ping 体验好 | 被 DDoS 干扰频繁、商业化弱 | 强反滥用 + 商业化清晰 |
| 监控宝 | 商业监控 | 企业客户多 | 贵、UI 陈旧、不接地气 | 个人/中小友好 + 价格亲民 |
| UptimeRobot | 海外监控 | 全球用户多、免费额度大 | 国内节点少、中文弱 | 国内本地化 + 三网节点 |
| Better Stack | 海外监控 | UI 漂亮、状态页强 | 价格贵、纯英文 | 中文 + 国内化定价 |
| Pingdom | 海外监控 | 老牌权威 | 极贵、面向大客户 | 价格优势 |
| Globalping | 海外众包 | 节点多、开源 | 只有 CLI/API、无监控 | 监控 + API 双场景 |
| ipinfo.io | API 工具 | 数据准、品牌强 | 只做 IP | 全栈网络数据 API |
| chinaz / aizhan | 站长工具 | SEO 流量大 | 工具简陋、拨测弱 | 拨测能力降维打击 |

---

## 4. 功能版图（全景）

下面是**全量功能清单**。每个功能有：标签（P0–P3）、阶段（S1–S4）、所属模块。

### 4.1 公开测试工具（不登录可用，SEO + 引流）

#### 拨测核心
| 功能 | P | S | 备注 |
|---|---|---|---|
| 多地 HTTP/HTTPS 拨测 | P0 | S1 | 含响应时间、状态码、TLS 握手耗时分解 |
| 多地 Ping | P0 | S1 | ICMP，含丢包率、RTT 分布 |
| 多地 TCPing | P0 | S1 | 指定端口连通性 |
| 多地 DNS 解析 | P0 | S1 | 含污染对比、TTL、解析路径 |
| 多地 Traceroute / MTR | P0 | S1 | 路径可视化、ASN 标注 |
| 多地 UDP 探测 | P1 | S2 | 53/123 等常用 UDP |
| 多地 HTTP/2 / HTTP/3 (QUIC) 检测 | P1 | S2 | 协议升级时代刚需 |
| 多地 WebSocket 连通性 | P2 | S3 | 实时应用场景 |
| 全节点带宽测速（下行/上行） | P1 | S2 | 类似 speedtest，每节点单独 |

#### 网络信息查询
| 功能 | P | S | 备注 |
|---|---|---|---|
| IP 查询（归属、ASN、ISP、地理） | P0 | S1 | 多 IP 库交叉验证 |
| IP 段 / CIDR 计算 | P0 | S1 | 纯前端 |
| IPv6 检测 / 转换 | P0 | S1 | 包含 v4/v6 互转 |
| WHOIS 查询 | P0 | S1 | 域名 + IP 双向 |
| ICP 备案查询 | P0 | S1 | **国内特色** |
| 工信部域名信息 | P0 | S1 | 部分接口需对接官方 |
| DNS 记录查询（A/AAAA/MX/TXT/CNAME/NS/SOA/CAA/DKIM/DMARC/SPF） | P0 | S1 | 完整记录类型 |
| 反向 DNS（PTR） | P1 | S1 | |
| ASN 查询 / ASN → IP 段 | P1 | S2 | 给运维用 |
| BGP 路由查询 | P2 | S3 | 看路由广播 |
| 端口连通性自检 | P0 | S1 | 用户输入 IP+Port |
| HTTP Header 查看 | P0 | S1 | 含安全头评分（HSTS、CSP 等） |
| SSL 证书查询（链、到期、SAN、协议、密码套件） | P0 | S1 | 含证书透明度日志 |
| SSL/TLS 安全评分 | P1 | S2 | 类似 SSL Labs |
| HTTP/2 / HTTP/3 支持检测 | P1 | S2 | |
| 网站安全头评分 | P1 | S2 | securityheaders.com 类型 |
| User-Agent 解析 | P1 | S1 | |
| Cookie / Set-Cookie 解析 | P1 | S1 | |
| Email 黑名单 / 反垃圾邮件 RBL 检测 | P2 | S3 | 站长向 |
| 邮件服务器健康（SMTP/IMAP/POP3 探测 + SPF/DKIM/DMARC 校验） | P2 | S3 | |

#### 一键诊断（差异化亮点 P0）
| 功能 | P | S | 备注 |
|---|---|---|---|
| 输入域名一键全面诊断 | P0 | S1 | 串联 DNS+HTTPS+Ping+Trace+SSL+备案+WHOIS+安全头 |
| 诊断报告分享链接（带签名，可被引用） | P0 | S1 | 含 OG 卡片、过期时间 |
| 诊断报告 PDF 导出 | P1 | S2 | 故障复盘场景 |
| 历史诊断对比 | P2 | S3 | 同一域名前后对比 |

#### 辅助工具（SEO 长尾，每个独立页面）
| 功能 | P | S | 备注 |
|---|---|---|---|
| JSON / YAML / XML 格式化 | P1 | S1 | |
| Base64 / URL / Unicode 编码 | P1 | S1 | |
| 时间戳转换 | P1 | S1 | |
| 哈希计算（MD5/SHA/CRC32） | P1 | S1 | |
| JWT 解码 | P1 | S1 | |
| 正则测试 | P1 | S1 | |
| Cron 表达式可视化 | P1 | S1 | |
| 二维码生成 / 解码 | P1 | S1 | |
| 颜色工具 / 字体工具（保留旧站存量） | P2 | S1 | SEO 价值 |
| 假数据生成（开发用） | P2 | S2 | |
| Markdown 预览 / 表格生成 | P2 | S2 | |

#### 免费证书工具（S2 切入,详 [20-free-cert.md](./20-free-cert.md)）
| 功能 | P | S | 备注 |
|---|---|---|---|
| 一键申请免费 SSL 证书（Let's Encrypt） | P0 | S2 | DNS-01 challenge,90 天有效 |
| 多 CA 自动路由 / 失败兜底（LE → ZeroSSL → Buypass） | P0 | S2 | CA 配额 70% 阈值切备份 |
| 自动续期（到期前 30 天） | P0 | S2 | 续期 job 持久化,失败 3 次告警 |
| DNS provider 凭据托管（Cloudflare/Aliyun/DNSPod/Route53/Gcloud） | P0 | S2 | 字段级加密 + KMS 信封 |
| 手动 DNS-01 模式（无凭据用户） | P1 | S2 | 30 分钟超时,SSE 推 challenge 状态 |
| 证书下载（一次性签名链接） | P0 | S2 | HMAC token,5 分钟过期 |
| 证书撤销 / 重新签发 | P1 | S2 | revoke 通过 ACME accountKey |
| 私钥加密落库（KMS 信封,阿里 + AWS 双路径） | P0 | S2 | D-FC-04 决策 |
| 通配符证书（*.example.com） | P1 | S2 | DNS-01 默认支持 |
| 付费 CA 扩展（OV/EV,DigiCert / Sectigo） | P2 | S3 | §20.X 接口兼容 |

### 4.2 账号 & 用户系统

| 功能 | P | S | 备注 |
|---|---|---|---|
| 邮箱注册登录 | P0 | S1 | 不强制手机 |
| 微信扫码登录 / GitHub OAuth | P0 | S2 | 国内+开发者两个入口 |
| 手机号登录（可选） | P1 | S2 | 部分用户偏好 |
| 邮箱验证 / 密码找回 | P0 | S1 | |
| 双因素认证（TOTP） | P1 | S2 | |
| 个人资料 / 头像 | P0 | S1 | |
| API Key 管理（生成、撤销、权限范围） | P0 | S2 | |
| 团队 / 组织（多用户协作） | P1 | S3 | |
| 团队成员角色（Owner/Admin/Member/Viewer） | P1 | S3 | |
| 操作审计日志（账号侧） | P1 | S3 | |
| 企业 SSO（SAML / OIDC） | P3 | S4 | 企业版 |
| 账号注销 / 数据导出（GDPR/PIPL 合规） | P0 | S1 | 必做 |

### 4.3 网站监控（核心商业模块）

| 功能 | P | S | 备注 |
|---|---|---|---|
| HTTP/HTTPS 监控 | P0 | S2 | 关键字断言、状态码断言、响应时间阈值 |
| Ping 监控 | P0 | S2 | |
| TCP 端口监控 | P0 | S2 | |
| DNS 监控（解析结果是否变更） | P0 | S2 | |
| SSL 证书到期监控 | P0 | S2 | 提前 30/15/7/1 天告警 |
| 域名到期监控（WHOIS） | P0 | S2 | |
| ICP 备案变更监控 | P1 | S2 | 国内特色 |
| 关键字监控（页面包含/不包含） | P0 | S2 | |
| JSON API 监控（断言字段值） | P1 | S2 | |
| 心跳监控（Heartbeat / Cron Job） | P1 | S2 | 反向监控：客户端 ping 服务端 |
| 浏览器级监控（Headless Chrome 拨测） | P2 | S3 | 真实用户视角 |
| 多步骤事务监控（登录 → 下单 → 支付） | P3 | S4 | 企业版 |
| 频率配置（10s / 30s / 1min / 5min / 15min） | P0 | S2 | 按订阅档位限制 |
| 多节点同时拨测 + 阈值判定（N 个节点失败才告警） | P0 | S2 | 关键防误报 |
| 监控分组 / 标签 | P0 | S2 | |
| 维护窗口（计划停机静默告警） | P1 | S2 | |
| 监控暂停 / 恢复 | P0 | S2 | |
| 批量导入 / 导出（CSV/JSON） | P1 | S2 | |

### 4.4 告警系统

| 功能 | P | S | 备注 |
|---|---|---|---|
| 邮件告警 | P0 | S2 | |
| Webhook 告警（自定义 payload） | P0 | S2 | |
| 微信告警（公众号模板消息或绑定个人微信） | P0 | S2 | 国内重点 |
| 企业微信机器人 | P0 | S2 | |
| 钉钉机器人 | P0 | S2 | |
| 飞书机器人 | P0 | S2 | |
| Telegram Bot | P1 | S2 | |
| Slack | P1 | S2 | |
| Discord | P1 | S2 | |
| 短信告警 | P2 | S3 | 成本高、合规重，企业版 |
| 电话语音告警 | P3 | S4 | 企业版，按次计费 |
| 告警通道分组 / 模板自定义 | P1 | S2 | |
| 告警升级策略（5 分钟未恢复升级到管理员） | P1 | S3 | |
| 告警静音 / 抑制（同类告警合并） | P0 | S2 | 防告警风暴 |
| 告警值班排班（On-call rotation） | P2 | S3 | SRE 用户刚需 |
| 告警确认 / 解决（acknowledge） | P1 | S3 | |
| 告警历史 / 统计 | P0 | S2 | |
| 告警延迟通知（连续失败 N 次才发） | P0 | S2 | 防误报 |

### 4.5 状态页（Status Page）

| 功能 | P | S | 备注 |
|---|---|---|---|
| 公开状态页（每个用户一个） | P0 | S2 | xxx.status.idcd.com |
| 自定义域名（CNAME） | P1 | S2 | status.usersite.com |
| 自定义品牌 / Logo / 主题色 | P1 | S2 | |
| 服务分组（Web / API / DB / CDN） | P0 | S2 | |
| 历史可用率展示（90/180/365 天） | P0 | S2 | |
| 事件公告 / Incident 时间线 | P0 | S2 | 重大故障通报 |
| 计划维护通告 | P1 | S2 | |
| 订阅状态页（邮件/RSS） | P1 | S2 | 用户侧订阅 |
| 状态页 API（外部嵌入） | P1 | S3 | |
| 多语言（中/英） | P2 | S3 | |
| 历史 SLA 月度报告 | P1 | S3 | |
| 私有状态页（仅授权用户可见） | P2 | S3 | |

### 4.6 数据可视化与报告

| 功能 | P | S | 备注 |
|---|---|---|---|
| 监控总览仪表盘 | P0 | S2 | |
| 单项监控详情（趋势图、热力图、节点对比） | P0 | S2 | |
| 自定义仪表盘 | P2 | S3 | |
| 月度 / 季度 SLA 报告（自动生成 + 邮件） | P1 | S3 | |
| 拨测原始数据下载 | P1 | S2 | CSV/JSON |
| 长期数据归档（1 年以上） | P2 | S3 | 按订阅档区分 |
| 故障复盘报告自动生成 | P2 | S3 | 含时间线、影响节点、根因建议 |

### 4.7 API 开放平台

| 功能 | P | S | 备注 |
|---|---|---|---|
| 拨测 API（一次性按需） | P0 | S2 | ping/tcp/http/dns/trace |
| 网络信息 API（IP/WHOIS/SSL/备案/ASN） | P0 | S2 | |
| 监控管理 API（CRUD 监控项） | P1 | S3 | |
| 告警事件 API（订阅事件流） | P1 | S3 | |
| Webhook 任务回调 | P1 | S3 | |
| SDK（JavaScript / Go / Python） | P1 | S3 | |
| CLI 工具（类 globalping） | P2 | S3 | 极客向 |
| API 文档站（OpenAPI + 交互式调试） | P0 | S2 | |
| API 速率限制 / 配额管理 | P0 | S2 | |
| API 用量统计 / 计费明细 | P0 | S2 | |

### 4.8 商业化（订阅 / 计费 / CPS）

| 功能 | P | S | 备注 |
|---|---|---|---|
| 订阅档位（Free/Pro/Team/Business） | P0 | S2 | |
| 月付 / 年付（年付折扣） | P0 | S2 | |
| 升级 / 降级 / 退订流程 | P0 | S2 | |
| 自动续费 / 续费提醒 | P0 | S2 | |
| 微信支付 / 支付宝 | P0 | S2 | 国内 |
| Stripe（出海） | P1 | S3 | |
| 余额 / 充值（API 按量付费） | P1 | S3 | |
| 发票 / 收据（电子发票合规） | P0 | S2 | 必做 |
| 优惠券 / 推广码 | P1 | S2 | |
| CPS 联盟（推广拉新返佣） | P1 | S3 | |
| 主机商 CPS 聚合（旁路收入） | P2 | S2 | |
| 退款流程 | P0 | S2 | |
| 用量超额提醒 / 自动停用 | P0 | S2 | |

### 4.9 节点系统（运维侧，用户感知弱但是核心壁垒）

| 功能 | P | S | 备注 |
|---|---|---|---|
| 自有节点部署 + 监控（Agent 健康度） | P0 | S1 | |
| 节点目录 / 公开节点列表（透明度卖点） | P0 | S1 | 含 ASN、运营商、地理位置 |
| 节点心跳 + 自动剔除 | P0 | S1 | |
| 任务调度（按地区/ISP/ASN 路由） | P0 | S1 | |
| 节点结果去噪 / 异常值剔除 | P0 | S1 | |
| 众包节点接入（开放 Agent + 积分激励） | P2 | S3 | 谨慎开启 |
| 节点贡献积分体系 | P2 | S3 | |
| 节点反作弊（同 ASN 限权、行为指纹） | P2 | S3 | |
| 节点版本升级（OTA） | P1 | S2 | |
| 节点配置中心（任务白名单、限速） | P0 | S1 | |
| 专属节点（企业版） | P3 | S4 | |

### 4.10 管理后台 / 运营

| 功能 | P | S | 备注 |
|---|---|---|---|
| 用户管理 / 封禁 / 重置 | P0 | S2 | |
| 订单 / 订阅管理 | P0 | S2 | |
| 退款 / 客服工单 | P1 | S2 | |
| 节点管理 / 健康看板 | P0 | S1 | |
| 任务队列监控 | P0 | S1 | |
| 数据看板（DAU、付费转化、节点利用率） | P1 | S2 | |
| 公告 / Banner 管理 | P1 | S2 | |
| 内容运营（文章、案例、文档） | P1 | S2 | |
| 反滥用控制台（黑名单、限速规则） | P0 | S2 | |
| 系统配置（订阅档位、价格、限额） | P0 | S2 | |
| 操作审计日志 | P0 | S2 | 谁改了什么 |

### 4.11 合规、安全、风控、反滥用

| 功能 | P | S | 备注 |
|---|---|---|---|
| ICP 备案 + 公安备案 | P0 | S1 | 国内站必备 |
| 用户隐私协议 / 服务协议 | P0 | S1 | |
| Cookie 同意 / 个人信息处理告知 | P0 | S1 | PIPL |
| 数据本地化 / 跨境传输声明 | P0 | S1 | |
| 用户数据导出 / 注销 | P0 | S1 | |
| 拒测黑名单（私有 IP/政府/银行/友站） | P0 | S1 | **核心** |
| 拨测限速（单 IP / 单用户 / 单目标） | P0 | S1 | **核心** |
| 拨测目标二次确认（高风险目标） | P0 | S1 | |
| 用户行为风控（异常调度模式识别） | P1 | S2 | |
| Cloudflare Turnstile / 人机校验 | P0 | S1 | |
| 测试报告水印（节点ID+时间+目标，可追溯） | P0 | S1 | |
| 7×24 滥用举报通道 | P0 | S2 | |
| 节点请求合法性签名（mTLS） | P0 | S1 | |
| 日志保留 6 个月（合规要求） | P0 | S1 | |
| 安全事件响应 SOP | P1 | S2 | 内部 |
| 定期渗透测试 | P2 | S3 | |
| 用户密码哈希（Argon2id） | P0 | S1 | |

### 4.12 内容运营 / SEO

| 功能 | P | S | 备注 |
|---|---|---|---|
| 工具页 SSR / SSG（SEO） | P0 | S1 | |
| 帮助中心（文档站） | P0 | S1 | |
| 博客 / 案例 / 故障复盘 | P1 | S2 | 长尾流量 |
| 大量长尾工具页（每个 1 个 URL） | P0 | S1 | SEO 关键 |
| 站内搜索 | P1 | S2 | |
| 多语言版本 | P2 | S3 | 出海 |
| API 文档站（独立子域） | P0 | S2 | |
| **MCP 文档站(mcp.idcd.com/docs)** | P1 | S3 | NEW v2,见 19 模块 |
| **CDN/云月度排行榜(/leaderboard)** | P1 | S2 | NEW v2,公共边缘 + 厂商退出通道 |
| 开发者社区（论坛或 Discussion） | P3 | S4 | |

### 4.13 证据与公证(Evidence-as-a-Service)— NEW v2

> 详见 18-evidence-and-attestation.md;v2 eng review(2026-05-13)CRITICAL GAP 闭合,详 DECISIONS §M(D4/D5/D6/D11)

| 功能 | P | S | 备注 |
|---|---|---|---|
| **Verdict 报告**(签名 + RFC3161 + 多节点交叉) | P0 | S2 | 4 个场景模板:SLA / 故障取证 / 合规自证 / 争议取证 |
| **attestation_record 充 WAL + step-level idempotency** | P0 | S2 | **CRITICAL GAP 闭合(v2 D4)**:每 step 写 success+external_id+idempotency_key;Worker crash 续跑无重复 sign |
| **Self-Verify Worker 独立部署** | P0 | S2 | **CRITICAL GAP 闭合(v2 D6)**:独立进程 / 独立 VPC subnet / 独立 KMS 客户端实例 / 仅调 verify 公开接口 |
| **Refund retry queue + 30min 道歉邮箱** | P0 | S2 | **CRITICAL GAP 闭合(v2 D5)**:聚合支付 refund 失败 retry(5min/30min);30min 强制道歉邮箱;refund_failed 状态入 admin dashboard + P0 告警 |
| 公开验签接口 attest.idcd.com/verify | P0 | S2 | 任意第三方上传 PDF 验签,不需登录;**revoke 期间仍可用**(已发报告永久可验签) |
| Verify 接口返回 report_type=observation_only | P0 | S2 | v2 D-Concern1:第三方解析时知"一手观测数据,非鉴定结论" |
| 报告 PDF + PAdES 签名嵌入 | P0 | S2 | 内嵌时间戳的标准格式;PAdES B-T 默认,S3 评估升 B-LT |
| KMS 密钥托管(云 KMS + 离线 root + 90 天轮换 + idempotency token) | P0 | S2 | 信任根架构;KMS sign 启用 idempotency token 防 WAL 重试重复 sign |
| 首次密钥仪式 + 公开 transparency | P0 | S2 | 借鉴 DNSSEC root key ceremony |
| **Backup HSM 独立重组加速通道** | P0 | S2 | **v2 D11**:冷硬件 1-of-1 独立路径;应急时 4h 加速 vs 12h Shamir 主路径;S2 上线前必演练 |
| **KMS 应急 SOP 模拟演练**(S2 前必做) | P0 | S2 | v2 D11:演练 5 持有人召回耗时 + Backup HSM 重组耗时,SLA 基于实测 |
| 报告自检 daily 抽样审计 | P0 | S2 | 10 份/日抽样独立验签 |
| 滥用举报通道 + 违规报告下架流程 | P0 | S2 | 防"诬告竞品"滥用 |
| Compliance 企业年订(50/200/不限 监控) | P0 | S2 | ¥3k / ¥12k / ¥30k/年 |
| 第三家 TSA 接入(NTSC 国内授时中心) | P1 | S3 | 提升国内司法场景认可度 |
| LLM 解读 + 根因建议草拟(per-Provider prompt + eval) | P1 | S3 | v2 D9:Claude/GPT 独立 prompt + 独立 eval ≥4.0 |
| 报告嵌入卡片(iframe / OG) | P1 | S3 | 企业用户文档嵌入 |
| 区块链锚定(以太坊/Polygon/Arweave) | P2 | S3 | 可选 add-on,默认 RFC3161 即够 |
| 司法鉴定所合作通道 | P2 | S3 | 高争议场景输入数据交合作鉴定所 |
| HSM 硬件密钥升级 | P3 | S4 | 企业 due diligence 触发时升级 |
| 白标 Attestation API | P3 | S4 | 企业用自家域名出具,后端仍用 idcd 签 |

### 4.14 AI Agent Observability + MCP Server — NEW v2

> 详见 19-ai-agent-observability.md;v2 eng review 后状态边界 + 计量边界明确,详 DECISIONS §M(D2/D3/D13)

| 功能 | P | S | 备注 |
|---|---|---|---|
| idcd-mcp server(mcp.idcd.com 独立子域) | P0 | S3 | sub-product 阵型;**业务 stateless + SSE 连接 stateful + LB sticky session**(v2 D13) |
| MCP 8 核心 tool(ping/http/dns/trace/ssl/diagnose/ip/whois) | P0 | S3 | S3 alpha |
| MCP 13 全量 tool(增 icp/create_monitor/check_monitor/generate_verdict/list_nodes) | P0 | S3 | S3 GA |
| **MCP token 三种形态,最长 90 天 auto_renewal,无永久** | P0 | S3 | **v2 D2**:personal 24h refresh / workspace 90d / service 90d + IP 白名单强制 |
| MCP 兼容客户端测试矩阵(Cursor/ClaudeCode/Codex) | P0 | S3 | 每发布前 smoke test;TODO:headless/CI 方案研究(CRITICAL GAP 8.5) |
| **MCP 文档站 docs.mcp.idcd.com**(独立子域) | P0 | S3 | **v2 D3**:Cloudflare Pages + Nextra SSG;mcp.idcd.com/docs 走 302 redirect |
| 自家 SDK(idcd-mcp-py / idcd-mcp-ts) | P1 | S3 | npm + pypi,MIT 开源 |
| LLM Endpoint 监控(M21) | P0 | S3 | OpenAI/Anthropic/Bedrock/自家 LLM |
| Tool/API Endpoint 监控(M22) | P0 | S3 | Agent 调用的工具/API |
| RAG/Vector Store 监控(M23) | P1 | S3 | Qdrant/Pinecone/pgvector/Elastic |
| Agent obs 告警维度(LLM 不可达/出口 P99/Tool 4xx 比例) | P0 | S3 | 配合 05-alerting;SSE 长连接 10k/实例 |
| **LLM 故障复盘自动起草(P1 提前)** | P1 | S2 | 30 分钟内 LLM 草拟时间线 + 根因 + 公告,强制人工审核;per-Provider prompt(D9)+ bootstrap 50 条数据集(D8,创始人 S2 前手动标注 ~25h) |
| **MCP units 独立计量池**(与 API 配额完全独立) | P0 | S3 | **v2 D2**:Free/Pro/Team/Business 各档 MCP units 与 API calls 是两条独立量表;用户控制台分别展示 |
| Agent Pro 档(¥299/月,1M MCP units/day 独立加大) | P1 | S3 | MCP 专门定价档,与订阅档独立 SKU |
| 提交 Anthropic MCP gallery | P1 | S3 | 渠道曝光 |
| OpenAI Agents Protocol 接入 | P2 | S3 | 视市场份额决定 |
| 白标 MCP server | P3 | S4 | 企业自家域名挂 idcd 后端 |
| Agent Output Quality 监控(M24) | P3 | S4 | LLM 评估器评分 |

---

## 5. 信息架构（页面级）

### 5.1 主站结构（idcd.com）

```
idcd.com
├── /                        首页（**v2: 3-hero 信息架构** = 一键诊断 + Verdict 样例 + MCP 接入)
├── /tools/                  ~50 个独立工具页
│   ├── /tools/ping          多地 Ping
│   ├── /tools/http          多地 HTTP
│   ├── /tools/dns           多地 DNS
│   ├── /tools/traceroute    路由追踪
│   ├── /tools/whois         WHOIS
│   ├── /tools/icp           备案查询
│   ├── /tools/ssl           SSL 证书
│   ├── /tools/ip            IP 查询
│   ├── /tools/...           ~50 个独立工具页
│   └── /tools/diagnose      一键诊断
├── /report/<id>             分享报告页（SSR、OG 卡片）
├── /verdict/<id>            **v2 NEW: 公开 Verdict 报告分享(签名 + 时间戳标识)**
├── /leaderboard             **v2 NEW: CDN / 云厂商月度独立排行榜**
├── /transparency            **v2 NEW: KMS 密钥仪式 / TSA 健康度 / 节点透明度 / 申诉记录**
├── /nodes                   公开节点目录（透明度）
├── /pricing                 定价(v2: 含 Verdict 件价 + Compliance 年订 + Agent Pro)
├── /api                     API 介绍 + 文档入口
├── /agent                   **v2 NEW: AI Agent / MCP 接入介绍页**
├── /status                  idcd 自己的状态页
├── /blog                    博客
├── /docs                    帮助中心
├── /about, /terms, /privacy 法律页
├── /legacy/*                **老站工具 nginx 转发(robots.txt noindex)**
└── /app/                    控制台（登录后）
    ├── /app/dashboard       仪表盘
    ├── /app/monitors        监控管理(v2: 含 Agent obs 监控类型)
    ├── /app/alerts          告警与通道
    ├── /app/status-pages    状态页管理
    ├── /app/reports         报告
    ├── /app/verdict         **v2 NEW: Verdict 订单 + 已生成报告**
    ├── /app/compliance      **v2 NEW: Compliance 年订配置 + 周/月度报告**
    ├── /app/mcp             **v2 NEW: MCP token 管理 + 用量看板**
    ├── /app/api-keys        API Key
    ├── /app/billing         账单订阅
    ├── /app/team            团队
    └── /app/settings        设置
```

### 5.2 独立子域

```
status.idcd.com              我们自己的状态页
docs.idcd.com                文档站
api.idcd.com                 API 入口
admin.idcd.com               管理后台（内部）
<user>.status.idcd.com       用户状态页（含自定义域支持）
attest.idcd.com              **v2 NEW: Attestation Service(Verdict 报告 + 验签 + transparency)**
mcp.idcd.com                 **v2 NEW: MCP Server(AI Agent 接入,业务 stateless + SSE stateful + LB sticky)**
docs.mcp.idcd.com            **v2 NEW(D3): MCP 文档站独立子域(Cloudflare Pages + Nextra SSG)**
                             **mcp.idcd.com/docs 走 302 redirect → docs.mcp.idcd.com**
```

---

## 6. 商业模式

### 6.1 收入结构（v2 修订,预期 12-18 月后)

| 来源 | 占比 | 备注 |
|---|---|---|
| 订阅 SaaS（Pro/Team/Business 月年付） | 40% | 监控告警 |
| **Evidence 件价(Verdict ¥199-999/份)** | 15% | **v2 NEW**,高毛利单次产品 |
| **Compliance 企业年订(¥3-30k/年)** | 15% | **v2 NEW**,企业法务/合规市场 |
| API 按量 + MCP 按量 | 10% | 开发者市场 + Agent 调用 |
| 主机商 CPS + 排行榜衍生 | 10% | 工具页推广 + /leaderboard 流量变现 |
| AdSense / 联盟 | 3% | 不影响体验前提下 |
| 企业版 / 私有部署 / 白标 | 7%（远期 50%+） | S4 起重点 |

> v2 改动:Evidence + Compliance 两栈共占 30%,把"个人站长付费"的天花板从"靠数量"转向"靠客单价"。同样的 1500 付费用户,v1.0 MRR ¥50k;v2.0 含 200 Verdict 件/月 + 50 Compliance 年订(均价 ¥10k/年) = 月增量 ¥75k(¥199-999 件价均价 ¥350 × 200 = ¥70k + ¥10k × 50/12 ≈ ¥42k),**MRR 翻倍以上**。

### 6.2 定价档（初版假设，待商业化模块细化）

| 档位 | 月费 | 监控数 | 频率 | 节点数 | 告警通道 | API 配额 | 团队 | 状态页 |
|---|---|---|---|---|---|---|---|---|
| Free | ¥0 | **5** | **5min** | 5 | **邮件** | 100/天 | 1 | 1（带 idcd 水印） |
| Pro | ¥29 | 50 | 1min | 全部 | 全通道 | 5,000/天 | 1 | 1（无水印 + 自定义域名） |
| Team | ¥99 | 200 | 30s | 全部 | 全通道 + 排班 | 30,000/天 | 5 | 3（无水印 + 自定义域名） |
| Business | ¥299 | 1000 | 10s | 全部 | 全通道 + 排班 + 升级 | 200,000/天 | 20 | 10（无水印 + 自定义域名） |
| Enterprise | — | — | — | — | — | — | — | — |

> 注 1：**Enterprise 档（私有部署 / 专属节点 / SSO）锁定到 S4 才推出**，S1–S3 阶段定价页面不展示该档。表格中显示为占位，不开放注册。
> 注 2：免费档"宽松型"是有意为之 —— 拉新优先，靠告警通道（微信/钉钉等）+ 监控频率 + 状态页去品牌作为主要转化点。
> 注 3：年付优惠 7-8 折，S2 上线时直接同步开放。
> **注 4 (v2 NEW):** Evidence + Compliance + Agent Pro 是**独立 SKU**,不与上面订阅档绑定。详见 09-billing §2.6 / §2.7 / §2.8。Pro+ 用户可一定折扣购买 Verdict 件价。
> **注 5 (v2 D2):** 表格中"API 配额"指 REST API 计量;**MCP units 是完全独立的另一池**,Free/Pro/Team/Business 各档分别有 100 / 5,000 / 30,000 / 200,000 MCP units/day。Agent Pro 是 MCP units 加大到 1M/day 的独立 SKU。用户控制台 `/app/usage` 同时展示两条独立 progress bar。

### 6.3 关键漏斗

```
公开工具页访问 (SEO)
  ↓ 转化率 ~5%
注册用户
  ↓ 转化率 ~10%
活跃监控用户（添加 ≥1 监控）
  ↓ 转化率 ~5%
付费用户
```

12 个月目标：日 IP 1 万、注册 5 千、付费 500 → MRR ¥15-30k

---

## 7. 阶段路线图

### S1（0–4 月）：MVP 上线，免费工具站立住

**重点**：拨测核心 + 公开测试工具 + 一键诊断 + 节点系统 + 反滥用底盘

**交付物**：
- **100+ 节点**（国内 30+ / 海外 70+，自购 IDC + 海外低配 VPS）
- 公开测试工具齐全（约 50 个页面）
- 一键诊断与分享报告
- 节点目录与透明度页
- 完整的反滥用、合规底盘
- 基础账号系统（仅用于保存测试历史 + 邮箱验证）

**SEO 重点**：每个工具一个独立 URL、SSR、关键词布局

### S2（4–8 月）：商业化启动 + Evidence MVP

**重点**：监控告警 + 状态页 + 订阅付费 + API beta + **Evidence MVP**

**交付物**：
- 全套监控类型 + 多通道告警（微信/钉钉/飞书/邮件/Webhook/Telegram）
- 状态页托管（免费档带水印 + Pro 起去水印 + 自定义域名 CNAME）
- 订阅档位 + **聚合支付 主通道（含微信/支付宝 ）** + 电子发票
- API beta 开放（限量内测）
- 管理后台 + 数据看板

**S2 v2 关键 milestone(eng review 后必做,详 17-roadmap M5-M8 + ENG-REVIEW-TODOS.md)**:
- **M4(S1 末)**:30 天 Anchor baseline 数据采集启动(D10);Backup HSM 采购(D11)
- **M5**:Anchor 阈值 calibration 报告(D10);LLM 复盘 eval 数据集首版 50 条 bootstrap(D8,创始人手动标注 ~25h);KMS 选型 + Root key 仪式准备
- **M6**:首次 KMS 应急 SOP 模拟演练(D11,1-2 天,实测 12h 主路径 + 4h Backup HSM);attestation_record WAL 实施(D4);Self-Verify Worker 独立部署(D6)
- **M7**:**Verdict 失败链路 staging 演示(D4/D5/D6,CRITICAL GAP 必演示)** — 注入 KMS/TSA/S3/refund 失败,验证 30min 内用户收到道歉邮箱 + DLQ 告警;Verdict 件价 + Compliance Starter 上线
- **M8**:LLM 故障复盘自动起草上线(K4 P1 提前;eval ≥4.0/5);Verdict transparency 页;Verdict 件价首月 ≥50 份

### S3（8–14 月）：规模化与生态

**重点**：API 商业化 + 团队协作 + 高级功能 + 内容运营

**交付物**：
- API 正式开放 + SDK + CLI
- 团队 / 组织
- 浏览器级监控
- 告警值班排班
- 月度 SLA 报告
- 博客 / 案例 / 故障复盘内容沉淀
- 众包节点试点

### S4（14 月+）：企业化

**重点**：私有部署 + 白标 + SSO + 销售驱动（**S1–S3 完全不出现**，避免分散精力）

**交付物**：
- Enterprise 档定价正式上线（官网首次出现）
- 企业 SSO（SAML/OIDC）
- 私有部署 / 专属节点
- 白标状态页
- 多步骤事务监控
- 销售线索 / 商机管理后台
- 合同管理 / 审计日志导出 / SLA 合同条款

---

## 8. 技术架构（高层骨架，下一阶段细化;v2 三栈 + D6/D11/D13 关键标注）

```
                      Cloudflare (CDN + WAF + Turnstile + LB sticky for SSE, D13)
                                 │
   ┌───────────┬────────────┬────┴─────┬─────────────────┬──────────────────────┐
   │           │            │          │                 │                      │
 idcd.com  api.idcd.com  status.*    docs.*       attest.idcd.com      mcp.idcd.com
 (Next.js) (API Gateway) (Next.js)   (SSG)        ┌────────┴────────┐   (multi-instance
                                                  │                 │    业务 stateless +
                                                  │                 │    SSE stateful, D13)
                                                  │   Verdict       │           │
                                                  │   Generator     │     ┌─────┴─────┐
                                                  │   Worker (Go)   │     │           │
                                                  │   + WAL on      │   MCP        docs.mcp.idcd.com
                                                  │   attestation_  │   Auth +      (Cloudflare Pages,
                                                  │   record (D4)   │   Dispatcher   D3 独立子域)
                                                  │                 │     │
                                                  │   ─────────────│
                                                  │   Self-Verify   │   302 redirect:
                                                  │   Worker (D6)   │   mcp.idcd.com/docs
                                                  │   独立进程 +    │   → docs.mcp.idcd.com
                                                  │   独立 VPC +    │
                                                  │   独立 KMS 客户端│
                                                  │                 │
                                                  │   ─────────────│
                                                  │   Refund Worker │
                                                  │   retry queue   │
                                                  │   30min 道歉邮箱│
                                                  │   (D5)          │
                                                  └────────┬────────┘
                                 │                         │
                ┌────────────────┼────────────────┐        │
                │                │                │        │
            App Service      Scheduler      Notification   │
              (Go/Node)        (Go)           (Go)         │
                                 │                         │
                    ┌────────────┼────────────┐            │
                    │            │            │            │
              PostgreSQL    TimescaleDB    Redis           │
              (业务 +       (时序结果      (队列/缓存       │
               idcd_attest  + Hypertable   + Streams +     │
               + idcd_mcp    monitor_check + Pub-Sub        │
               schema 隔离, mcp_tool_call)  for SSE cross-  │
               D1 跨 schema                  instance)     │
               不写 FK)                                     │
                                 │                         │
        ┌────────────────────────┼────────────────────────┐│
        │                        │                        ▼▼
   WebSocket Gateway        Cloud KMS (sign+verify,    External Trust:
   (mTLS + 7d cert,         idempotency token D4)      ├ DigiCert TSA
   CRL/OCSP 撤销 v2)        + Shamir 3-of-5 离线 root  ├ GlobalSign TSA
                            + **Backup HSM 独立**       ├ NTSC TSA (S3)
                            **重组加速通道(D11)**       └ LLM Provider
        │                        12h 主 / 4h Backup        (Claude/GPT
        ├─ Agent (国内 x N)                                 per-Provider
        ├─ Agent (海外 x N)                                 prompt + eval, D9)
        └─ Agent (众包)
        (Go 静态二进制，systemd)
```

### 关键技术选型预判（待技术架构 PRD 细化）
- **后端**：Go（agent、调度、网关）+ Node/Next.js（前端 + BFF）
- **数据库**：PostgreSQL + TimescaleDB 扩展（一套维护）+ Redis
- **队列**：Redis Stream / asynq / river
- **前端**：Next.js 15 + shadcn/ui + Tailwind + ECharts
- **基础设施**：Cloudflare 全套 + Hetzner（主控）+ 多家便宜 VPS（节点）
- **观测**：自家产品 dogfood + Grafana + Loki
- **CI/CD**：GitHub Actions + Docker

---

## 9. 关键风险与挑战

| 风险 | 影响 | 缓解 |
|---|---|---|
| 被滥用做 DDoS 工具 | 法律+品牌双重打击 | S1 就上完整反滥用底盘，不可妥协 |
| 国内合规（备案、内容、个保） | 上线被关 | 备案先行，合规模块 S1 完整交付 |
| 节点稳定性差 | 误报、口碑差 | 双节点确认机制 + 异常值过滤 + 节点 SLA 监控 |
| SEO 起不来 | 没流量没付费 | 工具页 SSR + 长尾关键词 + 内容运营从 S2 启动 |
| 同行 DDoS 报复 | 服务中断 | Cloudflare 全套 + 多区域容灾 |
| 节点成本失控 | 烧钱不可持续 | 节点利用率监控、按区域裁剪 |
| 付费转化低 | MRR 不增长 | 免费额度精算 + 关键告警入口设计 |
| 竞品价格战 | 利润压缩 | 差异化（API、一键诊断、报告） |
| **v2 NEW: Verdict 付费失败 = 品牌灾难** | 用户付 ¥299 拿不到报告 → 社交媒体发贴 | **CRITICAL GAP 闭合(D4/D5/D6)**:attestation_record WAL + 聚合支付 refund retry + 30min 道歉邮箱 + Self-Verify 独立部署;S2 上线前 staging 演示 |
| **v2 NEW: KMS sign key 失窃** | 信任根失守 = 公司死亡 | **D11 + Pre-4**:Shamir 3-of-5 12h 单路径(S2)+ Backup HSM 推迟 S4;S2 上线前必演练 12h 路径;revoke 期间 verify 接口仍可用;接受 SLA 滑至 24h+ 风险 |
| **v2 NEW: MCP token 泄露(Cursor 配置被偷)** | 企业用户爆账单 | **D2**:所有 token 最长 90d 无永久;service 强制 IP 白名单;GitHub 扫描自动失活(D-Concern6) |
| **v2 NEW: LLM 复盘幻觉/造谣/泄密** | 公开发声不可逆 | **D8/D9**:bootstrap 50 条 eval 数据集 + per-Provider eval ≥4.0;sanitize 禁用词字典;回流数据不发给 Provider train |
| **v2 NEW: Anchor 偏差阈值未校准** | 误报 / 漏报 → 数据污染未检出 | **D10**:S1 末 30 天 baseline + S2 前 calibration 报告;向前回溯审查防"渐进式造假"(D-Concern8) |
| **v2 NEW: 一人创业 7×24 SLA 不可达** | 夜间响应不了 → SLA breach | **D12**:3 档 SLA:纯自动(Verdict 失败)/ 1h 仅 P0(KMS/节点失窃)/ 24h 常规客服 |
| **v2 NEW: 三栈并行人力压力** | S2 Evidence + S3 MCP + S3 排行榜单人不可达 | 创始人 + CC + 多 AI 协同;eng review 已锁工程量;Pre-condition 待 CEO 答复 |

---

## 10. 下一步（逐模块细化清单）

下列每个 markdown 文件待展开为独立模块 PRD：

```
docs/prd/
├── OVERVIEW.md                 ← 本文件 (v2)
├── DECISIONS.md                单一真实来源决策清单 (含 §K v2 新增)
├── 01-branding.md              中英文双品牌候选与定调（决策 #8）
├── 02-public-tools.md          公开工具（拨测、查询、一键诊断、3-hero IA）
├── 03-account-system.md        账号、API Key、团队、MCP token (v2)
├── 04-monitoring.md            网站监控 + Agent obs 类型 (v2)
├── 05-alerting.md              告警通道与策略
├── 06-status-pages.md          状态页 + 自动 incident + AI 起草工作流 (v2)
├── 07-reports-and-dashboards.md 报表与可视化 + LLM 复盘自动起草 P1 (v2)
├── 08-open-api.md              开放 API 平台 + MCP 章节指向 19 (v2)
├── 09-billing.md               商业化 / 订阅 / 支付 / Verdict 件价 / Compliance 年订 (v2)
├── 10-nodes-and-agents.md      节点与 Agent 系统(100+ 节点 + S3 众包)+ mTLS 撤销 (v2)
├── 11-admin.md                 管理后台 + Verdict 工单 + KMS 仪式 (v2)
├── 12-compliance-and-abuse.md  合规、安全、反滥用 + Verdict 非鉴定声明 + 排行榜白名单 (v2)
├── 13-content-and-seo.md       内容运营 / SEO + /leaderboard 内容矩阵 (v2)
├── 14-tech-architecture.md     技术架构 + attest / mcp / kms 三栈 (v2)
├── 15-data-model.md            数据模型 + verdict_* + mcp_* + agent_obs_* 表 (v2)
├── 16-api-spec.md              API 详细规范 + /v1/verdict /v1/attest /v1/mcp /v1/agent-obs (v2)
├── 17-roadmap.md               详细排期 + Evidence MVP S2 + MCP S3 alpha/GA (v2)
├── 18-evidence-and-attestation.md  **v2 NEW** Evidence-as-a-Service 全模块
└── 19-ai-agent-observability.md    **v2 NEW** AI Agent obs + MCP server 全模块
```

> 推进顺序（v2 修订）：
> - **01-branding.md** 先做（品牌名是后续所有文案、Logo、域名注册的前提）
> - **模块组 A**：02 / 04 / 05 / 06（用户感知最强）
> - **模块组 B**：10 / 12（节点和合规是其他模块的依赖）
> - **模块组 C**：03 / 07 / 08 / 09 / 11
> - **模块组 D**：13 / 14 / 15 / 16 / 17
> - **模块组 E (v2 NEW)**:18-evidence(M5-M8 上线) / 19-ai-agent(M9-M14 上线)

---

## 11. 关键决策（已锁定）

> **详细决策清单见 `DECISIONS.md`**（含 40 + 9 项具体细节、影响传导、备选路径;v2 在 DECISIONS §K 新增 9 项 EXPANSION 决策）。本节保留高层决策摘要：

### 11.A v1.0 最高层 8 决策

| # | 决策项 | 锁定结论 | 影响范围 |
|---|---|---|---|
| 1 | **域名策略** | 单域名 idcd.com 走天下，中英双语同站（`/zh` `/en` 子路径，或按 Accept-Language 自动） | 信息架构、SEO、i18n、品牌 |
| 2 | **首批节点** | 重起步：~100+ 节点（国内 30+ / 海外 70+） | 预算、上线节奏、节点系统设计 |
| 3 | **众包节点** | S3 试点开放，仅作补充覆盖（家宽、冷门地区），不替代自有节点 | 节点架构需预留 Agent 开源 + 积分机制 |
| 4 | **支付通道** | **聚合支付主通道**(微信支付/支付宝),经营性 ICP 暂不办;S3+ 视量再评估自建商户号。详见 DECISIONS.md §H1 | 计费模块、跨境收款、合规 |
| 5 | **免费额度** | 宽松型：5 个监控 / 5min 频率 / 邮件告警 / 仅需验证邮箱 | 拉新策略、节点成本、转化漏斗 |
| 6 | **状态页商业化** | 免费档可用但带 `Powered by idcd` 水印；Pro 起去水印 + 自定义域名（CNAME） | 状态页模块、定价档差异化 |
| 7 | **企业版时机** | **S4（14 月+）才考虑**，S1–S3 完全不提，避免分散精力 | 路线图、首页内容、SEO 关键词、销售线索 |
| 8 | **品牌名** | ✅ **锁定 idcd**(2026-05-13);4 字母无语义组合,类似 vercel/stripe;域名 idcd.com 不变。详 `docs/prd/01-branding.md` | Logo、域名、品牌一致性 |

### 11.M v2.0 Eng Review 后续决策(2026-05-13,详见 DECISIONS §M + ENG-REVIEW-REPORT.md)

> 2026-05-13 用户走 /plan-eng-review,14 项 D 决策全部锁定 A(最完整路径)。本节是 v2 eng review 决策的高层摘要,**实施前必读 DECISIONS §M + ENG-REVIEW-TODOS.md**。

| # | 决策项 | 锁定结论 | Severity |
|---|---|---|---|
| D1 | 跨 schema FK | DDL 不写跨 schema FK,Repository 应用层 join,预留独立 cluster 能力 | HIGH |
| D2 | MCP token + 计量 | 所有 token 最长 90d auto_renewal 无永久;MCP units 与 API 配额完全独立池 | HIGH |
| D3 | MCP 文档站 hosting | 独立子域 docs.mcp.idcd.com + 302 redirect | MEDIUM |
| **D4** | **Verdict WAL 状态机** | **attestation_record 充 WAL + step idempotency + KMS idempotency token** | **CRITICAL** |
| **D5** | **聚合支付 refund 兑底** | **refund retry queue(5min/30min)+ 30min 强制道歉邮箱 + refund_failed 状态** | **CRITICAL** |
| **D6** | **Self-verify 独立** | **独立进程 / 独立 VPC subnet / 独立 KMS 客户端 / 仅调 verify 公开接口** | **CRITICAL** |
| D7 | 数据模型 3 修正 | 原子 UPDATE budget + session_id 索引 + 失败 case 7 天原 payload | MEDIUM |
| D8 | LLM eval cold start | 30 公开事故 + 20 dogfood + 创始人手动标注 25h(S2 前完成) | HIGH |
| D9 | Provider prompt 一致性 | per-Provider 独立 prompt + 独立 eval;baseline 仅 Claude+GPT | MEDIUM |
| D10 | Anchor 阈值 calibration | ×2/×3/×5 为 placeholder;S1 末 30 天 baseline + S2 前 calibration | HIGH |
| D11 | KMS 应急 SOP | **12h Shamir 单路径(Pre-4 调整)**;Backup HSM 推迟 S4;S2 前演练 12h 路径 | HIGH |
| D12 | 1h SLA 现实化 | 3 档:纯自动 / 1h 仅 P0 / 24h 常规 | HIGH |
| D13 | MCP SSE 状态边界 | 业务 stateless + SSE 连接 stateful;LB sticky;10k SSE/实例 | HIGH |
| D14 | TimescaleDB → CK 触发 | 单日 monitor_check >10GB 或 P99 write >100ms → 启动评估 | MEDIUM |

**S2 上线前必演示项**:
- **D4/D5/D6 Verdict 失败链路 staging 演示** — 注入 KMS/TSA/S3/refund 失败,验证 30min 道歉邮箱 + DLQ 告警
- **D11 KMS 应急 SOP 模拟演练** — 实测 5 持有人召回耗时 + Backup HSM 重组耗时
- **D8 LLM eval 数据集 50 条标注完成**

**实施工作量到 S2 上线**:human ~3-5 weeks + CC ~3-4 days(详 ENG-REVIEW-TODOS.md)

**未决问题(待 CEO / 创始人答复)**:
- Pre-1: 三栈并行 1 人 + AI 真能并行?
- Pre-2: KMS + TSA + LLM 月外部依赖成本 $200-1000 acceptable?
- D8 创始人手动标注 25h 是否优先?
- D11 Backup HSM 采购 ¥1000+ 是否优先?
- D12 个人 7×24 P0 响应是否接受(替代:招第二 Operator)?

---

### 11.K v2.0 EXPANSION 决策(2026-05-12 新增,详见 DECISIONS §K)

| # | 决策项 | 锁定结论 | 影响范围 |
|---|---|---|---|
| K1 | **三栈 sub-product 阵型** | Core(api.idcd.com)/ Evidence(attest.idcd.com)/ MCP(mcp.idcd.com)独立子域 + 独立计量 + 独立 SLA | 架构、品牌、企业白标可能性 |
| K2 | **签名密钥架构** | 云 KMS(AWS/阿里云)+ 离线 root(3-of-5 Shamir)+ 90 天 sign key 轮换;HSM S4 评估 | Evidence 模块、信任根、企业 due diligence |
| K3 | **MCP 鉴权** | 短期 token + 三种形态(personal/workspace/service)+ 可选 IP 白名单 | MCP server 模块、Agent 端凭证泄露防护 |
| K4 | **LLM 故障复盘自动起草** | 从 P3/S4 提至 P1/S2;强制人工审核 + AI 标识 + 离线 eval ≥ 4.0/5 | 04 监控、07 报表、19 模块 |
| K5 | **Verdict 计费** | 4 个件价档(¥199/299/499/999/份)+ 3 个 Compliance 年订档(¥3k/12k/30k/年) | 09 计费、18 模块 |
| K6 | **CDN/云排行榜边界** | 仅测厂商公开发布的"公共边缘 IP";厂商可在 /leaderboard/optout 申请退出 | 13 SEO、12 合规、法务风险 |
| K7 | **司法鉴定所合作** | S3 中后期评估;v2 报告明确"非鉴定结论",作为输入数据交合作鉴定所兜底 | 18 模块、12 合规边界 |
| K8 | **区块链锚定** | S3 评估 add-on;v2 起步 RFC3161 时间戳已足够,链锚作为可选信任叠加 | 18 模块、技术选型 |
| K9 | **新增模块** | 18-evidence-and-attestation.md + 19-ai-agent-observability.md;PRD 模块数 18→20 | 路线图、文档维护、新员工 onboarding |

### 决策一致性校验（v1 已同步修订）

- **第 6 章商业模式 / 6.2 定价档**：免费档调整为宽松型（5 监控 / 5min / 邮件告警）
- **第 7 章路线图 / S1**：首批节点目标改为 100+
- **第 7 章路线图 / S4**：企业版完全推迟到 S4，S1–S3 不出现任何企业版功能
- **第 4 章功能版图 / 4.5 状态页**：自定义域名标记为 Pro 起付费功能
- **第 4 章功能版图 / 4.8 商业化**：聚合支付 作为主通道（含微信/支付宝 ）；经营性 ICP 暂不办
- **第 4 章功能版图 / 4.9 节点系统**：众包节点优先级保持 P2 / 阶段 S3

### v2.0 (2026-05-12) 增量同步修订

- **§1.2 长版定位**:重写为三栈叠加(Core + Evidence + AI Agent/MCP)
- **§1.3 差异化表**:新增 4 行(可证据 / AI Agent 集成 / 故障复盘 AI 起草 / 权威排行榜)
- **§1.4 不做什么**:新增 3 条(不冒充司法鉴定 / 不替 Agent 决策 / 不主动适配所有 Agent 协议)
- **§4.12 内容运营**:新增 MCP 文档站 + /leaderboard
- **§4.13 (新增)**:Evidence-as-a-Service 全功能版图
- **§4.14 (新增)**:AI Agent Observability + MCP Server 全功能版图
- **§5.1 主站结构**:新增 /verdict /leaderboard /transparency /agent /app/verdict /app/compliance /app/mcp /legacy 共 8 个 IA 入口
- **§5.2 子域**:新增 attest.idcd.com + mcp.idcd.com + mcp.idcd.com/docs
- **§6.1 收入结构**:重写为 7 行(订阅 40% / Evidence 15% / Compliance 15% / API+MCP 10% / CPS+排行榜 10% / Ad 3% / 企业 7%)
- **§6.2 定价档备注 4**:Evidence + Compliance + Agent Pro 是独立 SKU,不与订阅档绑定
- **§11.K (新增)**:9 项 EXPANSION 决策摘要
- **第 10 章下一步清单**:模块数 18→20,新增 18-evidence + 19-ai-agent
- (后续模块的 §K 影响传导见 DECISIONS.md §K)

### v2.0 Eng Review 锁定 (2026-05-13) 同步修订

- **§1.4 不做什么**:新增 2 条(不签发永久 token / 不允许 verify 看到内部签名捷径)
- **§4.13 Evidence 版图**:补强 4 行(attestation_record WAL D4 / Self-Verify 独立 D6 / Refund retry 30min 道歉 D5 / Backup HSM 独立重组 D11)+ verify report_type D-Concern1 + KMS 应急演练 D11
- **§4.14 MCP 版图**:补强 3 行(SSE stateless+stateful D13 / token 三态 90d D2 / docs.mcp.idcd.com 独立子域 D3 / MCP units 独立池 D2 / per-Provider prompt+eval D9 / eval bootstrap D8)
- **§5.2 子域**:`mcp.idcd.com/docs` → `docs.mcp.idcd.com`(D3)
- **§6.2 定价档备注 5(新增)**:MCP units 与 API 配额完全独立池(D2)
- **§7 S2 路线图**:补 M4 baseline / M5 calibration+eval / M6 演练+WAL+Self-Verify / M7 staging 演示 / M8 LLM 复盘上线
- **§8 技术架构图**:补 Self-Verify Worker 独立(D6)+ Backup HSM 独立重组(D11)+ LB sticky(D13)+ docs.mcp 独立子域(D3)+ attestation_record WAL(D4)+ Refund retry+30min 道歉(D5)+ 跨 schema 不写 FK(D1)+ KMS idempotency token(D4)+ per-Provider prompt(D9)
- **§9 关键风险**:新增 7 个 v2 风险点(Verdict 失败 / KMS 失窃 / MCP token 泄露 / LLM 幻觉 / Anchor 阈值 / 1h SLA / 三栈人力)
- **§11.M (新增)**:14 项 D 决策 + 必演示项 + 工作量估算 + 未决问题
- (后续模块的 §M 影响传导见 DECISIONS.md §M;实施清单见 ENG-REVIEW-TODOS.md)
