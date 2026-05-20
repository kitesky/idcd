# MCP Server 专项测试报告 — 2026-05-20 部署 / 2026-05-21 修完

> 第一次把 `backend/apps/mcp` 部署到 staging（SG 节点 43.134.175.79）并做端到端测试,然后一口气把 6 处发现全修干净。

## 一句话结论

**协议层 + 工具层全绿;agent 拨测层留 2 个独立问题**。MCP server 镜像 build + Bearer 鉴权 + JSON-RPC + SSE + nginx mcp.idcd.com 反代全通;7 个工具(dns/ip/whois/ssl/http + 多步 diagnose 中的 4 项)返回真实数据;ping/traceroute 链路通到 agent 但 agent ICMP 发包失败(M8/M9,独立 task)。

---

## 修复对照表

| ID | 严重度 | 问题 | 修法 | 状态 |
|---|---|---|---|---|
| M1 | P0 | apiclient 不解 api 的 `data` wrapper → 所有字段空 | apiclient.do() 加 envelope 剥离 + 错误 envelope 解 code/message,3 个新单测覆盖 | ✅ |
| M2 | — | tools query 参数名错配 | **误诊**:mcp schema 字段(给 LLM 用)与 api 内部 query 名本是两层映射,所有 tool 已经正确,无需修 | ✅ |
| M3 | P1 | 工具强制 IDCD_API_KEY,破坏 v2 D2 双池独立 | 8 个 tool handler 全部移除 HasAPIKey 反向门;旧 TestNoAPIKey 重写为正向 contract | ✅ |
| M4 | P1 | staging 没 enrolled agent,probe 类返回 503 | sysctl `ping_group_range=0 65535` + 跑 agent 容器 host network + enroll → status=active | ✅ |
| M5 | P2 | tool 渲染层缺空字段守护,半空字符串 | ssl/ip/diagnose 渲染加 `if != ""` | ✅ |
| M6 | P2 | deploy.sh 没把 mcp 纳入烟测 | step 5 等 idcd-mcp healthy(允许 missing) + step 6 加 `/healthz` 探活 | ✅ |
| M7 | P0 | probe 工具按 sync 解析但 api 是 async | apiclient 加 `PollProbeTask`(1s/30s);ping/http/traceroute/diagnose 切 polling;参数名 + params 子对象修正 | ✅ |
| M8 | P2 | agent host network 下 ICMP socket OK 但 packets_received=0 | 留作独立任务 | 🔍 |
| M9 | P2 | agent traceroute 30s 未完成 | 同 M8 | 🔍 |

## E2E 复测结果(2026-05-21 修后)

```
[100] dns        ✓ DNS github.com (A): A: 20.205.243.166 (TTL: 1)
[101] ip         ✓ IP: 1.1.1.1 / ASN: AS13335 Cloudflare / 位置: South Brisbane, 澳大利亚 / ISP: Cloudflare, Inc
[102] whois      ✓ WHOIS: cloudflare.com / 注册商: Cloudflare, Inc. / 注册 2009 到期 2033
[103] ssl        ✓ 证书: cloudflare.com | 颁发者: WE1 | 有效期 2026-08-08 (还有 80 天)
[120] http       ✓ GET https://example.com  状态: 200 | 总耗时: 17ms
                   阶段: Connect 1ms · SSL 6ms · TTFB 17ms · Server 10ms / TLS: TLS 1.3
[121] traceroute △ probe task timed out before completion (M9)
[122] diagnose   ✓ DNS 2 records / HTTP 200 17ms / SSL valid expires 2026-07-01 / Ping 100% loss (M8)
[110] ping       △ 节点 nd_WqYjSR8Lpux2 / 发送 5 收到 0 / 100% loss (M8 agent bug)
```

**结论:7/8 工具协议+渲染层 100% 干净,1 个工具(traceroute)被 M9 卡;ping 渲染对了但底层 agent 拿不到 ICMP reply(M8)。**

---

## 已合入代码改动(等用户 commit)

| 文件 | 改动 |
|---|---|
| `backend/apps/mcp/Dockerfile` | 新建 |
| `backend/apps/mcp/cmd/mcp/main.go` | 加 `/healthz` |
| `backend/apps/mcp/internal/apiclient/client.go` | data wrapper unmarshal + PollProbeTask + ErrProbeTimeout |
| `backend/apps/mcp/internal/apiclient/client_test.go` | +3 单测(envelope/fallback/coded error) |
| `backend/apps/mcp/internal/integration/integration_test.go` | TestNoAPIKey 重写为正向 contract;TestPingWithMockAPI 改 async mock |
| `backend/apps/mcp/internal/tools/{ping,http,traceroute,diagnose}.go` | 改 PollProbeTask + 新 result schema + params 子对象 + 渲染空值守护 |
| `backend/apps/mcp/internal/tools/{ssl,ip}.go` | 渲染层空字段守护 |
| `backend/apps/mcp/internal/tools/{dns,whois}.go` | 移除 HasAPIKey 校验 |
| `backend/apps/mcp/go.sum` / `backend/apps/agent/go.sum` | `go mod tidy` 补齐传递依赖 |
| `backend/infra/docker/docker-compose.prod.yml` | 新增 mcp service entry |
| `backend/infra/nginx/nginx.conf` | 新增 mcp.idcd.com server block(SSE 长连接配置) |
| `backend/scripts/deploy.sh` | 烟测纳入 mcp |
| `TASKS.md` | M 系列 M1-M9 |

## SG 节点遗留状态

- `pat_mcptest01` (idcd_pat_mcptest_4b9c2a4334e49a6fe2a6b3a14d83a256) — 7 天测试 PAT,2026-05-27 过期
- 多个 `ent_*` 已 used,6h 过期自动清
- `nd_WqYjSR8Lpux2` — agent 节点,active,跑在 host network docker 容器 `idcd-agent-staging`
- `/opt/idcd/agent/{config.yaml,data/}` — agent 工作目录
- nginx `api.idcd.com.crt → staging.idcd.com.crt` symlink + `conf.d/admin-allowlist.conf` 空占位
- host `net.ipv4.ping_group_range=0 65535` + 写入 /etc/sysctl.conf 持久化
- `/opt/idcd/.env` 加 `MCP_INTERNAL_API_KEY=<PAT>`(M3 后不必要但留着不影响)

## 测试统计

- 单元测试:`go test ./apps/mcp/...` **93 passed in 5 packages**
- E2E 真实调用(staging):**6 个工具完美** + 2 个工具受 M8/M9 限制(链路本身 OK)
