# 20 · AI 网络诊断助手 / Diagnose Copilot

> 关联：OVERVIEW.md §1.2;DECISIONS.md §K(AI)、§M(Verdict);02-public-tools.md(13 个 probe 工具);04-monitoring.md;19-ai-agent-observability.md(MCP server);08-open-api.md
> 阶段主体：**M2(MVP) → M3(GA)**;不阻塞 v2 主线交付
> 品牌名占位:`idcd`

---

## 1. 模块定位

让访客以**对话方式**描述网络问题(可包含目标域名、症状、地理位置),由 LLM 自动编排调用 idcd 已有的 probe 工具集(13+ MCP tools),给出**根因诊断 + 修复建议**,可一键生成可分享报告。

### 1.1 一句话

> "山西的用户说我们网站打不开" → 30 秒内得到"山西联通 → 阿里云华北节点丢包 12%,建议加 CDN 或换电信线路"。

### 1.2 为什么 `idcd` 适合做

- **MCP 工具链完备**(见 §19):probe_ping / probe_http / probe_dns / probe_mtr / probe_ssl / probe_whois / probe_icp / lookup_ip / lookup_asn / lookup_bgp / diagnose_run 等 13 个 tool,已经为 Agent 使用而设计
- **全球节点天然 grounding**:LLM 不需要"猜",直接从真实节点拿数据,**不存在幻觉空间**
- **LLM 摘要能力已选定**(Pre-2 决策):阿里通义 + DeepSeek 自家底层,月成本 ¥300-500
- **零新基础设施**:纯组装,2 周可交付 MVP

### 1.3 关键指标

| 指标 | M2 MVP | M3 GA |
|---|---|---|
| 日活会话 | 100 | 2,000 |
| 单次诊断平均工具调用数 | 3-5 | 5-8 |
| 诊断到首字 P50 | ≤ 3s | ≤ 1.5s |
| 完整诊断 P95 | ≤ 30s | ≤ 15s |
| 用户主观满意率(👍/总评价) | ≥ 60% | ≥ 80% |
| 注册转化率(首次使用 → 注册) | ≥ 8% | ≥ 15% |
| LLM 单次诊断成本 | ≤ ¥0.1 | ≤ ¥0.05 |

---

## 2. 用户旅程

### 2.1 入口

1. **首页 HeroSearch**:在已有搜索框旁加 "用 AI 诊断" 按钮,带 Sparkles 图标
2. **/tools 工具页头部**:固定一条 "不知道用哪个工具?让 AI 帮你诊断 →"
3. **每个 probe 工具页(ping/http/dns 等)失败结果下**:"用 AI 解读此结果 →"
4. **/copilot 独立落地页**(本 PRD 主路径)

### 2.2 输入形态

**自由文本** + 可选结构化字段:

```
┌─────────────────────────────────────────────────────────────────┐
│ 描述你遇到的问题,或给我一个域名 + 现象描述                       │
│                                                                  │
│ [自由文本输入区,3-5 行]                                          │
│                                                                  │
│ 例:山西联通的用户访问 idcd.com 经常打不开,我自己电信网络正常    │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│ ▸ 目标域名/IP(可选,助手会自动提取)    [               ]        │
│ ▸ 报障地区(可选,选省/市/运营商)        [▼ 山西 - 联通       ]    │
│ ▸ 我的视角(可选,自动取 IP 推断)        [▼ 北京 - 电信       ]    │
└─────────────────────────────────────────────────────────────────┘
                                                       [ 开始诊断 ]
```

### 2.3 诊断输出(流式)

服务器走 SSE(沿用现有 `/api/diagnose/stream` 模式),逐步推送:

