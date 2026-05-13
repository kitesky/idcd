# 02 · 公开工具（拨测、网络信息查询、一键诊断、Verdict 入口、排行榜、MCP 入口)(v2)

> 关联：OVERVIEW.md §4.1、§4.13、§4.14
> 关联(v2):18-evidence(Verdict)、19-ai-agent(MCP)、13-content(排行榜)
> 阶段主体：S1 全量上线；S2/S3 增量;**v2 S2 新增 D05/V01-V04/L01-L04;S3 新增 A01-A03 MCP 入口**
> 是否登录可用：**全部公开工具不登录可用**，登录后享受历史记录、更高额度、可分享私有报告
> 品牌名占位：`idcd`

---

## 1. 模块定位与目标

公开工具是 `idcd` 的**流量入口**、**SEO 基本盘**、**信任建立点**。承担三个核心使命：

1. **拉新**：通过搜索引擎收口"多地 ping / 网站测速 / DNS 查询 / IP 查询"等高频长尾词
2. **建立信任**：让用户在不注册的前提下体验到产品质量（节点真实、结果准确、UI 现代）
3. **转化**：每个工具页都有"加入持续监控"、"生成 PDF 报告"、"API 接入"等转化锚点

### 关键指标（北极星）

| 指标 | S1 目标 | S2 目标 |
|---|---|---|
| 工具页日 UV | 1,000 | 10,000 |
| 工具页平均停留 | ≥ 60s | ≥ 90s |
| 工具页 → 注册转化率 | — | ≥ 3% |
| 单次测试 P95 出结果时间 | ≤ 8s | ≤ 5s |
| 报告分享率 | — | ≥ 5%（每 100 次测试有 5 次被点击分享） |

---

## 2. 公开工具全景清单

### 2.1 拨测类（核心，多节点）

| ID | 工具 | 路径 | 节点参与 | P | S | 关键输入 | 关键输出 |
|---|---|---|---|---|---|---|---|
| T01 | 多地 HTTP/HTTPS 拨测 | `/tools/http` | 全节点 | P0 | S1 | URL + method + 高级参数 | 节点维度耗时分解、状态码、TLS、Header |
| T02 | 多地 Ping | `/tools/ping` | 全节点 | P0 | S1 | host/IP + 次数 | RTT min/avg/max/mdev、丢包率 |
| T03 | 多地 TCPing | `/tools/tcping` | 全节点 | P0 | S1 | host + port | 连接耗时、成功率 |
| T04 | 多地 DNS 解析 | `/tools/dns` | 全节点 | P0 | S1 | 域名 + 记录类型 + 指定 DNS | 各节点解析结果、TTL、是否一致 |
| T05 | 多地 Traceroute | `/tools/traceroute` | 单/多节点 | P0 | S1 | host | 跳数、ASN、RTT、地理 |
| T06 | 多地 MTR | `/tools/mtr` | 单/多节点 | P0 | S1 | host + 次数 | 综合丢包/延迟矩阵 |
| T07 | 多地 UDP 探测 | `/tools/udp` | 全节点 | P1 | S2 | host + port + payload | 是否有响应、响应时间 |
| T08 | HTTP/2 / HTTP/3 检测 | `/tools/http3` | 全节点 | P1 | S2 | URL | 协议支持、协商结果、QUIC RTT |
| T09 | WebSocket 连通性 | `/tools/websocket` | 全节点 | P2 | S3 | wss URL | 握手耗时、心跳是否通 |
| T10 | 全节点带宽测速 | `/tools/speedtest` | 全节点 | P1 | S2 | 自动选最近 | 上下行 Mbps、抖动、延迟 |
| T11 | 多地 SMTP / IMAP / POP3 探测 | `/tools/mail-server` | 全节点 | P2 | S3 | host + port + 协议 | banner、TLS、连通 |

### 2.2 网络信息查询类（多为单次，部分依赖外部 API）

