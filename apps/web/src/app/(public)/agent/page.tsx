import Link from "next/link"
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

// ── 2. 接入方式 ──────────────────────────────────────────────────────────────

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
      </div>
    </section>
  )
}

// ── 3. MCP Tools 列表 ────────────────────────────────────────────────────────

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

// ── 4. Token 档位说明 ────────────────────────────────────────────────────────

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
      </div>
    </section>
  )
}

// ── 5. CTA 底部横幅 ──────────────────────────────────────────────────────────

function CtaBanner() {
  return (
    <section className="py-16 px-4 bg-primary/5 border-y">
      <div className="max-w-2xl mx-auto text-center">
        <p className="text-lg font-medium text-foreground mb-2">
          MCP Units 与 API 配额完全独立，Free 档即可体验
        </p>
        <p className="text-muted-foreground mb-6">
          注册后立即获得 MCP 访问权限，无需信用卡
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
      <Integrations />
      <McpTools />
      <TokenTiers />
      <CtaBanner />
    </main>
  )
}