```
[Step 1/?] 解析你的问题…
  ↳ 目标: idcd.com
  ↳ 报障视角: 山西-联通
  ↳ 对照视角: 北京-电信

[Step 2/?] 调用 probe_ping 从两个视角各 3 次…
  ✓ 北京-电信  → 12ms / 0% 丢包
  ✗ 山西-联通  → 240ms avg / 14% 丢包 ⚠

[Step 3/?] 调用 probe_mtr 看路径差异…
  ✓ 路径在 219.x.x.x(联通骨干)出现连续高延迟跳

[Step 4/?] 调用 probe_dns 排除解析问题…
  ✓ 两地解析结果一致,记录正常

[Step 5/?] 调用 lookup_bgp 看出口…
  ↳ 源 IP 属 AS37963(阿里云华北),BGP 路径在山西需 6 跳

[诊断结论]
  根因: 山西联通 → 阿里云华北 的网间互联质量异常,而非你的网站故障
  证据: 北京电信延迟 12ms 但山西联通 240ms,MTR 中 219.158.x.x 跳异常
  建议:
    1. 短期: 山西用户提示走 VPN / 切换运营商
    2. 中期: 接入 CDN(腾讯云/阿里云 CDN 均能解决,后者更便宜)
    3. 长期: 业务量大时考虑多线 BGP 接入或多云部署

[操作]
  [ 复制诊断报告 ]  [ 分享链接 ]  [ 创建持续监控 → /app/monitors/new ]
```

### 2.4 后置转化路径

每条诊断结果末尾**根据诊断类型推荐转化路径**:

| 诊断结果类型 | 推荐 CTA |
|---|---|
| 临时性网络抖动 | "创建持续监控,5 分钟内再现立刻告警" → `/app/monitors/new?target=…` |
| 证书/域名即将到期 | "把这个域名加入到期监控" → `/app/monitors/new?type=ssl_expiry` |
| DNS 解析异常 | "订阅 DNS 变更提醒" → `/app/monitors/new?type=dns` |
| 备案变化 | "添加 ICP 备案监控" → `/app/monitors/new?type=icp_change` |
| 跨网延迟 | "查看 CDN 节点对比工具"(链 §未来的多 CDN 对比功能) |
| 真实故障 | "生成证据报告" → 链网页存证功能(§未来) |

---

## 3. 技术架构

### 3.1 数据流

```
浏览器 /copilot                                idcd-api                          probe 节点
    │                                              │                                  │
    │  POST /v1/copilot/sessions { intent, ctx }   │                                  │
    │─────────────────────────────────────────────►│                                  │
    │                                              │  ① intent 解析(LLM call)        │
    │                                              │     提取 target / 症状关键词     │
    │                                              │                                  │
    │                                              │  ② 规划工具调用序列(LLM call)   │
    │                                              │     输出 plan: [ping, mtr, dns]  │
    │                                              │                                  │
    │  SSE stream {                                │                                  │
    │    type: "plan",  steps: [...]               │                                  │
    │    type: "step",  tool: "probe_ping",        │  ③ 顺序/并行执行 probe 调用      │
    │      status: "running"                       │─────────────────────────────────►│
    │    type: "step",  tool: "probe_ping",        │                                  │
    │      status: "done", result: {...}           │◄─────────────────────────────────│
    │    ...                                       │                                  │
    │    type: "verdict",                          │  ④ 综合诊断(LLM call,带工具结果) │
    │      reason: "...", confidence: 0.82,        │                                  │
    │      suggestions: [...],                     │                                  │
    │      cta: { ... }                            │                                  │
    │  } ◄─────────────────────────────────────────│                                  │
    │                                              │                                  │
    │  POST /v1/copilot/sessions/{id}/save         │  ⑤ 保存为可分享 report(Redis    │
    │     → /report/{id}                           │     7d TTL,复用现有 diagnose-   │
    │─────────────────────────────────────────────►│     store)                       │
```

### 3.2 LLM 调用(3 阶段)

按 Pre-2 决策,**自家底层(阿里通义 + DeepSeek)**:

| 阶段 | 模型 | 输入 | 输出 | 期望 token |
|---|---|---|---|---|
| ① intent 解析 | DeepSeek-v3 / qwen-turbo | 用户描述 + 可选字段 | `{target, symptoms[], userGeo, targetGeo}` JSON | in 500 / out 200 |
| ② plan 规划 | DeepSeek-v3 / qwen-plus | intent + 可用 tool schema | `[{tool, args, reason}]` 工具调用计划 | in 1500 / out 500 |
| ③ verdict 综合 | qwen-plus / DeepSeek-r1 | intent + plan + tool results | `{reason, evidence[], suggestions[], cta}` | in 4000 / out 1000 |