| ID | 工具 | 路径 | 数据源 | P | S |
|---|---|---|---|---|---|
| Q01 | IP 查询（归属、ASN、ISP） | `/tools/ip/<ip?>` | MaxMind + IPinfo + IP2Location 交叉 | P0 | S1 |
| Q02 | IP 段 / CIDR 计算 | `/tools/cidr` | 纯前端 | P0 | S1 |
| Q03 | IPv6 检测 / 转换 | `/tools/ipv6` | 纯前端 | P0 | S1 |
| Q04 | WHOIS 查询（域名） | `/tools/whois/<domain?>` | RDAP + 兜底 WHOIS server | P0 | S1 |
| Q05 | WHOIS 查询（IP） | `/tools/whois-ip` | RIR RDAP（ARIN/RIPE/APNIC/LACNIC/AFRINIC） | P0 | S1 |
| Q06 | ICP 备案查询 | `/tools/icp/<domain?>` | 工信部公示数据（自抓+缓存） | P0 | S1 |
| Q07 | 工信部域名信息 | `/tools/miit` | 工信部 | P1 | S2 |
| Q08 | DNS 记录全套（A/AAAA/MX/TXT/CNAME/NS/SOA/CAA） | `/tools/dns-records` | 多家公共 DNS 交叉 | P0 | S1 |
| Q09 | DNS 增强（DKIM/DMARC/SPF/BIMI） | `/tools/email-dns` | 公共 DNS + 协议解析 | P1 | S1 |
| Q10 | 反向 DNS（PTR） | `/tools/rdns` | 内置 DNS | P1 | S1 |
| Q11 | ASN 查询 / ASN → IP 段 | `/tools/asn/<asn?>` | RIPEstat / Team Cymru | P1 | S2 |
| Q12 | BGP 路由查询 | `/tools/bgp` | RIPEstat / bgpview API | P2 | S3 |
| Q13 | 端口连通性自检 | `/tools/port-check` | 节点 TCP 探测 | P0 | S1 |
| Q14 | HTTP Header 查看 + 安全头评分 | `/tools/headers` | 节点抓取 | P0 | S1 |
| Q15 | SSL 证书查询（链、SAN、协议、密码套件） | `/tools/ssl/<host?>` | 节点 TLS 握手 + CT 日志 | P0 | S1 |
| Q16 | SSL/TLS 安全评分（Labs 风格） | `/tools/ssl-score` | 节点扫描 + 评分引擎 | P1 | S2 |
| Q17 | HTTP/2 / HTTP/3 支持 | `/tools/http-version` | 节点协商 | P1 | S2 |
| Q18 | 网站安全头评分 | `/tools/security-headers` | 节点抓取 + 规则引擎 | P1 | S2 |
| Q19 | User-Agent 解析 | `/tools/ua` | 纯前端 ua-parser | P1 | S1 |
| Q20 | Cookie / Set-Cookie 解析 | `/tools/cookie` | 纯前端 | P1 | S1 |
| Q21 | Email RBL 黑名单检测 | `/tools/rbl` | 30+ RBL 服务器查询 | P2 | S3 |
| Q22 | CDN 识别 / WAF 识别 | `/tools/cdn-detect` | 节点抓 Header + 指纹库 | P1 | S2 |

### 2.3 一键诊断（差异化亮点）

| ID | 工具 | 路径 | P | S |
|---|---|---|---|---|
| D01 | 输入域名一键全面诊断 | `/tools/diagnose` | P0 | S1 |
| D02 | 诊断报告分享链接 | `/report/<id>` | P0 | S1 |
| D03 | 诊断报告 PDF 导出 | `/report/<id>/pdf` | P1 | S2 |
| D04 | 历史诊断对比 | `/report/diff?a=<id>&b=<id>` | P2 | S3 |
| **D05 (v2)** | **节点试运行(添加监控前预检)** | `/tools/dry-run` | P1 | S2 |
| **D06 (v2)** | **域名 vs 竞品对比报告** | `/tools/compare` | P2 | S3 |
| **D07 (v2)** | **DNS 时光机(改 DNS 后全球生效追踪)** | `/tools/dns-propagation` | P2 | S3 |

### 2.5 Verdict 公开页(v2 NEW,差异化护城河)

| ID | 工具 | 路径 | P | S |
|---|---|---|---|---|
| **V01** | Verdict 报告样例 + 下单入口 | `/verdict` | P0 | S2 |
| **V02** | 公开 Verdict 报告分享 | `/verdict/<id>` | P0 | S2 |
| **V03** | 公开验签(任意第三方上传 PDF) | `attest.idcd.com/verify` | P0 | S2 |
| **V04** | transparency 公开仪表盘 | `/transparency` | P0 | S2 |

### 2.6 排行榜 / 权威内容(v2 NEW)

| ID | 工具 | 路径 | P | S |
|---|---|---|---|---|
| **L01** | CDN / 云月度排行榜总览 | `/leaderboard` | P1 | S2 |
| **L02** | 历史月报告 | `/leaderboard/<YYYY-MM>` | P1 | S2 |
| **L03** | 测试方法学 | `/leaderboard/methodology` | P1 | S2 |
| **L04** | 厂商退出申请 | `/leaderboard/optout` | P0 | S2 |

### 2.7 AI Agent / MCP 入口(v2 NEW)

| ID | 工具 | 路径 | P | S |
|---|---|---|---|---|
| **A01** | MCP 接入介绍 + 配置生成器 | `/agent` | P1 | S3 |
| **A02** | MCP 文档站 | `mcp.idcd.com/docs` | P1 | S3 |
| **A03** | MCP 演示场景(交互式) | `/agent/playground` | P2 | S3 |

### 2.4 SEO 辅助工具（纯前端，每页独立 URL，撑长尾 SEO）

