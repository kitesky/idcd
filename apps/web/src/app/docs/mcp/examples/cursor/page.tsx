import type { Metadata } from "next"

export const metadata: Metadata = { title: "Cursor IDE 集成 | idcd MCP 文档" }

export default function CursorExamplePage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>Cursor IDE 集成</h1>
      <p>在 Cursor IDE 中集成 idcd MCP Server，让 AI 助手直接调用全球网络拨测能力。</p>

      <h2>前置条件</h2>
      <ul>
        <li>已安装 <a href="https://cursor.sh" target="_blank" rel="noopener noreferrer">Cursor IDE</a> v0.40+</li>
        <li>已在 <a href="/app/settings/api-keys">idcd 控制台</a> 创建 API Key</li>
        <li>已安装 Go 1.21+</li>
      </ul>

      <h2>安装 MCP Server</h2>
      <pre><code>go install github.com/kite365/idcd/apps/mcp/cmd/mcp@latest</code></pre>
      <p>获取 mcp 二进制路径：</p>
      <pre><code>which mcp</code></pre>

      <h2>配置 Cursor</h2>
      <p>打开 Cursor 设置（<code>Cmd+,</code>），进入 <strong>Features → MCP Servers</strong>，添加以下配置：</p>
      <pre><code>{`{
  "mcpServers": {
    "idcd": {
      "command": "/Users/yourname/go/bin/mcp",
      "env": {
        "IDCD_API_KEY": "sk_live_your_key_here"
      }
    }
  }
}`}</code></pre>
      <p>将路径替换为 <code>which mcp</code> 的输出。</p>

      <h2>或通过配置文件</h2>
      <p>在项目根目录创建 <code>.cursor/mcp.json</code>（仅对当前项目生效）：</p>
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
      <p>配置完成后，在 Cursor Composer 或 Chat 中描述网络检测需求：</p>
      <pre><code>{`你: 我的 API 服务器 api.example.com 在东南亚有没有访问问题？
AI: [调用 http tool，选择新加坡和东京节点]
检测结果：
Singapore — 200 OK (156ms)
Tokyo     — 200 OK (89ms)
Bangkok   — 连接超时 ❌
结论：泰国节点访问异常，建议检查 CDN 配置。`}</code></pre>

      <h2>可用工具</h2>
      <p>详见 <a href="/docs/mcp/tools">工具列表</a>，支持 ping、http、dns、traceroute、ssl、ip、whois、diagnose。</p>

      <h2>常见问题</h2>
      <h3>Cursor 无法找到 MCP server</h3>
      <p>使用 mcp 二进制的绝对路径（不要用 <code>~/</code> 或相对路径）。</p>

      <h3>工具调用返回错误</h3>
      <p>在终端测试连通性：</p>
      <pre><code>IDCD_API_KEY=sk_live_xxx mcp ping --target google.com</code></pre>
    </article>
  )
}
