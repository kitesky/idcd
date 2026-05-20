import Link from "next/link"
import { SearchCode, Bot, Network } from "lucide-react"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

export const metadata = {
  title: "AI Agent 接入 | idcd",
  description:
    "通过 MCP 协议让 AI Agent 调用 idcd 全球探测能力。支持 Cursor、Claude Code、Codex，13 个工具函数，Free 档即用。",
}

// ── 1. Hero ──────────────────────────────────────────────────────────────────

function Hero() {
  return (
    <section className="pt-20 pb-16 text-center px-4">
      <div className="flex flex-wrap justify-center gap-2 mb-6">
        <Badge variant="secondary">基于 MCP 标准协议</Badge>
        <Badge variant="secondary">13 个工具函数</Badge>
      </div>
      <div className="flex flex-wrap justify-center gap-8 mb-8 text-center">
        <div>
          <div className="text-3xl font-bold">13</div>
          <div className="text-sm text-muted-foreground">MCP 工具函数</div>
        </div>
        <div>
          <div className="text-3xl font-bold">30+</div>
          <div className="text-sm text-muted-foreground">全球探测节点</div>
        </div>
        <div>
          <div className="text-3xl font-bold">Free</div>
          <div className="text-sm text-muted-foreground">立即可用</div>
        </div>
      </div>
      <h1 className="text-4xl sm:text-5xl font-bold tracking-tight text-foreground mb-4">
        AI Agent 时代的可观测枢纽
      </h1>
      <p className="max-w-2xl mx-auto text-lg text-muted-foreground mb-8">
        通过标准 MCP 协议，让 Cursor、Claude Code、Codex 直接调用 idcd 的全球探测能力
      </p>
      <div className="flex flex-wrap justify-center gap-3">
        <Button asChild>
          <Link href="/docs/mcp">查看 MCP 文档</Link>
        </Button>
        <Button variant="outline" asChild>
          <Link href="/auth/register">免费开始</Link>
        </Button>
      </div>
    </section>
  )
}

// ── 2. 应用场景 ──────────────────────────────────────────────────────────────

const useCases = [
  {
    Icon: SearchCode,
    title: "DevOps 故障排查",
    desc: "让 Claude / Cursor 直接从全球节点 Ping、MTR、检测 SSL 到期，AI 自动输出根因报告，无需手动 SSH。",
    example: "「检查 api.example.com 在日本和欧洲的延迟，给我 MTR 路由图」",
  },
  {
    Icon: Bot,
    title: "CI/CD 自动监控",
    desc: "Service Token 接入流水线，每次部署后自动调用 probe_http + probe_ssl 验证上线健康状态，异常时 AI 写故障摘要。",
    example: "「部署完成后验证 HTTPS 证书和 API 响应时间，超 800ms 告警」",
  },
  {
    Icon: Network,
    title: "AI 应用出口观测",
    desc: "当你的 LLM 应用调用外部 API 时，idcd MCP 可实时测量每条出口链路质量，Agent 自动识别慢 API 并建议切换。",
    example: "「对比 OpenAI 和 Claude API 从中国大陆的延迟，推荐最优接入点」",
  },
]

function UseCases() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-2">典型应用场景</h2>
        <p className="text-center text-muted-foreground mb-8">你的 AI 助手现在能做这些</p>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {useCases.map((uc) => (
            <Card key={uc.title} className="flex flex-col">
              <CardHeader className="pb-3">
                <div className="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10">
                  <uc.Icon className="h-5 w-5 text-primary" />
                </div>
                <CardTitle className="text-base">{uc.title}</CardTitle>
              </CardHeader>
              <CardContent className="flex-1 flex flex-col gap-3">
                <p className="text-sm text-muted-foreground">{uc.desc}</p>
                <blockquote className="text-xs italic text-muted-foreground bg-muted rounded-md px-3 py-2 border-l-2 border-primary/40">
                  {uc.example}
                </blockquote>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 3. 接入方式 ──────────────────────────────────────────────────────────────

const integrations = [
  {
    title: "Cursor 接入",
    language: "json",
    code: `// .cursor/mcp.json
{
  "mcpServers": {
    "idcd": {
      "url": "https://mcp.idcd.com/sse",
      "token": "YOUR_TOKEN"
    }
  }
}`,
  },
  {
    title: "Claude Code 接入",
    language: "bash",
    code: `claude mcp add idcd \\
  --transport sse \\
  https://mcp.idcd.com/sse`,
  },
  {
    title: "Codex 接入",
    language: "bash",
    code: `export MCP_SERVER_URL=https://mcp.idcd.com/sse
export MCP_TOKEN=YOUR_TOKEN

codex --mcp $MCP_SERVER_URL`,
  },
]

function Integrations() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-8">三步接入，零学习成本</h2>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {integrations.map((item) => (
            <Card key={item.title} className="flex flex-col">
              <CardHeader>
                <CardTitle className="text-base">{item.title}</CardTitle>
              </CardHeader>
              <CardContent className="flex-1">
                <pre className="bg-muted rounded-md p-4 overflow-x-auto">
                  <code className="text-xs font-mono leading-relaxed whitespace-pre">
                    {item.code}
                  </code>
                </pre>
              </CardContent>
            </Card>
          ))}
        </div>
        <p className="text-center text-sm text-muted-foreground mt-6">
          还没有 Token？{" "}
          <Link href="/app/settings/tokens" className="text-primary underline underline-offset-4">
            前往控制台创建 →
          </Link>
        </p>
      </div>
    </section>
  )
}

// ── 4. MCP Tools 列表 ────────────────────────────────────────────────────────