| ID | 工具 | 路径 | P | S |
|---|---|---|---|---|
| U01 | JSON 格式化 / 校验 | `/tools/json` | P1 | S1 |
| U02 | YAML / TOML / INI 互转 | `/tools/yaml` | P1 | S1 |
| U03 | XML 格式化 | `/tools/xml` | P1 | S1 |
| U04 | Base64 编解码 | `/tools/base64` | P1 | S1 |
| U05 | URL 编解码 | `/tools/url-encode` | P1 | S1 |
| U06 | Unicode / Hex / Bin 互转 | `/tools/unicode` | P1 | S1 |
| U07 | 时间戳转换 | `/tools/timestamp` | P1 | S1 |
| U08 | 哈希计算（MD5/SHA-1/SHA-256/SHA-512/CRC32） | `/tools/hash` | P1 | S1 |
| U09 | JWT 解码 | `/tools/jwt` | P1 | S1 |
| U10 | 正则测试 | `/tools/regex` | P1 | S1 |
| U11 | Cron 表达式可视化 | `/tools/cron` | P1 | S1 |
| U12 | 二维码生成 / 解码 | `/tools/qrcode` | P1 | S1 |
| U13 | 颜色工具（RGB/HEX/HSL/调色板） | `/tools/color` | P2 | S1 |
| U14 | 假数据生成 | `/tools/faker` | P2 | S2 |
| U15 | Markdown 预览 | `/tools/markdown` | P2 | S2 |
| U16 | Diff 比较 | `/tools/diff` | P2 | S2 |
| U17 | UUID / NanoID 生成 | `/tools/uuid` | P1 | S1 |
| U18 | 密码强度 / 生成 | `/tools/password` | P1 | S1 |

> 辅助工具的目的是 SEO 长尾 + 工具站完整性，不是核心壁垒，**全部要求纯前端实现**，零后端成本。

---

## 3. 详细功能规格（拨测核心，逐项展开）

### 3.1 T01 多地 HTTP/HTTPS 拨测

#### 用户场景
- 站长怀疑某地区访问慢，想验证
- 排查 CDN 节点异常
- 上线前验证全网可达

#### 输入参数
| 参数 | 必填 | 默认 | 说明 |
|---|---|---|---|
| URL | 是 | — | 含协议；自动补 `https://` 兜底 |
| Method | 否 | GET | GET / HEAD / POST / PUT / DELETE / OPTIONS |
| Headers | 否 | — | 自定义请求头（JSON 或 KV） |
| Body | 否 | — | POST/PUT 时启用 |
| Follow Redirect | 否 | true | 跟随 3xx |
| Timeout | 否 | 10s | 单节点超时 |
| IP 版本 | 否 | auto | v4 / v6 / auto |
| 节点筛选 | 否 | 全部 | 国家 / 运营商 / ASN / Tag |
| 期望状态码 | 否 | 2xx-3xx | 用于"成功率"统计 |
| 期望关键字 | 否 | — | 响应体包含/不包含 |

#### 输出结构（节点维度）
- **耗时分解**：DNS 解析 / TCP 握手 / TLS 握手 / TTFB / 内容下载 / 总耗时
- **协议信息**：HTTP 版本、TLS 版本、密码套件、证书链 fingerprint
- **响应**：状态码、Final URL（重定向后）、响应大小、Content-Type、Content-Encoding
- **响应头**：完整 Header（敏感头脱敏选项）
- **响应体**：前 8KB 截断显示（不存储）+ 关键字命中
- **节点元数据**：节点ID、国家、城市、ISP、ASN、出口 IP

#### 聚合视图
- 全局：成功率、平均/中位/P95 总耗时
- 按地域聚合（中国大陆三网 / 海外大区）
- 地图视图（点 = 节点，颜色 = 耗时）
- 时序复测：支持"持续 30 秒、每秒一次"的连续测试

#### 边界与限制
- 单次测试最多并发 100 个节点（避免一次性消耗过多）
- 单 IP 限速：未登录 5 次/分钟、登录 20 次/分钟、付费 200 次/分钟
- 拒测黑名单匹配（见 12-compliance-and-abuse）
- POST/PUT body 大小 ≤ 32KB
- 响应体最多读取 1MB（防大文件耗节点流量）
- 不支持自定义 SOCKS/HTTP 代理（防滥用）

#### 转化锚点
- "持续监控此 URL" → 引导注册
- "下载完整报告" → 注册后免费
- "API 调用" → 链接到 API 文档

---

### 3.2 T02 多地 Ping

#### 输入
- host / IP
- 包数（默认 4，最大 20）
- 包大小（默认 56 字节，范围 32-1500）
- 间隔（默认 1s，最小 0.2s）
- IPv4 / IPv6
- 节点筛选

#### 输出
- 每节点：RTT min/avg/max/mdev、丢包率、TTL
- 时序图：实时绘制每个包的 RTT
- 地图：节点位置 + RTT 着色
- 异常标注：丢包率 > 5% 高亮，RTT > 200ms 标黄

