import type { Metadata } from "next"

export const metadata: Metadata = { title: "MCP 认证配置 | idcd 文档" }

export default function McpAuthPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>认证配置</h1>

      <h2>API Key 类型</h2>
      <table>
        <thead><tr><th>类型</th><th>有效期</th><th>适用场景</th></tr></thead>
        <tbody>
          <tr><td>Personal</td><td>24 小时</td><td>个人开发调试</td></tr>
          <tr><td>Workspace</td><td>90 天</td><td>团队共享</td></tr>
          <tr><td>Service</td><td>90 天（可续期）</td><td>生产环境集成</td></tr>
        </tbody>
      </table>
      <p><strong>注意</strong>：所有 Token 最长 90 天，无永久 Token（D2 决策）。</p>

      <h2>创建 API Key</h2>
      <ol>
        <li>登录 <a href="/app/settings/api-keys">idcd 控制台</a></li>
        <li>进入「设置 → API Keys」</li>
        <li>点击「创建新 Key」，选择类型和有效期</li>
        <li>复制 Key（只显示一次）</li>
      </ol>

      <h2>使用方式</h2>
      <h3>环境变量（推荐）</h3>
      <pre><code>export IDCD_API_KEY=sk_live_xxxxxxxx</code></pre>

      <h3>命令行参数</h3>
      <pre><code>mcp --api-key sk_live_xxxxxxxx</code></pre>

      <h2>权限范围</h2>
      <p>MCP Token 仅可访问拨测工具 API，与主账号 API 配额独立（D2 决策）。</p>
    </article>
  )
}
