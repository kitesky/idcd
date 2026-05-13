# 07 · 报表与数据可视化(v2)

> 关联：OVERVIEW.md §4.6、04 监控、05 告警、09 计费、18 Evidence(Verdict 报告)、19 AI Agent obs
> 关联(v2):DECISIONS.md §K4(LLM 故障复盘提前到 P1/S2)
> 阶段主体：S2 基础仪表盘 + **LLM 故障复盘自动起草上线(v2 P1 提前)**;S3 SLA 报告 + 自定义仪表盘;S4 高级分析
> 品牌名占位：`idcd`

---

## 1. 模块定位

可视化与报表是用户**感知产品价值**的核心。监控数据不可视化等于没有；告警事件没有复盘统计等于没有改进。本模块覆盖：

1. **运营级仪表盘**（控制台首屏，"现在什么状态"）
2. **诊断级详情**（单监控、单事件、单节点深入）
3. **管理级报表**（SLA 月报、季度回顾、年度总结）
4. **数据导出 / 归档**（合规与备份）

### 关键指标

| 指标 | S2 | S3 |
|---|---|---|
| 控制台首屏加载 P95 | ≤ 1.5s | ≤ 1s |
| 详情页（30 天数据）渲染 | ≤ 2s | ≤ 1.5s |
| SLA 报告生成时间 | — | ≤ 60s |
| 报告订阅打开率 | — | ≥ 40% |

---

## 2. 仪表盘层级

### 2.1 个人/团队总览（`/app/dashboard`）

#### 顶部卡片
- 监控总数 / 当前异常数
- 24h 告警事件数 / Ack 状态
- 当月可用率（综合）
- API 配额使用进度

#### 主区
- **当前异常列表**（红色高亮，按持续时间倒序）
- **24h 时序**：所有监控合并的"故障小时数"分布
- **TOP 10 不稳定监控**
- **节点健康**：异常节点数 + 状态分布

#### 侧栏
- 最近 5 个事件
- 团队待处理任务（如有团队）

#### 个性化
- 用户可隐藏 / 排序卡片
- 保存为默认布局

### 2.2 监控总览（`/app/monitors`）
- 见 04-monitoring §6.1（列表 + 卡片 + 看板视图）

### 2.3 单监控详情（`/app/monitors/<id>`）

#### 概览 Tab
- 当前状态 + 连续运行 / 异常时长
- 7d / 30d / 90d / 365d 可用率（切换）
- 响应时间趋势图（可缩放 / 拖拽）
- 节点维度对比图（每节点一条线）
- 最近 N 次拨测列表（可下钻）

#### 节点详情 Tab
- 每节点指标：成功率、平均响应、最近 100 次结果
- 节点对比表

#### 事件 Tab
- 历史告警事件时间线
- 单事件下钻

#### 配置 Tab
- 完整配置（含 YAML）

### 2.4 单事件详情（`/app/alerts/events/<id>`）
- 见 05-alerting §5.2

### 2.5 节点健康看板（`/app/nodes`，可选公开 /nodes）
- 见 10-nodes-and-agents §7

---

## 3. 图表类型与设计规范

### 3.1 主要图表

| 类型 | 用途 |
|---|---|
| Time Series（折线 / 面积） | 响应时间趋势、可用率趋势 |
| Heatmap（热力图） | 24h × 7d 异常分布、节点 × 时间矩阵 |
| Status Strip（状态条） | 90 天逐日上/下/降级 |
| Stacked Bar | 事件类型分布、告警通道分布 |
| Donut / Pie | 状态分布、节点国家分布 |
| Map（地理） | 节点地理 + 实时拨测结果 |
| Gauge | 配额 / 余额使用 |
| Table | 节点对比、事件列表 |
| Sparkline | 卡片内迷你趋势 |

### 3.2 交互
- 时间范围统一选择器（7d / 30d / 90d / 自定义）
- 鼠标悬停显示精确数值 + 时间
- 点击下钻
- 自动刷新（实时图表）/ 用户控制
- 暗色模式适配

### 3.3 颜色规范
- 状态：UP=绿、DEGRADED=黄、DOWN=红、MAINTENANCE=蓝、UNKNOWN=灰
- 严重程度：critical=深红、warning=橙、info=蓝
- 趋势：升=绿、降=红（按指标方向决定语义）