#### 边界
- ICMP 在部分节点（限制环境）可能不可用，自动 fallback 到 TCPing 80/443 并标注
- 拒绝对 RFC1918 私网、169.254/16、组播段、私有 IPv6 段执行
- 每节点单次最长执行 30s

---

### 3.3 T04 多地 DNS 解析（重点：污染检测）

#### 输入
- 域名
- 记录类型：A / AAAA / CNAME / MX / TXT / NS / SOA / CAA / SRV / PTR / 全部
- 指定 DNS Server：节点本地 / 自定义 / 公共 DNS 列表（114.114 / 223.5 / 8.8.8.8 / 1.1.1.1 / Quad9 等）
- IPv4 / IPv6 / 双栈
- EDNS Client Subnet 选项（高级）

#### 输出
- 每节点：解析结果 + TTL + 响应时间
- **一致性检测**：全节点解析结果做哈希聚合，发现差异即高亮（这是国内污染场景关键卖点）
- 解析路径：递归 / 权威（高级模式）
- DNSSEC 状态

#### 差异化展示
- "中国大陆 vs 海外"分组对比（同一域名两地解析不一致是劫持/污染信号）
- "权威 NS vs 公共 DNS"分组对比

#### 边界
- 一次最多解析 5 个记录类型
- 不允许查询非常规端口的 DNS（防 DNS 放大攻击中转）

---

### 3.4 T05/T06 Traceroute / MTR

#### 输入
- host / IP
- 协议：ICMP / UDP / TCP（默认 ICMP，可切 TCP 探测特定端口）
- 最大跳数（默认 30，最大 64）
- 节点筛选（建议选 1-5 个节点深度查，而不是全节点）

#### 输出
- 跳数表格：序号、IP、反向 DNS、ASN、ISP、地理、RTT × 3
- 路径可视化：
  - 节点 → 目标的水平时间线
  - 跨 ASN 高亮（标注 ASN 切换点）
  - 海外节点标注海缆出口
- 异常标注：连续 `* * *`、RTT 突增、ASN 反向跳变

#### 边界
- 单节点最长执行 60s
- TCP traceroute 仅允许 80/443/53/22 等常用端口

---

### 3.5 T10 全节点带宽测速

#### 输入
- 选定一个节点作为"测试服"（用户视角）
- 测试方向：下行 / 上行 / 双向
- 持续时间：5s / 10s / 30s

#### 输出
- 下行 Mbps、上行 Mbps、抖动 ms、延迟 ms
- 多节点对比表（用户可看到从北京电信节点访问东京节点的真实带宽）

#### 实现关键
- 节点之间互相充当 server，避免引入单点
- 限制单用户每天最多 10 次（带宽测试消耗大）
- 拒绝跨大区频繁测试（北京 → 东京 OK，北京 → 圣保罗触发限制）

---

### 3.6 Q06 ICP 备案查询（国内特色）

#### 输入
- 域名 / 备案号 / 主办单位

#### 输出
- 备案号、主办单位、性质（个人/企业）、审核日期、网站名称、首页地址
- 备案变更历史（如有）
- "本月最新备案"小标签（吸引站长）

#### 数据来源
- 工信部 ICP/IP 地址/域名信息备案管理系统（公示数据）
- 自抓+本地缓存，TTL 7-30 天
- 不依赖第三方付费 API，但首批爬取需要一次性投入

#### 合规
- 备案数据是公示信息，可合法展示
- 但**不能挂个人手机号等隐私**，需脱敏
- 提供"撤下我的备案信息"申诉入口（合规要求）

---

### 3.7 Q15 SSL 证书查询

#### 输入
- host:port（默认 443）
- SNI（可选自定义）
- 协议尝试：TLS 1.0 / 1.1 / 1.2 / 1.3 / 自动

#### 输出
- 证书链：每张证书的 CN/SAN/颁发者/有效期/序列号/SHA-256
- 协议支持矩阵：TLS 版本 × 密码套件
- OCSP / OCSP Stapling 状态
- 证书透明度日志（crt.sh 链接）
- 弱密码套件 / 过期证书警告
- HSTS / HPKP（已废弃，仅检测）

#### 转化锚点
- "监控此证书到期" → 引导加入证书到期监控

---

### 3.8 D01 一键诊断（核心差异化）

#### 用户操作
- 首页/工具页输入 `example.com` → 点击"一键诊断"
- 后台并行执行：
  - WHOIS（域名）
  - ICP 备案（仅 .cn 或国内主体）
  - DNS 解析（A/AAAA/MX/NS/CAA）
  - 多节点 HTTPS 拨测（首页 + favicon）
  - SSL 证书检查
  - 安全头评分
  - Ping / TCPing 443
  - 简化 traceroute（3 个代表节点）
  - 子域名（常见 www/api/cdn/mail/blog）的可达性

