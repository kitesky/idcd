import type { Metadata } from "next"

export const metadata: Metadata = { title: "MCP Server | idcd 文档" }

export default function McpPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>idcd MCP Server</h1>
      <p>idcd 提供 MCP（Model Context Protocol）server，让 AI 工具（Claude、Cursor、Codex CLI 等）直接调用全球网络拨测能力。</p>

      <h2>支持的工具</h2>
      <table>
        <thead><tr><th>工具</th><th>功能</th></tr></thead>
        <tbody>
          {[["ping","多节点 ICMP Ping 检测"],["http","HTTP/HTTPS 连通性检测"],["dns","DNS 解析查询"],["traceroute","路由追踪"],["ssl","SSL 证书检测"],["ip","IP 归属与 ASN 查询"],["whois","WHOIS 查询"],["diagnose","一键全面诊断"]].map(([tool, desc]) => (
            <tr key={tool}><td><code>{tool}</code></td><td>{desc}</td></tr>
          ))}
        </tbody>
      </table>

      <h2>安装</h2>
      <h3>stdio 模式（推荐 Claude Code / Cursor）</h3>
      <pre><code>go install github.com/kite365/idcd/apps/mcp/cmd/mcp@latest</code></pre>

      <h3>配置 API Key</h3>
      <pre><code>export IDCD_API_KEY=sk_live_your_key_here</code></pre>
      <p>前往 <a href="/app/settings/api-keys">idcd 控制台</a> 创建 API Key。</p>

      <h2>快速链接</h2>
      <ul>
        <li><a href="/docs/mcp/quickstart">5 分钟快速开始</a></li>
        <li><a href="/docs/mcp/authentication">认证配置</a></li>
        <li><a href="/docs/mcp/tools">工具列表</a></li>
        <li><a href="/docs/mcp/examples/claude-code">Claude Code 集成</a></li>
        <li><a href="/docs/mcp/examples/cursor">Cursor IDE 集成</a></li>
      </ul>
    </article>
  )
}
