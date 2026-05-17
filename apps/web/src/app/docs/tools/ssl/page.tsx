import type { Metadata } from "next"
import Link from "next/link"

export const metadata: Metadata = { title: "SSL 证书检测 | idcd 工具文档" }

export default function SslToolPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>SSL 证书检测</h1>
      <p>检测 HTTPS 网站的 SSL 证书有效性、到期时间、颁发机构和配置安全性。</p>

      <h2>功能说明</h2>
      <ul>
        <li>检测证书有效期，提前告警到期风险</li>
        <li>验证证书链完整性</li>
        <li>检查证书绑定的域名（SAN/CN）</li>
        <li>显示颁发机构（CA）信息</li>
        <li>检测 TLS 协议版本和加密套件</li>
      </ul>

      <h2>API 参数</h2>
      <table>
        <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>domain</code></td><td>string</td><td>是</td><td>域名（不含 https://）</td></tr>
          <tr><td><code>port</code></td><td>number</td><td>否</td><td>端口号，默认 443</td></tr>
        </tbody>
      </table>

      <h2>响应字段</h2>
      <table>
        <thead><tr><th>字段</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>valid</code></td><td>证书是否有效</td></tr>
          <tr><td><code>expires_at</code></td><td>到期时间（ISO 8601）</td></tr>
          <tr><td><code>days_remaining</code></td><td>剩余天数</td></tr>
          <tr><td><code>issuer</code></td><td>颁发机构</td></tr>
          <tr><td><code>subject</code></td><td>证书主体</td></tr>
          <tr><td><code>san</code></td><td>Subject Alternative Names 列表</td></tr>
          <tr><td><code>tls_version</code></td><td>TLS 协议版本</td></tr>
        </tbody>
      </table>

      <h2>示例</h2>
      <pre><code>{`curl -X POST https://api.idcd.com/v1/tools/ssl \\
  -H "Authorization: Bearer sk_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"domain": "example.com"}'`}</code></pre>

      <h3>典型响应</h3>
      <pre><code>{`{
  "valid": true,
  "expires_at": "2026-12-31T23:59:59Z",
  "days_remaining": 230,
  "issuer": "Let's Encrypt",
  "tls_version": "TLS 1.3"
}`}</code></pre>

      <h2>配合监控使用</h2>
      <p>在 <Link href="/app/monitors/new">新建监控</Link> 时选择「SSL 证书」类型，可在证书到期前 30/15/7 天自动发送告警。</p>

      <h2>在线工具</h2>
      <p>无需 API Key，直接在 <Link href="/tools/ssl">在线工具</Link> 中试用。</p>
    </article>
  )
}
