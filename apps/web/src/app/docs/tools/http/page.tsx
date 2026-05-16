import type { Metadata } from "next"

export const metadata: Metadata = { title: "HTTP/HTTPS 拨测 | idcd 工具文档" }

export default function HttpToolPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>HTTP/HTTPS 拨测</h1>
      <p>从全球多个节点检测 HTTP/HTTPS 接口的连通性、响应时间和状态码。</p>

      <h2>功能说明</h2>
      <ul>
        <li>支持 GET、POST、HEAD 等请求方法</li>
        <li>检测响应状态码、响应时间、响应体大小</li>
        <li>支持自定义请求头和请求体</li>
        <li>支持 HTTPS 证书验证（可选跳过）</li>
        <li>支持重定向跟踪</li>
      </ul>

      <h2>API 参数</h2>
      <table>
        <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>url</code></td><td>string</td><td>是</td><td>目标 URL（含协议）</td></tr>
          <tr><td><code>method</code></td><td>string</td><td>否</td><td>请求方法，默认 GET</td></tr>
          <tr><td><code>nodes</code></td><td>string[]</td><td>否</td><td>指定检测节点列表，不填则自动选择</td></tr>
          <tr><td><code>headers</code></td><td>object</td><td>否</td><td>自定义请求头</td></tr>
          <tr><td><code>body</code></td><td>string</td><td>否</td><td>请求体（POST 时使用）</td></tr>
          <tr><td><code>timeout</code></td><td>number</td><td>否</td><td>超时秒数，默认 10</td></tr>
          <tr><td><code>follow_redirects</code></td><td>boolean</td><td>否</td><td>是否跟随重定向，默认 true</td></tr>
        </tbody>
      </table>

      <h2>响应字段</h2>
      <table>
        <thead><tr><th>字段</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>status_code</code></td><td>HTTP 状态码</td></tr>
          <tr><td><code>response_time_ms</code></td><td>响应时间（毫秒）</td></tr>
          <tr><td><code>response_size</code></td><td>响应体大小（字节）</td></tr>
          <tr><td><code>node</code></td><td>检测节点信息</td></tr>
          <tr><td><code>error</code></td><td>错误信息（连接失败时）</td></tr>
        </tbody>
      </table>

      <h2>示例</h2>
      <h3>基本检测</h3>
      <pre><code>{`curl -X POST https://api.idcd.com/v1/tools/http \\
  -H "Authorization: Bearer sk_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"url": "https://example.com"}'`}</code></pre>

      <h3>指定节点 + 自定义头</h3>
      <pre><code>{`{
  "url": "https://api.example.com/health",
  "nodes": ["tokyo", "singapore", "us-east"],
  "headers": {
    "Accept": "application/json"
  }
}`}</code></pre>

      <h2>在线工具</h2>
      <p>无需 API Key，直接在 <a href="/tools/http">在线工具</a> 中试用。</p>
    </article>
  )
}