### 3.4 性能要求
- 单图渲染 < 200ms
- 大数据用降采样（如 90 天显示 1h 桶）
- 服务端预聚合 + 客户端缓存
- 懒加载：滚动到才渲染

---

## 4. 自定义仪表盘（S3）

### 4.1 概念
- 用户创建多个仪表盘
- 每个仪表盘由"卡片"组成
- 卡片类型 = 上述图表类型，绑定数据源

### 4.2 编辑器
- 拖拽布局
- 卡片库：选择监控、事件、节点指标作为数据源
- 时间范围 + 过滤器
- 模板变量（如 `$env`、`$region`）

### 4.3 共享 / 嵌入
- 团队成员共享
- 公开链接（带 token 的只读 URL）
- iframe 嵌入

### 4.4 预置模板
- "运营周报"模板
- "SRE 故障复盘"模板
- "性能基线"模板
- 用户可基于模板创建后修改

### 4.5 限制
- Pro：1 个自定义仪表盘
- Team：5 个
- Business：20 个

---

## 5. 可用率与 SLA 报告

### 5.1 可用率计算口径

详见 06-status-pages §13。核心：
- 按 60 秒切片
- 状态：UP / DEGRADED / DOWN
- 公式：`uptime = (up + degraded × 0.5) / total`（用户可选 degraded 算 0/0.5/1）

### 5.2 SLA 报告（S3）

#### 月度报告
- 自动月初生成上月数据
- 包含：
  - 整体可用率
  - 按监控 / 分组的可用率
  - 重大事件清单（持续 > 5 分钟）
  - 平均 MTTA / MTTR
  - SLA 承诺对比（如有用户自定义 SLA 目标）
- 邮件订阅 / PDF 下载 / 控制台查看

#### 季度 / 年度报告
- 趋势对比（环比、同比）
- 改进建议（基于规则引擎，S4 引入 LLM 生成）

#### 客户向 SLA 报告（Team+ 用户）
- 自定义 logo / 抬头
- 适合 SaaS 商发给客户

#### SLA 合同绑定（S4）
- 用户输入 SLA 承诺（如 99.95%）
- 报告自动对比：未达标显示退款义务（如适用）

---

## 6. 故障复盘报告（Postmortem)

> **v2 关键变更**:从 S3 / 规则引擎起草 → **S2 / LLM 自动起草(K4 决策)**。强制人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5 才允许新 prompt 上线。

### 6.1 LLM 自动起草工作流(v2, P1/S2 上线)

```
事件 resolve(monitor_event status=resolved)
    │
    ▼
T+5m  Postmortem Worker 拉取事件结构化输入:
        - 事件元数据(事件 ID / 类型 / 时间窗 / 严重程度)
        - 关联监控配置(目标 / 节点池 / 断言)
        - 时间线(每分钟节点表现 / 告警派发 / Ack / 恢复)
        - 节点维度数据(成功率 / 响应时间 / 路由变化)
        - DNS / SSL / 路由元数据(若可用)
        - 同区域历史相似事件(如有)
    │
    ▼
T+10m  调用 LLM(详 6.3),生成草稿(Markdown):
         - 概要(时间 / 影响范围 / 严重)
         - 时间线(从结构化输入翻译为可读叙述)
         - 受影响节点 + 节点偏差分析
         - 根因建议(LLM 推断,**标注"AI 建议,需验证"**)
         - 改进措施(LLM 建议,**标注"AI 建议"**)
    │
    ▼
T+10m  草稿入库,status=ai_drafted
        UI 灰色显示 + 顶部水印"AI 起草草稿,等待人工审核"
        发邮件 + 站内通知给事件 owner / 团队管理员
    │
    ▼
[强制人工审核]
        用户在 /app/reports/postmortems/<id> 编辑
        每段都可"接受 AI 草稿"或"重写"
        发布前必须勾选"已审核,我对内容负责"
    │
    ▼
status=published → 推送状态页(可选) + 团队分享 + PDF 导出 + 公开链接(可选)
```

### 6.2 输入数据(给 LLM 的 prompt 结构化)