const mcpTools = [
  { name: "probe_http", desc: "HTTP 可用性探测" },
  { name: "probe_ping", desc: "ICMP Ping 延迟检测" },
  { name: "probe_dns", desc: "DNS 解析查询" },
  { name: "probe_tcp", desc: "TCP 端口连通性" },
  { name: "probe_mtr", desc: "网络路由追踪" },
  { name: "probe_ssl", desc: "SSL 证书检测" },
  { name: "probe_whois", desc: "域名 Whois 查询" },
  { name: "probe_icp", desc: "ICP 备案查询" },
  { name: "lookup_ip", desc: "IP 归属地查询" },
  { name: "lookup_asn", desc: "ASN 自治域信息" },
  { name: "lookup_bgp", desc: "BGP 路由信息" },
  { name: "monitor_create", desc: "创建监控任务" },
  { name: "diagnose_run", desc: "一键网络诊断" },
]

function McpTools() {
  return (
    <section className="py-12 px-4 bg-muted/30">
      <div className="max-w-screen-xl mx-auto">
        <div className="text-center mb-8">
          <h2 className="text-2xl font-bold mb-2">13 个开箱即用的工具函数</h2>
          <p className="text-muted-foreground">覆盖全链路网络探测，让 Agent 随时掌握网络状况</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {mcpTools.map((tool) => (
            <Card key={tool.name}>
              <CardContent className="flex items-start gap-3 pt-4">
                <code className="text-xs font-mono text-primary bg-primary/10 rounded px-2 py-1 shrink-0 mt-0.5">
                  {tool.name}
                </code>
                <span className="text-sm text-muted-foreground leading-tight pt-1">
                  {tool.desc}
                </span>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    </section>
  )
}

// ── 5. Token 档位说明 ────────────────────────────────────────────────────────

const tokenTiers = [
  {
    name: "Personal Token",
    validity: "24h 有效",
    desc: "适合个人调试，快速试用全部 MCP 工具",
    badge: "调试",
  },
  {
    name: "Workspace Token",
    validity: "90d 有效",
    desc: "适合团队共享，多成员协作一个 MCP 端点",
    badge: "团队",
  },
  {
    name: "Service Token",
    validity: "90d 自动续期",
    desc: "适合 CI/CD 流水线，无感知自动刷新",
    badge: "自动化",
  },
]

function TokenTiers() {
  return (
    <section className="py-12 px-4">
      <div className="max-w-screen-xl mx-auto">
        <h2 className="text-2xl font-bold text-center mb-8">灵活的 Token 档位</h2>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {tokenTiers.map((tier) => (
            <Card key={tier.name}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className="text-base">{tier.name}</CardTitle>
                  <Badge variant="outline">{tier.badge}</Badge>
                </div>
                <p className="text-sm font-medium text-primary">{tier.validity}</p>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground">{tier.desc}</p>
              </CardContent>
            </Card>
          ))}
        </div>
        <div className="text-center mt-8">
          <Button asChild variant="outline">
            <Link href="/app/settings/tokens">进入控制台创建 Token</Link>
          </Button>
        </div>
      </div>
    </section>
  )
}

// ── 6. Agent Pro 定价 ────────────────────────────────────────────────────────

function AgentProSection() {
  return (
    <section className="py-12 px-4 bg-muted/20">
      <div className="max-w-2xl mx-auto">
        <Card className="border-primary/40 shadow-lg relative overflow-hidden">
          <div className="absolute top-0 right-0 m-4">
            <Badge className="bg-primary text-primary-foreground">推荐</Badge>
          </div>
          <CardHeader>
            <CardTitle className="text-xl">Agent Pro</CardTitle>
            <div className="flex items-baseline gap-1 mt-1">
              <span className="text-4xl font-bold">¥299</span>
              <span className="text-muted-foreground">/月</span>
              <span className="text-sm text-muted-foreground ml-2">（年付 ¥239/月）</span>
            </div>
          </CardHeader>
          <CardContent>
            <ul className="space-y-2 mb-6">
              {[
                "1,000,000 MCP Units/天（独立配额池）",
                "全部 13 个 MCP 工具函数",
                "全球 30+ 探测节点",
                "Service Token 不限数量",
                "SSE 并发连接 50 路",
                "优先技术支持",
              ].map((f) => (
                <li key={f} className="flex items-center gap-2 text-sm">
                  <span className="text-primary">✓</span>
                  {f}
                </li>
              ))}
            </ul>
            <p className="text-xs text-muted-foreground mb-4">
              MCP Units 与 API 配额完全独立，不影响现有订阅。可单独购买，无需升级主套餐。
            </p>
            <Button className="w-full" asChild>
              <Link href="/auth/register">立即开通</Link>
            </Button>
          </CardContent>
        </Card>
      </div>
    </section>
  )
}

// ── 7. CTA 底部横幅 ──────────────────────────────────────────────────────────

function CtaBanner() {
  return (
    <section className="py-16 px-4 bg-primary/5 border-y">
      <div className="max-w-2xl mx-auto text-center">
        <p className="text-lg font-medium text-foreground mb-2">
          MCP Units 与 API 配额完全独立，Free 档即可体验
        </p>
        <p className="text-muted-foreground mb-6">
          注册后立即获得 MCP 访问权限，Free 档含 10,000 MCP Units/天，无需信用卡
        </p>
        <Button size="lg" asChild>
          <Link href="/auth/register">注册免费账号</Link>
        </Button>
      </div>
    </section>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function AgentPage() {
  return (
    <main>
      <Hero />
      <UseCases />
      <Integrations />
      <McpTools />
      <TokenTiers />
      <AgentProSection />
      <CtaBanner />
    </main>
  )
}
