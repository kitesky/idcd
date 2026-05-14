import type { Metadata } from "next"

export const metadata: Metadata = { title: "MCP 工具列表 | idcd 文档" }

const TOOLS = [
  { name: "ping",       desc: "多节点 ICMP Ping 检测",          params: "target (域名/IP), nodes (可选), count (可选)" },
  { name: "http",       desc: "HTTP/HTTPS 连通性检测",           params: "url, method (可选), nodes (可选)" },
  { name: "dns",        desc: "DNS 解析查询",                    params: "domain, type (A/AAAA/MX/TXT 等)" },
  { name: "traceroute", desc: "路由追踪",                        params: "target, nodes (可选)" },
  { name: "ssl",        desc: "SSL 证书检测",                    params: "domain" },
  { name: "ip",         desc: "IP 归属与 ASN 查询",              params: "ip" },
  { name: "whois",      desc: "WHOIS 查询",                      params: "domain" },
  { name: "diagnose",   desc: "一键全面诊断（并发多项检测）",     params: "target" },
]

export default function McpToolsPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>MCP 工具列表</h1>
      <p>以下工具可通过 MCP Server 在 Claude、Cursor 等 AI 工具中直接调用。</p>
      {TOOLS.map(tool => (
        <div key={tool.name}>
          <h2><code>{tool.name}</code></h2>
          <p>{tool.desc}</p>
          <p><strong>参数：</strong>{tool.params}</p>
        </div>
      ))}
    </article>
  )
}