```yaml
event:
  id: inc_xxx
  type: down|degraded
  monitor_id: m_xxx
  target: "https://example.com"
  severity: minor|major|critical
  started_at: ...
  resolved_at: ...
  duration_seconds: ...

timeline:
  - at: 14:30
    nodes: [nd_us_lax_01, ...]
    state: 4/5 nodes failed
    sample_response: "connection timeout"
  - at: 14:35
    actor: user_xxx
    action: acknowledged
  - ...

nodes:
  - id: nd_us_lax_01
    failure_pattern: timeout_consistent
    asn: AS7922
  - ...

similar_history:
  - last_30_days_count: 2
  - last_3_months_count: 5
  - pattern: weekly_friday_evening
```

### 6.3 LLM 选型与 Prompt 约束(v2 D9 + D-Concern7)

**LLM Provider 抽象层(v2 D9 + B0a 锁定)**:
- 详 14 §4.11 — 接口统一,后端可插拔
- **prompt 不保证跨 Provider 一致**:同一 prompt template 在不同 Provider 输出风格 / schema / 鲁棒性不同
- **baseline = 阿里通义(qwen-max)+ DeepSeek**(B0a):
  - 国内 LLM 主选,合规友好 + 与阿里云主控同地 latency 低
  - 月成本 ¥300-500/月(S2),vs Claude/GPT 节省 ~70%
- 企业用户接入自家 LLM(Claude / GPT / 自部署)时:**需自行 prompt 调优 + 自行 eval**(≥4.0/5 才可用);企业版控制台暴露 prompt template + eval pipeline 给企业用户

**Prompt 版本控制**:
- 每个 prompt 有 (provider, version) 二元 key,如 `(claude, v2.3)` / `(gpt, v2.3)`
- 变更走 review;**per-Provider 独立 eval ≥ 4.0/5 才能 ship**
- 跨 Provider 升级不耦合(Claude v2.4 可独立 ship,不需等 GPT v2.4)

**输出约束**(避免幻觉):
- 必须以"AI 草稿,需验证"开头
- 严格 Markdown schema(章节固定)
- "根因建议" 段必须用 "推测 / 可能 / 建议进一步检查" 等弱化措辞,**禁止断言**
- 任何具体责任方(如"AWS 出口故障")必须标注"基于公开信息推断,非事实"
- **不允许提及任何具体人名 / 团队 / 组织的过失**(法律边界)

**输出后处理 sanitize(v2 D-Concern7 增强 — 禁用词字典)**:
- 移除任何 HTML/Script(纯 Markdown)
- URL 必须 https + 白名单域(防钓鱼)
- 长度限制(超长截断 + 标注)
- **禁用词字典**(D-Concern7,S2 上线前构建):
  - 法律强词:"违法"、"侵权"、"过错"、"责任"、"赔偿"、"诉讼"
  - 武断判断词:"明显是"、"显然由于"、"必然是"、"罪魁祸首"
  - 责任分派词:"X 公司未能"、"X 团队的过失"、"X 服务商的责任"
  - LLM 输出包含禁用词 → 替换为弱化措辞("可能与 X 有关"、"建议进一步核查")
- 字典维护:`/admin/llm/sanitize_dict`,Operator 可增删词条;改动入 audit_log

### 6.4 离线 eval(必须 ≥ 4.0/5 才允许新 prompt 上线;v2 D8 bootstrap 方案锁定)

- **数据集**:每月 50 个真实事故(人工标注真实复盘)

**Cold Start Bootstrap(v2 D8,S2 上线前必完成)**:
- **30 个公开事故**:从 AWS / Cloudflare / Azure / 阿里云 / 腾讯云 等公开发布的历史故障公告 (status page / blog) 提取
  - 来源:`https://status.aws.amazon.com/`、`https://www.cloudflare.com/cloudflare-status/`、`https://aws.amazon.com/premiumsupport/technology/pes/`、阿里云历史故障公告
  - 每个事故标注:时间线 / 影响范围 / 公开根因 / 改进措施(基于厂商公告)
- **20 个内部 dogfood 事故**:S1 启动后 idcd 自家故障 + Beta 测试用户的故障案例
- **创始人手动标注**:50 个事故 × ~30min 标注 = ~25h 总投入(S2 上线前完成)
- **数据集存储**:`/data/llm-eval/incident-corpus.jsonl`,版本化(每月新增数据后 bump)
- **后期补充**:S2 上线后,Verdict 用户授权下的故障案例可入数据集(隐私脱敏)

