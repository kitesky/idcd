import type { Metadata } from "next"

export const metadata: Metadata = { title: "Claude Code 集成 | idcd MCP 文档" }

export default function ClaudeCodeExamplePage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>Claude Code 集成</h1>
      <p>在 Claude Code 中集成 idcd MCP Server，让 Claude 直接调用全球网络拨测能力。</p>

      <h2>前置条件</h2>
      <ul>
        <li>已安装 <a href="https://claude.ai/code" target="_blank" rel="noopener noreferrer">Claude Code</a></li>
        <li>已在 <a href="/app/settings/api-keys">idcd 控制台</a> 创建 API Key</li>
        <li>已安装 Go 1.21+（用于编译 MCP server）</li>
      </ul>

      <h2>安装 MCP Server</h2>
      <pre><code>go install github.com/kite365/idcd/apps/mcp/cmd/mcp@latest</code></pre>
      <p>安装后 <code>mcp</code> 命令将在 <code>$GOPATH/bin</code> 中可用。</p>

      <h2>添加到 Claude Code</h2>
      <h3>方式一：命令行直接添加（推荐）</h3>
      <pre><code>{`# 设置 API Key 环境变量
export IDCD_API_KEY=sk_live_your_key_here

# 添加 MCP server
claude mcp add idcd -- $(which mcp)`}</code></pre>

      <h3>方式二：通过配置文件</h3>
      <p>编辑 <code>~/.claude.json</code>：</p>
      <pre><code>{`{
  "mcpServers": {
    "idcd": {
      "command": "/path/to/mcp",
      "env": {
        "IDCD_API_KEY": "sk_live_your_key_here"
      }
    }
  }
}`}</code></pre>

      <h2>使用示例</h2>
      <p>添加成功后，在 Claude Code 对话中直接描述网络检测需求：</p>
      <pre><code>{`你: 检测一下 google.com 从全球主要节点的 Ping 延迟
Claude: [调用 ping tool]
检测结果：
节点: Tokyo JP     — 28ms ✓
节点: Singapore SG — 45ms ✓
节点: US East      — 98ms ✓
平均延迟: 57ms | 丢包率: 0%

你: 我的网站 https://example.com 在中国大陆可以访问吗？
Claude: [调用 http tool，选择中国大陆节点]
HTTP 检测结果（北京节点）：
状态码: 200 OK
响应时间: 342ms
结论：可以正常访问`}</code></pre>

      <h2>可用工具</h2>
      <p>集成后 Claude 可以使用以下工具，详见 <a href="/docs/mcp/tools">工具列表</a>：</p>
      <ul>
        <li><code>ping</code> — 多节点 ICMP Ping</li>
        <li><code>http</code> — HTTP/HTTPS 连通性检测</li>
        <li><code>dns</code> — DNS 解析查询</li>
        <li><code>traceroute</code> — 路由追踪</li>
        <li><code>ssl</code> — SSL 证书检测</li>
        <li><code>ip</code> — IP 归属查询</li>
        <li><code>whois</code> — WHOIS 查询</li>
        <li><code>diagnose</code> — 一键全面诊断</li>
      </ul>

      <h2>常见问题</h2>
      <h3>提示&quot;找不到 mcp 命令&quot;</h3>
      <p>确认 <code>$GOPATH/bin</code> 在 PATH 中：</p>
      <pre><code>export PATH=&quot;$PATH:$(go env GOPATH)/bin&quot;</code></pre>

      <h3>API Key 无效</h3>
      <p>前往 <a href="/app/settings/api-keys">控制台</a> 检查 Key 是否过期，Personal Key 有效期为 24 小时。</p>
    </article>
  )
}
