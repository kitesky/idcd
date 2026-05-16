import type { Metadata } from "next"

export const metadata: Metadata = { title: "WHOIS 查询 | idcd 工具文档" }

export default function WhoisToolPage() {
  return (
    <article className="prose prose-zinc dark:prose-invert max-w-none">
      <h1>WHOIS 查询</h1>
      <p>查询域名注册信息，包括注册商、注册人、到期时间和 DNS 服务器。</p>

      <h2>功能说明</h2>
      <ul>
        <li>查询域名注册商和注册状态</li>
        <li>显示域名到期时间，预防域名被抢注</li>
        <li>列出权威 DNS 服务器（Nameservers）</li>
        <li>支持 .com/.net/.org/.io/.cn 等主流顶级域</li>
      </ul>

      <h2>API 参数</h2>
      <table>
        <thead><tr><th>参数</th><th>类型</th><th>必填</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>domain</code></td><td>string</td><td>是</td><td>查询的域名（支持带/不带 www）</td></tr>
        </tbody>
      </table>

      <h2>响应字段</h2>
      <table>
        <thead><tr><th>字段</th><th>说明</th></tr></thead>
        <tbody>
          <tr><td><code>registrar</code></td><td>注册商名称</td></tr>
          <tr><td><code>created_at</code></td><td>注册时间</td></tr>
          <tr><td><code>expires_at</code></td><td>到期时间</td></tr>
          <tr><td><code>updated_at</code></td><td>最后更新时间</td></tr>
          <tr><td><code>nameservers</code></td><td>权威 DNS 服务器列表</td></tr>
          <tr><td><code>status</code></td><td>域名状态（clientTransferProhibited 等）</td></tr>
          <tr><td><code>raw</code></td><td>原始 WHOIS 文本</td></tr>
        </tbody>
      </table>

      <h2>示例</h2>
      <pre><code>{`curl -X POST https://api.idcd.com/v1/tools/whois \\
  -H "Authorization: Bearer sk_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{"domain": "example.com"}'`}</code></pre>

      <h3>典型响应</h3>
      <pre><code>{`{
  "registrar": "GoDaddy.com, LLC",
  "created_at": "1995-08-14T00:00:00Z",
  "expires_at": "2027-08-13T04:00:00Z",
  "nameservers": ["ns1.example.com", "ns2.example.com"],
  "status": ["clientDeleteProhibited", "clientTransferProhibited"]
}`}</code></pre>

      <h2>在线工具</h2>
      <p>无需 API Key，直接在 <a href="/tools/whois">在线工具</a> 中试用。</p>
    </article>
  )
}