**评估维度**(per-Provider 独立评估,baseline = 阿里通义 + DeepSeek):
- 时间线准确性(0-5)
- 根因建议合理性(0-5)
- 改进措施可行性(0-5)
- 措辞专业性 / 无幻觉(0-5)
- 综合得分 ≥ 4.0/5 才 ship
- **国内 LLM 中文复盘表现需重点验证**(qwen-max 中文 fluency 优于 GPT,但鲁棒性 / 系统性矛盾检测可能略弱,需 25h 标注实测)

**失败处理**:eval 不达标 → 回退到上版 prompt + 通知工程团队改进

**eval pipeline CI 集成**:
- 每次 prompt 变更走 GitHub Actions:跑全 50 条数据集 → 输出 5 维度分数 → 综合 ≥4.0 才允许 merge
- 月度报告:`/admin/llm/eval-trend` 展示 prompt 历史版本的 eval 分数趋势

### 6.5 用户对 AI 草稿的反馈循环(v2 D-Concern7 隐私边界)

- 每个 AI 段落旁有"接受 / 重写 / 删除"按钮
- 用户"重写"的内容入回流数据集,用于改 prompt(可选;用户需在 /app/settings/llm-feedback 主动开启)
- 每月 prompt 改进 ship 时,自动重跑 eval,记录历史趋势

**回流数据隐私边界(v2 D-Concern7 锁定)**:
- 用户重写内容**仅用于内部 eval 数据集** + 内部 prompt 改进
- **绝不发送给 LLM Provider(Anthropic / OpenAI / 阿里云)做模型 train**(不开启 API 的 training_opt_in)
- 回流前自动 sanitize:剔除任何具体公司名 / 个人名 / IP / URL
- 用户可在 /app/settings/llm-feedback 一键导出 + 一键删除所有回流数据
- Anthropic / OpenAI API 调用统一设置 `data_use_for_training=false`(API 参数明确)

### 6.6 边界与拒绝场景

LLM 起草**不发动**的场景:
- 事件持续时间 < 5 分钟(短事件没复盘价值)
- 监控类型 = 心跳 / 域名到期 / SSL 到期(单点信息,不需复盘)
- 用户在监控设置 / 账号设置中关闭"AI 自动起草"

### 6.7 与状态页的协同(详见 06-status-pages.md §X 新增工作流)

- 重大事件触发后,**也自动生成"公告草稿"**(更短,面向客户而非内部)
- 草稿走同样人工审核流;通过后推送状态页 + 邮件订阅者 + 微信 / 钉钉

### 6.8 故障复盘模板(M5-M8 默认模板,可在 §6.3 prompt 中配置)

```markdown
# 事件 #inc_xxx 复盘

> **AI 起草草稿,需人工审核**(发布前请检查每段)

## 概要
- 时间:[from] ~ [to](持续 [duration])
- 影响:[scope]
- 严重程度:[severity]

## 时间线
[由结构化输入翻译,LLM 不"创造"新事件,只描述结构化数据]

## 受影响范围
- 受影响节点:[list]
- 受影响监控:[list]
- 受影响用户感知(若已知)

## 根因建议(AI 推断,需验证)
[弱化措辞,只列可能方向,不断言]

## 改进措施(AI 建议,需团队评估)
- 短期:[list]
- 中期:[list]
- 长期:[list]

## 处理人 / 备注
[人工补充]

## 审核记录
- AI 草稿生成时间:...
- 人工审核人:...
- 审核完成时间:...
```

---

## 7. 数据导出

### 7.1 导出维度

| 维度 | 格式 |
|---|---|
| 监控配置 | JSON / YAML / CSV |
| 拨测原始结果 | CSV / JSON（按时间段）|
| 聚合数据 | CSV |
| 告警事件 | CSV / JSON |
| SLA 报告 | PDF / Markdown |
| 节点列表（含贡献节点）| CSV |

### 7.2 导出方式

- 控制台一键下载（小数据）
- 异步任务（> 10 万行）→ 邮件发链接
- 定时导出 + S3 / OSS / 用户网盘（S3）

### 7.3 限制
- Free：可导出但限 7 天数据
- Pro / Team / Business 按订阅档保留期决定

---

