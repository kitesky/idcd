# 01 · 品牌定调与命名

> 状态:**已锁定(2026-05-13)**
> 关联决策:OVERVIEW.md §11 决策 #8 / DECISIONS.md §A

---

## 1. 决策(LOCKED 2026-05-13)

### 品牌名 = **idcd**

- **中文名**:idcd(不译,保持原 4 字母)
- **英文名**:idcd(小写,所有场合)
- **域名**:`idcd.com`(已持有,不变)
- **调性**:无明确语义的字母组合,类似 `vercel` / `stripe` / `cloudflare` — 现代、技术、不卖弄

### 命名理念

**idcd 是一个反品牌的品牌名**:

- 不追求"领域暗示"(故意不含 net / probe / monitor / sonar 等词)
- 不追求"叙事性"(没有"听起来像 X"的故事)
- 不追求"中英双名"(中文和英文是同一个字母组合)
- **优势**:无商标冲突 / 无翻译歧义 / 4 字母好打 / 域名 .com 已持有
- **代价**:品牌建设全靠产品 + 内容(无名字本身的传播红利)

### 域名策略

| 域名 | 状态 | 用途 |
|---|---|---|
| `idcd.com` | ✅ 持有 | 主站 + 所有子域 |
| `api.idcd.com` | 子域(免费) | API |
| `attest.idcd.com` | 子域(免费,v2) | Verdict / Attestation |
| `mcp.idcd.com` | 子域(免费,v2) | MCP Server |
| `docs.idcd.com` | 子域(免费) | 主文档 |
| `docs.mcp.idcd.com` | 子域(免费,v2 D3) | MCP 文档 |
| `status.idcd.com` | 子域(免费) | idcd 自家状态页 |
| `*.status.idcd.com` | 泛子域(免费) | 用户状态页 |
| `idcd.cn` | 待评估 | 国内备案可能需要 |
| `idcd.io` | 待评估 | 备用 / 防抢注 |

**只买 `.com`(已有)+ 可选 `.cn`(国内备案)+ 可选 `.io`(防抢注)。其他 TLD 不投入**。

### 商标

- 国内 35/42 类商标:**S2 上线前注册**(自助 ¥2k-3k);"idcd" 4 字母组合做商标的成功率较低,但仍要尝试
- 海外商标:S3 出海前评估
- 若 "idcd" 商标被驳回,考虑"idcd.com"做组合商标 + Logo 一并提交

---

## 2. 品牌叙事(基础版)

### 一句话

> **idcd 是网络可观测平台 + 可证据的网络第三方公证 + AI Agent 时代的可观测枢纽。**

(三栈一体,详 OVERVIEW §1.2)

### About 段落

> idcd 来自 internet diagnostics + clustered distributed — 四个字母,不解释。
> 我们做的事情很简单:**给互联网装一组遍布全球的眼睛,让你能从任意地点 / 任意时刻 / 看任意网站**。
> 这套能力延展出三个产品:基础拨测监控、Evidence-as-a-Service 公证报告、MCP server 给 AI Agent 用。
> idcd.com 是我们的访问入口,也是我们的全部 — 没有公司故事,没有创始人神话,只有产品。

(可在 about 页 / 文档 / 营销素材中复用)

### 调性关键词

- **技术** — 不卖弄,不浮夸,术语用对
- **冷静** — 监控数据是观测事实,不需要情感修饰
- **透明** — 公开节点目录 / 公开 transparency / 公开 verify
- **极简** — 4 字母品牌名 + 直接的功能命名(`idcd_ping` / `idcd_diagnose` / Verdict / Attest / MCP)

---

## 3. Logo 方向(待 DESIGN.md 设计)

### 风格关键词

- 单色 / 双色,避免渐变(技术品牌通常单色)
- 等宽字体或 sans-serif(类似 IBM Plex Mono / Inter)
- 几何元素(网格 / 信号 / 节点),不要拟物
- 黑白可识别(暗色 / 浅色模式都好用)

> ⚠️ **颜色已锁定**: 详见 `DESIGN.md` §3，采用 **shadcn/ui blue theme**。
> 本节仅保留历史候选记录，供 Logo 设计参考。

### 颜色候选(历史记录)

| 方案 | 主色 | 辅色 | 暗示 |
|---|---|---|---|
| 海洋深蓝 | #0A2540 | #00D4FF | 节点 / 远方 / 可信 |
| 雷达绿 | #00FF95 | #1A1A1A | 信号 / 拨测 / 极客 |
| 心电图红 | #FF3D5A | #1A1A1A | 心跳 / 监控 / 警觉 |

### Logo 实施

待 DESIGN.md 锁定颜色 + 字体后,用 AI(Midjourney / DALL-E)生成 3-5 个 Logo 草图 → 创始人选定 → SVG 矢量化。

---

## 4. 用词规范

为保证 PRD / 营销 / 文档 一致,固定下列用词:

| 概念 | 标准用词 | 禁用 |
|---|---|---|
| 平台名 | idcd | iDcd / IDCD / Idcd(大小写敏感,统一小写) |
| 拨测 | 拨测(中文)/ probe(英文) | 测速 / 测试(歧义) |
| 监控 | 监控 / monitor | 检测 / 监测(混用) |
| 模块/服务名 | Evidence-as-a-Service / Evidence 模块 | (无) |
| 报告本身 | Verdict 报告 / verdict report | 鉴定报告 / 司法鉴定 |
| 报告内容 | 观测数据 / 观测记录 | 证据(单独使用易产生歧义) |
| 信任根 | KMS / sign key / root key | 私钥(不专业) |
| MCP 工具 | tool / mcp tool | function / interface |

---

## 5. 待定项(S2/S3 推进)

- [ ] `idcd.cn` 域名注册(国内备案需要时)
- [ ] `idcd.io` 域名注册(防抢注 + 出海备胎)
- [ ] 国内 35/42 类商标注册(S2 上线前)
- [ ] Logo SVG 设计稿(走 DESIGN.md → AI 生成 → 选定)
- [ ] 出海英文 about 页面措辞(S3 出海前)

---

## 6. v1 → v2 → v2.1 变更

- v1:7 组候选名(NetSonar / Pingscope / NetPulse / Beacon ...),未选
- v2(2026-05-12 CEO Plan):scope 扩张到三栈,品牌仍待定
- **v2.1(2026-05-13 Eng Review + Brand Lock)**:品牌锁定为 idcd,67 个 PRD `<Brand>` 占位 + 多处 `<brand>` 小写占位全部替换为 idcd;域名 idcd.com 维持