**单次会话总成本**:
- in token: ~6000 → ¥0.004(qwen 在线 ¥0.0008/1k token)
- out token: ~1700 → ¥0.003
- 加 tool 调用 N 个 × probe 内部成本 → 总 ¥0.05-0.10/次

### 3.3 工具调用契约

复用现有 MCP server(§19),让 copilot **作为另一个 MCP client** 通过内部 endpoint 调用:

```go
// apps/api/internal/copilot/orchestrator.go
type Orchestrator struct {
    llm  LLMClient        // qwen / deepseek wrapper
    mcp  MCPClient        // 内部 SSE client → mcp.idcd.com
}

func (o *Orchestrator) Run(ctx, intent, plan) <-chan Event {
    out := make(chan Event)
    go func() {
        defer close(out)
        for _, step := range plan.Steps {
            out <- Event{Type: "step_start", Tool: step.Tool}
            res, err := o.mcp.Call(ctx, step.Tool, step.Args)
            out <- Event{Type: "step_done", Tool: step.Tool, Result: res, Err: err}
        }
        verdict, _ := o.llm.Verdict(ctx, intent, plan, results)
        out <- Event{Type: "verdict", Payload: verdict}
    }()
    return out
}
```

复用 §19 的 MCP server **意味着 copilot 的所有改进同时惠及外部 MCP 客户端**。

### 3.4 安全 / 滥用防护

| 风险 | 缓解 |
|---|---|
| LLM prompt injection 让助手执行恶意目标 | (a) 工具参数 type-safe schema,LLM 输出 JSON 解析失败/越界直接拒;(b) target 域名走 SSRF 防御网关(已有);(c) 用户 IP 限速 |
| 用户拿来跑大量诊断(撸羊毛) | Free 5 次/日(IP+cookie),Pro 100 次/日,Pro+ 500 次/日;Service Token 走 MCP unit 配额(K1) |
| LLM 输出敏感内容(政治/色情) | qwen / DeepSeek 都自带审核;输出落地前过一遍敏感词过滤 |
| LLM 幻觉(给出不存在的结论) | (a) 所有诊断结论都必须**有 tool result 作为 evidence**,LLM prompt 强制"无证据不结论";(b) UI 上每条结论都点击展开看原始工具输出 |
| 用户绕过付费墙 | session 落 Redis,匿名 IP 限速;登录用户走 quota |

---

## 4. UI 设计要点

### 4.1 页面: `/copilot`

按 shadcn/ui + zinc 主题,沿用 `/agent` 落地页风格:

```
┌──────────────────────────────────────────────────────────────┐
│ Hero                                                          │
│   "你的网络问题,让 AI 替你诊断"                                │
│   sub: "不知道该用哪个工具?描述现象,30 秒拿结论"               │
│   [Badge: 全球 30+ 节点] [Badge: 13 个诊断工具]               │
│                                                               │
│   ┌─── 输入框 ───────────────────────────────────────────┐    │
│   │ 描述你的问题…                                         │    │
│   │ [textarea]                                            │    │
│   └───────────────────────────────────────────────────────┘    │
│   ▸ 高级选项(目标/地区/视角)                                  │
│                                              [ 开始诊断 ]      │
├──────────────────────────────────────────────────────────────┤
│ Diagnose Stream(诊断中)                                       │
│   流式步骤展示(见 §2.3),每个 step 一张 Card                  │
│   每个 step Card 可展开看 raw tool output                     │
├──────────────────────────────────────────────────────────────┤
│ Verdict(诊断结论)                                             │
│   大字 + Badge(confidence) + 建议列表 + CTA 按钮组            │
├──────────────────────────────────────────────────────────────┤
│ 历史(登录后)                                                  │
│   左侧 sidebar:最近 10 次诊断                                 │
└──────────────────────────────────────────────────────────────┘
```

### 4.2 复用 / 新建组件

| 组件 | 状态 |
|---|---|
| `<HeroSearch>` | 复用 |
| `<DiagnoseStream>` | **新建**(从 `/tools/diagnose/diagnose-client.tsx` 抽取共用) |
| `<VerdictCard>` | **新建** |
| `<ToolResultDrawer>` | **新建**(展开看 raw tool output) |
| `<ShareButton>` | 复用 `/report/[id]` 的 |

---

## 5. 计费 / 配额