## 8. 数据保留与归档（与 04-monitoring §9 同步）

| 类型 | Free | Pro | Team | Business |
|---|---|---|---|---|
| 原始拨测 | 7d | 30d | 90d | 180d |
| 小时聚合 | 30d | 180d | 1y | 2y |
| 日聚合 | 1y | 2y | 3y | 5y |
| 告警事件 | 30d | 1y | 3y | 5y |

超出保留期：
- 原始数据被聚合替代后删除
- 用户可付费延长归档（add-on）
- 删除前 30 天通知用户

---

## 9. 报告订阅与推送

### 9.1 订阅类型

| 报告 | 频率 | 渠道 |
|---|---|---|
| 每日告警摘要 | 每天 | 邮件 |
| 每周运营周报 | 每周一 | 邮件 + Slack |
| 月度 SLA 报告 | 每月 1 号 | 邮件 + PDF |
| 季度 / 年度回顾 | 季度初 / 年初 | 邮件 + PDF |
| 用量月度账单 | 每月 1 号 | 邮件 |

### 9.2 配置
- `/app/settings/notifications/reports`
- 每种报告独立开关
- 选择通道 + 时区 + 发送时间

### 9.3 PDF 报告
- 模板化（含 Logo、抬头、签名）
- 文字 + 图表（PNG / SVG）
- 适合企业级转发 / 存档

---

## 10. 报告分享

### 10.1 分享链接
- 任何报告 / 仪表盘可生成只读分享链接
- token 校验
- 可设过期时间
- 访问统计（PV / UV）

### 10.2 嵌入
- iframe 嵌入用户网站
- 仪表盘 Widget（JS 嵌入）

### 10.3 权限
- 默认仅团队内可见
- 公开链接需主动生成
- 公开链接可撤销

---

## 11. 高级分析（S3+）

### 11.1 异常检测
- 时序异常检测（基于历史基线）
- 监控响应时间突增警告
- 节点对比异常（一节点异于群体）

### 11.2 趋势预测
- SLA 趋势预测（按当前趋势预计本月可用率）
- 容量预测（API 配额按当前消耗几天用完）

### 11.3 根因建议（S4，规则引擎 + LLM）
- 事件发生后自动分析关联指标
- 输出可能原因：DNS / 网络 / 证书 / 目标服务器 / 上游服务商
- 仅作为参考，不替代人工

---

## 12. 数据模型（聚合层）

```
monitor_check_aggregate_hour
  monitor_id, node_id (nullable),
  bucket_at,
  total, up_count, degraded_count, down_count,
  avg_response_ms, p50, p95, p99,
  min_response_ms, max_response_ms,
  errors_by_type (jsonb)

monitor_check_aggregate_day
  -- 同上，按日

sla_report
  id, owner_id, period (month|quarter|year),
  period_start, period_end,
  uptime_overall, mtta_avg, mttr_avg,
  events_count, critical_events_count,
  monitors_breakdown (jsonb),
  generated_at, file_url

dashboard
  id, owner_id, name, layout (jsonb),
  variables (jsonb), shared_with (user_id[]|public_token),
  created_at, updated_at

dashboard_widget
  id, dashboard_id, type, position (jsonb),
  data_source (jsonb), filters (jsonb), display (jsonb)

scheduled_report
  id, owner_id, type, frequency, time, timezone,
  channels (jsonb), recipients,
  next_run_at, last_run_at

postmortem
  id, event_id, owner_id,
  title, body (markdown),
  generated_by (ai|user|hybrid),       -- v2: ai = LLM 起草; hybrid = AI 起草 + 人工编辑
  status (ai_drafting|ai_drafted|under_review|published|rejected),  -- v2
  llm_model, llm_prompt_version,        -- v2
  ai_draft_at, reviewer_id, review_completed_at,  -- v2
  ai_segments_accepted_count, ai_segments_rewritten_count,  -- v2 (feedback loop)
  published_to_status_page,
  shared_token, created_at
```

---

## 13. 性能与优化

### 13.1 预聚合
- 后台定时任务（每分钟 / 每小时）预聚合
- 查询时优先用聚合数据
- 长时段：日聚合够用；短时段：小时 + 分钟聚合

### 13.2 缓存
- 仪表盘卡片结果缓存 30s-60s
- 用户拉到 95%+ 命中
- 重大数据变化（事件创建 / 状态变化）触发 invalidate