#### 输出报告页（`/report/<id>`）
- **总评分**：0-100 综合分（DNS 健康 / 可达性 / 证书 / 速度 / 安全 / 备案 6 维度加权）
- **关键发现**：自动生成 5-10 条要点（"证书 30 天后过期"、"美国节点访问超时"、"未启用 HSTS"）
- **逐项详情**：可折叠的每个检查的完整结果
- **分享按钮**：复制链接 / 二维码 / 微信分享卡片 / 嵌入代码

#### 报告页特性
- 公开默认（带签名 ID，不可猜测：`/report/r_a1b2c3d4e5`）
- 登录用户可设为"仅我可见"或"密码保护"
- 30 天后自动过期（公开），登录用户可永久保留
- 报告页 SSR + OG 卡片（Twitter Card、微信卡片）
- 自动加水印：节点ID+时间戳+检查项（防伪造）

#### 转化锚点（关键）
- 顶部 Banner："发现 3 个潜在问题，加入持续监控以便第一时间发现 →"
- 证书项："监控此证书到期 →"
- 备案项："监控备案变更 →"
- 速度图："订阅性能监控 →"
- **v2 NEW**:"将此次诊断生成 Verdict 报告(¥299,签名 + 时间戳) →"(故障复盘 / SLA 索赔场景)

---

### 3.9 D05 节点试运行(v2 NEW)

#### 用户场景
- 用户准备添加生产监控,先确认"我要测的目标从哪些节点能通?哪些通不了?"
- 避免上线后发现某些节点根本不可达(如国内节点测海外网站被防火墙误判 / 海外节点测国内 IP 被运营商封)
- 真正的"避免误报"前置工具

#### 输入
- 目标 URL / IP / host:port
- 监控类型(HTTP / Ping / TCP / DNS)
- 期望节点池(国家 / 运营商 / Tag 多选)

#### 输出
- 节点可达性矩阵:
  - ✅ 通(响应正常)
  - ⚠️ 慢(响应 > 阈值,可能 unstable)
  - ❌ 不通(超时 / 拒绝 / 防火墙)
- 推荐节点池配置(自动建议剔除不通 / 不稳定的节点)
- "一键创建监控,使用推荐节点池"按钮(转化到 04 监控)

#### 转化锚点
- "用推荐节点池创建监控"
- Pro+ 用户:可保存节点池配置为模板,后续监控复用

#### 边界
- 不计入"监控历史"(纯探测性测试,不留存)
- 限速:登录用户每 5 分钟 1 次,Free 档每天 10 次,Pro+ 不限
- 单次最多覆盖 50 节点(更多需要走"添加监控")

---

### 3.10 D06 域名 vs 竞品对比报告(v2 NEW)

#### 用户场景
- 销售向客户证明"我家的网站比竞品快"
- 上线前对比"和友站的可用性对比"
- SEO 报告"我家网站在国内访问比 X 快 200ms"

#### 输入
- 域名 A + 域名 B(可扩到 5 个)
- 时间窗(过去 1h / 24h / 7d / 30d,Pro+ 可拉 90d)

#### 输出
- 双栏对照:
  - 响应时间分布(P50/P95/P99)
  - 可用率
  - CDN / WAF 识别
  - 证书安全性
  - HTTP 协议升级状态
- **可选生成 Verdict 报告**(销售素材,带签名 + 时间戳)

#### 边界
- 必须验证用户对其中至少 **1 个域名的所有权**(对自家 vs 竞品场景);不允许"任意域名 vs 任意域名"(防恶意打分)
- 报告内容**禁止包含主观贬损评价**(LLM 输出 sanitize)

---

### 3.11 D07 DNS 时光机(v2 NEW)

#### 用户场景
- 用户改了 DNS 解析,想知道"全球各地多久能生效"
- DNS 故障复盘:从 90 节点视角观察 TTL 过期 + 缓存刷新
- 验证 DNS 厂商切换是否生效(从 DNSPod 切到 Cloudflare 等)

#### 输入
- 域名 + 记录类型(A / AAAA / CNAME / MX / TXT)
- 期望值(用户填新解析结果)

#### 输出
- 时间线图:每个节点何时开始返回新值
- 节点维度表:各节点当前解析 / 上次更新时间 / TTL
- 历史快照:过去 24h 解析变化追踪(每 5 分钟一次)

---

### 3.12 V01 Verdict 报告样例 + 下单入口(v2 NEW, 详 18 §2.1)

详见 18-evidence-and-attestation.md。本节仅说明公开页面入口:
- `/verdict` 页面:展示 4 个模板(SLA / 故障取证 / 合规 / 法务)价格 + 样例 PDF 缩略图
- 点击模板 → 跳到下单流程(详 18 §3.2 端到端流程)
- 公开样例报告:展示"过去某次重大网络事件"的 Verdict 长什么样(教育用户产品价值)

---

## 4. 信息架构（IA）

### 4.1 工具页统一模板

每个工具页结构一致，便于 SEO 和用户学习成本：

