import type { Metadata } from "next"

export const metadata: Metadata = { title: "MCP 快速开始 | idcd 文档" }

export default function McpQuickstartPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>5 分钟接入 Claude Code</h1>

      <p><strong>步骤 1</strong>：获取 API Key</p>
      <p>前往 <a href="/app/settings/api-keys">idcd 控制台</a> 创建 API Key。</p>

      <p><strong>步骤 2</strong>：安装 MCP server</p>
      <pre><code>go install github.com/kite365/idcd/apps/mcp/cmd/mcp@latest</code></pre>

      <p><strong>步骤 3</strong>：添加到 Claude Code</p>
      <pre><code>claude mcp add idcd -- /path/to/mcp --api-key sk_live_xxx</code></pre>
      <p>或设置环境变量后：</p>
      <pre><code>{`export IDCD_API_KEY=sk_live_xxx
claude mcp add idcd -- /path/to/mcp`}</code></pre>

      <p><strong>步骤 4</strong>：在对话中使用</p>
      <pre><code>{`你: 检测一下 google.com 的全球 Ping 延迟
Claude: [调用 ping tool]
PING google.com
节点: Tokyo JP — 32ms ✓
节点: Singapore SG — 45ms ✓
平均: 38ms | 丢包: 0%`}</code></pre>

      <h2>Cursor IDE 集成</h2>
      <p>在 Cursor 设置中添加 MCP server 配置：</p>
      <pre><code>{`{
  "mcpServers": {
    "idcd": {
      "command": "/path/to/mcp",
      "args": ["--api-key", "sk_live_xxx"]
    }
  }
}`}</code></pre>

      <h2>下一步</h2>
      <ul>
        <li><a href="/docs/mcp/authentication">认证配置</a> — API Key 管理和权限控制</li>
        <li><a href="/docs/mcp/tools">工具列表</a> — 所有可用工具详细说明</li>
      </ul>
    </article>
  )
}