### 13.3 大数据
- 90 天 +：自动降采样到日粒度
- 用户下载原始：异步任务 + S3 URL

### 13.4 数据库
- 时序数据走 TimescaleDB（PG 扩展）/ ClickHouse（S3 大流量后）
- 自动分区（按日 / 周）
- 旧分区压缩 / 归档

---

## 14. 与其他模块接口

| 模块 | 接口 |
|---|---|
| `04-monitoring.md` | 数据源（monitor_check） |
| `05-alerting.md` | 数据源（alert_event） |
| `06-status-pages.md` | SLA 计算口径一致 |
| `08-open-api.md` | 报告查询 API |
| `09-billing.md` | 数据保留 / 自定义仪表盘配额 |
| `10-nodes-and-agents.md` | 节点健康数据 |

---

## 15. 阶段交付清单

### S2（4–8 月）
- 总览仪表盘（控制台首屏）
- 单监控详情（概览 + 节点对比 + 事件列表）
- 单事件详情
- 节点健康看板
- 数据导出（基础：CSV/JSON）
- 月度 SLA 报告（基础，邮件订阅）
- ~~故障复盘自动起草（基础规则）~~  → **替换为 v2 LLM 起草工作流**
- **v2 NEW: LLM 故障复盘自动起草工作流(P1 提前,K4)— 离线 eval ≥ 4.0/5 才上线**
- **v2 NEW: AI 标识 + 强制人工审核 + sanitize**
- **v2 NEW: 反馈循环数据收集(用户接受 / 重写 / 删除统计)**

### S3（8–14 月）
- 自定义仪表盘（拖拽编辑器 + 5 个预置模板）
- SLA 报告增强（季度/年度、客户向、自定义抬头）
- 故障复盘编辑器 + 发布到状态页
- 报告分享与嵌入
- 定时导出 + S3 / OSS
- 异常检测 + 趋势预测
- 容量预测
- **v2 NEW: LLM 复盘 prompt 多模板(SaaS / 内部 / 客户向)**
- **v2 NEW: 多 LLM 投票复盘(可选,降低单 LLM 幻觉)**
- **v2 NEW: 与 Verdict 报告联动(故障取证模板)**

### S4（14+ 月）
- ~~LLM 根因建议~~ → **v2: 已在 S2 P1 上线**
- 多账号合并仪表盘（企业）
- 实时大屏（电视墙模式）
- 高级数据分析（自定义 SQL 查询）
- **v2 NEW: M24 Agent Output Quality 仪表盘**

---

## 16. 风险与开放问题

| 风险 | 缓解 |
|---|---|
| 大数据查询慢 | 严格预聚合 + 降采样 + 异步导出 |
| 自定义仪表盘复杂度高 | 优先模板 + 拖拽简化 |
| 报告生成耗费资源 | 后台队列 + 限速 + 用户档位区分 |
| SLA 计算口径分歧 | 配置可选 + 报告内附口径说明 |
| PDF 渲染卡顿 | Headless Chrome 集群 + 异步生成 |
| **v2 NEW: LLM 复盘幻觉 / 造谣 / 泄密** | Prompt 约束 + 强制人工审核 + AI 标识 + sanitize + 离线 eval ≥ 4.0/5;输出禁止断言责任方 + 弱化措辞 |
| **v2 NEW: LLM 调用费用爆炸** | 单 prompt token 上限 + 用户档位限制 + 月预算硬上限 + 缓存相似事件 |

---

## 17. 开放决策点

- [ ] 自定义仪表盘 S3 上还是 S2 就上？
- [ ] ~~LLM 根因建议是否包月还是按次?~~ → **v2: 内置 Pro 起,不单独计费;Compliance 企业档无限**
- [ ] SLA 报告中 degraded 算几折（0 / 0.5 / 1）？默认建议 0.5
- [ ] 公开仪表盘是否允许（隐私 vs 透明）？
- [ ] 实时大屏目标客户是？是否值得 S4 单独投入
- [ ] **v2 NEW** 多 LLM 投票复盘(降低幻觉):S3 评估
- [ ] **v2 NEW** LLM 复盘是否生成"客户向"模板(更短 / 更礼貌):S2 末根据用户反馈决