```
┌──────────────────────────────────────────────────────┐
│  [工具名]                                  [中/English] │
│  一句话说明 + 1 张示意图                                  │
├──────────────────────────────────────────────────────┤
│  [输入区]   主输入框 + 高级参数（折叠）                      │
│             [开始测试]   [API 调用]                       │
├──────────────────────────────────────────────────────┤
│  [节点筛选]  按国家 / 运营商 / Tag 多选                     │
├──────────────────────────────────────────────────────┤
│  [结果区]                                              │
│   • 全局摘要卡片                                          │
│   • 节点维度结果表                                         │
│   • 地图视图                                             │
│   • 时序/趋势（如适用）                                     │
│   • [生成完整报告] [分享] [API 调用] [加入监控]              │
├──────────────────────────────────────────────────────┤
│  [SEO 内容区]                                          │
│   • 工具说明（300-800 字）                                │
│   • 常见问题 FAQ                                         │
│   • 相关工具推荐                                          │
│   • 应用场景                                             │
├──────────────────────────────────────────────────────┤
│  [全站底栏]  其他工具索引                                   │
└──────────────────────────────────────────────────────┘
```

### 4.2 首页结构(v2: 3-Hero 信息架构)

> v2 关键变更:首页从单一 hero 改为 **3-hero 并列**,对应三栈产品(Core / Evidence / MCP)。每个 hero 针对不同 persona,首屏即明确产品差异化。

```
┌─────────────────────────────────────────────────────────────────────┐
│ 顶部导航:Logo / 工具 / 监控 / Verdict / MCP / Pricing / 文档 / 登录   │
└─────────────────────────────────────────────────────────────────────┘

╔═══════════════════════════════════════════════════════════════════════╗
║ Hero 1:一键诊断(普通用户排障)                                          ║
║                                                                       ║
║   "网站慢? 打不开? 一键看清问题在哪。"                                  ║
║   [输入域名/IP/URL]  [开始诊断]                                        ║
║   小字:免费 / 不需登录 / 100+ 节点 / 5 秒出报告                        ║
║   样例:点击 example.com / cloudflare.com 试试                          ║
╚═══════════════════════════════════════════════════════════════════════╝

╔═══════════════════════════════════════════════════════════════════════╗
║ Hero 2:Verdict 报告(企业法务/合规/SLA 索赔)            (v2 NEW)        ║
║                                                                       ║
║   "为你的网络争议生成法律级证据。"                                      ║
║                                                                       ║
║   [缩略图:一份 Verdict 报告 PDF + 签名章 + 时间戳]                     ║
║                                                                       ║
║   • 多节点交叉验证   • RFC3161 时间戳   • 公开可验签                    ║
║   • SLA 索赔 / 故障取证 / 等保自证 / 法务取证 四个模板                 ║
║   ¥199-999/份  [生成示例报告] [了解更多]                                ║
║   样例:看看一份 Verdict 长什么样(分享链接)                             ║
╚═══════════════════════════════════════════════════════════════════════╝

╔═══════════════════════════════════════════════════════════════════════╗
║ Hero 3:MCP 接入(AI Agent / Cursor / Claude Code 用户)  (v2 NEW)        ║
║                                                                       ║
║   "让你的 AI 知道全球网络真相。"                                        ║
║                                                                       ║
║   [代码块:                                                            ║
║      # 在 Cursor / Claude Code 配置                                   ║
║      mcp.idcd.com                                                     ║
║      ✓ idcd_ping  ✓ idcd_http_probe  ✓ idcd_diagnose ... 13 tools     ║
║   ]                                                                   ║
║                                                                       ║
║   [一键复制配置] [打开 MCP 文档] [免费 alpha 邀请码]                    ║
╚═══════════════════════════════════════════════════════════════════════╝

↓ 滚动后:
信任带:节点数 100+ / 国家 N+ / 累计 Verdict 报告 X 份 / MCP 月调用 X+
↓
功能矩阵:4 列(工具 / 监控 / Evidence / Agent obs)
↓
权威背书(v2 NEW):
  - "本月 CDN 排行榜" 卡片(/leaderboard 引流)
  - "transparency 公开" 卡片(/transparency 引流)
↓
为谁服务:个人站长 / SRE / 企业法务 / Agent 开发者 四栏(v2 加 1 栏)
↓
价格预告(S2 启用,含三栈定价):
  - Pro/Team/Business 订阅
  - Verdict 件价 + Compliance 年订
  - Agent Pro 档
↓
文档与社区:常规文档 + MCP 文档站
↓
底部:完整工具索引 + 三栈快速入口
```

**首屏移动端处理**:Hero 1/2/3 改为可左右滑动的卡片(Tab 切换),默认 Hero 1。

**A/B 测试**(S2 末):
- 测试 3-hero 序列(诊断 → Verdict → MCP)vs(Verdict → 诊断 → MCP)的转化率
- 测试 Hero 2/3 是否真的促进高 ARPU 档转化(对比 v1 单 hero baseline)