| 档位 | Copilot 限额 | 备注 |
|---|---|---|
| 匿名(IP 限速) | 3 次/日/IP | 防爬虫,引导注册 |
| Free | 5 次/日 | 历史保留 7 天 |
| Pro | 100 次/日 | 历史保留 90 天,可下载 PDF |
| Pro+ | 500 次/日 | 历史保留 1 年,可批量诊断 |
| Service Token(MCP) | 走 MCP unit 配额(K1) | 独立池,不占 Copilot 额度 |

每次诊断的工具调用**也吃 API 配额**(§09-billing),但 LLM 推理成本由 idcd 吸收(已计入档位定价)。

---

## 6. 阶段拆解

### 6.1 M2 (MVP, ~2 周)

- [ ] `/copilot` 落地页 + 输入表单
- [ ] 后端 `POST /v1/copilot/sessions` + SSE stream
- [ ] LLM 编排:intent 解析 + plan 规划 + verdict 综合(单 LLM 客户端,先用 qwen-plus 一种)
- [ ] 工具调用:复用现有 probe handler(不走 MCP server,直接 internal call,减少 hop)
- [ ] 流式 UI(从 `/tools/diagnose` 抽取 + 改造)
- [ ] 复用 `/report/[id]` 做分享
- [ ] 匿名 + 登录两种入口
- [ ] IP 限速 + Redis 配额计数
- [ ] 单元测试:LLM mock + 工具 mock,覆盖 plan 解析 / verdict 解析 / SSE 流

**M2 验收**:
- 给 5 个真实问题(选择题:DNS 污染 / 跨网延迟 / 证书过期 / 备案失效 / 单纯抖动),AI 能给出正确根因 ≥ 3/5
- P95 延迟 ≤ 30s

### 6.2 M3 (GA, ~2 周)

- [ ] 接 MCP server(§19)正式通道,不再走 internal call
- [ ] 双模型 fallback(qwen + DeepSeek,主备 + 成本最优路由)
- [ ] PDF 下载(走 Evidence 模板 §18)
- [ ] 登录后历史 + 收藏夹
- [ ] 后置 CTA 转化路径全打通(到 monitors / cdn 对比 / 存证)
- [ ] 离线 eval:每周 50 条真实事故人工评分,prompt 版本管理
- [ ] 滥用防护监控 dashboard

---

## 7. 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| LLM 给出错误结论误导用户 | 中-高 | 强证据链 + UI 上每条结论可展开看原始工具输出 + "AI 草稿"水印(参 §19 §2.4)|
| LLM 成本失控 | 中 | 单次会话硬上限 8 个 tool 调用 + 4000 token verdict;每个档位日额度;qwen + DeepSeek 双供应商防价格突变 |
| 工具调用串行慢 | 中 | LLM plan 输出明确 `parallel: true` 的步可并发执行;复用现有 probe 任务并发框架 |
| 用户隐私(对话被 LLM 训练) | 高 | 阿里通义 / DeepSeek 企业版都支持"不入训练";合同写明;隐私页明示 |
| MCP server 还没 GA 就被 Copilot 拖死 | 中 | M2 走 internal call 不走 MCP,M3 再切;切换通过 feature flag |

---

## 8. 决策点(留待 ENG-REVIEW)

| D | 内容 | 候选 | 默认 |
|---|---|---|---|
| D-Copilot-1 | LLM 主模型选哪个 | qwen-plus / DeepSeek-v3 / 二者 fallback | 二者 fallback(M3),M2 先 qwen-plus |
| D-Copilot-2 | 工具调用是否走 MCP server | 走 MCP / 直 internal | M2 直 internal,M3 走 MCP |
| D-Copilot-3 | 历史保留是否分档差异化 | 全档 7 天 / 按档(7/90/365) | 按档(已写入 §5) |
| D-Copilot-4 | 评分回流是否开启 | 默认关闭 / 默认开启(可关) | 默认开启,匿名化后入 eval 集 |
| D-Copilot-5 | PDF 下载是否要 KMS 签名 | 都要 / 只 Pro+ 要 | 只 Pro+ 要(默认 PDF 无签,签名版走 §18 Evidence)|

---

> **状态:Draft,本 PRD 不阻塞 v2 主线,M2 开工前请 review §6 拆解 + §8 决策点。**
