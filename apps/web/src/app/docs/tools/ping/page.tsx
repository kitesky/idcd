import type { Metadata } from "next"

export const metadata: Metadata = { title: "多地 Ping | idcd 工具文档" }

export default function PingToolPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>多地 Ping</h1>
      <p>从全球多个节点同时发送 ICMP Ping，测量延迟和丢包率，快速判断网络质量。</p>

      <h2>功能说明</h2>
      <ul>
        <li>支持域名和 IP 地址</li>
        <li>自动从多个大洲节点同时检测</li>
        <li>返回平均延迟、最小/最大延迟、丢包率</li>
        <li>可自定义发包数量</li>
      </ul>

      <h2>API 参数</h2>
      <table>
        <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>target</code></td><td>string</td><td>是</td><td>目标域名或 IP</td></tr>
          <tr><td><code>nodes</code></td><td>string[]</td><td>否</td><td>指定节点，不填自动选择全球代表节点</td></tr>
          <tr><td><code>count</code></td><td>number</td><td>否</td><td>每个节点发包数，默认 5，最大 20</td></tr>
        </tbody>
      </table>

      <h2>响应字段</h2>
      <table>
        <thead><tr><th>字段</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>avg_ms</code></td><td>平均延迟（毫秒）</td></tr>
          <tr><td><code>min_ms</code></td><td>最小延迟</td></tr>
          <tr><td><code>max_ms</code></td><td>最大延迟</td></tr>
          <tr><td><code>loss_pct</code></td><td>丢包率（%）</td></tr>
          <tr><td><code>node</code></td><td>检测节点（城市/运营商）</td></tr>
        </tbody>
      </table>

      <h2>示例</h2>
      <pre><code>{`curl -X POST https://api.idcd.com/v1/tools/ping \\
  -H "Authorization: Bearer sk_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "target": "google.com",
    "nodes": ["tokyo", "singapore", "us-east", "eu-west"],
    "count": 5
  }'`}</code></pre>

      <h3>典型响应</h3>
      <pre><code>{`{
  "results": [
    { "node": "Tokyo JP",       "avg_ms": 28, "loss_pct": 0 },
    { "node": "Singapore SG",   "avg_ms": 45, "loss_pct": 0 },
    { "node": "Virginia US",    "avg_ms": 102, "loss_pct": 0 },
    { "node": "Frankfurt DE",   "avg_ms": 78, "loss_pct": 0 }
  ]
}`}</code></pre>

      <h2>在线工具</h2>
      <p>无需 API Key，直接在 <a href="/tools/ping">在线工具</a> 中试用。</p>
    </article>
  )
}