### 4.3 URL 与 SEO 策略

- 工具页 URL 短、稳定、语义化：`/tools/<tool-name>`
- 部分工具支持参数化 URL：`/tools/ip/8.8.8.8` `/tools/whois/example.com`（关键 SEO）
- 所有工具页 SSR（Next.js App Router + Static Generation 优先）
- 每个工具页独立 `<title>` `<meta description>` `<h1>` `<canonical>`
- `sitemap.xml` 自动生成 + 提交百度/Google/必应
- 工具页之间相互链接（底部 Related Tools）形成内链网络
- `hreflang` 双语标签

---

## 5. 公开 API（与 §08-open-api 关联）

每个公开工具背后是一组对应的公开 API：

| 工具 | API Endpoint |
|---|---|
| HTTP 拨测 | `POST /v1/probe/http` |
| Ping | `POST /v1/probe/ping` |
| DNS | `POST /v1/probe/dns` |
| Traceroute | `POST /v1/probe/traceroute` |
| 一键诊断 | `POST /v1/diagnose` |
| IP 查询 | `GET /v1/ip/<ip>` |
| WHOIS | `GET /v1/whois/<domain>` |
| ICP | `GET /v1/icp/<domain>` |
| SSL | `GET /v1/ssl?host=<host>` |
| 报告查询 | `GET /v1/report/<id>` |

#### 公开 API 限速

> **决策 D1**：第三方匿名 API 不开放。下列限速适用于：
> - **自家工具页前端调用**（带 Cloudflare Turnstile 验证 + 短期 session）
> - **登录用户 / API Key** 走 08 §13 的档位限速

| 调用来源 | 限速 |
|---|---|
| 工具页前端（Turnstile 通过） | IP 维度 30 次/小时 |
| 登录 Free | 100 次/天（与 API Key 配额一致） |
| 登录 Pro | 5,000 次/天 |
| API Key | 按 08 §13.1 档位 |

> **例外开放端点**（无需登录、无需 Turnstile）：仅状态页公开数据 `/v1/status/<slug>/*` 与节点目录 `/v1/nodes`。

详见 `08-open-api.md`。

---

## 6. 数据模型概览（详见 15-data-model.md）

主要实体（仅列出关键字段示意）：

```
probe_task
  id, type (http|ping|dns|...), target, params (jsonb),
  initiated_by (user_id|null), client_ip, user_agent,
  node_selection (jsonb), created_at, completed_at, status

probe_result
  id, task_id, node_id, raw (jsonb), summary (jsonb),
  duration_ms, success, error, created_at

report
  id (r_xxx), task_ids (jsonb array), target_domain,
  owner_id (nullable), visibility (public|private|password),
  password_hash, expires_at, summary (jsonb), score,
  created_at

ip_info_cache
  ip, asn, isp, country, region, city, raw, source, ttl

whois_cache / icp_cache / ssl_cache
  key (domain or ip), raw, parsed (jsonb), fetched_at, ttl
```

---

## 7. 关键交互流程

### 7.1 单次 HTTP 拨测全流程

```
User → /tools/http (输入 URL)
  → 前端 fetch POST /v1/probe/http (含 Turnstile token)
  → Gateway:
      • 校验 Turnstile / API Key
      • 限速检查
      • 拒测黑名单
      • 创建 probe_task
  → Scheduler:
      • 按节点筛选规则下发任务
      • 通过 WSS push 到节点
  → Nodes (并行):
      • 执行 HTTP 请求
      • 上报 probe_result
  → Aggregator:
      • 聚合结果、计算 summary
      • 标记 task completed
      • 异步缓存
  → Frontend (SSE / 长轮询):
      • 实时接收节点结果
      • 渲染表格 + 地图
  → User 看到结果
```

### 7.2 一键诊断全流程

```
User → /tools/diagnose (输入 domain)
  → 创建 diagnosis_session (内含 N 个 probe_task)
  → 并行启动所有 probe_task
  → 创建 report (status=running)
  → 前端跳转 /report/<id> (SSR 渲染骨架 + SSE 拉子任务结果)
  → 用户看到逐项填充的报告
  → 全部完成后 status=done，可分享、下载 PDF
```

---

## 8. 性能与可用性目标

| 指标 | 目标 |
|---|---|
| 工具页 LCP（首屏） | ≤ 1.5s（国内 CDN 缓存命中） |
| 拨测 API 服务端开销 | ≤ 100ms（不含节点执行） |
| 单次 ping 测试 P95 | ≤ 5s |
| 单次 HTTP 拨测 P95 | ≤ 8s |
| 一键诊断 P95 | ≤ 20s |
| 公开 API 可用性 | ≥ 99.5% |
| 节点结果丢失率 | ≤ 1% |
| 工具页 SEO 收录率 | ≥ 80%（提交 sitemap 后 30 天） |

---

## 9. 反滥用与安全（要点，详见 §12）

- 所有公开测试入口必须经过 Cloudflare Turnstile
- 拒测目标黑名单：RFC1918 / 政府 / 银行 / 友站 / 用户举报名单
- 单 IP / 单用户多维度限速
- 测试结果保留可追溯水印（节点 ID + 时间 + 目标 + 调用方 IP）
- 高风险目标二次确认弹窗（"该目标可能涉及关键基础设施，请确认你对此次测试有合法权限"）
- 滥用举报通道（每个报告页底部）

---

## 10. i18n（中英双语）

- 路径：默认中文 `/tools/...`；英文 `/en/tools/...`
- 工具说明、FAQ、SEO 文案：中英文独立维护（不靠机器翻译）
- 报告页：用户可切换报告语言
- 数字 / 时间 / 时区：本地化

---

## 11. 阶段交付清单

### S1（0–4 月）必须交付
- T01 / T02 / T03 / T04 / T05 / T06
- Q01–Q05 / Q06 / Q08–Q10 / Q13–Q15 / Q19 / Q20
- U01–U13、U17、U18（约 15 个辅助工具）
- D01 / D02（一键诊断 + 分享）
- 工具页统一模板 + 首页 + sitemap
- 中文为主，英文先骨架（标题+主体翻译，FAQ 后补）

### S2（4–8 月）增量
- T07 / T08 / T10
- Q07 / Q09 / Q11 / Q16 / Q17 / Q18 / Q22
- D03（PDF 导出）
- U14–U16（辅助工具补全）
- 工具页英文 FAQ + 全部翻译
- 报告页可视化升级（地图 + 时序图）
- **v2 NEW: 首页改造为 3-Hero IA(诊断 / Verdict / MCP);移动端 Hero 卡片化**
- **v2 NEW: D05 节点试运行(添加监控前预检)**
- **v2 NEW: V01 /verdict 入口 + V02 /verdict/<id> 公开报告分享**
- **v2 NEW: V03 公开验签 attest.idcd.com/verify + V04 /transparency**
- **v2 NEW: L01-L04 排行榜入口(总览 / 历史 / 方法学 / 退出申请)**
- **v2 NEW: 一键诊断报告底部加 "生成 Verdict 报告" 转化锚点**

### S3（8–14 月）增量
- T09 / T11
- Q12 / Q21
- D04（诊断对比）
- 工具页 A/B 实验框架
- Pro 用户："工具 + 监控"无缝衔接体验
- **v2 NEW: D06 域名 vs 竞品对比报告**
- **v2 NEW: D07 DNS 时光机**
- **v2 NEW: A01 /agent 接入介绍页 + A02 mcp.idcd.com/docs**
- **v2 NEW: A03 MCP 交互式演示场景**

---

## 12. 与其他模块的依赖

| 依赖 | 说明 |
|---|---|
| `10-nodes-and-agents.md` | 所有拨测类工具依赖节点系统 |
| `12-compliance-and-abuse.md` | 黑名单、限速、Turnstile |
| `08-open-api.md` | 所有工具的公开 API 实现一致 |
| `03-account-system.md` | 登录后历史记录、提升额度 |
| `04-monitoring.md` | 转化锚点指向"持续监控" |
| `13-content-and-seo.md` | 工具页 SEO 文案、FAQ 内容运营 |

---

## 13. 风险与开放问题

| 风险 | 影响 | 缓解 |
|---|---|---|
| ICP 备案数据爬取被工信部限制 | 备案查询失效 | 多源容灾（自抓 + 公示 + 申报通道）；缓存策略 |
| SEO 起势慢于预期 | 流量不足 | 长尾关键词 + 友站换链 + 内容运营 + 社区曝光（V2EX/NodeSeek/即刻） |
| 公开 API 被爬虫滥用 | 节点资源浪费 | Turnstile + 多维度限速 + WAF 规则 + IP 信誉库 |
| 节点能力差异导致结果不可比 | 用户困惑 | 节点能力分级（Tier1/2/3）+ 结果带节点元信息 |
| 一键诊断耗节点资源大 | 成本高 | 单用户每小时上限、对未登录限更严、Pro 用户更高优先级队列 |
| 报告 ID 被遍历 | 隐私泄露 | 用 nanoid(10+) 不可猜测 + 速率限制 + 30 天过期默认 |

---

## 14. 开放决策点

下面这些后续展开 PRD 时再敲定：

- [ ] 公开节点目录页是否展示节点 IP？还是只展示 ASN+ISP+城市？（透明度 vs 反向利用）
- [ ] 工具页是否带 AdSense？放在哪些位置不破坏体验？
- [ ] 一键诊断的 6 个维度评分权重？需要做 A/B 实验
- [ ] 报告默认过期时间 30 天合适吗？（业内 7–90 天均有）
- [ ] 英文版 SEO 是抢占国内英文站长，还是认真做海外（北美 vs 东南亚优先级）
